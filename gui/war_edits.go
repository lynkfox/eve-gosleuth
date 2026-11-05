package gui

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

var errNoWarBattle = errors.New("That battle is no longer in the war. Open the war again to see it as it is now.")

// WarEditMark is an edit a battle's row shows, which can be undone from it
type WarEditMark struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"` // merge, split, or ignore
}

// WarBattleKillmail is a battle's killmail in the Split window
type WarBattleKillmail struct {
	ID       int       `json:"id"`
	Time     time.Time `json:"time"`
	System   string    `json:"system"`
	SystemID int       `json:"systemId"`
	Ship     string    `json:"ship"`
	Victim   string    `json:"victim"` // the pilot, or the group for a structure
	Side     string    `json:"side"`   // the side its loss counts for: a, b, or others
	ISK      float64   `json:"isk"`
}

// WarNote is a note on a battle's row
type WarNote struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// warBattleOf finds a row of the report by its id, nil if it has none
func warBattleOf(data *aggregate.WarData, battleID string) *aggregate.WarBattle {
	index := slices.IndexFunc(data.Battles, func(battle *aggregate.WarBattle) bool { return battle.ID == battleID })
	if index < 0 {
		return nil
	}
	return data.Battles[index]
}

/*
	changeWarEdits applies a change to a war's edits and rebuilds its report from what it holds. The change reads the figures as they are now, so a row from an older report is refused, and nothing is saved if it refuses.

Parameters:
  - id: the war
  - change: changes the edits, given the report's figures

Returns:
  - WarReport: the report as it is now
  - error: errNoWar, a side no longer existing named, what the change refused, or a user readable message
*/
func (a *App) changeWarEdits(id int, change func(*retrieval.WarEdits, *aggregate.WarData) error) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, found := a.cache.WarByID(id)
	if !found {
		return report, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return report, err
	}
	data, _ := a.buildWarData(war, sides)

	if err := func() error {
		a.warMu.Lock()
		defer a.warMu.Unlock()
		// RemoveWar takes only warMu, so it may have gone since the check above
		if _, found := a.cache.WarByID(id); !found {
			return errNoWar
		}
		edits := a.cache.WarEditsOf(id)
		if err := change(&edits, data); err != nil {
			return err
		}
		a.cache.SaveWarEdits(id, edits)
		return nil
	}(); err != nil {
		return report, err
	}
	a.saveCacheQuietly()

	a.lastReport, a.lastTarget = nil, nil
	return a.buildWarReport(war, sides), nil
}

/*
	MergeWarBattles makes battles of a war's report one battle, saved with their killmails so it holds through an update. Only rows move: no ISK or ship total changes.

Parameters:
  - id: the war
  - battleIDs: the rows, at least two

Returns:
  - WarReport: the report as it is now
  - error: errNoWarBattle if a row has gone, or a user readable message
*/
func (a *App) MergeWarBattles(id int, battleIDs []string) (WarReport, error) {
	return a.changeWarEdits(id, func(edits *retrieval.WarEdits, data *aggregate.WarData) error {
		groups := [][]int{}
		for _, battleID := range battleIDs {
			battle := warBattleOf(data, battleID)
			if battle == nil {
				return errNoWarBattle
			}
			groups = append(groups, slices.Clone(battle.KillmailIDs))
		}
		if len(groups) < 2 {
			return errors.New("Pick a battle to merge it with")
		}
		edits.AddEdit(retrieval.WarEditMerge, groups)
		return nil
	})
}

/*
	SplitWarBattle moves killmails out of a battle of a war's report into a new one, saved with their ids so it holds through an update. Only rows move: no ISK or ship total changes.

Parameters:
  - id: the war
  - battleID: the row
  - moved: the killmails to move, some of those it holds but not all

Returns:
  - WarReport: the report as it is now
  - error: errNoWarBattle if the row has gone, or a user readable message
*/
func (a *App) SplitWarBattle(id int, battleID string, moved []int) (WarReport, error) {
	return a.changeWarEdits(id, func(edits *retrieval.WarEdits, data *aggregate.WarData) error {
		battle := warBattleOf(data, battleID)
		if battle == nil {
			return errNoWarBattle
		}
		leaving, stays := []int{}, []int{}
		for _, killmail := range battle.KillmailIDs {
			if slices.Contains(moved, killmail) {
				leaving = append(leaving, killmail)
			} else {
				stays = append(stays, killmail)
			}
		}
		if len(leaving) == 0 || len(stays) == 0 {
			return errors.New("Tick some of the battle's killmails to move, but not all of them")
		}
		edits.AddEdit(retrieval.WarEditSplit, [][]int{stays, leaving})
		return nil
	})
}

/*
	IgnoreWarBattle sets a battle of a war's report aside: it stays on the lists, and leaves every total with whatever trash and asset denial went with it. UndoWarEdit takes it back.

Parameters:
  - id: the war
  - battleID: the row

Returns:
  - WarReport: the report as it is now
  - error: errNoWarBattle if the row has gone, or a user readable message
*/
func (a *App) IgnoreWarBattle(id int, battleID string) (WarReport, error) {
	return a.changeWarEdits(id, func(edits *retrieval.WarEdits, data *aggregate.WarData) error {
		battle := warBattleOf(data, battleID)
		if battle == nil {
			return errNoWarBattle
		}
		edits.AddEdit(retrieval.WarEditIgnore, [][]int{slices.Clone(battle.KillmailIDs)})
		return nil
	})
}

