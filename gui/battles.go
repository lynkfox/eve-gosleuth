package gui

import (
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// battleMinPilots is the fewest pilots, both sides together, a battle needs for the Battles tab
const battleMinPilots = 10

// Battle is a row of the Battles tab: a battle found on the target's killmails, see aggregate.FindBattles
type Battle struct {
	ID    string    `json:"id"`
	Start time.Time `json:"start"` // its first killmail
	End   time.Time `json:"end"`   // its last killmail
	// the system with the most of its killmails, the first of them on a tie, which its battle report links are for
	System SystemSummary `json:"system"`
	// the other systems it spread to, by name, in the order it reached them
	OtherSystems []string `json:"otherSystems"`
	Pilots       int      `json:"pilots"`    // the pilots on its killmails, victims and attackers, both sides
	Ships        int      `json:"ships"`     // the ships those pilots flew, each pilot's hull counted once
	Killmails    int      `json:"killmails"` // the ships destroyed in it
	ZkillURL     string   `json:"zkillUrl"`
	EveToolsURL  string   `json:"evetoolsUrl"`
	WarbeaconURL string   `json:"warbeaconUrl"`
}

/*
	buildBattles lists the battles found on a target's killmails with at least battleMinPilots pilots, for its Battles tab

Parameters:
  - battles: the collated battles, by id
  - killmails: the target's expanded killmails, which the battles' killmail ids are read from

Returns:
  - []Battle: newest first. Empty if there are none.
*/
func buildBattles(battles map[string]aggregate.BattleFought, killmails []*retrieval.Killmail) []Battle {
	byID := make(map[int]*retrieval.Killmail, len(killmails))
	for _, km := range killmails {
		byID[km.KillmailID] = km
	}

	rows := []Battle{}
	for id, battle := range battles {
		found := []*retrieval.Killmail{}
		for _, killmailID := range battle.KillmailIDs {
			if km, ok := byID[killmailID]; ok {
				found = append(found, km)
			}
		}
		if len(found) == 0 || len(battle.UniquePilots) < battleMinPilots {
			continue
		}
		slices.SortStableFunc(found, func(a, b *retrieval.Killmail) int { return a.KillmailTime.Compare(b.KillmailTime) })

		row := Battle{
			ID:           id,
			Start:        found[0].KillmailTime,
			End:          found[len(found)-1].KillmailTime,
			OtherSystems: []string{},
			Pilots:       len(battle.UniquePilots),
			Killmails:    len(found),
		}
		main := battleSystem(found)
		row.System = structureSystem(main)
		ships := map[[2]int]struct{}{}
		seen := map[int]bool{}
		for _, km := range found {
			if km.System != nil && (main == nil || km.System.SystemID != main.SystemID) && !seen[km.System.SystemID] {
				seen[km.System.SystemID] = true
				row.OtherSystems = append(row.OtherSystems, km.System.Name)
			}
			for _, flown := range battleShips(km) {
				ships[flown] = struct{}{}
			}
		}
		row.Ships = len(ships)

		if main != nil {
			middle := row.Start.Add(row.End.Sub(row.Start) / 2)
			row.ZkillURL = api.ZkillRelatedURL(main.SystemID, middle)
			row.EveToolsURL = api.EveToolsRelatedURL(main.SystemID, middle)
			row.WarbeaconURL = api.WarbeaconRelatedURL(main.SystemID, middle)
		}
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b Battle) int { return b.Start.Compare(a.Start) })
	return rows
}

// battleSystem is the system with the most of a battle's killmails, the first reached on a tie, nil when none is known. killmails are oldest first.
func battleSystem(killmails []*retrieval.Killmail) *retrieval.System {
	counts := map[int]int{}
	var best *retrieval.System
	for _, km := range killmails {
		if km.System == nil {
			continue
		}
		counts[km.System.SystemID]++
		if best == nil || counts[km.System.SystemID] > counts[best.SystemID] {
			best = km.System
		}
	}
	return best
}

// battleShips are the pilot and hull type of each pilot on a killmail, victim and attackers, leaving out NPCs and structures, which have no pilot
func battleShips(km *retrieval.Killmail) [][2]int {
	ships := [][2]int{}
	add := func(pilot *retrieval.Pilot, ship *retrieval.Item) {
		if pilot == nil || pilot.CharacterID == 0 || ship == nil {
			return
		}
		ships = append(ships, [2]int{pilot.CharacterID, ship.ItemID})
	}
	if km.Victim != nil {
		add(km.Victim.Pilot, km.Victim.Ship)
	}
	for _, attacker := range km.Attackers {
		add(attacker.Pilot, attacker.Ship)
	}
	return ships
}
