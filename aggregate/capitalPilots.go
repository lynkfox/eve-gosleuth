package aggregate

import (
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// CapitalShipPilots is a capital ship type and the target's pilots seen flying it
type CapitalShipPilots struct {
	Ship   *retrieval.Item
	Pilots []*retrieval.Pilot // by name, each pilot once
	Seen   map[int]Sighting   // each pilot's newest killmail flying it, by character id
}

// Sighting is when and where a pilot was last seen flying a ship
type Sighting struct {
	Time   time.Time
	System *retrieval.System // nil when not known
}

// killmailParticipant is anyone on a killmail, victim or attacker, with the ship and the corp and alliance they flew for on it
type killmailParticipant struct {
	pilot    *retrieval.Pilot
	ship     *retrieval.Item
	corp     *retrieval.Corporation
	alliance *retrieval.Alliance
}

func participants(km *retrieval.Killmail) []killmailParticipant {
	found := []killmailParticipant{}
	if km.Victim != nil {
		found = append(found, killmailParticipant{km.Victim.Pilot, km.Victim.Ship, km.Victim.CorpAtKill, km.Victim.AllianceAtKill})
	}
	for _, attacker := range km.Attackers {
		found = append(found, killmailParticipant{attacker.Pilot, attacker.Ship, attacker.CorpAtKill, attacker.AllianceAtKill})
	}
	return found
}

// flewForTarget reports whether a corp and alliance on a killmail are a corp or alliance target, or one of its holdings
func flewForTarget(corp *retrieval.Corporation, alliance *retrieval.Alliance, target *retrieval.IntelTarget) bool {
	return target.IsGroup(groupIDs(corp, alliance))
}

// isCapital reports whether a ship is a capital: one of comm.CapitalGroups, or comm.CapitalTypes
func isCapital(ship *retrieval.Item) bool {
	return comm.CapitalTypes.ContainsIDNumber(ship.ItemID) || comm.CapitalGroups.ContainsGroupOf(ship.ItemID)
}

/*
	GetCapitalPilots finds the corp or alliance target's pilots flying capitals on the killmails, as victim or attacker, grouped by capital ship type.

A pilot counts when they flew for the target on that killmail, so a pilot who has since left still shows for the capitals they flew while a member.

Parameters:
  - killmails: the expanded killmails from CollectedData
  - target: a corporation or alliance target

Returns:
  - []CapitalShipPilots: each capital ship type flown, by name, with its pilots by name and each pilot's newest sighting (Seen). Empty for other target types or when none were flown.
*/
func GetCapitalPilots(killmails []*retrieval.Killmail, target *retrieval.IntelTarget) []CapitalShipPilots {
	byShip := map[int]*CapitalShipPilots{}

	for _, km := range killmails {
		for _, seen := range participants(km) {
			if seen.pilot == nil || seen.pilot.NPC || seen.ship == nil || !isCapital(seen.ship) {
				continue
			}
			if !flewForTarget(seen.corp, seen.alliance, target) {
				continue
			}

			capital, found := byShip[seen.ship.ItemID]
			if !found {
				capital = &CapitalShipPilots{Ship: seen.ship, Seen: map[int]Sighting{}}
				byShip[seen.ship.ItemID] = capital
			}
			if !slices.ContainsFunc(capital.Pilots, func(pilot *retrieval.Pilot) bool { return pilot.CharacterID == seen.pilot.CharacterID }) {
				capital.Pilots = append(capital.Pilots, seen.pilot)
			}
			if last, found := capital.Seen[seen.pilot.CharacterID]; !found || km.KillmailTime.After(last.Time) {
				capital.Seen[seen.pilot.CharacterID] = Sighting{Time: km.KillmailTime, System: km.System}
			}
		}
	}

	capitals := []CapitalShipPilots{}
	for _, capital := range byShip {
		slices.SortFunc(capital.Pilots, func(a, b *retrieval.Pilot) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
		capitals = append(capitals, *capital)
	}
	slices.SortFunc(capitals, func(a, b CapitalShipPilots) int { return strings.Compare(a.Ship.Name, b.Ship.Name) })

	return capitals
}

/*
	HasLeftTarget reports whether a capital pilot is no longer in the target's alliance, by their current corp.

Corps change often, so only leaving the alliance counts: for an alliance target, being in another alliance or none; for a corp target, leaving the corp's alliance,
or leaving the corp itself when it has no alliance.

Parameters:
  - pilot: the pilot, with their current corporation and its alliance
  - target: a corporation or alliance target. A corporation target's Expanded is its *retrieval.Corporation, with its current alliance.

Returns:
  - bool: true if they have left. False when their current corp is not known, ie a deleted character.
*/
func HasLeftTarget(pilot *retrieval.Pilot, target *retrieval.IntelTarget) bool {
	corp := pilot.Corporation
	if corp == nil {
		return false
	}
	allianceID := 0
	if corp.Alliance != nil {
		allianceID = corp.Alliance.AllianceID
	}
	// moving to one of the target's holdings is not leaving
	if target.IsGroup(corp.CorporationID, allianceID) {
		return false
	}

	switch target.TypeOfEntity {
	case types.ALLIANCE:
		return allianceID != target.IDNumber
	case types.CORPORATION:
		if targetCorp, ok := target.Expanded.(*retrieval.Corporation); ok && targetCorp != nil && targetCorp.Alliance != nil {
			return allianceID != targetCorp.Alliance.AllianceID
		}
		return corp.CorporationID != target.IDNumber
	}
	return false
}
