package retrieval_test

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// MockZkillClient embeds the interface to satisfy its unexported methods, only GetZkillResults is implemented
type MockZkillClient struct {
	api.ZkillClient
	killmails []api.ZkbResponse
	err       error
	paging    *api.ZkillPaging          // when set, records the paging it was asked for
	byID      map[int][]api.ZkbResponse // when set, each entity's own list, and killmails the list for any other
	errByID   map[int]error
}

// GetZkillResults answers with the whole list, as if it were all of zKillboard's, whatever the paging
func (m MockZkillClient) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	if m.paging != nil {
		*m.paging = paging
	}
	if list, found := m.byID[entityId]; found {
		err := m.errByID[entityId]
		return api.ZkillList{Killmails: list, Ended: err == nil}, err
	}
	return api.ZkillList{Killmails: m.killmails, Ended: m.err == nil}, m.err
}

// MockESIClient embeds the interface, only GetKillmail is implemented
type MockESIClient struct {
	api.ESIClient
	failingIDs map[int]error
}

func (m MockESIClient) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	if err, found := m.failingIDs[zkillmail.KillmailID]; found {
		return nil, err
	}
	return &api.ESIKillmail{KillmailID: zkillmail.KillmailID}, nil
}

func CreateZkillmails(ids ...int) []api.ZkbResponse {
	killmails := []api.ZkbResponse{}
	for _, id := range ids {
		killmails = append(killmails, api.ZkbResponse{KillmailID: id})
	}
	return killmails
}

func CreateTarget(zkill MockZkillClient, esi MockESIClient) retrieval.IntelTarget {
	return retrieval.IntelTarget{
		IDNumber:     12345,
		TypeOfEntity: types.PILOT,
		Clients: retrieval.CollectionClientPool{
			ZkillClient: zkill,
			ESIClient:   esi,
		},
	}
}

func TestCollectKillmails(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("all killmails retrieved", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3)}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.NoError(t, err)
		assert.Equal(t, 3, len(*killmails))
		for index, km := range *killmails {
			assert.Equal(t, index+1, km.Zkillmail.KillmailID)
			assert.Equal(t, index+1, km.ESIKillmail.KillmailID)
		}
	})

	t.Run("partial zkill pull returns collected killmails and error", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2), err: api.ErrOnlyPartialZkillPull}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.Equal(t, 2, len(*killmails))
	})

	t.Run("failed esi killmail is skipped", func(t *testing.T) {
		esiErr := errors.New("esi failure")
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3)}, MockESIClient{failingIDs: map[int]error{2: esiErr}})

		killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
		assert.ErrorIs(t, err, esiErr)
		assert.Equal(t, 2, len(*killmails))
		assert.Equal(t, 1, (*killmails)[0].ESIKillmail.KillmailID)
		assert.Equal(t, 3, (*killmails)[1].ESIKillmail.KillmailID)
	})

	t.Run("partial zkill pull and failed esi killmail are both reported", func(t *testing.T) {
		esiErr := errors.New("esi failure")
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2), err: api.ErrOnlyPartialZkillPull}, MockESIClient{failingIDs: map[int]error{1: esiErr}})

		killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.ErrorIs(t, err, esiErr)
		assert.Equal(t, 1, len(*killmails))
		assert.Equal(t, 2, (*killmails)[0].ESIKillmail.KillmailID)
	})

	t.Run("no killmails found", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: []api.ZkbResponse{}}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.NoError(t, err)
		assert.Empty(t, *killmails)
	})
}

// DatedESIClient serves killmails with set times and can say which it already holds, like the cache. It records which killmails it was asked to fetch, and which to keep.
type DatedESIClient struct {
	api.ESIClient
	times   map[int]time.Time
	cached  map[int]bool
	mu      *sync.Mutex // killmails are fetched in parallel
	fetched *[]int
	stored  *[]int
	failing map[int]error
	lookups *[]int // the killmails the cache was asked whether it holds
}

func (m DatedESIClient) StoreKillmail(killmail *api.ESIKillmail) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stored != nil {
		*m.stored = append(*m.stored, killmail.KillmailID)
	}
}

func (m DatedESIClient) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.fetched = append(*m.fetched, zkillmail.KillmailID)
	if err, found := m.failing[zkillmail.KillmailID]; found {
		return nil, err
	}
	return &api.ESIKillmail{KillmailID: zkillmail.KillmailID, KillmailTime: m.times[zkillmail.KillmailID]}, nil
}

