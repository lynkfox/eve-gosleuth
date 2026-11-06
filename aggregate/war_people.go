package aggregate

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// WarPilot is a pilot's row in the Pilots tab, one per side they counted for
type WarPilot struct {
	CharacterID  int
	Name         string
	Corporation  WarGroup  // the latest they were seen in during the war
	Alliance     *WarGroup // of that corp at the time, nil for none
	ISKDestroyed float64   // the full value of every killmail they are on, on those a side's own loss is not
	Killmails    int       // of those, the ones that are not a capsule
	Capsules     int       // of those, the ones that are a capsule
}

// WarHull is a ship flown by a side
type WarHull struct {
	TypeID int
	Name   string
	Seen   int // each pilot once per battle
	Lost   int
}

// WarPilotStint is a corp and alliance a pilot was seen in during the war
type WarPilotStint struct {
	CorporationID, AllianceID int
	Corporation, Alliance     string
	From                      time.Time // first seen in it
}

// WarParticipant is a pilot under a corp in the Participants tab
type WarParticipant struct {
	CharacterID int
	Name        string
	Stints      []WarPilotStint // oldest first, nil unless the pilot changed corp or alliance during the war
}

// WarParticipantCorp is a corp under an alliance, with the pilots seen in it
type WarParticipantCorp struct {
	Corp   WarGroup
	Pilots []WarParticipant
}

// WarParticipantGroup is the top of the Participants tab: an alliance with its Corps, or a corp with no alliance with its Pilots
type WarParticipantGroup struct {
	Group  WarGroup
	Count  int // pilots, each once
	Corps  []WarParticipantCorp
	Pilots []WarParticipant
}

// WarSidePeople is who fought on a side, from its counted battles as edited. Trash and ignored battles are in none of it.
type WarSidePeople struct {
	Pilots       []WarPilot            // most ISK destroyed first
	Flown        []WarHull             // most seen first
	Participants []WarParticipantGroup // most pilots first
}

// warStintKey is a corp and alliance at a kill
type warStintKey struct{ corp, ally int }

type warPilotTally struct {
	isk      float64
	kills    int
	capsules int
}

// warPeople tallies who fought, per side
type warPeople struct {
	pilots     map[WarSideKey]map[int]*warPilotTally
	flown      map[WarSideKey]map[int]*WarHull
	members    map[WarSideKey]map[warStintKey]map[int]bool // the pilots seen in each corp and alliance
	stints     map[int]map[warStintKey]time.Time           // by pilot, when first seen in each corp and alliance
	sided      map[int]bool                                // pilots who count for a side somewhere in the war
	pilotNames map[int]string
	corpNames  map[int]string
	allyNames  map[int]string
	shipNames  map[int]string
}

func newWarPeople() *warPeople {
	return &warPeople{
		pilots:     map[WarSideKey]map[int]*warPilotTally{WarSideA: {}, WarSideB: {}, WarSideNone: {}},
		flown:      map[WarSideKey]map[int]*WarHull{WarSideA: {}, WarSideB: {}, WarSideNone: {}},
		members:    map[WarSideKey]map[warStintKey]map[int]bool{WarSideA: {}, WarSideB: {}, WarSideNone: {}},
		stints:     map[int]map[warStintKey]time.Time{},
		sided:      map[int]bool{},
		pilotNames: map[int]string{},
		corpNames:  map[int]string{},
		allyNames:  map[int]string{},
		shipNames:  map[int]string{},
	}
}

// readNames keeps the names on an expanded killmail
func (p *warPeople) readNames(km *retrieval.Killmail) {
	read := func(pilot *retrieval.Pilot, corp *retrieval.Corporation, ally *retrieval.Alliance, ship *retrieval.Item) {
		if pilot != nil && pilot.Name != "" {
			p.pilotNames[pilot.CharacterID] = pilot.Name
		}
		if corp != nil && corp.Name != "" {
			p.corpNames[corp.CorporationID] = corp.Name
		}
		if ally != nil && ally.Name != "" {
			p.allyNames[ally.AllianceID] = ally.Name
		}
		if ship != nil && ship.Name != "" {
			p.shipNames[ship.ItemID] = ship.Name
		}
	}
	if km.Victim != nil {
		read(km.Victim.Pilot, km.Victim.CorpAtKill, km.Victim.AllianceAtKill, km.Victim.Ship)
	}
	for _, attacker := range km.Attackers {
		if attacker != nil {
			read(attacker.Pilot, attacker.CorpAtKill, attacker.AllianceAtKill, attacker.Ship)
		}
	}
}

