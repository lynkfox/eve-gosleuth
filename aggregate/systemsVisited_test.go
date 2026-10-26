package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/stretchr/testify/assert"
)

// visited builds a SystemVisted with a killmail at each of the given times
func visited(name string, times ...time.Time) aggregate.SystemVisted {
	return aggregate.SystemVisted{System: &retrieval.System{Name: name}, TotalKillmails: len(times), Activity: times}
}

func TestGetMostVisitedSystems(t *testing.T) {
	day := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)

	systems := map[int]aggregate.SystemVisted{
		1: visited("Jita", day),
		2: visited("J105443", day, day.Add(time.Hour), day.Add(-24*time.Hour)),
		3: visited("Tama", day, day.Add(time.Hour)),
		4: visited("Amamake", day, day.Add(-48*time.Hour)),
	}

	t.Run("most killmails first, ties broken by unique days, cut to the limit", func(t *testing.T) {
		top := aggregate.GetMostVisitedSystems(3, systems)

		names := []string{}
		for _, system := range top {
			names = append(names, system.System.Name)
		}
		assert.Equal(t, []string{"J105443", "Amamake", "Tama"}, names)
	})

	t.Run("a limit above the number of systems returns them all", func(t *testing.T) {
		assert.Len(t, aggregate.GetMostVisitedSystems(10, systems), 4)
	})

	t.Run("unique days and first and last seen come from the killmail times", func(t *testing.T) {
		system := systems[2]

		assert.Equal(t, 2, system.UniqueDays())
		assert.Equal(t, day.Add(-24*time.Hour), system.FirstSeen())
		assert.Equal(t, day.Add(time.Hour), system.LastSeen())
	})
}