func (m DatedESIClient) CachedKillmail(killmailID int) *api.ESIKillmail {
	if m.lookups != nil {
		m.mu.Lock()
		*m.lookups = append(*m.lookups, killmailID)
		m.mu.Unlock()
	}
	if !m.cached[killmailID] {
		return nil
	}
	return &api.ESIKillmail{KillmailID: killmailID, KillmailTime: m.times[killmailID]}
}

func TestCollectKillmailsLimits(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	// zKillboard lists newest first: killmails 1 and 2 are this week, 3 to 5 are months old
	times := map[int]time.Time{1: now.AddDate(0, 0, -1), 2: now.AddDate(0, 0, -3), 3: now.AddDate(0, -2, 0), 4: now.AddDate(0, -3, 0), 5: now.AddDate(0, -4, 0)}

	collect := func(limits retrieval.KillmailLimits, cached ...int) (ids []int, fetched []int) {
		esi := DatedESIClient{times: times, cached: map[int]bool{}, mu: &sync.Mutex{}, fetched: &fetched}
		for _, id := range cached {
			esi.cached[id] = true
		}
		target := retrieval.IntelTarget{IDNumber: 12345, TypeOfEntity: types.PILOT, Clients: retrieval.CollectionClientPool{
			ZkillClient: MockZkillClient{killmails: CreateZkillmails(1, 2, 3, 4, 5)},
			ESIClient:   esi,
		}}

		killmails, err := retrieval.CollectKillmails(target, limits)
		assert.NoError(t, err)
		esi.mu.Lock()
		defer esi.mu.Unlock()
		slices.Sort(fetched)
		for _, km := range *killmails {
			ids = append(ids, km.ESIKillmail.KillmailID)
		}
		return ids, fetched
	}

	t.Run("collects at most Max killmails, newest first", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 2})

		assert.Equal(t, []int{1, 2}, ids)
		assert.Equal(t, []int{1, 2}, fetched)
	})

	t.Run("AllKillmails collects every killmail listed", func(t *testing.T) {
		ids, _ := collect(retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.Equal(t, []int{1, 2, 3, 4, 5}, ids)
	})

	t.Run("AllKillmails still stops at the first killmail older than NewerThan", func(t *testing.T) {
		ids, _ := collect(retrieval.KillmailLimits{Max: retrieval.AllKillmails, NewerThan: now.AddDate(0, 0, -7)})

		assert.Equal(t, []int{1, 2}, ids)
	})

	t.Run("stops at the first killmail older than NewerThan", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 20, NewerThan: now.AddDate(0, 0, -7)})

		assert.Equal(t, []int{1, 2}, ids)
		assert.Subset(t, fetched, []int{1, 2, 3}, "killmail 3 is fetched to find its date, and the older ones may be fetched alongside it")
	})

	t.Run("when even the newest killmail is older than NewerThan, it asks to drop the date limit", func(t *testing.T) {
		var asked time.Time
		ids, _ := collect(retrieval.KillmailLimits{Max: 3, NewerThan: now.AddDate(0, 0, -2), ConfirmIgnoreDate: func(time.Time) bool { t.Error("asked"); return true }})
		assert.Equal(t, []int{1}, ids, "a newer killmail first never asks")

		ids, _ = collect(retrieval.KillmailLimits{Max: 3, NewerThan: now, ConfirmIgnoreDate: func(newest time.Time) bool { asked = newest; return false }})
		assert.Empty(t, ids, "kept the date limit")
		assert.Equal(t, times[1], asked, "asked with the newest killmail's time")

		ids, _ = collect(retrieval.KillmailLimits{Max: 3, NewerThan: now, ConfirmIgnoreDate: func(time.Time) bool { return true }})
		assert.Equal(t, []int{1, 2, 3}, ids, "dropped the date limit, Max still applies")
	})

	t.Run("with IncludeCached, cached killmails beyond Max are used without fetching them", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 2, IncludeCached: true}, 4)

		assert.Equal(t, []int{1, 2, 4}, ids)
		assert.Equal(t, []int{1, 2}, fetched)
	})

	t.Run("with IncludeCached, cached killmails older than NewerThan are used", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 20, NewerThan: now.AddDate(0, 0, -7), IncludeCached: true}, 3, 5)

		assert.Equal(t, []int{1, 2, 3, 5}, ids)
		assert.Subset(t, fetched, []int{1, 2})
		assert.NotContains(t, fetched, 3, "the cache's copy gives its date")
	})

	t.Run("without IncludeCached, cached killmails beyond the limits are left out", func(t *testing.T) {
		ids, _ := collect(retrieval.KillmailLimits{Max: 2}, 3, 4, 5)

		assert.Equal(t, []int{1, 2}, ids)
	})
}