// warFlownShip is true for a type that is a ship, which leaves out capsules, structures, and fighters
func warFlownShip(typeID int) bool {
	fit := comm.FitTypeOf(typeID)
	return fit != nil && comm.ShipCategories.ContainsIDNumber(fit.CategoryID) && !comm.CapsuleGroups.ContainsIDNumber(fit.GroupID)
}

func (p *warPeople) hull(side WarSideKey, typeID int) *WarHull {
	hull := p.flown[side][typeID]
	if hull == nil {
		hull = &WarHull{TypeID: typeID}
		p.flown[side][typeID] = hull
	}
	return hull
}

// noteSided notes who counts for a side in a battle that is not ignored. All of a war's battles go through it before any is counted.
func (p *warPeople) noteSided(battle *warBattleOf) {
	for _, kill := range battle.kills {
		for _, party := range kill.parties {
			if party.char != 0 && !party.ignored && kill.effective(party) != WarSideNone {
				p.sided[party.char] = true
			}
		}
	}
}

// outOfTheWar is true for a pilot of a side on a killmail from their time in an NPC corp, which is left out, so they show only on their side and not under Others (user, 2026-11-05)
func (p *warPeople) outOfTheWar(party warParty) bool {
	return party.char != 0 && !party.hasGroup && p.sided[party.char]
}

// addBattle counts a battle that is not ignored: a pilot on one of its killmails counts for the side they count for on that killmail, as everywhere in a war
func (p *warPeople) addBattle(battle *warBattleOf, expanded map[int]*retrieval.Killmail) {
	type sighting struct {
		side       WarSideKey
		char, ship int
	}
	seen := map[sighting]bool{}
	sight := func(kill *warKill, party warParty, corp, ally, ship int) WarSideKey {
		side := kill.effective(party)
		if party.char == 0 {
			return side
		}
		if p.pilots[side][party.char] == nil {
			p.pilots[side][party.char] = &warPilotTally{}
		}
		key := warStintKey{corp, ally}
		if p.members[side][key] == nil {
			p.members[side][key] = map[int]bool{}
		}
		p.members[side][key][party.char] = true
		if p.stints[party.char] == nil {
			p.stints[party.char] = map[warStintKey]time.Time{}
		}
		if first, known := p.stints[party.char][key]; !known || kill.at.Before(first) {
			p.stints[party.char][key] = kill.at
		}
		if at := (sighting{side, party.char, ship}); warFlownShip(ship) && !seen[at] {
			seen[at] = true
			p.hull(side, ship).Seen++
		}
		return side
	}

	for _, kill := range battle.kills {
		km := expanded[kill.id]
		if km == nil {
			continue
		}
		p.readNames(km)
		esi := kill.raw.ESIKillmail
		victim := kill.parties[0]
		victimSide := kill.effective(victim)
		loss := warLossOf(kill, km)
		capsule := comm.CapsuleGroups.ContainsGroupOf(loss.typeID)
		if !victim.ignored && !p.outOfTheWar(victim) {
			sight(kill, victim, intOf(esi.Victim.CorporationID), intOf(esi.Victim.AllianceID), esi.Victim.ShipTypeID)
			if loss.ship && warFlownShip(loss.typeID) {
				p.hull(victimSide, loss.typeID).Lost++
			}
		}
		credited := map[int]bool{}
		for index, attacker := range esi.Attackers {
			party := kill.parties[index+1]
			if party.ignored || p.outOfTheWar(party) {
				continue
			}
			side := sight(kill, party, intOf(attacker.CorporationID), intOf(attacker.AllianceID), attacker.ShipTypeID)
			if party.char == 0 || credited[party.char] || victim.ignored || (side != WarSideNone && side == victimSide) {
				continue
			}
			credited[party.char] = true
			p.pilots[side][party.char].isk += loss.isk
			if capsule {
				p.pilots[side][party.char].capsules++
			} else {
				p.pilots[side][party.char].kills++
			}
		}
	}
}

// stintsOf is a pilot's corps and alliances, oldest first
func (p *warPeople) stintsOf(char int) []WarPilotStint {
	stints := []WarPilotStint{}
	for key, from := range p.stints[char] {
		stints = append(stints, WarPilotStint{CorporationID: key.corp, AllianceID: key.ally, Corporation: p.corpNames[key.corp], Alliance: p.allyNames[key.ally], From: from})
	}
	slices.SortFunc(stints, func(a, b WarPilotStint) int {
		return cmp.Or(a.From.Compare(b.From), cmp.Compare(a.CorporationID, b.CorporationID), cmp.Compare(a.AllianceID, b.AllianceID))
	})
	return stints
}

