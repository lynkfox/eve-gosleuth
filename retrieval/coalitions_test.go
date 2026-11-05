package retrieval_test

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func coalitionMembers() []retrieval.CoalitionMember {
	return []retrieval.CoalitionMember{
		{Type: "ALLIANCE", ID: 99000001, Name: "Alpha Alliance"},
		{Type: "CORPORATION", ID: 98000002, Name: "Beta Corp"},
	}
}

func TestCoalitions(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	newCache := func(t *testing.T) (*retrieval.CachedEveIDs, string) {
		path := filepath.Join(t.TempDir(), "cache.db")
		return retrieval.NewCache(&RawESIClient{}, path), path
	}

	t.Run("a saved coalition gets a negative id and is found by id and name, listed by name ignoring case", func(t *testing.T) {
		cache, _ := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "zeta", Members: coalitionMembers()})
		cache.SaveCoalition(retrieval.Coalition{Name: "Alpha"})

		assert.Less(t, saved.ID, 0)
		assert.False(t, saved.SetAt.IsZero())
		found, ok := cache.CoalitionByID(saved.ID)
		assert.True(t, ok)
		assert.Equal(t, coalitionMembers(), found.Members)
		named, ok := cache.CoalitionNamed("ZETA")
		assert.True(t, ok)
		assert.Equal(t, saved.ID, named.ID)
		_, ok = cache.CoalitionNamed("nobody")
		assert.False(t, ok)

		names := []string{}
		for _, coalition := range cache.Coalitions() {
			names = append(names, coalition.Name)
		}
		assert.Equal(t, []string{"Alpha", "zeta"}, names)
	})

	t.Run("saving with the id again renames it, and survives a reload", func(t *testing.T) {
		cache, path := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "Old", IncludeStructures: true, Members: coalitionMembers()})
		saved.Name = "New"
		cache.SaveCoalition(saved)
		assert.Len(t, cache.Coalitions(), 1)
		assert.NoError(t, retrieval.SaveCache(cache))

		reloaded := retrieval.NewCache(&RawESIClient{}, path)
		found, ok := reloaded.CoalitionByID(saved.ID)
		assert.True(t, ok)
		assert.Equal(t, "New", found.Name)
		assert.True(t, found.IncludeStructures)
		assert.Equal(t, coalitionMembers(), found.Members)
	})

	t.Run("a removed coalition is gone from the session and the database", func(t *testing.T) {
		cache, path := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "Gone"})
		assert.NoError(t, retrieval.SaveCache(cache))
		cache.RemoveCoalition(saved.ID)
		cache.RemoveCoalition(-12345) // not saved, changes nothing

		_, ok := cache.CoalitionByID(saved.ID)
		assert.False(t, ok)
		_, ok = retrieval.NewCache(&RawESIClient{}, path).CoalitionByID(saved.ID)
		assert.False(t, ok)
	})

	t.Run("a removed coalition's id is not given to the next", func(t *testing.T) {
		cache, _ := newCache(t)
		first := cache.SaveCoalition(retrieval.Coalition{Name: "First"})
		cache.RemoveCoalition(first.ID)
		second := cache.SaveCoalition(retrieval.Coalition{Name: "Second"})
		third := cache.SaveCoalition(retrieval.Coalition{Name: "Third"})

		assert.NotEqual(t, first.ID, second.ID)
		assert.NotEqual(t, second.ID, third.ID)
		assert.NotEqual(t, first.ID, third.ID)
	})

	t.Run("a coalition restored with its id keeps it", func(t *testing.T) {
		cache, _ := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "Back"})
		cache.RemoveCoalition(saved.ID)
		restored := cache.SaveCoalition(saved)
		assert.Equal(t, saved.ID, restored.ID)
	})

	t.Run("ClearAll leaves coalitions unless the user data is cleared too", func(t *testing.T) {
		cache, path := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "Mine"})
		assert.NoError(t, retrieval.SaveCache(cache))

		cache.ClearAll(false)
		_, ok := cache.CoalitionByID(saved.ID)
		assert.True(t, ok)
		_, ok = retrieval.NewCache(&RawESIClient{}, path).CoalitionByID(saved.ID)
		assert.True(t, ok)

		cache.ClearAll(true)
		_, ok = cache.CoalitionByID(saved.ID)
		assert.False(t, ok)
	})

	t.Run("a changed copy leaves the cache's alone", func(t *testing.T) {
		cache, _ := newCache(t)
		saved := cache.SaveCoalition(retrieval.Coalition{Name: "Copy", Members: coalitionMembers()})
		saved.Members[0].Name = "Changed"
		found, _ := cache.CoalitionByID(saved.ID)
		assert.Equal(t, "Alpha Alliance", found.Members[0].Name)
	})
}

