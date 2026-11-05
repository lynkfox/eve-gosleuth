package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCollectIntelWithAffiliates(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC()
	const main, affiliate = 99000001, 98000009
	setup := func(t *testing.T) (*App, *zkillRecorder) {
		app, zkill, _ := newCollectApp(t, map[int][]api.ZkbResponse{
			main:      {coalitionKillmail(10, now.Add(-time.Hour), 98000100, main, 98000777)},
			affiliate: {coalitionKillmail(20, now.Add(-2*time.Hour), affiliate, 0, 98000777)},
		})
		app.settings.Preferences.IncludeHoldings = false
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: main, MainName: "Main", HoldingType: "CORPORATION", HoldingID: affiliate, HoldingName: "Affiliate"})
		return app, zkill
	}

	t.Run("a plain search follows the limit, leaving the affiliate out", func(t *testing.T) {
		app, zkill := setup(t)
		report, err := app.CollectIntel(main, "ALLIANCE")
		assert.NoError(t, err)
		assert.Len(t, report.Killmails, 1)
		assert.NotContains(t, zkill.asked, affiliate)
	})

	t.Run("asked for as an affiliate group, the affiliate is listed", func(t *testing.T) {
		app, zkill := setup(t)
		report, err := app.CollectIntelWithAffiliates(main, "ALLIANCE")
		assert.NoError(t, err)
		assert.Len(t, report.Killmails, 2)
		assert.Contains(t, zkill.asked, affiliate)
		assert.False(t, app.searchPreferences().IncludeHoldings, "only for that search")
	})
}

func TestGetSearchGroups(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("nothing saved is an empty list", func(t *testing.T) {
		groups, err := newIntelApp(t).GetSearchGroups()
		assert.NoError(t, err)
		assert.Equal(t, []SearchGroup{}, groups)
	})

	t.Run("coalitions with members come first, then each main once with its count, each by name", func(t *testing.T) {
		app := newIntelApp(t)
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "CORPORATION", MainID: 2, MainName: "zebra corp", HoldingType: "CORPORATION", HoldingID: 21, HoldingName: "A"})
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 1, MainName: "Apple", HoldingType: "CORPORATION", HoldingID: 11, HoldingName: "A"})
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 1, MainName: "Apple", HoldingType: "ALLIANCE", HoldingID: 12, HoldingName: "B"})
		app.cache.SaveCoalition(retrieval.Coalition{Name: "No One Yet"})
		held := app.cache.SaveCoalition(retrieval.Coalition{Name: "The Pact", Members: []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: 5, Name: "E"}, {Type: "CORPORATION", ID: 6, Name: "F"}, {Type: "CORPORATION", ID: 7, Name: "G"}}})

		groups, err := app.GetSearchGroups()

		assert.NoError(t, err)
		assert.Equal(t, []SearchGroup{
			{Name: "The Pact", Type: "COALITION", ID: held.ID, Count: 3},
			{Name: "Apple", Type: "ALLIANCE", ID: 1, Count: 2},
			{Name: "zebra corp", Type: "CORPORATION", ID: 2, Count: 1},
		}, groups)
	})
}
