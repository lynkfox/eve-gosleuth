package aggregate

import (
	"cmp"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// AllyActivity is an alliance, or a corporation in none, that attacked alongside the target without being in its alliance
type AllyActivity struct {
	Alliance    *retrieval.Alliance    // the ally, nil when it is a corporation in no alliance
	Corporation *retrieval.Corporation // the ally when Alliance is nil
	Killmails   int                    // the target's kills it was on
	Killed      int                    // the target's kills where the victim was in it, ie a blue the target also shot, or a third party
	PilotIDs    map[int]struct{}
	Activity    []time.Time // one time per killmail
}

// AllyKey names an ally by its type, ALLIANCE or CORPORATION, and id
type AllyKey struct {
	Type types.EntityType
	ID   int
}

// ID returns the ally's alliance or corporation id
func (a AllyActivity) ID() int {
	if a.Alliance != nil {
		return a.Alliance.AllianceID
	}
	return a.Corporation.CorporationID
}

// Name returns the ally's alliance or corporation name
func (a AllyActivity) Name() string {
	if a.Alliance != nil {
		return a.Alliance.Name
	}
	return a.Corporation.Name
}

// Type returns ALLIANCE or CORPORATION, whichever the ally is
func (a AllyActivity) Type() types.EntityType {
	if a.Alliance != nil {
		return types.ALLIANCE
	}
	return types.CORPORATION
}

// UniqueDays returns how many different days (EVE time) the ally was on the target's kills
func (a AllyActivity) UniqueDays() int {
	return uniqueDays(a.Activity)
}

// FirstSeen returns the time of the oldest kill the ally shared with the target
func (a AllyActivity) FirstSeen() time.Time {
	return slices.MinFunc(a.Activity, time.Time.Compare)
}

// LastSeen returns the time of the most recent kill the ally shared with the target
func (a AllyActivity) LastSeen() time.Time {
	return slices.MaxFunc(a.Activity, time.Time.Compare)
}

/*
	GetTopAllies ranks the alliances, and corporations in no alliance, that attacked alongside a pilot, corporation, or alliance target, outside its own alliance.

Only the target's kills count, since on a loss no one else is on its side. On each kill the target's own group is left out as it was then: its alliance,
or its corporation when it was in none. NPCs, structures, and pilots in NPC corporations are left out too.
Each ally also counts the target's kills its own members were the victim of.

Parameters:
  - limit: the maximum number of allies to return
  - killmails: the expanded killmails from CollectedData
  - target: a PILOT, CORPORATION, or ALLIANCE target
  - ignored: allies the user chose to leave out for this target, so the next ones take their places. nil for none.
  - skipKilled: also leave out every ally the target killed one of on these killmails, its Killed above 0

Returns:
  - []AllyActivity: the top allies, every alliance before the corporations in none, then most shared kills first,
    ties broken by the most unique days, then the most pilots, then by name. Empty when there are none.
*/
func GetTopAllies(limit int, killmails []*retrieval.Killmail, target *retrieval.IntelTarget, ignored map[AllyKey]bool, skipKilled bool) []AllyActivity {
	allies := map[types.EntityType]map[int]*AllyActivity{types.ALLIANCE: {}, types.CORPORATION: {}}
	type key = AllyKey
	killed := map[key]int{} // the target's kills by the victim's alliance, or corp in none

	for _, km := range killmails {
		ownAlliance, ownCorp, attacked := targetGroupOnKill(km, target)
		if !attacked {
			continue
		}
		if victim := km.Victim; victim != nil {
			if victim.AllianceAtKill != nil {
				killed[key{types.ALLIANCE, victim.AllianceAtKill.AllianceID}]++
			} else if victim.CorpAtKill != nil {
				killed[key{types.CORPORATION, victim.CorpAtKill.CorporationID}]++
			}
		}

		onThisKillmail := map[types.EntityType]map[int]bool{types.ALLIANCE: {}, types.CORPORATION: {}}
		for _, attacker := range km.Attackers {
			if attacker.Pilot == nil || attacker.Pilot.NPC || attacker.CorpAtKill == nil || api.IsNPCCorporation(attacker.CorpAtKill.CorporationID) {
				continue
			}

			ally := AllyActivity{Alliance: attacker.AllianceAtKill, Corporation: attacker.CorpAtKill}
			if ally.Alliance != nil {
				ally.Corporation = nil
			}
			if (ally.Alliance != nil && ally.Alliance.AllianceID == ownAlliance) || (ally.Alliance == nil && ally.Corporation.CorporationID == ownCorp) {
				continue
			}
			if ignored[key{ally.Type(), ally.ID()}] {
				continue
			}

			found, seen := allies[ally.Type()][ally.ID()]
			if !seen {
				ally.PilotIDs = map[int]struct{}{}
				found = &ally
				allies[ally.Type()][ally.ID()] = found
			}
			found.PilotIDs[attacker.Pilot.CharacterID] = struct{}{}
			if !onThisKillmail[ally.Type()][ally.ID()] {
				onThisKillmail[ally.Type()][ally.ID()] = true
				found.Killmails++
				found.Activity = append(found.Activity, km.KillmailTime)
			}
		}
	}

	// unique days by ally, counted once rather than in every comparison
	ranked := []AllyActivity{}
	days := map[key]int{}
	for _, byID := range allies {
		for _, ally := range byID {
			ally.Killed = killed[key{ally.Type(), ally.ID()}]
			if skipKilled && ally.Killed > 0 {
				continue
			}
			ranked = append(ranked, *ally)
			days[key{ally.Type(), ally.ID()}] = ally.UniqueDays()
		}
	}
	slices.SortFunc(ranked, func(a, b AllyActivity) int {
		// every alliance before the corporations in none
		if a.Type() != b.Type() {
			if a.Type() == types.ALLIANCE {
				return -1
			}
			return 1
		}
		if byKillmails := cmp.Compare(b.Killmails, a.Killmails); byKillmails != 0 {
			return byKillmails
		}
		if byDays := cmp.Compare(days[key{b.Type(), b.ID()}], days[key{a.Type(), a.ID()}]); byDays != 0 {
			return byDays
		}
		if byPilots := cmp.Compare(len(b.PilotIDs), len(a.PilotIDs)); byPilots != 0 {
			return byPilots
		}
		return cmp.Compare(a.Name(), b.Name())
	})

	return ranked[:min(limit, len(ranked))]
}

/*
	targetGroupOnKill finds the target among a killmail's attackers, and the group it flew for there

Returns:
  - allianceID: the target's alliance on the kill, 0 when it was in none
  - corporationID: the target's corporation on the kill, the group left out when it was in no alliance
  - attacked: the target was one of the attackers, false on its losses and killmails it was not on
*/
func targetGroupOnKill(km *retrieval.Killmail, target *retrieval.IntelTarget) (allianceID int, corporationID int, attacked bool) {
	for _, attacker := range km.Attackers {
		corp, alliance := attacker.CorpAtKill, attacker.AllianceAtKill
		switch target.TypeOfEntity {
		case types.PILOT:
			attacked = attacker.Pilot != nil && !attacker.Pilot.NPC && attacker.Pilot.CharacterID == target.IDNumber
		case types.CORPORATION:
			attacked = corp != nil && corp.CorporationID == target.IDNumber
		case types.ALLIANCE:
			attacked = alliance != nil && alliance.AllianceID == target.IDNumber
		}
		if !attacked {
			continue
		}
		if alliance != nil {
			allianceID = alliance.AllianceID
		}
		if corp != nil {
			corporationID = corp.CorporationID
		}
		return allianceID, corporationID, true
	}
	return 0, 0, false
}
