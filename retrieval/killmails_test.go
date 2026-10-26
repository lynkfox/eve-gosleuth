package retrieval_test

import (
	"errors"
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
}

func (m MockZkillClient) GetZkillResults(entityId int, entityType types.EntityType) (*[]api.ZkbResponse, error) {
	return &m.killmails, m.err
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

// DatedESIClient serves killmails with set times and can say which it already holds, like the cache. It records which killmails it was asked to fetch.
type DatedESIClient struct {
	api.ESIClient
	times   map[int]time.Time
	cached  map[int]bool
	mu      *sync.Mutex // killmails are fetched in parallel
	fetched *[]int
}

func (m DatedESIClient) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	*m.fetched = append(*m.fetched, zkillmail.KillmailID)
	return &api.ESIKillmail{KillmailID: zkillmail.KillmailID, KillmailTime: m.times[zkillmail.KillmailID]}, nil
}

func (m DatedESIClient) CachedKillmail(killmailID int) *api.ESIKillmail {
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

	t.Run("stops at the first killmail older than NewerThan", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 20, NewerThan: now.AddDate(0, 0, -7)})

		assert.Equal(t, []int{1, 2}, ids)
		assert.Subset(t, fetched, []int{1, 2, 3}, "killmail 3 is fetched to find its date, and the older ones may be fetched alongside it")
	})

	t.Run("with IncludeCached, cached killmails beyond Max are used without fetching them", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 2, IncludeCached: true}, 4)

		assert.Equal(t, []int{1, 2, 4}, ids)
		assert.Equal(t, []int{1, 2}, fetched)
	})

	t.Run("with IncludeCached, cached killmails older than NewerThan are used", func(t *testing.T) {
		ids, fetched := collect(retrieval.KillmailLimits{Max: 20, NewerThan: now.AddDate(0, 0, -7), IncludeCached: true}, 3, 5)

		assert.Equal(t, []int{1, 2, 3, 5}, ids)
		assert.Subset(t, fetched, []int{1, 2, 3})
	})

	t.Run("without IncludeCached, cached killmails beyond the limits are left out", func(t *testing.T) {
		ids, _ := collect(retrieval.KillmailLimits{Max: 2}, 3, 4, 5)

		assert.Equal(t, []int{1, 2}, ids)
	})
}
