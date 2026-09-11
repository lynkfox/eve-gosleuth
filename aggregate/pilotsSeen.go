package aggregate

import (
	"slices"
	"sort"

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

		if target.TypeOfEntity == types.PILOT && IsTarget(target, pilot.Pilot.CharacterID) {
			continue
		}

		sameGroupAsTarget := IsSameGroupAsTarget(target, *pilot.Pilot)

		// if the pilot seen is in the same Corp/Alliance as the target OR if they are otherwise seen as an attacker on the same killmails
		// indicating they are an out of corp related alt
		if sameGroupAsTarget || slices.Contains(target_seen_data_as_reference.OtherAttackerPilotIDs, pilot.Pilot.CharacterID) {
			pilotStats = append(pilotStats, pData{ID: id, UniqueDays: pilot.UniqueDays, TotalKillmails: pilot.TotalKillmails, Pilot: pilot})
		}
	}

	sort.Slice(pilotStats, func(i, j int) bool {
		if pilotStats[i].TotalKillmails == pilotStats[j].TotalKillmails {
			return pilotStats[i].UniqueDays > pilotStats[j].UniqueDays
		} else {
			return pilotStats[i].TotalKillmails > pilotStats[j].TotalKillmails
		}
	})

	var topPilots []PilotSeen
	for i := 0; i < limit && i < len(pilotStats); i++ {
		topPilots = append(topPilots, pilotStats[i].Pilot)
	}

	return topPilots
}

func (p *PilotSeen) CalculateDaysSeenOverTime() int {
	p.UniqueDays = len(p.DaysActive)
	return p.UniqueDays
}
