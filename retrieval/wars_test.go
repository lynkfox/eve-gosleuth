package retrieval_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func warKillmail(id int, at time.Time, total float64) retrieval.RawKillmail {
	esi := &api.ESIKillmail{KillmailID: id, KillmailTime: at, SolarSystemID: 30000001, Victim: api.ESIVictim{ShipTypeID: 587}}
	return retrieval.RawKillmail{
		ESIKillmail: esi,
		Zkillmail:   &api.ZkbResponse{KillmailID: id, KillmailTime: at, Zkb: api.Zkb{TotalValue: total, Hash: "h"}},
		Via:         &retrieval.TargetGroup{ID: 1},
	}
}

func testWar() retrieval.War {
	return retrieval.War{
		Name:     "Alpha vs Bravo",
		SideA:    retrieval.WarSide{Type: "ALLIANCE", ID: 1, Name: "Alpha", WithAffiliates: true},
		SideB:    retrieval.WarSide{Type: "CORPORATION", ID: 2, Name: "Bravo"},
		FromDate: "2026-10-01",
		AddedA:   []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: 7, Name: "Seven"}, {Type: "CORPORATION", ID: 8, Name: "Eight", FromDate: "2026-10-05"}},
		Reach:    []retrieval.WarReach{{Group: "Alpha", Oldest: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Complete: true}},
	}
}

func TestWarDates(t *testing.T) {
	war := retrieval.War{FromDate: "2026-10-01", UntilDate: "2026-10-10"}
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), war.Start())
	assert.True(t, retrieval.War{}.Start().IsZero())
	assert.True(t, retrieval.War{FromDate: "nonsense"}.Start().IsZero())

	assert.False(t, war.Within(time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)))
	assert.True(t, war.Within(time.Date(2026, 10, 1, 0, 0, 1, 0, time.UTC)))
	assert.True(t, war.Within(time.Date(2026, 10, 10, 23, 59, 59, 0, time.UTC)), "the whole of the last day")
	assert.False(t, war.Within(time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)))
	assert.True(t, retrieval.War{}.Within(time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.True(t, retrieval.War{UntilDate: "oops"}.Within(time.Now()))
}

func TestWarRunKey(t *testing.T) {
	base := testWar()
	key := base.CurrentRunKey()

	reordered := testWar()
	reordered.AddedA[0], reordered.AddedA[1] = reordered.AddedA[1], reordered.AddedA[0]
	assert.Equal(t, key, reordered.CurrentRunKey(), "the order of added groups does not matter")

	untouched := testWar()
	untouched.UntilDate = "2026-11-01"
	untouched.AddedA[1].FromDate = "2026-10-20"
	untouched.Name = "Renamed"
	assert.Equal(t, key, untouched.CurrentRunKey(), "UntilDate, added dates and the name are applied to what is held")

	for name, change := range map[string]func(*retrieval.War){
		"side A":      func(w *retrieval.War) { w.SideA.ID = 99 },
		"side B":      func(w *retrieval.War) { w.SideB.Type = "ALLIANCE" },
		"affiliates":  func(w *retrieval.War) { w.SideA.WithAffiliates = false },
		"from":        func(w *retrieval.War) { w.FromDate = "2026-09-01" },
		"added group": func(w *retrieval.War) { w.AddedB = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: 7}} },
		"swapped":     func(w *retrieval.War) { w.SideA, w.SideB = w.SideB, w.SideA },
	} {
		changed := testWar()
		change(&changed)
		assert.NotEqual(t, key, changed.CurrentRunKey(), name)
	}
}