func TestCollectKillmailsRateLimited(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	limited := fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, api.ErrRateLimited)
	target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3, 4, 5)}, MockESIClient{failingIDs: map[int]error{3: limited, 4: limited, 5: limited}})

	killmails, err := retrieval.CollectKillmails(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

	assert.ErrorIs(t, err, api.ErrRateLimited)
	assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
	assert.Len(t, *killmails, 2, "the killmails the ESI rate limited are skipped")
}

func TestCollectKillmailsStopped(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	stopped := fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, api.ErrStopped)
	target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3, 4)}, MockESIClient{failingIDs: map[int]error{2: stopped, 3: stopped, 4: stopped}})

	killmails, err := retrieval.CollectKillmails(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

	assert.ErrorIs(t, err, api.ErrStopped, "a stopped search collects no more")
	assert.Len(t, *killmails, 1, "with the killmails collected before it")
}

// WholeZkillmail is a zKillboard list entry with the whole killmail, as zKillboard sends them
func WholeZkillmail(id int, at time.Time) api.ZkbResponse {
	return api.ZkbResponse{KillmailID: id, KillmailTime: at, SolarSystemID: 30000142, Victim: api.ESIVictim{ShipTypeID: 587}, Attackers: []api.ESIAttacker{{ShipTypeID: 11202}}}
}

func TestCollectKillmailsFromZkill(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	times := map[int]time.Time{1: now.AddDate(0, 0, -1), 2: now.AddDate(0, 0, -2), 3: now.AddDate(0, 0, -3), 4: now.AddDate(0, -2, 0)}

	collect := func(list []api.ZkbResponse, limits retrieval.KillmailLimits, failing map[int]error, cached ...int) (ids, fetched, stored []int, err error) {
		esi := DatedESIClient{times: times, cached: map[int]bool{}, mu: &sync.Mutex{}, fetched: &fetched, stored: &stored, failing: failing}
		for _, id := range cached {
			esi.cached[id] = true
		}
		target := retrieval.IntelTarget{IDNumber: 12345, TypeOfEntity: types.PILOT, Clients: retrieval.CollectionClientPool{
			ZkillClient: MockZkillClient{killmails: list},
			ESIClient:   esi,
		}}

		killmails, err := retrieval.CollectKillmails(target, limits)
		esi.mu.Lock()
		defer esi.mu.Unlock()
		for _, km := range *killmails {
			ids = append(ids, km.ESIKillmail.KillmailID)
		}
		return ids, fetched, stored, err
	}

	t.Run("killmails zKillboard sent whole are read from it and cached, only one sent in part is fetched from the ESI", func(t *testing.T) {
		list := []api.ZkbResponse{WholeZkillmail(1, times[1]), {KillmailID: 2}, WholeZkillmail(3, times[3])}

		ids, fetched, stored, err := collect(list, retrieval.KillmailLimits{Max: retrieval.AllKillmails}, nil)

		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, ids)
		assert.Equal(t, []int{2}, fetched)
		assert.Equal(t, []int{1, 3}, stored)
	})

	t.Run("the cache's copy is used before zKillboard's, every killmail sent whole being cached as it is listed", func(t *testing.T) {
		list := []api.ZkbResponse{WholeZkillmail(1, times[1]), WholeZkillmail(2, times[2])}

		ids, fetched, stored, err := collect(list, retrieval.KillmailLimits{Max: retrieval.AllKillmails}, nil, 2)

		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2}, ids)
		assert.Empty(t, fetched)
		assert.Equal(t, []int{1, 2}, stored)
	})

	t.Run("the date limit is read from zKillboard's copies, and those past it are cached for later searches", func(t *testing.T) {
		list := []api.ZkbResponse{WholeZkillmail(1, times[1]), WholeZkillmail(2, times[2]), WholeZkillmail(4, times[4])}

		ids, fetched, stored, err := collect(list, retrieval.KillmailLimits{Max: retrieval.AllKillmails, NewerThan: now.AddDate(0, 0, -7)}, nil)

		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2}, ids)
		assert.Empty(t, fetched)
		assert.Equal(t, []int{1, 2, 4}, stored)
	})

	t.Run("after a 429, killmails zKillboard sent whole are still used", func(t *testing.T) {
		limited := fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, api.ErrRateLimited)
		list := []api.ZkbResponse{WholeZkillmail(1, times[1]), {KillmailID: 2}, WholeZkillmail(3, times[3])}

		ids, _, _, err := collect(list, retrieval.KillmailLimits{Max: retrieval.AllKillmails}, map[int]error{2: limited})

		assert.ErrorIs(t, err, api.ErrRateLimited)
		assert.Equal(t, []int{1, 3}, ids)
	})
}

