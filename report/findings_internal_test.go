package report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func findingsFixture() Input {
	var grid [7][24]int
	for d := 1; d <= 3; d++ {
		for h := 19; h <= 22; h++ {
			grid[d][h] = 20
		}
		grid[d][18] = 3
	}
	grid[5][3] = 2
	var weeks []Week
	for i := 0; i < 12; i++ {
		weeks = append(weeks, Week{Start: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7*i), Kills: 2 + i*2, Losses: i})
	}
	return Input{
		Doctrines: []Doctrine{{ShipName: "Ferox", Role: "Shield DPS"}, {ShipName: "Guardian", Role: "Logi"}, {ShipName: "Sabre"}},
		Capitals: []CapitalClass{
			{Name: "Titans", Pilots: []CapitalPilot{{Name: "A"}, {Name: "B"}}},
			{Name: "Dreadnoughts and Lancers", Pilots: []CapitalPilot{{Name: "A"}, {Name: "C"}}},
			{Name: "Freighters", Pilots: []CapitalPilot{{Name: "X"}, {Name: "Y"}, {Name: "Z"}}},
		},
		Activity: Activity{Grid: grid, Weeks: weeks, TimeZone: "EUTZ"},
	}
}

func TestKeyFindings(t *testing.T) {
	got := keyFindings(findingsFixture())
	assert.Equal(t, []string{
		"Flies 3 potential doctrines: Ferox (Shield DPS), Guardian (Logi), Sabre → Potential Fleet Doctrines",
		"3 capital pilots: 2 titans, 2 dreadnoughts and lancers → Capital Pilots",
		"Peak Tue to Thu, 19:00 to 23:00 EVE (EUTZ) → Activity",
		"Activity rising: 122 killmails in the last 4 weeks against 74 in the 4 before → Activity",
	}, got)
}

func TestKeyFindingsLeavesOutTrivial(t *testing.T) {
	in := Input{Capitals: []CapitalClass{{Name: "Freighters", Pilots: []CapitalPilot{{Name: "X"}}}}}
	assert.Empty(t, keyFindings(in))
}
