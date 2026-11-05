package aggregate

import (
	"cmp"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
)

const (
	warTrashWindow = 4 * time.Hour      // how long after the battle a station dies in ends that later killmails in its system are trash
	warRepeatGap   = 2 * time.Minute    // a pilot losing two ships closer than this after a station died is a repeat victim: too short to reship, session timers being what they are
	warRepeatReach = 7 * 24 * time.Hour // how long after a station died a repeat victim is still looked for
	warPodGap      = 5 * time.Minute    // a pilot's capsule dying this soon after their ship says they were in it: trash is empty ships
	warDaysMedium  = 3 * 24 * time.Hour // from a ref or asset denial to a medium structure's death
	warDaysLarge   = 4 * 24 * time.Hour // the same for any other
)

// warTrashMark says a killmail is station trash or asset denial, which are one bucket
type warTrashMark struct {
	owner  WarSideKey // the side whose station it goes with
	anchor int        // the killmail of the station it goes with, 0 for none: a station on a same-side kill that did not die soon after
	// definite is the stronger grade: a repeat victim's empty ships, an anchorable, a capital with its own station at top damage, anything with only its own station on it, or a capital with nothing fitted before its station's loss. The rest are probable.
	definite bool
}

// warTrash is how a war's killmails sort before its report's battles are found
type warTrash struct {
	marks    map[int]warTrashMark
	anchors  map[int]bool   // the stations trash was looked for after: their battles count, whatever is left in them
	stations []*warKill     // every upwell structure and control tower lost, oldest first
	battles  []*warBattleOf // the battles of what is not trash
}

func warVictimType(kill *warKill) int {
	return kill.raw.ESIKillmail.Victim.ShipTypeID
}

// warIsStation is whether a type is a structure pilots keep ships at: not a customs office or moon drill
func warIsStation(typeID int) bool {
	return comm.UpwellStructureGroups.ContainsGroupOf(typeID) || comm.ControlTowerGroups.ContainsGroupOf(typeID)
}

// warIsFixture is whether a type is a structure, a station's fighter, or one of NotTrashGroups, which is never trash or asset denial however it died
func warIsFixture(typeID int) bool {
	return warIsStation(typeID) || comm.PocoGroups.ContainsGroupOf(typeID) || comm.MinorStructureGroups.ContainsGroupOf(typeID) || comm.StandupFighterGroups.ContainsGroupOf(typeID) ||
		comm.NotTrashGroups.ContainsGroupOf(typeID)
}

// warIsStationPart is whether an attacker's ship is a station, one of its batteries, or its fighters
func warIsStationPart(ship int) bool {
	return warIsStation(ship) || comm.PosBatteryGroups.ContainsGroupOf(ship) || comm.StandupFighterGroups.ContainsGroupOf(ship)
}

// warStationFiring is whether an attacker's ship says a station was under attack: its Standup Fighters, or a station or battery of an owner that lost no such station in the battle, as died says
func warStationFiring(ship int, died map[int]bool) bool {
	switch {
	case comm.StandupFighterGroups.ContainsGroupOf(ship):
		return true
	case warIsStation(ship):
		return !died[ship]
	case comm.PosBatteryGroups.ContainsGroupOf(ship):
		return len(died) == 0
	}
	return false
}

// warOwnStationOnKill is whether an attacker on a killmail is a station, one of its batteries, or its fighters on the victim's side, whoever else shot: a station does not shoot its own side's capital by accident
func warOwnStationOnKill(kill *warKill) bool {
	side := kill.parties[0].side
	for index, attacker := range kill.raw.ESIKillmail.Attackers {
		if kill.parties[index+1].side == side && warIsStationPart(attacker.ShipTypeID) {
			return true
		}
	}
	return false
}

// warIsCapital is whether a type is a capital ship
func warIsCapital(typeID int) bool {
	return comm.CapitalTypes.ContainsIDNumber(typeID) || comm.CapitalGroups.ContainsGroupOf(typeID)
}

// warUnfitted is whether a killmail's victim had nothing in a high, mid, or low slot. Rigs and every hold can hold anything.
func warUnfitted(kill *warKill) bool {
	return !slices.ContainsFunc(kill.raw.ESIKillmail.Victim.Items, func(item api.ESIKillmailItem) bool {
		slot, _ := comm.FitSlotOf(item.Flag)
		return slot == comm.SlotHigh || slot == comm.SlotMid || slot == comm.SlotLow
	})
}

// warOwnStationTopDamage is whether a station of the victim's side, or its battery or fighters, did the most damage on a killmail, a tie counting
func warOwnStationTopDamage(kill *warKill) bool {
	top := 0
	for _, attacker := range kill.raw.ESIKillmail.Attackers {
		top = max(top, attacker.DamageDone)
	}
	side := kill.parties[0].side
	for index, attacker := range kill.raw.ESIKillmail.Attackers {
		if attacker.DamageDone == top && kill.parties[index+1].side == side && warIsStationPart(attacker.ShipTypeID) {
			return true
		}
	}
	return false
}

