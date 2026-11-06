package aggregate

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// WarEditMark is an edit a battle's row shows, which can be undone from it
type WarEditMark struct {
	ID   int
	Kind string
}

/*
	warFoughtBeside works out the side each third party group in a battle fought beside: the opposite of whichever side it shot at or was shot by on more killmails.
	A group that fought both evenly, or neither, is left out. So is one where that side has no pilot on a killmail in any system the group fought in: having the same enemy elsewhere is not fighting beside them, as a battle can link systems by the enemy's pilots alone.

Parameters:
  - battle: a battle as FindBattlesOf found it, before any edit

Returns:
  - map[warGroupKey]WarSideKey: each group's side
*/
func warFoughtBeside(battle *warBattleOf) map[warGroupKey]WarSideKey {
	against := map[warGroupKey]*[3]int{}
	sidePilotsIn := map[WarSideKey]map[int]bool{WarSideA: {}, WarSideB: {}}
	foughtIn := map[warGroupKey]map[int]bool{}
	for _, kill := range battle.kills {
		system := kill.raw.ESIKillmail.SolarSystemID
		for _, party := range kill.parties {
			if party.side != WarSideNone && party.char != 0 {
				sidePilotsIn[party.side][system] = true
			}
			if party.side == WarSideNone && party.hasGroup {
				if foughtIn[party.group] == nil {
					foughtIn[party.group] = map[int]bool{}
				}
				foughtIn[party.group][system] = true
			}
		}
		counted := map[[2]int]bool{}
		add := func(group warGroupKey, x WarSideKey) {
			mark := [2]int{int(group.Type)*1_000_000_000 + group.ID, int(x)}
			if counted[mark] {
				return
			}
			counted[mark] = true
			if against[group] == nil {
				against[group] = &[3]int{}
			}
			against[group][x]++
		}
		victim := kill.parties[0]
		for _, attacker := range kill.parties[1:] {
			if attacker.hasGroup && attacker.side == WarSideNone && victim.side != WarSideNone {
				add(attacker.group, victim.side)
			}
			if victim.hasGroup && victim.side == WarSideNone && attacker.side != WarSideNone {
				add(victim.group, attacker.side)
			}
		}
	}
	beside := map[warGroupKey]WarSideKey{}
	for group, tally := range against {
		side := WarSideNone
		switch {
		case tally[WarSideA] > tally[WarSideB]:
			side = WarSideB
		case tally[WarSideB] > tally[WarSideA]:
			side = WarSideA
		}
		if side == WarSideNone {
			continue
		}
		for system := range foughtIn[group] {
			if sidePilotsIn[side][system] {
				beside[group] = side
				break
			}
		}
	}
	return beside
}

// warHolding is the battle holding the most of these killmails, the earliest on a tie, nil if none holds any
func warHolding(battles []*warBattleOf, ids []int) *warBattleOf {
	wanted := map[int]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var best *warBattleOf
	bestCount := 0
	for _, battle := range battles {
		count := 0
		for _, kill := range battle.kills {
			if wanted[kill.id] {
				count++
			}
		}
		if count == 0 {
			continue
		}
		if count > bestCount || count == bestCount && warEarlier(battle.kills[0], best.kills[0]) {
			best, bestCount = battle, count
		}
	}
	return best
}

func warEarlier(a, b *warKill) bool {
	return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id)) < 0
}

func warSortKills(kills []*warKill) {
	slices.SortStableFunc(kills, func(a, b *warKill) int { return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id)) })
}

// warMerge makes the battles holding each group of killmails one battle. Fewer than two found changes nothing, as its battles have gone or are one already.
func warMerge(battles []*warBattleOf, edit retrieval.WarBattleEdit) []*warBattleOf {
	found := []*warBattleOf{}
	for _, ids := range edit.Groups {
		if battle := warHolding(battles, ids); battle != nil && !slices.Contains(found, battle) {
			found = append(found, battle)
		}
	}
	if len(found) < 2 {
		return battles
	}
	kept := found[0]
	kept.edited = true
	kept.marks = append(kept.marks, WarEditMark{ID: edit.ID, Kind: edit.Kind})
	for _, battle := range found[1:] {
		kept.kills = append(kept.kills, battle.kills...)
		for _, mark := range battle.marks {
			// two pieces of a split share its mark
			if !slices.Contains(kept.marks, mark) {
				kept.marks = append(kept.marks, mark)
			}
		}
		battles = slices.DeleteFunc(battles, func(candidate *warBattleOf) bool { return candidate == battle })
	}
	warSortKills(kept.kills)
	return battles
}

