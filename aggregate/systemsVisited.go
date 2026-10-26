package aggregate

import (
	"slices"
	"sort"
	"time"
)

/*
	GetMostVisitedSystems ranks the systems the target's killmails happened in

Parameters:
  - limit: the maximum number of systems to return
  - systems: the SystemsVisted from CollectedData

Returns:
  - []SystemVisted: the top systems, most killmails first, ties broken by the most unique days active
*/
func GetMostVisitedSystems(limit int, systems map[int]SystemVisted) []SystemVisted {
	// unique days are counted once per system, rather than in every comparison
	type systemDays struct {
		system SystemVisted
		days   int
	}
	ranked := []systemDays{}
	for _, system := range systems {
		ranked = append(ranked, systemDays{system: system, days: system.UniqueDays()})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].system.TotalKillmails == ranked[j].system.TotalKillmails {
			return ranked[i].days > ranked[j].days
		}
		return ranked[i].system.TotalKillmails > ranked[j].system.TotalKillmails
	})

	top := []SystemVisted{}
	for _, entry := range ranked[:min(limit, len(ranked))] {
		top = append(top, entry.system)
	}
	return top
}

// UniqueDays returns how many different days (EVE time) the system had killmails on
func (s SystemVisted) UniqueDays() int {
	return uniqueDays(s.Activity)
}

// FirstSeen returns the time of the oldest killmail in the system
func (s SystemVisted) FirstSeen() time.Time {
	return slices.MinFunc(s.Activity, time.Time.Compare)
}

// LastSeen returns the time of the most recent killmail in the system
func (s SystemVisted) LastSeen() time.Time {
	return slices.MaxFunc(s.Activity, time.Time.Compare)
}

// uniqueDays returns how many different days (EVE time) the killmail times fall on
func uniqueDays(activity []time.Time) int {
	days := map[string]struct{}{}
	for _, killmailTime := range activity {
		days[DayActive(killmailTime)] = struct{}{}
	}
	return len(days)
}