func TestCoalitionGroups(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 99000001, MainName: "Alpha Alliance", HoldingType: "CORPORATION", HoldingID: 98000009, HoldingName: "Alpha Holdings"})
	// beta's affiliate is also a member, and a member's affiliate is found once
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "CORPORATION", MainID: 98000002, MainName: "Beta Corp", HoldingType: "ALLIANCE", HoldingID: 99000001, HoldingName: "Alpha Alliance"})
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "CORPORATION", MainID: 98000002, MainName: "Beta Corp", HoldingType: "CORPORATION", HoldingID: 98000009, HoldingName: "Alpha Holdings"})
	// a main the member holds for is not pulled in
	cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 99000500, MainName: "Big Main", HoldingType: "ALLIANCE", HoldingID: 99000001, HoldingName: "Alpha Alliance"})
	coalition := retrieval.Coalition{ID: -1, Name: "Both", Members: append(coalitionMembers(), retrieval.CoalitionMember{Type: "ALLIANCE", ID: 99000001, Name: "Alpha Alliance"})}

	t.Run("members then their affiliates, each once, members marked", func(t *testing.T) {
		assert.Equal(t, []retrieval.TargetGroup{
			{Type: types.ALLIANCE, ID: 99000001, Name: "Alpha Alliance", Member: true},
			{Type: types.CORPORATION, ID: 98000002, Name: "Beta Corp", Member: true},
			{Type: types.CORPORATION, ID: 98000009, Name: "Alpha Holdings"},
		}, cache.CoalitionGroups(coalition, true))
	})

	t.Run("without Include Affiliates only the members", func(t *testing.T) {
		groups := cache.CoalitionGroups(coalition, false)
		assert.Len(t, groups, 2)
		for _, group := range groups {
			assert.True(t, group.Member)
		}
	})
}

func TestCoalitionTargetIsGroup(t *testing.T) {
	target := retrieval.IntelTarget{IDNumber: -1700000000000, TypeOfEntity: types.COALITION, Holdings: []retrieval.TargetGroup{
		{Type: types.ALLIANCE, ID: 99000001, Member: true},
		{Type: types.CORPORATION, ID: 98000002, Member: true},
		{Type: types.CORPORATION, ID: 98000009},
	}}

	assert.True(t, target.IsGroupTarget())
	assert.True(t, (&retrieval.IntelTarget{TypeOfEntity: types.CORPORATION}).IsGroupTarget())
	assert.False(t, (&retrieval.IntelTarget{TypeOfEntity: types.PILOT}).IsGroupTarget())

	assert.True(t, target.IsGroup(98000777, 99000001), "a corp in a member alliance")
	assert.True(t, target.IsGroup(98000002, 0), "a member corp")
	assert.True(t, target.IsGroup(98000009, 0), "a member's affiliate")
	assert.False(t, target.IsGroup(98000777, 99000777))
	assert.False(t, target.IsGroup(0, 0))
	assert.False(t, target.IsGroup(target.IDNumber, target.IDNumber), "the coalition's own id never matches")
	assert.Equal(t, "COALITION", types.COALITION.String())
}

// idRecorder records the entity ids zKillboard is asked for
type idRecorder struct {
	MockZkillClient
	mu    *sync.Mutex
	asked *[]int
	fors  *[]string
}

func (r idRecorder) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	r.mu.Lock()
	*r.asked = append(*r.asked, entityId)
	*r.fors = append(*r.fors, paging.For)
	r.mu.Unlock()
	return r.MockZkillClient.GetZkillResults(entityId, entityType, paging)
}