func TestSavedWars(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	newCache := func(t *testing.T) (*retrieval.CachedEveIDs, string) {
		path := filepath.Join(t.TempDir(), "cache.db")
		return retrieval.NewCache(&RawESIClient{}, path), path
	}

	t.Run("a war gets a negative id, round trips through the database, and is found by id and name", func(t *testing.T) {
		cache, path := newCache(t)
		saved := cache.SaveWar(testWar())
		other := cache.SaveWar(retrieval.War{Name: "aardvark"})
		assert.Less(t, saved.ID, 0)
		assert.NotEqual(t, saved.ID, other.ID)
		assert.False(t, saved.SetAt.IsZero())
		assert.NoError(t, retrieval.SaveCache(cache))

		reloaded := retrieval.NewCache(&RawESIClient{}, path)
		found, ok := reloaded.WarByID(saved.ID)
		assert.True(t, ok)
		assert.Equal(t, saved, found)
		assert.Equal(t, testWar().AddedA, found.AddedA)
		assert.Equal(t, testWar().Reach, found.Reach)
		named, ok := reloaded.WarNamed("ALPHA VS bravo")
		assert.True(t, ok)
		assert.Equal(t, saved.ID, named.ID)
		_, ok = reloaded.WarNamed("nobody")
		assert.False(t, ok)
		names := []string{}
		for _, war := range reloaded.Wars() {
			names = append(names, war.Name)
		}
		assert.Equal(t, []string{"aardvark", "Alpha vs Bravo"}, names)
	})

	t.Run("saving with the id replaces, and a changed copy leaves the cache's alone", func(t *testing.T) {
		cache, _ := newCache(t)
		saved := cache.SaveWar(testWar())
		saved.AddedA[0].Name = "Changed"
		saved.Reach[0].Group = "Changed"
		found, _ := cache.WarByID(saved.ID)
		assert.Equal(t, "Seven", found.AddedA[0].Name)
		assert.Equal(t, "Alpha", found.Reach[0].Group)

		found.Name = "New name"
		cache.SaveWar(found)
		assert.Len(t, cache.Wars(), 1)
		again, _ := cache.WarByID(saved.ID)
		assert.Equal(t, "New name", again.Name)
	})

	t.Run("a removed war takes its killmails, saved or not, and not those of a war with a similar id", func(t *testing.T) {
		cache, path := newCache(t)
		at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		war := cache.SaveWar(testWar())
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(1, at, 10)})
		assert.NoError(t, retrieval.SaveCache(cache))
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(2, at, 10)}) // not saved yet
		cache.AddWarKillmails(war.ID*10, []retrieval.RawKillmail{warKillmail(3, at, 10)})
		cache.AddWarKillmails(war.ID*10, []retrieval.RawKillmail{warKillmail(4, at, 10)})

		cache.RemoveWar(war.ID)
		_, ok := cache.WarByID(war.ID)
		assert.False(t, ok)
		assert.Zero(t, cache.WarKillmailCount(war.ID))
		assert.Equal(t, 2, cache.WarKillmailCount(war.ID*10), "-5 is not -55")
		assert.NoError(t, retrieval.SaveCache(cache))
		reloaded := retrieval.NewCache(&RawESIClient{}, path)
		assert.Empty(t, reloaded.WarKillmails(war.ID))
		assert.Len(t, reloaded.WarKillmails(war.ID*10), 2)
		cache.RemoveWar(12345) // not saved, changes nothing
	})

	t.Run("a war's killmails round trip, oldest first, replaced by id, with no Via", func(t *testing.T) {
		cache, path := newCache(t)
		war := cache.SaveWar(testWar())
		day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(3, day(3), 30), warKillmail(1, day(1), 10), {ESIKillmail: warKillmail(9, day(9), 1).ESIKillmail}, {}})
		assert.NoError(t, retrieval.SaveCache(cache))
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(2, day(2), 20), warKillmail(3, day(3), 33)})

		for _, c := range []*retrieval.CachedEveIDs{cache, retrieval.NewCache(&RawESIClient{}, path)} {
			held := c.WarKillmails(war.ID)
			if c != cache {
				assert.Len(t, held, 2, "only what was saved")
				continue
			}
			if assert.Len(t, held, 3) {
				assert.Equal(t, []int{1, 2, 3}, []int{held[0].ESIKillmail.KillmailID, held[1].ESIKillmail.KillmailID, held[2].ESIKillmail.KillmailID})
				assert.Equal(t, 33.0, held[2].Zkillmail.Zkb.TotalValue, "replaced")
				assert.Equal(t, "h", held[0].Zkillmail.Zkb.Hash)
				assert.Equal(t, day(1), held[0].Zkillmail.KillmailTime)
				assert.Equal(t, 1, held[0].Zkillmail.KillmailID)
				assert.Nil(t, held[0].Via)
				assert.Equal(t, 587, held[0].ESIKillmail.Victim.ShipTypeID)
			}
			assert.Equal(t, 3, c.WarKillmailCount(war.ID))
		}
		assert.Empty(t, cache.WarKillmails(-1), "not nil for none")
		assert.NotNil(t, cache.WarKillmails(-1))
	})

	t.Run("a plain clear keeps a war and its killmails, one with user data removes them", func(t *testing.T) {
		cache, path := newCache(t)
		at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		war := cache.SaveWar(testWar())
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(1, at, 10)})
		cache.StoreKillmail(warKillmail(50, at, 1).ESIKillmail)
		assert.NoError(t, retrieval.SaveCache(cache))
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(2, at, 10)}) // unsaved

		cache.ClearAll(false)
		assert.Nil(t, cache.CachedKillmail(50), "the cache's own killmails are emptied")
		_, ok := cache.WarByID(war.ID)
		assert.True(t, ok)
		assert.Equal(t, 2, cache.WarKillmailCount(war.ID))
		cache.ClearOlderThan(time.Now().Add(time.Hour), false)
		assert.Equal(t, 2, cache.WarKillmailCount(war.ID))
		_, ok = retrieval.NewCache(&RawESIClient{}, path).WarByID(war.ID)
		assert.True(t, ok)

		cache.ClearAll(true)
		_, ok = cache.WarByID(war.ID)
		assert.False(t, ok)
		assert.Zero(t, cache.WarKillmailCount(war.ID))
		reloaded := retrieval.NewCache(&RawESIClient{}, path)
		_, ok = reloaded.WarByID(war.ID)
		assert.False(t, ok)
		assert.Empty(t, reloaded.WarKillmails(war.ID))
	})

	t.Run("a cache clear with user data removes an unsaved war killmail too", func(t *testing.T) {
		cache, _ := newCache(t)
		war := cache.SaveWar(testWar())
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(1, time.Now(), 10)})
		cache.ClearAll(true)
		assert.Zero(t, cache.WarKillmailCount(war.ID))
	})

	t.Run("without a database a war still holds its killmails for the session", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "no-such-dir", "\x00cache.db"))
		war := cache.SaveWar(testWar())
		cache.AddWarKillmails(war.ID, []retrieval.RawKillmail{warKillmail(1, time.Now(), 10)})
		assert.Equal(t, 1, cache.WarKillmailCount(war.ID))
		cache.RemoveWar(war.ID)
		assert.Zero(t, cache.WarKillmailCount(war.ID))
	})
}