// warNearest is the killmail nearest in time to another in its system, else the nearest anywhere, the first of them on a tie
func warNearest(kill *warKill, among []*warKill) *warKill {
	var best *warKill
	var bestGap time.Duration
	bestHere := false
	for _, other := range among {
		here := other.raw.ESIKillmail.SolarSystemID == kill.raw.ESIKillmail.SolarSystemID
		gap := kill.at.Sub(other.at).Abs()
		if best == nil || here && !bestHere || here == bestHere && gap < bestGap {
			best, bestGap, bestHere = other, gap, here
		}
	}
	return best
}

// warSplit moves killmails out of a battle into a new one. A killmail new since the edit goes where the killmail nearest it went. It changes nothing unless both pieces would have one of the killmails it was saved with.
func warSplit(battles []*warBattleOf, edit retrieval.WarBattleEdit) []*warBattleOf {
	if len(edit.Groups) != 2 {
		return battles
	}
	battle := warHolding(battles, slices.Concat(edit.Groups[0], edit.Groups[1]))
	if battle == nil {
		return battles
	}
	listedStays, listedMoved := map[int]bool{}, map[int]bool{}
	for _, id := range edit.Groups[0] {
		listedStays[id] = true
	}
	for _, id := range edit.Groups[1] {
		listedMoved[id] = true
	}
	stays, moved, listed, unlisted := []*warKill{}, []*warKill{}, []*warKill{}, []*warKill{}
	for _, kill := range battle.kills {
		switch {
		case listedMoved[kill.id]:
			moved = append(moved, kill)
			listed = append(listed, kill)
		case listedStays[kill.id]:
			stays = append(stays, kill)
			listed = append(listed, kill)
		default:
			unlisted = append(unlisted, kill)
		}
	}
	if len(stays) == 0 || len(moved) == 0 {
		return battles
	}
	for _, kill := range unlisted {
		if listedMoved[warNearest(kill, listed).id] {
			moved = append(moved, kill)
		} else {
			stays = append(stays, kill)
		}
	}
	warSortKills(stays)
	warSortKills(moved)

	mark := WarEditMark{ID: edit.ID, Kind: edit.Kind}
	battle.kills, battle.edited = stays, true
	piece := &warBattleOf{kills: moved, edited: true, marks: slices.Clone(battle.marks)}
	battle.marks = append(battle.marks, mark)
	piece.marks = append(piece.marks, mark)
	return append(battles, piece)
}