// warOwnStationOnly is whether nothing but the victim's own side's stations, batteries, or fighters are on a killmail
func warOwnStationOnly(kill *warKill) bool {
	return kill.sameSide && !slices.ContainsFunc(kill.raw.ESIKillmail.Attackers, func(attacker api.ESIAttacker) bool { return !warIsStationPart(attacker.ShipTypeID) })
}

// warOwnPodOnly is a capsule killed by nothing but its own side's stations, ie a pilot fetching event implants at home: it has nothing to do with a war
func warOwnPodOnly(kill *warKill) bool {
	return comm.CapsuleGroups.ContainsGroupOf(warVictimType(kill)) && warOwnStationOnly(kill)
}

// warIsAnchorable is whether a type is a deployable that always counts as trash, ie a mobile warp disruptor or depot
func warIsAnchorable(typeID int) bool {
	fit := comm.FitTypeOf(typeID)
	return fit != nil && comm.AnchorableCategories.ContainsIDNumber(fit.CategoryID) && !comm.NotTrashGroups.ContainsGroupOf(typeID)
}

// warSidesShot is whether a killmail's victim was shot by its own side alone, or by the other side at all, rather than only by those on no side
func warSidesShot(kill *warKill) bool {
	if kill.sameSide {
		return true
	}
	other := warOtherSide(kill.parties[0].side)
	return slices.ContainsFunc(kill.parties[1:], func(attacker warParty) bool { return attacker.side == other })
}

// warEmptyShips leaves out of a pilot's losses, oldest first, each run of them closer than warPodGap with their capsule dying during it or within warPodGap after: they were in those. pods are that pilot's capsule losses there.
func warEmptyShips(losses []*warKill, pods []time.Time) []*warKill {
	empty := []*warKill{}
	for start := 0; start < len(losses); {
		end := start
		for end+1 < len(losses) && losses[end+1].at.Sub(losses[end].at) < warPodGap {
			end++
		}
		from, until := losses[start].at, losses[end].at.Add(warPodGap)
		if !slices.ContainsFunc(pods, func(pod time.Time) bool { return !pod.Before(from) && !pod.After(until) }) {
			empty = append(empty, losses[start:end+1]...)
		}
		start = end + 1
	}
	return empty
}

// warTimerDays is how long before a station's death a ref or asset denial goes with it
func warTimerDays(typeID int) time.Duration {
	if comm.MediumStructureTypes.ContainsIDNumber(typeID) {
		return warDaysMedium
	}
	return warDaysLarge
}

// warOtherSide is the other of sides A and B
func warOtherSide(side WarSideKey) WarSideKey {
	if side == WarSideA {
		return WarSideB
	}
	return WarSideA
}

// shotAt is whether a side killed the other on any of a battle's killmails
func (b *warBattleOf) shotAt(by WarSideKey) bool {
	other := warOtherSide(by)
	for _, kill := range b.kills {
		if kill.parties[0].side != other {
			continue
		}
		for _, attacker := range kill.parties[1:] {
			if attacker.side == by {
				return true
			}
		}
	}
	return false
}

// counts is whether a battle is in the report: one side killed the other, or a station that has trash died in it
func (t *warTrash) counts(battle *warBattleOf) bool {
	return battle.counted() || slices.ContainsFunc(battle.kills, func(kill *warKill) bool { return t.anchors[kill.id] })
}