func TestCollectKillmailsPaging(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	paged := func(limits retrieval.KillmailLimits) api.ZkillPaging {
		var paging api.ZkillPaging
		target := CreateTarget(MockZkillClient{paging: &paging}, MockESIClient{})
		_, err := retrieval.CollectKillmails(target, limits)
		assert.NoError(t, err)
		return paging
	}

	t.Run("a max count pages until that many are listed, as far as zKillboard serves", func(t *testing.T) {
		assert.Equal(t, api.ZkillPaging{Killmails: 5000, MaxPages: api.ZKILL_MAX_PAGES}, paged(retrieval.KillmailLimits{Max: 5000, Pages: 3}))
	})

	t.Run("all killmails pages as far as Pages, or DefaultKillmailPages", func(t *testing.T) {
		assert.Equal(t, api.ZkillPaging{MaxPages: 40}, paged(retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: 40}))
		assert.Equal(t, api.ZkillPaging{MaxPages: retrieval.DefaultKillmailPages}, paged(retrieval.KillmailLimits{Max: retrieval.AllKillmails}))
	})

	t.Run("the date limit goes to zKillboard's paging, so it stops at the page reaching it", func(t *testing.T) {
		newerThan := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, api.ZkillPaging{Killmails: 20, MaxPages: api.ZKILL_MAX_PAGES, NewerThan: newerThan}, paged(retrieval.KillmailLimits{Max: 20, NewerThan: newerThan}))
	})
}

