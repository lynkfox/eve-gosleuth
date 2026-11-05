package gui

import (
	"errors"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

var errNoWarBattle = errors.New("That battle is no longer in the war. Open the war again to see it as it is now.")

// builtWar is the figures of the war's report last built, so an edit can read which killmails a row holds
type builtWar struct {
	id   int
	data *aggregate.WarData
}

// WarEditMark is an edit a battle's row shows, which can be undone from it
type WarEditMark struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"` // merge or split
}

// WarBattleKillmail is a battle's killmail in the Split window
type WarBattleKillmail struct {
	ID     int       `json:"id"`
	Time   time.Time `json:"time"`
	System string    `json:"system"`
	Ship   string    `json:"ship"`
	Victim string    `json:"victim"` // the pilot, or the group for a structure
	Side   string    `json:"side"`   // the side its loss counts for: a, b, or others
	ISK    float64   `json:"isk"`
}

// warBuilt is the figures of a war's report, building them if that report is not the last built. The caller must hold a.mu.
func (a *App) warBuilt(war retrieval.War, sides aggregate.WarSides) *aggregate.WarData {
	if a.lastWar == nil || a.lastWar.id != war.ID {
		a.buildWarReport(war, sides)
	}
	return a.lastWar.data
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
	changeWarEdits applies a change to a war's edits and rebuilds its report from what it holds. The change reads the report it was asked from, and nothing is saved if it refuses.

Parameters:
  - id: the war
  - change: changes the edits, given the report's figures as last built

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
	data := a.warBuilt(war, sides)

	a.warMu.Lock()
	edits := a.cache.WarEditsOf(id)
	if err := change(&edits, data); err != nil {
		a.warMu.Unlock()
		return report, err
	}
	a.cache.SaveWarEdits(id, edits)
	a.warMu.Unlock()
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
	data := a.warBuilt(war, sides)
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
			row.System = km.System.Name
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
