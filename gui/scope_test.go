package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/stretchr/testify/assert"
)

func TestBuildScope(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, time.October, d, 12, 0, 0, 0, time.UTC) }
	limits := SearchLimits{MaxKillmails: 500, KillmailsNewerThanDays: 30}
	cards := []KillmailCard{{Time: day(5)}, {Time: day(2)}, {Time: day(9), Via: "Holding A"}, {Time: day(1), Via: "Gone Holding"}}

	t.Run("the main then each affiliate, by the list each killmail came from", func(t *testing.T) {
		target := &retrieval.IntelTarget{TypeOfEntity: types.CORPORATION, Expanded: &retrieval.Corporation{Name: "Main Corp"},
			Holdings: []retrieval.TargetGroup{{Type: types.CORPORATION, ID: 2, Name: "Holding A"}, {Type: types.CORPORATION, ID: 3, Name: "Holding B"}}}
		scope := buildScope(target, cards, limits)
		assert.Equal(t, []string{"At most 500 killmails", "Killmails newer than 30 days"}, scope.Limits)
		assert.Equal(t, []ScopeSource{
			{Name: "Main Corp", Main: true, Killmails: 2, Oldest: day(2), Newest: day(5)},
			{Name: "Holding A", Killmails: 1, Oldest: day(9), Newest: day(9)},
			{Name: "Holding B"},
		}, scope.Sources)
	})

	t.Run("no sources without affiliates", func(t *testing.T) {
		scope := buildScope(&retrieval.IntelTarget{TypeOfEntity: types.CORPORATION}, cards, limits)
		assert.Empty(t, scope.Sources)
		assert.NotNil(t, scope.Sources, "an empty list, not null, for the frontend")
	})
}

func TestScopeLimits(t *testing.T) {
	assert.Equal(t, []string{"At most 20,000 killmails", "Killmails of any age"}, scopeLimits(SearchLimits{MaxKillmails: 20000}))
	assert.Equal(t, []string{"At most 500 killmails", "Killmails newer than 30 days"},
		scopeLimits(SearchLimits{MaxKillmails: 500, KillmailsNewerThanDays: 30, UseCachedKillmails: true, IncludeHoldings: true}), "no cache or affiliates line")
	assert.Equal(t, []string{"All killmails on 10 zKillboard pages", "Killmails newer than 1 week"}, scopeLimits(SearchLimits{MaxKillmails: -1, KillmailPages: 10, KillmailsNewerThanDays: 7}))
	assert.Equal(t, "All killmails on 1 zKillboard page", scopeLimits(SearchLimits{MaxKillmails: -1, KillmailPages: 1})[0])
}