func TestCollectKillmailsWithHoldings(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	hours := func(n int) time.Time { return now.Add(-time.Duration(n) * time.Hour) }
	holdingCorp := retrieval.TargetGroup{Type: types.CORPORATION, ID: 98000002, Name: "Holding Corp"}
	holdingAlliance := retrieval.TargetGroup{Type: types.ALLIANCE, ID: 99000003, Name: "Holding Alliance"}
	zkill := MockZkillClient{byID: map[int][]api.ZkbResponse{
		99000001: {WholeZkillmail(10, hours(1)), WholeZkillmail(11, hours(4)), WholeZkillmail(12, hours(8))},
		98000002: {WholeZkillmail(20, hours(2)), WholeZkillmail(11, hours(4)), WholeZkillmail(21, hours(6))},
		99000003: {WholeZkillmail(30, hours(3))},
	}}
	target := func(holdings ...retrieval.TargetGroup) retrieval.IntelTarget {
		return retrieval.IntelTarget{IDNumber: 99000001, TypeOfEntity: types.ALLIANCE, Holdings: holdings,
			Clients: retrieval.CollectionClientPool{ZkillClient: zkill, ESIClient: MockESIClient{}}}
	}
	collect := func(target retrieval.IntelTarget, limits retrieval.KillmailLimits) (ids []int, via map[int]string, err error) {
		killmails, err := retrieval.CollectKillmails(target, limits)
		via = map[int]string{}
		for _, km := range *killmails {
			ids = append(ids, km.ESIKillmail.KillmailID)
			if km.Via != nil {
				via[km.ESIKillmail.KillmailID] = km.Via.Name
			}
		}
		return ids, via, err
	}

	t.Run("the holdings' killmails are merged in newest first, each once, one on both lists being the target's", func(t *testing.T) {
		ids, via, err := collect(target(holdingCorp, holdingAlliance), retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.NoError(t, err)
		assert.Equal(t, []int{10, 20, 30, 11, 21, 12}, ids)
		assert.Equal(t, map[int]string{20: "Holding Corp", 21: "Holding Corp", 30: "Holding Alliance"}, via)
	})

	t.Run("Max and the date limit apply to the merged list", func(t *testing.T) {
		ids, _, _ := collect(target(holdingCorp, holdingAlliance), retrieval.KillmailLimits{Max: 3})
		assert.Equal(t, []int{10, 20, 30}, ids)

		ids, _, _ = collect(target(holdingCorp, holdingAlliance), retrieval.KillmailLimits{Max: retrieval.AllKillmails, NewerThan: hours(5)})
		assert.Equal(t, []int{10, 20, 30, 11}, ids)
	})

	t.Run("with holdings each list's progress says whose it is", func(t *testing.T) {
		var asked []string
		recording := target(holdingCorp)
		recording.Expanded = &retrieval.Alliance{AllianceID: 99000001, Name: "Main Alliance"}
		recording.Clients.ZkillClient = forClient{MockZkillClient: zkill, asked: &asked}

		_, _, err := collect(recording, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.NoError(t, err)
		assert.Equal(t, []string{"Main Alliance (main)", "holding Holding Corp"}, asked)
	})

	t.Run("lists zKillboard sent without times are merged by killmail id", func(t *testing.T) {
		untimed := target(holdingCorp)
		untimed.Clients.ZkillClient = MockZkillClient{byID: map[int][]api.ZkbResponse{
			99000001: CreateZkillmails(30, 10),
			98000002: CreateZkillmails(40, 20),
		}}

		ids, via, err := collect(untimed, retrieval.KillmailLimits{Max: 3})

		assert.NoError(t, err)
		assert.Equal(t, []int{40, 30, 20}, ids, "the holding's newest are not left behind the target's whole list")
		assert.Equal(t, map[int]string{40: "Holding Corp", 20: "Holding Corp"}, via)
	})

	t.Run("without holdings only the target's list is used", func(t *testing.T) {
		ids, via, err := collect(target(), retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.NoError(t, err)
		assert.Equal(t, []int{10, 11, 12}, ids)
		assert.Empty(t, via)
	})

	t.Run("a holding's list failing still uses what was listed, and says which holding", func(t *testing.T) {
		failing := target(holdingCorp)
		failingZkill := zkill
		failingZkill.errByID = map[int]error{98000002: api.ErrOnlyPartialZkillPull}
		failing.Clients.ZkillClient = failingZkill

		ids, _, err := collect(failing, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.ErrorContains(t, err, "Holding Corp")
		assert.Equal(t, []int{10, 20, 11, 21, 12}, ids)
	})
}

// forClient records whose list each zKillboard pull says it is
type forClient struct {
	MockZkillClient
	asked *[]string
}

func (f forClient) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	*f.asked = append(*f.asked, paging.For)
	return f.MockZkillClient.GetZkillResults(entityId, entityType, paging)
}

func TestCollectKillmailsCacheLookups(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	lookedUp := func(limits retrieval.KillmailLimits) []int {
		var fetched, lookups []int
		esi := DatedESIClient{times: map[int]time.Time{}, cached: map[int]bool{}, mu: &sync.Mutex{}, fetched: &fetched, lookups: &lookups}
		// timed, as one without a time is looked up to find it
		now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
		list := []api.ZkbResponse{}
		for id := 1; id <= 5; id++ {
			list = append(list, api.ZkbResponse{KillmailID: id, KillmailTime: now.Add(-time.Duration(id) * time.Hour)})
		}
		target := CreateTarget(MockZkillClient{killmails: list}, MockESIClient{})
		target.Clients.ESIClient = esi
		_, err := retrieval.CollectKillmails(target, limits)
		assert.NoError(t, err)
		return lookups
	}

	t.Run("only the killmails within Max are looked up in the cache", func(t *testing.T) {
		assert.Equal(t, []int{1, 2}, lookedUp(retrieval.KillmailLimits{Max: 2}))
	})

	t.Run("with IncludeCached every listed killmail is", func(t *testing.T) {
		assert.Equal(t, []int{1, 2, 3, 4, 5}, lookedUp(retrieval.KillmailLimits{Max: 2, IncludeCached: true}))
	})
}

func TestCollectKillmailsESILimit(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	over := retrieval.MaxESIKillmails + 5
	ids := make([]int, over)
	for index := range ids {
		ids[index] = index + 1
	}
	// entries zKillboard sent without their killmails, each needing the ESI
	target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(ids...)}, MockESIClient{})

	killmails, err := retrieval.CollectKillmails(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails})

	assert.ErrorIs(t, err, retrieval.ErrTooManyESIKillmails)
	assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
	assert.Len(t, *killmails, retrieval.MaxESIKillmails, "the rest are skipped")
}