/*
	findWarTrash sorts out station trash and asset denial, see .claude/docs/war-plan.md's Stage 3.

Parameters:
  - kills: a war's killmails, each once, in any order

Returns:
  - *warTrash: its marks by killmail id, and the battles found without them
*/
func findWarTrash(kills []*warKill) *warTrash {
	t := &warTrash{marks: map[int]warTrashMark{}, anchors: map[int]bool{}}

	ordered := slices.Clone(kills)
	slices.SortStableFunc(ordered, func(a, b *warKill) int { return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id)) })
	bySystem := map[int][]*warKill{}
	for _, kill := range ordered {
		system := kill.raw.ESIKillmail.SolarSystemID
		bySystem[system] = append(bySystem[system], kill)
		if warIsStation(warVictimType(kill)) {
			t.stations = append(t.stations, kill)
		}
	}

	// the stations of a side that died in a battle that counts
	// the station a side loses in a killmail's system soon after it, if any
	stationSoon := func(kill *warKill, side WarSideKey) *warKill {
		next := warStationAfter(t.stations, kill, side)
		if next != nil && !kill.at.Before(next.at.Add(-warTimerDays(warVictimType(next)))) {
			return next
		}
		return nil
	}
	// denialOpen is whether a killmail can still be asset denial: not once the side has lost a station in that system, unless it loses another soon after (user, 2026-11-05)
	denialOpen := func(kill *warKill, side WarSideKey) bool {
		return stationSoon(kill, side) != nil || !warStationBefore(t.stations, kill, side)
	}
	// markDenial marks asset denial, of the stronger grade if sure, or if ifLost and a station is lost after it, as without one it may never die (user, 2026-11-05)
	markDenial := func(kill *warKill, side WarSideKey, sure bool, ifLost bool) {
		mark := warTrashMark{owner: side, definite: sure}
		if next := stationSoon(kill, side); next != nil {
			mark.anchor, mark.definite = next.id, sure || ifLost
		}
		t.marks[kill.id] = mark
	}
	notTrash := func() []*warKill {
		return slices.DeleteFunc(slices.Clone(ordered), func(kill *warKill) bool { return t.marks[kill.id] != (warTrashMark{}) })
	}

	// what is never a fight, so it stays out of the battles
	for _, kill := range ordered {
		side, typeID := kill.parties[0].side, warVictimType(kill)
		switch {
		case side == WarSideNone:
		// asset denial: a side's capital with nothing fitted, or with its own station on the kill, whoever else shot it
		case warIsCapital(typeID) && (warUnfitted(kill) || warOwnStationOnKill(kill)) && denialOpen(kill, side):
			markDenial(kill, side, warOwnStationTopDamage(kill), warUnfitted(kill))
		// asset denial: anything else with nothing but its own station on the kill. A capsule is never it, and warOwnPodOnly leaves those out of the war.
		case warOwnStationOnly(kill) && !warIsFixture(typeID) && !comm.CapsuleGroups.ContainsGroupOf(typeID) && denialOpen(kill, side):
			markDenial(kill, side, true, false)
		// trash: an anchorable the war's sides shot, wherever and whenever. It goes with a station below, if one died before it.
		case warIsAnchorable(typeID) && warSidesShot(kill):
			t.marks[kill.id] = warTrashMark{owner: side, definite: true}
		}
	}

	battleOf := func(kills []*warKill) map[int]*warBattleOf {
		found := map[int]*warBattleOf{}
		for _, battle := range warBattlesOf(kills) {
			for _, kill := range battle.kills {
				found[kill.id] = battle
			}
		}
		return found
	}
	firstBattles := battleOf(notTrash())
	anchors := []*warKill{} // newest first, so a killmail goes with the last station to die before it
	for _, station := range slices.Backward(t.stations) {
		if battle := firstBattles[station.id]; battle != nil && battle.counted() && station.parties[0].side != WarSideNone {
			anchors = append(anchors, station)
			t.anchors[station.id] = true
		}
	}
	// an anchorable goes with the last station its side lost in its system before it
	for _, kill := range ordered {
		mark := t.marks[kill.id]
		if !warIsAnchorable(warVictimType(kill)) || mark == (warTrashMark{}) || mark.anchor != 0 {
			continue
		}
		index := slices.IndexFunc(anchors, func(anchor *warKill) bool {
			return anchor.at.Before(kill.at) && !kill.at.After(anchor.at.Add(warRepeatReach)) && anchor.parties[0].side == mark.owner &&
				anchor.raw.ESIKillmail.SolarSystemID == kill.raw.ESIKillmail.SolarSystemID
		})
		if index >= 0 {
			mark.anchor = anchors[index].id
			t.marks[kill.id] = mark
		}
	}

	// shotBack is whether a killmail is in a later battle than the station's where its owner killed the other side: its fleet came back, and that is a fight
	shotBack := func(kill *warKill, battles map[int]*warBattleOf, anchor *warKill, owner WarSideKey) bool {
		later := battles[kill.id]
		return later != nil && later != battles[anchor.id] && later.shotAt(owner)
	}

	mayBeTrash := func(kill *warKill, owner WarSideKey) bool {
		victim := kill.parties[0]
		// blue on blue is trash whichever side it is on, as nobody was fighting
		return !victim.ignored && (kill.sameSide || victim.side == owner || victim.side == WarSideNone) && !warIsFixture(warVictimType(kill)) &&
			!comm.CapsuleGroups.ContainsGroupOf(warVictimType(kill)) && t.marks[kill.id] == (warTrashMark{})
	}

	// each pilot's capsule losses, by system
	type podKey struct{ system, char int }
	pods := map[podKey][]time.Time{}
	for _, kill := range ordered {
		if char := kill.parties[0].char; char != 0 && comm.CapsuleGroups.ContainsGroupOf(warVictimType(kill)) {
			key := podKey{kill.raw.ESIKillmail.SolarSystemID, char}
			pods[key] = append(pods[key], kill.at)
		}
	}

	// repeat victims
	for _, anchor := range anchors {
		owner := anchor.parties[0].side
		losses := map[int][]*warKill{}
		for _, kill := range bySystem[anchor.raw.ESIKillmail.SolarSystemID] {
			char := kill.parties[0].char
			if char == 0 || !kill.at.After(anchor.at) || kill.at.After(anchor.at.Add(warRepeatReach)) || !mayBeTrash(kill, owner) || shotBack(kill, firstBattles, anchor, owner) {
				continue
			}
			losses[char] = append(losses[char], kill)
		}
		for char, all := range losses {
			mine := warEmptyShips(all, pods[podKey{anchor.raw.ESIKillmail.SolarSystemID, char}])
			var lastShip *warKill
			repeat := false
			for _, kill := range mine {
				if lastShip != nil && kill.at.Sub(lastShip.at) < warRepeatGap {
					repeat = true
				}
				lastShip = kill
			}
			if repeat {
				for _, kill := range mine {
					t.marks[kill.id] = warTrashMark{owner: owner, anchor: anchor.id, definite: true}
				}
			}
		}
	}

	// after the battle, found again without those so that it ends
	ended := battleOf(notTrash())
	for _, anchor := range anchors {
		battle := ended[anchor.id]
		if battle == nil {
			continue
		}
		owner := anchor.parties[0].side
		end := battle.kills[len(battle.kills)-1].at
		system := anchor.raw.ESIKillmail.SolarSystemID
		losses := map[int][]*warKill{}
		for _, kill := range bySystem[system] {
			if !kill.at.After(end) || kill.at.After(end.Add(warTrashWindow)) || !mayBeTrash(kill, owner) || shotBack(kill, ended, anchor, owner) || warEscaping(kill) {
				continue
			}
			losses[kill.parties[0].char] = append(losses[kill.parties[0].char], kill)
		}
		for char, all := range losses {
			for _, kill := range warEmptyShips(all, pods[podKey{system, char}]) {
				t.marks[kill.id] = warTrashMark{owner: owner, anchor: anchor.id, definite: warIsAnchorable(warVictimType(kill))}
			}
		}
	}
	t.battles = warBattlesOf(notTrash())

	// asset denial: a fitted capital its own side killed, losing a station there soon after. What is already marked keeps its mark and grade.
	for _, kill := range ordered {
		if kill.sameSide && warIsCapital(warVictimType(kill)) && t.marks[kill.id] == (warTrashMark{}) && stationSoon(kill, kill.parties[0].side) != nil {
			markDenial(kill, kill.parties[0].side, false, false)
		}
	}
	return t
}