/*
	UndoWarEdit takes back an edit of a war's battles.

Parameters:
  - id: the war
  - editID: the edit, from a row's WarEditMark

Returns:
  - WarReport: the report as it is now
  - error: a user readable message
*/
func (a *App) UndoWarEdit(id int, editID int) (WarReport, error) {
	return a.changeWarEdits(id, func(edits *retrieval.WarEdits, _ *aggregate.WarData) error {
		edits.RemoveEdit(editID)
		return nil
	})
}

/*
	GetWarBattleKillmails lists a battle's killmails for the Split window, oldest first, from what the war holds. Its station trash is not among them, as that follows its station.

Parameters:
  - id: the war
  - battleID: the row

Returns:
  - []WarBattleKillmail: its killmails, never nil
  - error: errNoWarBattle if the row has gone, or a user readable message
*/
func (a *App) GetWarBattleKillmails(id int, battleID string) (out []WarBattleKillmail, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	war, found := a.cache.WarByID(id)
	if !found {
		return nil, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return nil, err
	}
	data, _ := a.buildWarData(war, sides)
	battle := warBattleOf(data, battleID)
	if battle == nil {
		return nil, errNoWarBattle
	}
	sideOf := map[int]aggregate.WarSideKey{}
	for _, km := range data.Killmails {
		sideOf[km.Killmail.KillmailID] = km.VictimSide
	}
	out = make([]WarBattleKillmail, 0, len(battle.Killmails))
	for _, km := range battle.Killmails {
		row := WarBattleKillmail{ID: km.KillmailID, Time: km.KillmailTime, Side: warSideName(sideOf[km.KillmailID]), ISK: km.TotalValue}
		if km.System != nil {
			row.System, row.SystemID = km.System.Name, km.System.SystemID
		}
		if victim := km.Victim; victim != nil {
			if victim.Ship != nil {
				row.Ship = victim.Ship.Name
			}
			switch {
			case victim.Pilot != nil:
				row.Victim = victim.Pilot.Name
			case victim.AllianceAtKill != nil:
				row.Victim = victim.AllianceAtKill.Name
			case victim.CorpAtKill != nil:
				row.Victim = victim.CorpAtKill.Name
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// warNoteLimit and systemNicknameLimit are the longest text kept, as the inputs ask, in case of a paste around them
const (
	warNoteLimit        = 1000
	systemNicknameLimit = 60
)

// limitText trims text and cuts it to a length in characters
func limitText(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return strings.TrimSpace(string(runes))
}

/*
	SaveWarBattleNote adds, changes, or removes a note on a battle of a war's report. It follows the battle's killmails through updates and edits, and changes no figure.

Parameters:
  - id: the war
  - battleID: the row, for a new note
  - noteID: the note to change, 0 for a new one
  - text: its text, empty to remove it

Returns:
  - WarReport: the report as it is now
  - error: errNoWarBattle if the row has gone, or a user readable message
*/
func (a *App) SaveWarBattleNote(id int, battleID string, noteID int, text string) (WarReport, error) {
	text = limitText(text, warNoteLimit)
	return a.changeWarEdits(id, func(edits *retrieval.WarEdits, data *aggregate.WarData) error {
		if noteID != 0 {
			edits.SetNote(noteID, text)
			return nil
		}
		if text == "" {
			return nil
		}
		battle := warBattleOf(data, battleID)
		if battle == nil {
			return errNoWarBattle
		}
		edits.AddNote(slices.Clone(battle.KillmailIDs), text)
		return nil
	})
}

/*
	GetSystemNicknames returns the user's names for systems shown everywhere. A war's own come with its report.

Returns:
  - map[int]string: by system id, never nil
  - error: a user readable message
*/
func (a *App) GetSystemNicknames() (names map[int]string, err error) {
	defer recoverAsError(&err)
	a.waitReady()
	return a.cache.SystemNicknamesOf(), nil
}

/*
	SetSystemNickname names a system, everywhere or in one war only, as the user chooses when naming it. Named everywhere, the war's own name for it goes, so the new one shows.

Parameters:
  - systemID: the system
  - nickname: its name, empty to remove it from that scope
  - warID: the war open, 0 for none
  - everywhere: for the whole app, else only in that war

Returns:
  - error: errNoWar if there is no such war, or a user readable message
*/
func (a *App) SetSystemNickname(systemID int, nickname string, warID int, everywhere bool) (err error) {
	defer recoverAsError(&err)
	a.waitReady()
	a.warMu.Lock()
	defer a.warMu.Unlock()

	if warID == 0 && !everywhere {
		return errNoWar
	}
	if warID != 0 {
		if _, found := a.cache.WarByID(warID); !found {
			return errNoWar
		}
	}
	nickname = limitText(nickname, systemNicknameLimit)
	if everywhere {
		a.cache.SetSystemNickname(systemID, nickname)
	}
	if warID != 0 {
		edits := a.cache.WarEditsOf(warID)
		if everywhere || nickname == "" {
			delete(edits.SystemNames, systemID)
		} else {
			if edits.SystemNames == nil {
				edits.SystemNames = map[int]string{}
			}
			edits.SystemNames[systemID] = nickname
		}
		a.cache.SaveWarEdits(warID, edits)
	}
	a.saveCacheQuietly()
	return nil
}