func warByName(a, b string) int { return cmp.Compare(strings.ToLower(a), strings.ToLower(b)) }

// participants lists a set of pilots by name
func (p *warPeople) participants(chars map[int]bool) []WarParticipant {
	out := make([]WarParticipant, 0, len(chars))
	for char := range chars {
		row := WarParticipant{CharacterID: char, Name: p.pilotNames[char]}
		if stints := p.stintsOf(char); len(stints) > 1 {
			row.Stints = stints
		}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b WarParticipant) int {
		return cmp.Or(warByName(a.Name, b.Name), cmp.Compare(a.CharacterID, b.CharacterID))
	})
	return out
}

// side makes a side's lists
func (p *warPeople) side(side WarSideKey) WarSidePeople {
	out := WarSidePeople{Pilots: []WarPilot{}, Flown: []WarHull{}, Participants: []WarParticipantGroup{}}
	corpGroup := func(id int) WarGroup {
		return WarGroup{Type: types.CORPORATION.String(), ID: id, Name: p.corpNames[id]}
	}
	allyGroup := func(id int) WarGroup {
		return WarGroup{Type: types.ALLIANCE.String(), ID: id, Name: p.allyNames[id]}
	}

	for char, tally := range p.pilots[side] {
		// the latest on this side, as a pilot on both sides has a corp on each
		stints := slices.DeleteFunc(p.stintsOf(char), func(stint WarPilotStint) bool {
			return !p.members[side][warStintKey{stint.CorporationID, stint.AllianceID}][char]
		})
		latest := stints[len(stints)-1]
		row := WarPilot{CharacterID: char, Name: p.pilotNames[char], Corporation: corpGroup(latest.CorporationID), ISKDestroyed: tally.isk, Killmails: tally.kills, Capsules: tally.capsules}
		if latest.AllianceID != 0 {
			ally := allyGroup(latest.AllianceID)
			row.Alliance = &ally
		}
		out.Pilots = append(out.Pilots, row)
	}
	slices.SortFunc(out.Pilots, func(a, b WarPilot) int {
		return cmp.Or(cmp.Compare(b.ISKDestroyed, a.ISKDestroyed), cmp.Compare(b.Killmails, a.Killmails), warByName(a.Name, b.Name), cmp.Compare(a.CharacterID, b.CharacterID))
	})

	for typeID, hull := range p.flown[side] {
		row := *hull
		row.Name = p.shipNames[typeID]
		if fit := comm.FitTypeOf(typeID); row.Name == "" && fit != nil {
			row.Name = fit.Name
		}
		out.Flown = append(out.Flown, row)
	}
	slices.SortFunc(out.Flown, func(a, b WarHull) int {
		return cmp.Or(cmp.Compare(b.Seen, a.Seen), warByName(a.Name, b.Name), cmp.Compare(a.TypeID, b.TypeID))
	})

	// a corp with no alliance sits at the top, the rest under their alliance
	byAlliance := map[int]map[int]map[int]bool{}
	for key, chars := range p.members[side] {
		if key.ally == 0 {
			out.Participants = append(out.Participants, WarParticipantGroup{Group: corpGroup(key.corp), Count: len(chars), Pilots: p.participants(chars)})
			continue
		}
		if byAlliance[key.ally] == nil {
			byAlliance[key.ally] = map[int]map[int]bool{}
		}
		byAlliance[key.ally][key.corp] = chars
	}
	for ally, corps := range byAlliance {
		group := WarParticipantGroup{Group: allyGroup(ally), Corps: []WarParticipantCorp{}}
		everyone := map[int]bool{}
		for corp, chars := range corps {
			group.Corps = append(group.Corps, WarParticipantCorp{Corp: corpGroup(corp), Pilots: p.participants(chars)})
			for char := range chars {
				everyone[char] = true
			}
		}
		group.Count = len(everyone)
		slices.SortFunc(group.Corps, func(a, b WarParticipantCorp) int {
			return cmp.Or(cmp.Compare(len(b.Pilots), len(a.Pilots)), warByName(a.Corp.Name, b.Corp.Name), cmp.Compare(a.Corp.ID, b.Corp.ID))
		})
		out.Participants = append(out.Participants, group)
	}
	slices.SortFunc(out.Participants, func(a, b WarParticipantGroup) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), warByName(a.Group.Name, b.Group.Name), cmp.Compare(a.Group.Type, b.Group.Type), cmp.Compare(a.Group.ID, b.Group.ID))
	})
	return out
}