// warStationAfter is the first station a side lost in a killmail's system after it, nil for none
func warStationAfter(stations []*warKill, kill *warKill, side WarSideKey) *warKill {
	for _, station := range stations {
		if station.at.After(kill.at) && station.parties[0].side == side && station.raw.ESIKillmail.SolarSystemID == kill.raw.ESIKillmail.SolarSystemID {
			return station
		}
	}
	return nil
}

// warEscaping is whether a loss looks like a fleet trying to get out: a fitted ship with more than one attacker (user, 2026-11-05). A repeat victim's ship is trash all the same.
func warEscaping(kill *warKill) bool {
	return !warUnfitted(kill) && len(kill.raw.ESIKillmail.Attackers) > 1
}

// warStationBefore is whether a side lost a station in a killmail's system before it
func warStationBefore(stations []*warKill, kill *warKill, side WarSideKey) bool {
	return slices.ContainsFunc(stations, func(station *warKill) bool {
		return station.at.Before(kill.at) && station.parties[0].side == side && station.raw.ESIKillmail.SolarSystemID == kill.raw.ESIKillmail.SolarSystemID
	})
}

// warRefFighters is one side's station under attack in one system of a battle: its Standup Fighters lost or used, or a station of its firing that did not die in the battle
type warRefFighters struct {
	side, listed WarSideKey // listed is by SideOf alone, side also takes in a third party fighting beside it
	group        warGroupKey
	system       int
	first, last  time.Time
}

// confirmedBy is the upwell structure whose death makes these fighters a station ref, nil for none
func (f *warRefFighters) confirmedBy(stations []*warKill) *warKill {
	for _, station := range stations {
		victim := station.parties[0]
		sameOwner := (f.listed != WarSideNone && victim.side == f.listed) || (f.listed == WarSideNone && victim.hasGroup && victim.group == f.group)
		if !sameOwner || station.raw.ESIKillmail.SolarSystemID != f.system || !comm.UpwellStructureGroups.ContainsGroupOf(warVictimType(station)) {
			continue
		}
		if !station.at.Before(f.first) && !station.at.After(f.last.Add(warTimerDays(warVictimType(station)))) {
			return station
		}
	}
	return nil
}
