package gui

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHoldingMain(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	app := func(includeHoldings bool) *App {
		cache := retrieval.NewCache(api.NewESIClient(), filepath.Join(t.TempDir(), "cache.db"))
		settings := defaultSettings()
		settings.IncludeHoldings = includeHoldings
		a := &App{cache: cache, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: settings}
		for _, main := range []string{"Zeta Alliance", "Alpha Alliance"} {
			cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: len(main), MainName: main, HoldingType: "CORPORATION", HoldingID: 98000002, HoldingName: "Holding Corp"})
		}
		return a
	}
	holding := &retrieval.IntelTarget{IDNumber: 98000002, TypeOfEntity: types.CORPORATION}

	t.Run("a holding is searched as its main, the first by name when it has several", func(t *testing.T) {
		main, found := app(true).holdingMain(holding)

		assert.True(t, found)
		assert.Equal(t, "Alpha Alliance", main.MainName)
	})

	t.Run("not while the Include holdings preference is off", func(t *testing.T) {
		_, found := app(false).holdingMain(holding)
		assert.False(t, found)
	})

	t.Run("a corp that holds for none, or a pilot, is searched itself", func(t *testing.T) {
		a := app(true)
		_, found := a.holdingMain(&retrieval.IntelTarget{IDNumber: 98000009, TypeOfEntity: types.CORPORATION})
		assert.False(t, found)
		_, found = a.holdingMain(&retrieval.IntelTarget{IDNumber: 98000002, TypeOfEntity: types.PILOT})
		assert.False(t, found)
	})
}

func TestTogetherURL(t *testing.T) {
	t.Run("an ally's one search has the target and every holding searched with it", func(t *testing.T) {
		target := &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE,
			Holdings: []retrieval.TargetGroup{{Type: types.CORPORATION, ID: 98000009, Name: "Holding Inc."}, {Type: types.ALLIANCE, ID: 99013580, Name: "High Bear Nation"}}}

		groups := []api.ZkillEntity{{Type: types.ALLIANCE, ID: 99013330}, {Type: types.CORPORATION, ID: 98000009}, {Type: types.ALLIANCE, ID: 99013580}}
		assert.Equal(t, api.ZkillTogetherURL(groups, api.ZkillEntity{Type: types.ALLIANCE, ID: 99014851}), togetherURL(types.ALLIANCE, 99014851, target))
	})
}

func TestCoalitionsFollowAffiliate(t *testing.T) {
	app, _, _ := newCollectApp(t, nil)
	estates := retrieval.CoalitionMember{Type: "ALLIANCE", ID: 700, Name: "Estates"}
	other := retrieval.CoalitionMember{Type: "ALLIANCE", ID: 800, Name: "Other"}
	ldev := retrieval.CoalitionMember{Type: "ALLIANCE", ID: 900, Name: "LDEV"}
	link := retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 900, MainName: "LDEV", HoldingType: "ALLIANCE", HoldingID: 700, HoldingName: "Estates"}
	has := app.cache.SaveCoalition(retrieval.Coalition{Name: "Has it", Members: []retrieval.CoalitionMember{estates, other}})
	both := app.cache.SaveCoalition(retrieval.Coalition{Name: "Has both", Members: []retrieval.CoalitionMember{estates, ldev}})
	without := app.cache.SaveCoalition(retrieval.Coalition{Name: "Without", Members: []retrieval.CoalitionMember{other}})

	app.cache.SaveHoldingLink(link)
	assert.ElementsMatch(t, []string{"Has it", "Has both"}, app.coalitionsFollowAffiliate(link))

	saved, _ := app.cache.CoalitionByID(has.ID)
	assert.ElementsMatch(t, []retrieval.CoalitionMember{other, ldev}, saved.Members, "its main takes its place")
	card := app.coalitionCard(saved)
	if assert.Len(t, card.Affiliates, 1) {
		assert.Equal(t, "Estates", card.Affiliates[0].Name)
	}

	saved, _ = app.cache.CoalitionByID(both.ID)
	assert.Equal(t, []retrieval.CoalitionMember{ldev}, saved.Members, "a main already there is not added twice")

	saved, _ = app.cache.CoalitionByID(without.ID)
	assert.Equal(t, []retrieval.CoalitionMember{other}, saved.Members)

	assert.Empty(t, app.coalitionsFollowAffiliate(link), "nothing left to move")

	t.Run("an import moves a member of a coalition the same file adds", func(t *testing.T) {
		app, _, _ := newCollectApp(t, nil)
		app.applyImport(importPlan{
			newHoldings: []retrieval.HoldingLink{link},
			coalitions:  []plannedCoalition{{name: "From the file", members: []retrieval.CoalitionMember{estates}}},
		}, false)
		saved, found := app.cache.CoalitionNamed("From the file")
		assert.True(t, found)
		assert.Equal(t, []retrieval.CoalitionMember{ldev}, saved.Members)
	})

	t.Run("saving a link only moves a member when the link is new, so a note edit leaves one added back by hand", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "Hand made", [2]string{"URSA PRIMUS", "CORPORATION"})

		holdings, err := app.SaveHolding(99013330, "ALLIANCE", "URSA.", "URSA PRIMUS", "CORPORATION", "")
		assert.NoError(t, err)
		assert.Equal(t, []string{"Hand made"}, holdings.CoalitionsMoved)
		cards, _ := app.GetCoalitions()
		assert.Equal(t, []string{"URSA."}, memberNames(cards[0].Members))
		assert.Equal(t, []string{"URSA PRIMUS"}, memberNames(cards[0].Affiliates))

		_, err = app.AddCoalitionMember(card.ID, "URSA PRIMUS", "CORPORATION")
		assert.NoError(t, err)
		holdings, err = app.SaveHolding(99013330, "ALLIANCE", "URSA.", "URSA PRIMUS", "CORPORATION", "a new note")
		assert.NoError(t, err)
		assert.Empty(t, holdings.CoalitionsMoved)
		cards, _ = app.GetCoalitions()
		assert.Contains(t, memberNames(cards[0].Members), "URSA PRIMUS")
	})
}
