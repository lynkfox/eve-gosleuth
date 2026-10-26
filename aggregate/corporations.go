package aggregate

import (
	"slices"
	"sort"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// CorporationActivity is a corporation's part in a set of killmails, counted from the corp each participant was in at the time of the kill
type CorporationActivity struct {
	Corporation *retrieval.Corporation
	Alliance    *retrieval.Alliance // the corp's alliance on its most recent killmail, nil if it had none
	Killmails   int
	Kills       int // killmails with at least one attacker from the corp
	Losses      int // killmails where the victim was from the corp
	PilotIDs    map[int]struct{}
	Activity    []time.Time // one time per killmail
}

// UniqueDays returns how many different days (EVE time) the corp was on killmails
func (c CorporationActivity) UniqueDays() int {
	return uniqueDays(c.Activity)
}

// FirstSeen returns the time of the corp's oldest killmail
func (c CorporationActivity) FirstSeen() time.Time {
	return slices.MinFunc(c.Activity, time.Time.Compare)
}

// LastSeen returns the time of the corp's most recent killmail
func (c CorporationActivity) LastSeen() time.Time {
	return slices.MaxFunc(c.Activity, time.Time.Compare)
}

/*
	GetTopCorporations ranks the corporations on a set of killmails, such as for a system target.

Each killmail counts once per corp, as a kill if any attacker was from the corp and as a loss if the victim was. NPC attackers (rats) are left out, structures count for their owning corp.

Parameters:
  - limit: the maximum number of corporations to return
  - killmails: the expanded killmails from CollectedData

Returns:
  - []CorporationActivity: the top corporations, most killmails first, ties broken by the most unique days, then the most pilots
*/
func GetTopCorporations(limit int, killmails []*retrieval.Killmail) []CorporationActivity {
	corporations := map[int]*CorporationActivity{}
	allianceSeenAt := map[int]time.Time{}

	for _, km := range killmails {
		// a corp can be on both sides of one killmail (an awox), so its roles are collected before counting
		type roles struct{ attacked, lost bool }
		onThisKillmail := map[int]*roles{}

		record := func(pilot *retrieval.Pilot, corp *retrieval.Corporation, alliance *retrieval.Alliance, asVictim bool) {
			if corp == nil || (pilot != nil && pilot.NPC && api.IsNPCCorporation(corp.CorporationID)) {
				return
			}

			activity, found := corporations[corp.CorporationID]
			if !found {
				activity = &CorporationActivity{Corporation: corp, PilotIDs: map[int]struct{}{}}
				corporations[corp.CorporationID] = activity
			}

			role, counted := onThisKillmail[corp.CorporationID]
			if !counted {
				role = &roles{}
				onThisKillmail[corp.CorporationID] = role
				activity.Killmails++
				activity.Activity = append(activity.Activity, km.KillmailTime)
			}
			if asVictim {
				role.lost = true
			} else {
				role.attacked = true
			}

			if pilot != nil && !pilot.NPC {
				activity.PilotIDs[pilot.CharacterID] = struct{}{}
			}

			// killmails are not collected in time order, the alliance is taken from the corp's most recent one
			if !km.KillmailTime.Before(allianceSeenAt[corp.CorporationID]) {
				activity.Alliance = alliance
				allianceSeenAt[corp.CorporationID] = km.KillmailTime
			}
		}

		record(km.Victim.Pilot, km.Victim.CorpAtKill, km.Victim.AllianceAtKill, true)
		for _, attacker := range km.Attackers {
			record(attacker.Pilot, attacker.CorpAtKill, attacker.AllianceAtKill, false)
		}

		for corporationID, role := range onThisKillmail {
			if role.attacked {
				corporations[corporationID].Kills++
			}
			if role.lost {
				corporations[corporationID].Losses++
			}
		}
	}

	ranked := []CorporationActivity{}
	days := map[int]int{} // unique days by corp id, counted once rather than in every comparison
	for _, activity := range corporations {
		ranked = append(ranked, *activity)
		days[activity.Corporation.CorporationID] = activity.UniqueDays()
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Killmails != ranked[j].Killmails {
			return ranked[i].Killmails > ranked[j].Killmails
		}
		if days[ranked[i].Corporation.CorporationID] != days[ranked[j].Corporation.CorporationID] {
			return days[ranked[i].Corporation.CorporationID] > days[ranked[j].Corporation.CorporationID]
		}
		return len(ranked[i].PilotIDs) > len(ranked[j].PilotIDs)
	})

	return ranked[:min(limit, len(ranked))]
}