func TestWarSideGroups(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 1, MainName: "Alpha", HoldingType: "CORPORATION", HoldingID: 11, HoldingName: "Alpha Holding"})
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 1, MainName: "Alpha", HoldingType: "ALLIANCE", HoldingID: 12, HoldingName: "Alpha Friends"})

	t.Run("a single corp or alliance is alone unless it brings its affiliates", func(t *testing.T) {
		groups, err := cache.WarSideGroups(retrieval.WarSide{Type: "ALLIANCE", ID: 1, Name: "Alpha"})
		assert.NoError(t, err)
		assert.Equal(t, []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: 1, Name: "Alpha", Member: true}}, groups)

		groups, err = cache.WarSideGroups(retrieval.WarSide{Type: "ALLIANCE", ID: 1, Name: "Alpha", WithAffiliates: true})
		assert.NoError(t, err)
		assert.Equal(t, []retrieval.TargetGroup{
			{Type: types.ALLIANCE, ID: 1, Name: "Alpha", Member: true},
			{Type: types.ALLIANCE, ID: 12, Name: "Alpha Friends"},
			{Type: types.CORPORATION, ID: 11, Name: "Alpha Holding"},
		}, groups)

		groups, _ = cache.WarSideGroups(retrieval.WarSide{Type: "CORPORATION", ID: 5, Name: "Loner", WithAffiliates: true})
		assert.Len(t, groups, 1)
	})

	t.Run("a coalition brings its members and their affiliates, each once", func(t *testing.T) {
		coalition := cache.SaveCoalition(retrieval.Coalition{Name: "Coal", Members: []retrieval.CoalitionMember{
			{Type: "ALLIANCE", ID: 1, Name: "Alpha"}, {Type: "CORPORATION", ID: 11, Name: "Alpha Holding"}, {Type: "ALLIANCE", ID: 1, Name: "Alpha"},
		}})
		groups, err := cache.WarSideGroups(retrieval.WarSide{Type: "COALITION", ID: coalition.ID, Name: "Coal"})
		assert.NoError(t, err)
		assert.Equal(t, []retrieval.TargetGroup{
			{Type: types.ALLIANCE, ID: 1, Name: "Alpha", Member: true},
			{Type: types.CORPORATION, ID: 11, Name: "Alpha Holding", Member: true},
			{Type: types.ALLIANCE, ID: 12, Name: "Alpha Friends"},
		}, groups)
	})

	t.Run("a coalition no longer saved stops it, naming it", func(t *testing.T) {
		_, err := cache.WarSideGroups(retrieval.WarSide{Type: "COALITION", ID: -42, Name: "Vanished"})
		assert.ErrorIs(t, err, retrieval.ErrWarSideGone)
		assert.ErrorContains(t, err, "Vanished")
		_, err = cache.WarSideGroups(retrieval.WarSide{Type: "PILOT", ID: 1})
		assert.Error(t, err)
	})
}

