package aggregate

import (
	"slices"
	"sort"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

type PilotSeen struct {
	Pilot                 *retrieval.Pilot     `json:"pilot"`
	ShipsFlown            map[int]ShipFlown    `json:"-"`
	SystemsVisited        map[int]SystemVisted `json:"-"`
	TotalKillmails        int                  `json:"total_killmails"`
	TimesAttacker         int                  `json:"kms_as_attacker"`
	TimesVictim           int                  `json:"kms_as_victim"`
	AttackerKillmailIDs   []int                `json:"attacker_killmail_ids"`
	OtherAttackerPilotIDs []int                `json:"other_atacker_ids"`
	VictimKillmailIDs     []int                `json:"victim_killmail_ids"`
	KillerPilotIDs        []int                `json:"attacked_by_ids"`
	HoursActive           map[int]int          `json:"hours_active"`
	DaysActive            []string             `json:"days_active"`
	BattlesFoughtIDs      map[string]struct{}  `json:"-"`
	UniqueDays            int                  `json:"unique_days"`
	CorporationsAtKill    map[int]struct{}     `json:"-"`          // every corp the pilot was in on the collected killmails
	AlliancesAtKill       map[int]struct{}     `json:"-"`          // every alliance the pilot was in on the collected killmails
	FirstSeen             time.Time            `json:"first_seen"` // time of the oldest killmail this pilot is on
	LastSeen              time.Time            `json:"last_seen"`  // time of the most recent killmail this pilot is on
}

func GetMostFrequentFleetmates(limit int, pilots map[int]PilotSeen, target *retrieval.IntelTarget) []PilotSeen {
	type pData struct {
		ID             int
		UniqueDays     int
		TotalKillmails int
		Pilot          PilotSeen
	}
	pilotStats := []pData{}

	var target_seen_data_as_reference PilotSeen

	if target.TypeOfEntity == types.PILOT {
		target_seen_data_as_reference = pilots[target.IDNumber]
	}

	for id, pilot := range pilots {

		// NPC and structure attackers are only ship types, not fleetmates
		if pilot.Pilot.NPC || (target.TypeOfEntity == types.PILOT && IsTarget(target, pilot.Pilot.CharacterID)) {
			continue
		}

		sameGroupAsTarget := IsSameGroupAsTarget(target, *pilot.Pilot)

		// if the pilot seen is in the same Corp/Alliance as the target OR if they are otherwise seen as an attacker on the same killmails
		// indicating they are an out of corp related alt
		if sameGroupAsTarget || slices.Contains(target_seen_data_as_reference.OtherAttackerPilotIDs, pilot.Pilot.CharacterID) {
			pilotStats = append(pilotStats, pData{ID: id, UniqueDays: pilot.UniqueDays, TotalKillmails: pilot.TotalKillmails, Pilot: pilot})
		}
	}

	candidates := []PilotSeen{}
	for _, stat := range pilotStats {
		candidates = append(candidates, stat.Pilot)
	}

	return rankPilots(limit, candidates)
}

/*
	GetTopMembers ranks the pilots who flew for a corporation or alliance target on its killmails

A pilot counts as a member if they were in the target corp or alliance on at least one collected killmail, so pilots who have since left are included.

Parameters:
  - limit: the maximum number of members to return
  - pilots: the PilotsSeen from CollectedData
  - target: a CORPORATION or ALLIANCE target, other types have no members

Returns:
  - []PilotSeen: the top members, most killmails first, ties broken by the most unique days active
*/
func GetTopMembers(limit int, pilots map[int]PilotSeen, target *retrieval.IntelTarget) []PilotSeen {
	members := []PilotSeen{}
	for _, pilot := range pilots {
		if pilot.FlewFor(target) {
			members = append(members, pilot)
		}
	}

	return rankPilots(limit, members)
}

/*
	GetTopPilots ranks every pilot on the killmails, such as for a system target. NPC and structure attackers are left out.

Parameters:
  - limit: the maximum number of pilots to return
  - pilots: the PilotsSeen from CollectedData

Returns:
  - []PilotSeen: the top pilots, most killmails first, ties broken by the most unique days active
*/
func GetTopPilots(limit int, pilots map[int]PilotSeen) []PilotSeen {
	players := []PilotSeen{}
	for _, pilot := range pilots {
		if !pilot.Pilot.NPC {
			players = append(players, pilot)
		}
	}

	return rankPilots(limit, players)
}

// FlewFor returns true if the pilot was in the target corporation or alliance, or one of its holdings, on any collected killmail
func (p PilotSeen) FlewFor(target *retrieval.IntelTarget) bool {
	for corporationID := range p.CorporationsAtKill {
		if target.IsGroup(corporationID, 0) {
			return true
		}
	}
	for allianceID := range p.AlliancesAtKill {
		if target.IsGroup(0, allianceID) {
			return true
		}
	}
	return false
}

// rankPilots sorts pilots by most killmails, ties broken by most unique days, and returns up to limit of them
func rankPilots(limit int, pilots []PilotSeen) []PilotSeen {
	sort.Slice(pilots, func(i, j int) bool {
		if pilots[i].TotalKillmails == pilots[j].TotalKillmails {
			return pilots[i].UniqueDays > pilots[j].UniqueDays
		}
		return pilots[i].TotalKillmails > pilots[j].TotalKillmails
	})

	return pilots[:min(limit, len(pilots))]
}

func (p *PilotSeen) CalculateDaysSeenOverTime() int {
	p.UniqueDays = len(p.DaysActive)
	return p.UniqueDays
}
