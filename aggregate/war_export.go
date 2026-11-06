package aggregate

import (
	"cmp"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// WarPresence is what killmails show of a side's citadels and POCOs in a system. POS are left out, as invaders often anchor them. It says what was seen, never what stands there now.
type WarPresence struct {
	Seen      bool      // a citadel, POCO, or Standup Fighter of the side on a killmail, lost or firing
	FirstLost time.Time // when the side first lost a citadel or POCO there, zero for never
}

// WarSystemPresence is each side's WarPresence in a system
type WarSystemPresence struct {
	A, B WarPresence
}

// WarCorpAppearance is a corp under its alliance
type WarCorpAppearance struct {
	Corp    WarGroup
	Battles int
}

// WarGroupAppearance is the battles a group was seen in on one side: an alliance with its Corps, or a corp with no alliance
type WarGroupAppearance struct {
	Group   WarGroup
	Battles int
	Corps   []WarCorpAppearance // most battles first
}

// WarExport are the figures only the war's export shows, read from the battles as edited, with trash and ignored battles in none of it
type WarExport struct {
	Presence    map[int]WarSystemPresence // by system id, only those with something seen
	Appearances map[WarSideKey][]WarGroupAppearance
}

// warAppearanceKey is a group on a side
type warAppearanceKey struct {
	side  WarSideKey
	stint warStintKey
}

// warExportOf tallies the export's figures. Names come from people, which has read every killmail of the battles counted.
func warExportOf(battles []*warBattleOf, people *warPeople) WarExport {
	export := WarExport{Presence: map[int]WarSystemPresence{}, Appearances: map[WarSideKey][]WarGroupAppearance{}}
	corps := map[warAppearanceKey]int{}
	tops := map[warAppearanceKey]int{} // a corp only where it has no alliance, else the alliance alone

	for _, battle := range battles {
		if battle.ignored {
			continue
		}
		seenCorps, seenTops := map[warAppearanceKey]bool{}, map[warAppearanceKey]bool{}
		for _, kill := range battle.kills {
			esi := kill.raw.ESIKillmail
			note := func(party warParty, corp, ally, ship int, lost bool) {
				if party.ignored {
					return
				}
				side := kill.effective(party)
				if party.hasGroup {
					seenCorps[warAppearanceKey{side, warStintKey{corp, ally}}] = true
					top := warStintKey{ally: ally}
					if ally == 0 {
						top = warStintKey{corp: corp}
					}
					seenTops[warAppearanceKey{side, top}] = true
				}
				export.note(side, esi.SolarSystemID, ship, kill.at, lost)
			}
			note(kill.parties[0], intOf(esi.Victim.CorporationID), intOf(esi.Victim.AllianceID), esi.Victim.ShipTypeID, true)
			for index, attacker := range esi.Attackers {
				note(kill.parties[index+1], intOf(attacker.CorporationID), intOf(attacker.AllianceID), attacker.ShipTypeID, false)
			}
		}
		for key := range seenCorps {
			corps[key]++
		}
		for key := range seenTops {
			tops[key]++
		}
	}

	corpGroup := func(id int) WarGroup { return WarGroup{Type: "CORPORATION", ID: id, Name: people.corpNames[id]} }
	allyGroup := func(id int) WarGroup { return WarGroup{Type: "ALLIANCE", ID: id, Name: people.allyNames[id]} }
	for key, count := range tops {
		row := WarGroupAppearance{Group: corpGroup(key.stint.corp), Battles: count, Corps: []WarCorpAppearance{}}
		if key.stint.ally != 0 {
			row.Group = allyGroup(key.stint.ally)
			for corp, battles := range corps {
				if corp.side == key.side && corp.stint.ally == key.stint.ally {
					row.Corps = append(row.Corps, WarCorpAppearance{Corp: corpGroup(corp.stint.corp), Battles: battles})
				}
			}
			slices.SortFunc(row.Corps, func(a, b WarCorpAppearance) int {
				return cmp.Or(cmp.Compare(b.Battles, a.Battles), warByName(a.Corp.Name, b.Corp.Name), cmp.Compare(a.Corp.ID, b.Corp.ID))
			})
		}
		export.Appearances[key.side] = append(export.Appearances[key.side], row)
	}
	for side := range export.Appearances {
		slices.SortFunc(export.Appearances[side], func(a, b WarGroupAppearance) int {
			return cmp.Or(cmp.Compare(b.Battles, a.Battles), warByName(a.Group.Name, b.Group.Name), cmp.Compare(a.Group.Type, b.Group.Type), cmp.Compare(a.Group.ID, b.Group.ID))
		})
	}
	return export
}

// note records a structure of a side of the war on a killmail in a system
func (e WarExport) note(side WarSideKey, systemID, ship int, at time.Time, lost bool) {
	if side == WarSideNone {
		return
	}
	station := comm.UpwellStructureGroups.ContainsGroupOf(ship) || comm.PocoGroups.ContainsGroupOf(ship)
	if !station && !comm.StandupFighterGroups.ContainsGroupOf(ship) {
		return
	}
	entry := e.Presence[systemID]
	presence := &entry.A
	if side == WarSideB {
		presence = &entry.B
	}
	presence.Seen = true
	if lost && station && (presence.FirstLost.IsZero() || at.Before(presence.FirstLost)) {
		presence.FirstLost = at
	}
	e.Presence[systemID] = entry
}
