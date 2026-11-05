package gui

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	warAlpha = 99000001 // an alliance, side A
	warBeta  = 98000002 // a corporation, side B
	warGamma = 98000555 // a corporation on neither side
)

// pagingRecorder lists killmails by entity like zkillRecorder, and keeps how far each list was asked to read
type pagingRecorder struct {
	mu     sync.Mutex
	lists  map[int][]api.ZkbResponse
	paging []api.ZkillPaging
}

func (p *pagingRecorder) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paging = append(p.paging, paging)
	return api.ZkillList{Killmails: p.lists[entityId], Ended: true}, nil
}

func (p *pagingRecorder) newerThan() []time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	times := []time.Time{}
	for _, paging := range p.paging {
		times = append(times, paging.NewerThan)
	}
	return times
}

func (p *pagingRecorder) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paging = nil
}

// warApp is an app with the zKillboard recorder in place of newCollectApp's
func warApp(t *testing.T, lists map[int][]api.ZkbResponse) (*App, *pagingRecorder) {
	app, _, _ := newCollectApp(t, nil)
	zkill := &pagingRecorder{lists: lists}
	app.clients.ZkillClient = zkill
	return app, zkill
}

func alphaSide() WarSideCard { return WarSideCard{Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"} }
func betaSide() WarSideCard  { return WarSideCard{Type: "CORPORATION", ID: warBeta, Name: "Beta"} }

// savedWar saves Alpha against Beta from the given date
func savedWar(t *testing.T, app *App, from string) WarCard {
	card, err := app.SaveWar(WarCard{SideA: alphaSide(), SideB: betaSide(), FromDate: from})
	assert.NoError(t, err)
	return card
}

// fightLists are two killmails of Alpha and Beta fighting
func fightLists(at time.Time) map[int][]api.ZkbResponse {
	return map[int][]api.ZkbResponse{
		warAlpha: {coalitionKillmail(10, at.Add(-time.Hour), 98000100, warAlpha, warBeta)},
		warBeta:  {coalitionKillmail(11, at.Add(-55*time.Minute), 98000100, warAlpha, warBeta)},
	}
}

func killmailIDs(report WarReport) []int {
	ids := []int{}
	for _, km := range report.Killmails {
		ids = append(ids, km.ID)
	}
	return ids
}

func TestSaveWar(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)

	t.Run("an empty name becomes A vs B, and the sides keep their names", func(t *testing.T) {
		app, _ := warApp(t, nil)
		card, err := app.SaveWar(WarCard{SideA: alphaSide(), SideB: betaSide(), AddedA: []WarAddedCard{{Type: "ALLIANCE", ID: 1, Name: "Ignored"}}})

		assert.NoError(t, err)
		assert.Less(t, card.ID, 0)
		assert.Equal(t, "Alpha vs Beta", card.Name)
		assert.Equal(t, 1, card.SideA.Count)
		assert.Equal(t, 1, card.SideB.Count)
		assert.Equal(t, []WarAddedCard{}, card.AddedA, "added groups change only through AddWarGroup")
		cards, _ := app.GetWars()
		assert.Len(t, cards, 1)
	})

	t.Run("every refusal saves nothing", func(t *testing.T) {
		app, _ := warApp(t, nil)
		app.cache.SaveCoalition(retrieval.Coalition{Name: "Gone Soon", Members: []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"}}})
		coalition := app.cache.Coalitions()[0]
		overlap := app.cache.SaveCoalition(retrieval.Coalition{Name: "Overlap", Members: []retrieval.CoalitionMember{{Type: "CORPORATION", ID: warBeta, Name: "Beta"}, {Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"}}})
		_, err := app.SaveWar(WarCard{Name: "Taken", SideA: alphaSide(), SideB: betaSide()})
		assert.NoError(t, err)

		coalitionSide := func(c retrieval.Coalition) WarSideCard { return WarSideCard{Type: "COALITION", ID: c.ID, Name: c.Name} }
		tests := map[string]WarCard{
			"no side A":            {SideB: betaSide()},
			"no side B":            {SideA: alphaSide()},
			"a pilot":              {SideA: WarSideCard{Type: "PILOT", ID: 2121139046, Name: "Luxxianna"}, SideB: betaSide()},
			"a system":             {SideA: WarSideCard{Type: "SYSTEM", ID: 30000142, Name: "Jita"}, SideB: betaSide()},
			"a coalition not kept": {SideA: WarSideCard{Type: "COALITION", ID: -999, Name: "Nowhere"}, SideB: betaSide()},
			"the same entity":      {SideA: alphaSide(), SideB: alphaSide()},
			"a member direct on B": {SideA: coalitionSide(coalition), SideB: alphaSide()},
			"the same coalition":   {SideA: coalitionSide(coalition), SideB: coalitionSide(coalition)},
			"coalitions sharing":   {SideA: coalitionSide(coalition), SideB: coalitionSide(overlap)},
			"a taken name":         {Name: " TAKEN ", SideA: alphaSide(), SideB: betaSide()},
			"a long name":          {Name: strings.Repeat("x", maxCoalitionName+1), SideA: alphaSide(), SideB: betaSide()},
			"until before from":    {SideA: alphaSide(), SideB: betaSide(), FromDate: "2026-05-02", UntilDate: "2026-05-01"},
			"an unreadable from":   {SideA: alphaSide(), SideB: betaSide(), FromDate: "May 1st"},
			"an unreadable until":  {SideA: alphaSide(), SideB: betaSide(), UntilDate: "2026-13-40"},
			"a war that is gone":   {ID: -42, SideA: alphaSide(), SideB: betaSide()},
		}
		for name, card := range tests {
			_, err := app.SaveWar(card)
			assert.Error(t, err, name)
			if len(err.Error()) > 0 {
				assert.Equal(t, strings.ToUpper(err.Error()[:1]), err.Error()[:1], name+": a capitalized sentence")
			}
		}
		cards, _ := app.GetWars()
		assert.Len(t, cards, 1, "only the first was saved")
	})

	t.Run("a rolling start becomes a fixed date", func(t *testing.T) {
		app, _ := warApp(t, nil)
		card, err := app.SaveWar(WarCard{SideA: alphaSide(), SideB: betaSide(), FromDays: 30, FromDate: "2020-01-01"})

		assert.NoError(t, err)
		assert.Equal(t, time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02"), card.FromDate)
		assert.Equal(t, 0, card.FromDays)
		fixed, err := app.SaveWar(WarCard{ID: card.ID, SideA: alphaSide(), SideB: betaSide(), FromDate: card.FromDate, UntilDate: card.FromDate})
		assert.NoError(t, err)
		assert.Equal(t, card.FromDate, fixed.FromDate)
		assert.Equal(t, card.FromDate, fixed.UntilDate, "a one day war")
	})

	t.Run("changing a war keeps what it holds, and a rename moves its recent search", func(t *testing.T) {
		app, _ := warApp(t, nil)
		card := savedWar(t, app, "")
		war, _ := app.cache.WarByID(card.ID)
		run := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		war.AddedA = []retrieval.WarAddedGroup{{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}}
		war.AddedB = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: 5, Name: "Five", FromDate: "2026-05-02"}}
		war.LastRun, war.NewestKillmail, war.RunKey = run, run, "key"
		war.Reach = []retrieval.WarReach{{Group: "Alpha", Oldest: run}}
		app.cache.SaveWar(war)
		app.cache.AddRecentSearch(retrieval.RecentSearch{Name: war.Name, Type: warType, ID: war.ID}, 10)

		card.Name = "The Big One"
		saved, err := app.SaveWar(card)

		assert.NoError(t, err)
		after, _ := app.cache.WarByID(card.ID)
		assert.Equal(t, "The Big One", saved.Name)
		assert.Equal(t, war.AddedA, after.AddedA)
		assert.Equal(t, war.AddedB, after.AddedB)
		assert.Equal(t, run, after.LastRun)
		assert.Equal(t, run, after.NewestKillmail)
		assert.Equal(t, "key", after.RunKey)
		assert.Equal(t, war.Reach, after.Reach)
		recent, _ := app.GetRecentSearches()
		assert.Equal(t, []retrieval.RecentSearch{{Name: "The Big One", Type: warType, ID: card.ID}}, recent)

		_, err = app.SaveWar(WarCard{ID: card.ID, Name: "the big one", SideA: alphaSide(), SideB: betaSide()})
		assert.NoError(t, err, "its own name in another case")
	})

	t.Run("a removed war takes its killmails and recent search with it", func(t *testing.T) {
		app, _ := warApp(t, nil)
		card := savedWar(t, app, "")
		app.cache.AddRecentSearch(retrieval.RecentSearch{Name: card.Name, Type: warType, ID: card.ID}, 10)
		app.cache.AddRecentSearch(retrieval.RecentSearch{Name: "Other", Type: "ALLIANCE", ID: 3}, 10)

		assert.NoError(t, app.RemoveWar(card.ID))
		assert.NoError(t, app.RemoveWar(card.ID), "twice changes nothing")

		cards, _ := app.GetWars()
		assert.Empty(t, cards)
		recent, _ := app.GetRecentSearches()
		assert.Equal(t, []retrieval.RecentSearch{{Name: "Other", Type: "ALLIANCE", ID: 3}}, recent)
		assert.Equal(t, 0, app.cache.WarKillmailCount(card.ID))
	})
}

func TestOpenWar(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)

	t.Run("a war with no run has an empty report with no null lists, due for a full run", func(t *testing.T) {
		app, zkill := warApp(t, nil)
		card := savedWar(t, app, "")
		app.lastTarget = &TargetResult{ID: 1}
		app.lastReport = &IntelReport{}

		report, err := app.OpenWar(card.ID)

		assert.NoError(t, err)
		assert.True(t, report.FullRunDue)
		assert.Equal(t, card.ID, report.War.ID)
		assert.Nil(t, app.lastTarget)
		assert.Nil(t, app.lastReport)
		assert.Empty(t, zkill.newerThan(), "opening asks zKillboard for nothing")
		recent, _ := app.GetRecentSearches()
		assert.Equal(t, []retrieval.RecentSearch{{Name: "Alpha vs Beta", Type: warType, ID: card.ID}}, recent)

		data, _ := json.Marshal(report)
		for _, key := range []string{"battles", "classes", "systems", "killmails", "labelNames", "otherGroups", "reach"} {
			assert.NotContains(t, string(data), `"`+key+`":null`, key)
		}
		assert.Contains(t, string(data), `"citadels":[]`)
	})

	t.Run("a coalition that is gone stops it, naming the coalition", func(t *testing.T) {
		app, _ := warApp(t, nil)
		coalition := app.cache.SaveCoalition(retrieval.Coalition{Name: "Vanishing", Members: []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"}}})
		card, err := app.SaveWar(WarCard{SideA: WarSideCard{Type: "COALITION", ID: coalition.ID, Name: coalition.Name}, SideB: betaSide()})
		assert.NoError(t, err)
		app.cache.RemoveCoalition(coalition.ID)

		_, openErr := app.OpenWar(card.ID)
		_, collectErr := app.CollectWar(card.ID)
		_, addErr := app.AddWarGroup(card.ID, "a", WarAddedCard{Type: "CORPORATION", ID: warGamma, Name: "Gamma"})
		_, removeErr := app.RemoveWarGroup(card.ID, "CORPORATION", warGamma)

		for _, err := range []error{openErr, collectErr, addErr, removeErr} {
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "Vanishing")
			}
		}
		_, err = app.OpenWar(-77)
		assert.ErrorIs(t, err, errNoWar)
	})
}

func TestCollectWar(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("the first run reads in full from the start, then an update only from the newest killmail less 5 days", func(t *testing.T) {
		app, zkill := warApp(t, fightLists(now))
		card := savedWar(t, app, "2026-03-01")
		start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

		report, err := app.CollectWar(card.ID)

		assert.NoError(t, err)
		assert.Equal(t, []time.Time{start, start}, zkill.newerThan())
		assert.ElementsMatch(t, []int{10, 11}, killmailIDs(report))
		assert.False(t, report.FullRunDue)
		assert.Equal(t, 2, report.War.Killmails)
		assert.False(t, report.War.LastRun.IsZero())
		for _, km := range report.Killmails {
			assert.Contains(t, []string{"a", "b", "others"}, km.Side)
		}
		war, _ := app.cache.WarByID(card.ID)
		assert.Equal(t, now.Add(-55*time.Minute), war.NewestKillmail)
		sides, err := app.warSides(war)
		assert.NoError(t, err)
		assert.Equal(t, war.RunKeyFor(sides.A, sides.B), war.RunKey)
		assert.Len(t, war.Reach, 2, "a list per side")

		zkill.reset()
		report, err = app.CollectWar(card.ID)
		assert.NoError(t, err)
		want := now.Add(-55 * time.Minute).Add(-5 * 24 * time.Hour)
		assert.Equal(t, []time.Time{want, want}, zkill.newerThan())
		assert.Len(t, report.Killmails, 2)

		zkill.reset()
		_, err = app.SaveWar(WarCard{ID: card.ID, SideA: alphaSide(), SideB: WarSideCard{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}, FromDate: "2026-03-01"})
		assert.NoError(t, err)
		_, err = app.CollectWar(card.ID)
		assert.NoError(t, err)
		assert.Equal(t, []time.Time{start, start}, zkill.newerThan(), "a side changed, so every list is read again from the start")
	})

	t.Run("an added group makes the next run a full one", func(t *testing.T) {
		app, zkill := warApp(t, fightLists(now))
		card := savedWar(t, app, "")
		_, err := app.CollectWar(card.ID)
		assert.NoError(t, err)

		report, err := app.AddWarGroup(card.ID, "a", WarAddedCard{Type: "CORPORATION", ID: warGamma, Name: "Gamma"})
		assert.NoError(t, err)
		assert.True(t, report.FullRunDue)
		assert.Equal(t, []WarAddedCard{{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}}, report.War.AddedA)

		zkill.reset()
		_, err = app.CollectWar(card.ID)
		assert.NoError(t, err)
		assert.Equal(t, []time.Time{{}, {}, {}}, zkill.newerThan(), "three lists, the added group's too, all from the beginning")
	})

	t.Run("a held killmail stays in the report when the list no longer reaches it", func(t *testing.T) {
		app, zkill := warApp(t, fightLists(now))
		card := savedWar(t, app, "")
		_, err := app.CollectWar(card.ID)
		assert.NoError(t, err)

		zkill.lists = map[int][]api.ZkbResponse{warBeta: {coalitionKillmail(12, now.Add(-30*time.Minute), 98000100, warAlpha, warBeta)}}
		report, err := app.CollectWar(card.ID)

		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{10, 11, 12}, killmailIDs(report))
		opened, err := app.OpenWar(card.ID)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{10, 11, 12}, killmailIDs(opened), "opening reads the same from what it holds")
	})

	t.Run("killmails outside its dates are left out", func(t *testing.T) {
		lists := fightLists(now)
		app, _ := warApp(t, lists)
		card, err := app.SaveWar(WarCard{SideA: alphaSide(), SideB: betaSide(), UntilDate: now.Add(-90 * time.Minute).Format("2006-01-02")})
		assert.NoError(t, err)

		report, err := app.CollectWar(card.ID)

		assert.NoError(t, err)
		if now.Add(-time.Hour).Format("2006-01-02") != now.Add(-90*time.Minute).Format("2006-01-02") {
			assert.Empty(t, report.Killmails)
		}
	})

	t.Run("an earlier Stop does not end a new run", func(t *testing.T) {
		app, _ := warApp(t, fightLists(now))
		card := savedWar(t, app, "")
		app.StopSearch()

		report, err := app.CollectWar(card.ID)

		assert.NoError(t, err)
		assert.False(t, report.Stopped)
		assert.ElementsMatch(t, []int{10, 11}, killmailIDs(report))
	})

	t.Run("extending the end date reads what the earlier end left out", func(t *testing.T) {
		app, zkill := warApp(t, fightLists(now))
		card, err := app.SaveWar(WarCard{SideA: alphaSide(), SideB: betaSide(), UntilDate: now.AddDate(0, 0, -3).Format("2006-01-02")})
		assert.NoError(t, err)

		report, err := app.CollectWar(card.ID)
		assert.NoError(t, err)
		assert.Empty(t, report.Killmails)
		war, _ := app.cache.WarByID(card.ID)
		assert.True(t, war.NewestKillmail.IsZero(), "killmails past the end date do not move it")

		zkill.reset()
		card.UntilDate = ""
		_, err = app.SaveWar(card)
		assert.NoError(t, err)
		report, err = app.CollectWar(card.ID)

		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{10, 11}, killmailIDs(report))
		assert.Equal(t, []time.Time{war.Start(), war.Start()}, zkill.newerThan(), "it lists from where it was")
	})
}

func TestIgnoreWarGroup(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	app, _ := warApp(t, nil)
	card := savedWar(t, app, "")
	gamma := WarAddedCard{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}

	report, err := app.IgnoreWarGroup(card.ID, gamma)
	assert.NoError(t, err)
	assert.Equal(t, []WarAddedCard{{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}}, report.War.Ignored)

	report, err = app.IgnoreWarGroup(card.ID, gamma)
	assert.NoError(t, err)
	assert.Len(t, report.War.Ignored, 1, "ignoring it again changes nothing")

	card.Name = "Renamed"
	saved, err := app.SaveWar(card)
	assert.NoError(t, err)
	assert.Len(t, saved.Ignored, 1, "saving the war's setup keeps it")

	_, err = app.IgnoreWarGroup(card.ID, WarAddedCard{Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"})
	assert.EqualError(t, err, "Alpha is already on a side")

	report, err = app.AddWarGroup(card.ID, "a", gamma)
	assert.NoError(t, err)
	assert.Empty(t, report.War.Ignored, "adding it to a side stops ignoring it")
	assert.Len(t, report.War.AddedA, 1)
	_, err = app.RemoveWarGroup(card.ID, gamma.Type, gamma.ID)
	assert.NoError(t, err)

	_, err = app.IgnoreWarGroup(card.ID, gamma)
	assert.NoError(t, err)
	report, err = app.RemoveWarGroup(card.ID, gamma.Type, gamma.ID)
	assert.NoError(t, err)
	assert.Empty(t, report.War.Ignored, "the x takes it back")

	_, err = app.IgnoreWarGroup(card.ID, gamma)
	assert.NoError(t, err)
	card.SideB = WarSideCard{Type: gamma.Type, ID: gamma.ID, Name: gamma.Name}
	saved, err = app.SaveWar(card)
	assert.NoError(t, err)
	assert.Empty(t, saved.Ignored, "a group moved onto a side is no longer ignored")
}

func TestApplyWarRun(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	setup := func(t *testing.T) (*App, retrieval.War) {
		app, _ := warApp(t, nil)
		card := savedWar(t, app, "")
		war, _ := app.cache.WarByID(card.ID)
		war.Reach = []retrieval.WarReach{{Group: "Alpha", Oldest: base, Complete: false}, {Group: "Beta", Oldest: base, Complete: true}}
		war.NewestKillmail = base
		war.RunKey = "old"
		return app, app.cache.SaveWar(war)
	}

	t.Run("a full run replaces the reach and sets the run key", func(t *testing.T) {
		app, war := setup(t)
		app.applyWarRun(war.ID, warRunUpdate{full: true, runKey: "new", newest: base.Add(time.Hour), complete: true,
			reach: []retrieval.WarReach{{Group: "Gamma", Oldest: base, Complete: true}}})

		after, _ := app.cache.WarByID(war.ID)
		assert.Equal(t, []retrieval.WarReach{{Group: "Gamma", Oldest: base, Complete: true}}, after.Reach)
		assert.Equal(t, "new", after.RunKey)
		assert.Equal(t, base.Add(time.Hour), after.NewestKillmail)
		assert.False(t, after.LastRun.IsZero())
	})

	t.Run("an update replaces only the entries that did not come back complete", func(t *testing.T) {
		app, war := setup(t)
		app.applyWarRun(war.ID, warRunUpdate{runKey: "new", newest: base.Add(-time.Hour), complete: true, reach: []retrieval.WarReach{
			{Group: "Alpha", Oldest: base.Add(time.Hour), Complete: true},
			{Group: "Beta", Oldest: base.Add(2 * time.Hour), Complete: false},
			{Group: "Delta", Oldest: base.Add(3 * time.Hour), Complete: false},
		}})

		after, _ := app.cache.WarByID(war.ID)
		assert.Equal(t, []retrieval.WarReach{
			{Group: "Alpha", Oldest: base, Complete: false},
			{Group: "Beta", Oldest: base.Add(2 * time.Hour), Complete: false},
			{Group: "Delta", Oldest: base.Add(3 * time.Hour), Complete: false},
		}, after.Reach)
		assert.Equal(t, base, after.NewestKillmail, "never moved back")
	})

	t.Run("a stopped run or one with a list error keeps the old run key, so the next reads in full", func(t *testing.T) {
		app, war := setup(t)
		app.applyWarRun(war.ID, warRunUpdate{full: true, runKey: "new", complete: false})

		after, _ := app.cache.WarByID(war.ID)
		assert.Equal(t, "old", after.RunKey)
		assert.NotEqual(t, after.CurrentRunKey(), after.RunKey)
	})

	t.Run("a war removed during the run is not saved again", func(t *testing.T) {
		app, war := setup(t)
		app.cache.RemoveWar(war.ID)
		app.applyWarRun(war.ID, warRunUpdate{full: true, runKey: "new", complete: true})
		_, found := app.cache.WarByID(war.ID)
		assert.False(t, found)
	})
}

func TestReachLines(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	app, _ := warApp(t, nil)
	card := savedWar(t, app, "")
	war, _ := app.cache.WarByID(card.ID)
	oldest := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	war.Reach = []retrieval.WarReach{{Group: "Alpha", Oldest: oldest}, {Group: "Beta", Oldest: oldest, Complete: true}}
	app.cache.SaveWar(war)

	report, err := app.OpenWar(card.ID)

	assert.NoError(t, err)
	assert.Equal(t, []WarReachLine{{Group: "Alpha", Oldest: oldest}}, report.Reach, "only the lists that fell short")
}

func TestAddWarGroup(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	app, _ := warApp(t, fightLists(time.Now().UTC()))
	card := savedWar(t, app, "")
	gamma := WarAddedCard{Type: "CORPORATION", ID: warGamma, Name: "Gamma"}

	t.Run("a group joins from the beginning or from a date", func(t *testing.T) {
		report, err := app.AddWarGroup(card.ID, "a", gamma)
		assert.NoError(t, err)
		assert.Equal(t, []WarAddedCard{gamma}, report.War.AddedA)
		assert.True(t, report.FullRunDue)

		dated := WarAddedCard{Type: "ALLIANCE", ID: 4242, Name: "Delta", FromDate: "2026-05-02"}
		report, err = app.AddWarGroup(card.ID, "B", dated)
		assert.NoError(t, err)
		assert.Equal(t, []WarAddedCard{dated}, report.War.AddedB)
		war, _ := app.cache.WarByID(card.ID)
		assert.Equal(t, "2026-05-02", war.AddedB[0].FromDate)
	})

	t.Run("refusals change nothing", func(t *testing.T) {
		before, _ := app.cache.WarByID(card.ID)
		tests := map[string]struct {
			side  string
			group WarAddedCard
		}{
			"a side that is not":      {"c", WarAddedCard{Type: "CORPORATION", ID: 7, Name: "Seven"}},
			"a pilot":                 {"a", WarAddedCard{Type: "PILOT", ID: 7, Name: "Seven"}},
			"a bad date":              {"a", WarAddedCard{Type: "CORPORATION", ID: 7, Name: "Seven", FromDate: "soon"}},
			"already added":           {"b", gamma},
			"already a side's member": {"b", WarAddedCard{Type: "CORPORATION", ID: warBeta, Name: "Beta"}},
			"an alliance of a side":   {"b", WarAddedCard{Type: "ALLIANCE", ID: warAlpha, Name: "Alpha"}},
		}
		for name, test := range tests {
			_, err := app.AddWarGroup(card.ID, test.side, test.group)
			assert.Error(t, err, name)
		}
		_, err := app.AddWarGroup(-5, "a", gamma)
		assert.ErrorIs(t, err, errNoWar)
		after, _ := app.cache.WarByID(card.ID)
		assert.Equal(t, before.AddedA, after.AddedA)
		assert.Equal(t, before.AddedB, after.AddedB)
	})

	t.Run("a group is taken off again", func(t *testing.T) {
		report, err := app.RemoveWarGroup(card.ID, "CORPORATION", warGamma)
		assert.NoError(t, err)
		assert.Equal(t, []WarAddedCard{}, report.War.AddedA)
		assert.Len(t, report.War.AddedB, 1)
		_, err = app.RemoveWarGroup(-5, "CORPORATION", warGamma)
		assert.ErrorIs(t, err, errNoWar)
	})
}

func TestSearchWar(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	newApp := func(t *testing.T) *App {
		app := newIntelApp(t)
		app.sdeCheckedAt = time.Now()
		return app
	}
	save := func(t *testing.T, app *App, name string) WarCard {
		card, err := app.SaveWar(WarCard{Name: name, SideA: alphaSide(), SideB: betaSide()})
		assert.NoError(t, err)
		return card
	}

	t.Run("by type WAR, by id or name, from the cache", func(t *testing.T) {
		app := newApp(t)
		card := save(t, app, "The War")

		byID, err := app.SearchTarget("", "WAR", card.ID)
		assert.NoError(t, err)
		byName, err := app.SearchTarget("the war", "WAR", 0)
		assert.NoError(t, err)

		for _, result := range []TargetResult{byID, byName} {
			assert.Equal(t, "WAR", result.Type)
			assert.Equal(t, card.ID, result.ID)
			assert.Equal(t, "The War", result.Display.Name)
			if assert.NotNil(t, result.War) {
				assert.Equal(t, card.ID, result.War.ID)
			}
			assert.Nil(t, result.Coalition)
			assert.Nil(t, result.Holdings)
		}
		recent, _ := app.GetRecentSearches()
		assert.Equal(t, []retrieval.RecentSearch{{Name: "The War", Type: "WAR", ID: card.ID}}, recent)
		data, _ := json.Marshal(byID)
		assert.Contains(t, string(data), `"war":{`)
		plain, _ := json.Marshal(TargetResult{})
		assert.NotContains(t, string(plain), `"war"`)

		_, err = app.SearchTarget("", "WAR", -99)
		assert.ErrorIs(t, err, errNoWar)
		_, err = app.SearchTarget("Nothing", "WAR", 0)
		assert.ErrorIs(t, err, errNoWar)
	})

	t.Run("a typed name of a war alone is the war, else it is a choice among what the ESI found", func(t *testing.T) {
		app := newApp(t)
		card := save(t, app, "Nobody At All")
		result, err := app.SearchTarget("Nobody At All", "", 0)
		assert.NoError(t, err)
		assert.Equal(t, "WAR", result.Type)
		assert.Equal(t, card.ID, result.ID)

		other := save(t, app, "URSA.")
		result, err = app.SearchTarget("URSA.", "", 0)
		assert.NoError(t, err)
		assert.Empty(t, result.Type)
		if assert.Len(t, result.Choices, 2) {
			assert.Equal(t, TargetChoice{Type: "WAR", ID: other.ID, Name: "URSA."}, result.Choices[0])
			assert.Equal(t, "ALLIANCE", result.Choices[1].Type)
		}
	})

	t.Run("a name that is a coalition and a war keeps both", func(t *testing.T) {
		app := newApp(t)
		coalition := coalitionWith(t, app, "Twins", [2]string{"URSA.", "ALLIANCE"})
		war := save(t, app, "twins")

		result, err := app.SearchTarget("Twins", "", 0)

		assert.NoError(t, err)
		assert.Equal(t, []TargetChoice{{Type: "COALITION", ID: coalition.ID, Name: "Twins"}, {Type: "WAR", ID: war.ID, Name: "twins"}}, result.Choices)
	})
}

func TestWarFullRunOnNewAffiliate(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC().Truncate(time.Second)
	app, zkill := warApp(t, fightLists(now))
	side := alphaSide()
	side.WithAffiliates = true
	card, err := app.SaveWar(WarCard{SideA: side, SideB: betaSide(), FromDate: "2026-03-01"})
	assert.NoError(t, err)
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err = app.CollectWar(card.ID)
	assert.NoError(t, err)
	zkill.reset()
	report, err := app.CollectWar(card.ID)
	assert.NoError(t, err)
	assert.False(t, report.FullRunDue)
	assert.NotContains(t, zkill.newerThan(), start, "an update reads only what is new")

	app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: warAlpha, MainName: "Alpha", HoldingType: "CORPORATION", HoldingID: warGamma, HoldingName: "Gamma"})
	opened, err := app.OpenWar(card.ID)
	assert.NoError(t, err)
	assert.True(t, opened.FullRunDue, "the new affiliate's list has not been read")
	zkill.reset()
	_, err = app.CollectWar(card.ID)
	assert.NoError(t, err)
	assert.Contains(t, zkill.newerThan(), start)
}

func TestWarReportStage3(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC().Truncate(time.Second)
	const standupTemplar, astrahus = 47035, 35832

	t.Run("a war with no run has its days and marks as empty lists", func(t *testing.T) {
		app, _ := warApp(t, map[int][]api.ZkbResponse{})
		report, err := app.OpenWar(savedWar(t, app, "2026-03-01").ID)
		assert.NoError(t, err)
		assert.NotNil(t, report.Days)
		assert.Empty(t, report.Days)
	})

	t.Run("the report carries a ref mark, trash, and each day's figures with their efficiency", func(t *testing.T) {
		fighter := coalitionKillmail(20, now.Add(-90*time.Hour), 98000100, warAlpha, warBeta)
		fighter.Victim.ShipTypeID = standupTemplar
		fighter.Victim.CharacterID = nil
		station := coalitionKillmail(21, now.Add(-2*time.Hour), 98000100, warAlpha, warBeta)
		station.Victim.ShipTypeID = astrahus
		station.Victim.CharacterID = nil
		trash := coalitionKillmail(22, now.Add(-time.Hour), 98000100, warAlpha, warBeta)
		app, _ := warApp(t, map[int][]api.ZkbResponse{warBeta: {fighter, station, trash}})

		report, err := app.CollectWar(savedWar(t, app, "2026-03-01").ID)
		assert.NoError(t, err)

		assert.Equal(t, 1, report.B.StationRefs)
		assert.Zero(t, report.B.RefAttempts)
		assert.Equal(t, 1, report.B.ShipsTrash)
		if assert.NotEmpty(t, report.Days) {
			shipsTrash := 0
			for _, day := range report.Days {
				shipsTrash += day.B.ShipsTrash
			}
			assert.Equal(t, 1, shipsTrash)
		}
		marked, destroyed, trashed := 0, 0, 0
		for _, battle := range report.Battles {
			assert.NotNil(t, battle.StationRefs)
			for _, ref := range battle.StationRefs {
				assert.Equal(t, "b", ref.Side)
				if ref.Destroyed {
					destroyed++
					assert.Equal(t, "Astrahus", ref.Structure)
					continue
				}
				marked++
				assert.False(t, ref.Attempt)
				assert.NotEmpty(t, ref.Structure)
			}
			trashed += battle.B.TrashKillmails
		}
		assert.Equal(t, 1, marked)
		assert.Equal(t, 1, destroyed)
		assert.Equal(t, 1, trashed)
	})
}

func TestWarOtherSystemReports(t *testing.T) {
	main := &retrieval.System{SystemID: 31001677, Name: "J113551"}
	other := &retrieval.System{SystemID: 31002328, Name: "J155928"}
	unseen := &retrieval.System{SystemID: 31000001, Name: "J100001"}
	start := time.Date(2026, 11, 5, 16, 40, 0, 0, time.UTC)
	battle := &aggregate.WarBattle{
		Start: start, End: start.Add(4 * time.Hour),
		OtherDetails: []*retrieval.System{other, unseen},
		Killmails: []*retrieval.Killmail{
			{System: main, KillmailTime: start.Add(4 * time.Hour)},
			{System: other, KillmailTime: start},
			{System: other, KillmailTime: start.Add(10 * time.Minute)},
			{KillmailTime: start},
		},
	}

	reports := warOtherSystemReports(battle)

	if assert.Len(t, reports, 2) {
		// at the middle of its own killmails, not the battle's, which is 18:40
		assert.Equal(t, "J155928", reports[0].Name)
		assert.Equal(t, "https://zkillboard.com/related/31002328/202611051600/", reports[0].ZkillURL)
		assert.Equal(t, "https://br.evetools.org/related/31002328/202611051600", reports[0].EveToolsURL)
		assert.Equal(t, "https://warbeacon.net/br/related/31002328/202611051600/", reports[0].WarbeaconURL)
		// with no killmail counted there, the battle's middle
		assert.Equal(t, "https://zkillboard.com/related/31000001/202611051800/", reports[1].ZkillURL)
	}
	assert.NotNil(t, warOtherSystemReports(&aggregate.WarBattle{}))
}

func TestWarSearchGroups(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	app, _ := warApp(t, nil)
	card := savedWar(t, app, "")

	card.Name = "The Big One"
	renamed, err := app.SaveWar(card)
	assert.NoError(t, err)
	assert.Equal(t, "The Big One", renamed.Name)

	groups, err := app.GetSearchGroups()
	assert.NoError(t, err)
	if assert.Len(t, groups, 1) {
		assert.Equal(t, SearchGroup{Name: "The Big One", Type: warType, ID: card.ID, Sides: []string{"Alpha", "Beta"}}, groups[0])
	}
}

func TestWarBattleEdits(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC()
	// two fights three hours apart
	lists := map[int][]api.ZkbResponse{warAlpha: {
		coalitionKillmail(10, now.Add(-5*time.Hour), 98000100, warAlpha, warBeta),
		coalitionKillmail(11, now.Add(-5*time.Hour+5*time.Minute), 98000100, warAlpha, warBeta),
		coalitionKillmail(12, now.Add(-2*time.Hour), 98000100, warAlpha, warBeta),
	}}
	app, _ := warApp(t, lists)
	card := savedWar(t, app, "")
	before, err := app.CollectWar(card.ID)
	assert.NoError(t, err)
	if !assert.Len(t, before.Battles, 2) {
		return
	}
	newer, older := before.Battles[0], before.Battles[1]

	t.Run("refusals save nothing", func(t *testing.T) {
		_, err := app.MergeWarBattles(card.ID, []string{newer.ID})
		assert.Error(t, err)
		_, err = app.MergeWarBattles(card.ID, []string{newer.ID, "gone"})
		assert.ErrorIs(t, err, errNoWarBattle)
		_, err = app.SplitWarBattle(card.ID, older.ID, nil)
		assert.Error(t, err)
		_, err = app.SplitWarBattle(card.ID, older.ID, []int{10, 11})
		assert.Error(t, err, "not all of them")
		_, err = app.GetWarBattleKillmails(card.ID, "gone")
		assert.ErrorIs(t, err, errNoWarBattle)
		assert.Empty(t, app.cache.WarEditsOf(card.ID).Edits)
	})

	t.Run("merge, split, and undo keep the totals and outlast opening it again", func(t *testing.T) {
		merged, err := app.MergeWarBattles(card.ID, []string{newer.ID, older.ID})
		assert.NoError(t, err)
		assert.Len(t, merged.Battles, 1)
		assert.Equal(t, before.A, merged.A)
		assert.Equal(t, []WarEditMark{{ID: 1, Kind: "merge"}}, merged.Battles[0].Edits)

		kms, err := app.GetWarBattleKillmails(card.ID, merged.Battles[0].ID)
		assert.NoError(t, err)
		assert.Len(t, kms, 3)
		assert.Equal(t, 10, kms[0].ID, "oldest first")
		split, err := app.SplitWarBattle(card.ID, merged.Battles[0].ID, []int{kms[2].ID})
		assert.NoError(t, err)
		assert.Len(t, split.Battles, 2)
		assert.Equal(t, before.A, split.A)

		opened, err := app.OpenWar(card.ID)
		assert.NoError(t, err)
		assert.Len(t, opened.Battles, 2)
		assert.Len(t, opened.Battles[0].Edits, 2, "the piece keeps the merge it came from")

		_, err = app.UndoWarEdit(card.ID, 2)
		assert.NoError(t, err)
		undone, err := app.UndoWarEdit(card.ID, 1)
		assert.NoError(t, err)
		assert.Equal(t, before.Battles, undone.Battles)
	})

	t.Run("an edit can start from a war whose report was not the last built", func(t *testing.T) {
		app.lastWar = nil
		merged, err := app.MergeWarBattles(card.ID, []string{newer.ID, older.ID})
		assert.NoError(t, err)
		assert.Len(t, merged.Battles, 1)
	})
}