func TestWarEdits(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	path := filepath.Join(t.TempDir(), "cache.db")
	load := func() *retrieval.CachedEveIDs {
		cache, err := retrieval.LoadCache(nil, path)
		assert.NoError(t, err)
		return cache
	}
	save := func(cache *retrieval.CachedEveIDs) {
		assert.NoError(t, retrieval.SaveCache(cache))
		assert.NoError(t, cache.Close())
	}
	cache := load()
	war := cache.SaveWar(testWar())
	assert.Empty(t, cache.WarEditsOf(war.ID).Edits)

	edits := retrieval.WarEdits{}
	edits.AddEdit(retrieval.WarEditMerge, [][]int{{1}, {2}})
	edits.AddEdit(retrieval.WarEditSplit, [][]int{{3}, {4}})
	assert.Equal(t, []int{1, 2}, []int{edits.Edits[0].ID, edits.Edits[1].ID})
	cache.SaveWarEdits(war.ID, edits)

	// what comes back is a copy
	got := cache.WarEditsOf(war.ID)
	got.Edits[0].Groups[0][0] = 99
	assert.Equal(t, 1, cache.WarEditsOf(war.ID).Edits[0].Groups[0][0])

	// across sessions
	save(cache)
	cache = load()
	got = cache.WarEditsOf(war.ID)
	assert.Len(t, got.Edits, 2)
	assert.True(t, got.RemoveEdit(1))
	assert.False(t, got.RemoveEdit(1))
	got.AddEdit(retrieval.WarEditMerge, [][]int{{5}, {6}})
	assert.Equal(t, 3, got.Edits[1].ID, "an id is not given twice while later ones remain")

	// a clear leaves them, unless told not to
	cache.ClearAll(false)
	assert.Len(t, cache.WarEditsOf(war.ID).Edits, 2)

	// and they go with the war, also from the database
	save(cache)
	cache = load()
	cache.RemoveWar(war.ID)
	assert.Empty(t, cache.WarEditsOf(war.ID).Edits)
	save(cache)
	cache = load()
	assert.Empty(t, cache.WarEditsOf(war.ID).Edits)
	assert.NoError(t, cache.Close())
}