func TestCollectKillmailsOfCoalition(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	hours := func(n int) time.Time { return now.Add(-time.Duration(n) * time.Hour) }
	members := []retrieval.TargetGroup{
		{Type: types.ALLIANCE, ID: 99000001, Name: "Alpha Alliance", Member: true},
		{Type: types.CORPORATION, ID: 98000002, Name: "Beta Corp", Member: true},
		{Type: types.CORPORATION, ID: 98000009, Name: "Alpha Holdings"},
	}
	const coalitionID = -1700000000000
	setup := func(holdings []retrieval.TargetGroup) (retrieval.IntelTarget, *[]int, *[]string) {
		var asked []int
		var fors []string
		zkill := idRecorder{mu: &sync.Mutex{}, asked: &asked, fors: &fors, MockZkillClient: MockZkillClient{byID: map[int][]api.ZkbResponse{
			99000001: {WholeZkillmail(10, hours(1)), WholeZkillmail(11, hours(4))},
			98000002: {WholeZkillmail(20, hours(2)), WholeZkillmail(11, hours(4))}, // 11: one member killing another, on both lists
			98000009: {WholeZkillmail(30, hours(3))},
		}}}
		return retrieval.IntelTarget{IDNumber: coalitionID, Name: "The Coalition", TypeOfEntity: types.COALITION, Holdings: holdings,
			Clients: retrieval.CollectionClientPool{ZkillClient: zkill, ESIClient: MockESIClient{}}}, &asked, &fors
	}

	t.Run("no list for the coalition itself, each killmail once, Via naming who it was listed for", func(t *testing.T) {
		target, asked, fors := setup(members)
		killmails, err := retrieval.CollectKillmails(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.NoError(t, err)
		assert.NotContains(t, *asked, coalitionID)
		assert.ElementsMatch(t, []int{99000001, 98000002, 98000009}, *asked)
		assert.ElementsMatch(t, []string{"member Alpha Alliance", "member Beta Corp", "affiliate Alpha Holdings"}, *fors)

		ids, via := []int{}, map[int]string{}
		for _, km := range *killmails {
			ids = append(ids, km.ESIKillmail.KillmailID)
			if assert.NotNil(t, km.Via, "every killmail has a Via") {
				via[km.ESIKillmail.KillmailID] = km.Via.Name
			}
		}
		assert.Equal(t, []int{10, 20, 30, 11}, ids)
		assert.Equal(t, map[int]string{10: "Alpha Alliance", 20: "Beta Corp", 30: "Alpha Holdings", 11: "Alpha Alliance"}, via)
	})

	t.Run("an id below 0 is never asked of zKillboard", func(t *testing.T) {
		target, asked, _ := setup([]retrieval.TargetGroup{{Type: types.CORPORATION, ID: -5, Name: "Local"}})
		target.TypeOfEntity = types.ALLIANCE
		target.IDNumber = -7
		_, err := retrieval.CollectKillmails(target, retrieval.KillmailLimits{Max: 5})

		assert.ErrorIs(t, err, retrieval.ErrLocalID)
		assert.Empty(t, *asked)
	})
}

func TestCoalitionStructureKillmails(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	zkill := &filteredZkill{lists: map[string][]api.ZkbResponse{
		"kills/groupID/1657/allianceID/99000001":     {WholeZkillmail(30, now)},
		"losses/groupID/1657/corporationID/98000009": {WholeZkillmail(40, now)},
		"losses/groupID/1657/corporationID/98000002": {WholeZkillmail(41, now)},
	}}
	cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
	target := retrieval.IntelTarget{IDNumber: -1700000000000, TypeOfEntity: types.COALITION, Name: "Coalition",
		Holdings: []retrieval.TargetGroup{
			{Type: types.ALLIANCE, ID: 99000001, Name: "Alpha", Member: true},
			{Type: types.CORPORATION, ID: 98000002, Name: "Beta", Member: true},
			{Type: types.CORPORATION, ID: 98000009, Name: "Alpha Holdings"},
		},
		Clients: retrieval.CollectionClientPool{ZkillClient: api.ZkillConnection{Client: zkill}, ESIClient: cache}}

	found, err := retrieval.CollectStructureKillmails(target, retrieval.KillmailLimits{Max: 5})

	assert.NoError(t, err)
	assert.Len(t, found.Used, 1)
	assert.Equal(t, 99000001, found.Used[0].Via.ID)
	assert.Len(t, found.Lost, 2)
	for _, endpoint := range zkill.asked {
		assert.NotContains(t, endpoint, "-1700000000000", "the coalition's own id is never asked for")
		assert.NotContains(t, endpoint, "/-")
	}
}

func TestLocalIDsNeverReachTheESI(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	esi := &RawESIClient{}
	cache := retrieval.NewCache(esi, filepath.Join(t.TempDir(), "cache.db"))

	_, err := cache.GetIfNotCached(-5, types.ALLIANCE)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetAllianceInfo(-5)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetCorporationInfo(-5)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetEsiResponse(-5, types.CORPORATION)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetAllianceCorporations(-5)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetAllianceHistory(-5)
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetNamesFromEveIDs([]int{98000001, -5})
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetCharacterAffiliations([]int{-5})
	assert.ErrorIs(t, err, retrieval.ErrLocalID)
	_, err = cache.GetEntitiesFromEveIDs([]int{-5})
	assert.ErrorIs(t, err, retrieval.ErrLocalID)

	assert.Equal(t, 0, esi.total(), "nothing reached the ESI")
}