/*
	warEditedBattles is a war's battles as its report shows them: those that count, with the user's edits applied. Edits come after trash is decided and after each third party's side is, so they only regroup rows and move no total.

Parameters:
  - trash: what findWarTrash made of the war's killmails
  - edits: the user's edits, oldest first. One that finds nothing changes nothing.

Returns:
  - []*warBattleOf: the battles, each with its id
*/
func warEditedBattles(trash *warTrash, edits []retrieval.WarBattleEdit) []*warBattleOf {
	battles := make([]*warBattleOf, 0, len(trash.battles))
	for _, battle := range trash.battles {
		beside := warFoughtBeside(battle)
		for _, kill := range battle.kills {
			kill.beside = beside
		}
		if trash.counts(battle) {
			battles = append(battles, battle)
		}
	}
	for _, edit := range edits {
		switch edit.Kind {
		case retrieval.WarEditMerge:
			battles = warMerge(battles, edit)
		case retrieval.WarEditSplit:
			battles = warSplit(battles, edit)
		case retrieval.WarEditIgnore:
			if battle := warHolding(battles, slices.Concat(edit.Groups...)); battle != nil {
				battle.ignored = true
				saved := map[int]bool{}
				for _, id := range slices.Concat(edit.Groups...) {
					saved[id] = true
				}
				for _, kill := range battle.kills {
					if !saved[kill.id] {
						battle.ignoredJoined++
					}
				}
				battle.marks = append(battle.marks, WarEditMark{ID: edit.ID, Kind: edit.Kind})
			}
		}
	}

	// an unedited battle keeps its id, then an edited one takes its first killmail's, which two pieces can share
	taken := map[string]bool{}
	for _, battle := range battles {
		if !battle.edited {
			battle.id = battle.fought.BattleIDReference
			taken[battle.id] = true
		}
	}
	slices.SortStableFunc(battles, func(a, b *warBattleOf) int {
		return cmp.Or(a.kills[0].at.Compare(b.kills[0].at), cmp.Compare(a.kills[0].id, b.kills[0].id))
	})
	for _, battle := range battles {
		if battle.edited {
			first := battle.kills[0]
			battle.id = fmt.Sprintf("%d/%s", first.raw.ESIKillmail.SolarSystemID, first.at.Format("200601021504"))
			if taken[battle.id] {
				battle.id = fmt.Sprintf("%s-%d", battle.id, first.id)
			}
			taken[battle.id] = true
		}
	}
	return battles
}

/*
	warIgnoring finds a war's battles and trash with the battles the user ignored set aside: those killmails and whatever trash or asset denial went with a station among them.
	Trash and battles are found again on what is left, so a station in an ignored battle stops confirming a mark or ending asset denial elsewhere.

Parameters:
  - kills: the war's killmails
  - edits: the user's edits, oldest first

Returns:
  - *warTrash: the trash of what is left
  - []*warBattleOf: the battles, the ignored among them
  - map[int]warTrashMark: the marks of the trash that went with an ignored battle, by killmail id
*/
func warIgnoring(kills []*warKill, edits []retrieval.WarBattleEdit) (*warTrash, []*warBattleOf, map[int]warTrashMark) {
	trash := findWarTrash(kills)
	battles := warEditedBattles(trash, edits)
	ignored := slices.DeleteFunc(slices.Clone(battles), func(battle *warBattleOf) bool { return !battle.ignored })
	if len(ignored) == 0 {
		return trash, battles, nil
	}

	gone := map[int]bool{}
	for _, battle := range ignored {
		for _, kill := range battle.kills {
			gone[kill.id] = true
		}
	}
	stations := map[int]bool{}
	marks := map[int]warTrashMark{}
	for id, mark := range trash.marks {
		if gone[mark.anchor] {
			marks[id] = mark
			stations[mark.anchor] = true
		}
	}
	for id := range marks {
		gone[id] = true
	}

	rest := slices.DeleteFunc(slices.Clone(kills), func(kill *warKill) bool { return gone[kill.id] })
	trash = findWarTrash(rest)
	// their ignore edits have done their work, and must not catch a battle of what is left
	battles = warEditedBattles(trash, slices.DeleteFunc(slices.Clone(edits), func(edit retrieval.WarBattleEdit) bool { return edit.Kind == retrieval.WarEditIgnore }))
	taken := map[string]bool{}
	for _, battle := range battles {
		taken[battle.id] = true
	}
	for _, battle := range ignored {
		if taken[battle.id] {
			battle.id = fmt.Sprintf("%s-%d", battle.id, battle.kills[0].id)
		}
		taken[battle.id] = true
	}
	for id := range stations {
		trash.anchors[id] = true // so its trash finds the row it died on
	}
	return trash, append(battles, ignored...), marks
}

// warAttachNotes puts each note on the battle holding most of its killmails. One whose battle has gone stays saved and shows nowhere.
func warAttachNotes(battles []*warBattleOf, notes []retrieval.WarBattleNote) {
	for _, note := range notes {
		if battle := warHolding(battles, note.Killmails); battle != nil {
			battle.notes = append(battle.notes, note)
		}
	}
}
