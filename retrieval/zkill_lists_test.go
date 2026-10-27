package retrieval_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

// fakeZkill serves an entity's killmail list in pages of 200, as zKillboard's api does, and records the pages asked for
type fakeZkill struct {
	mu    sync.Mutex
	list  []api.ZkbResponse // newest first
	pages []int
}

func (f *fakeZkill) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var page int
	if _, err := fmt.Sscanf(endpoint, "corporationID/98000001/page/%d/", &page); err != nil {
		return nil, err
	}
	f.pages = append(f.pages, page)
	start := min((page-1)*api.ZKILL_PAGE_SIZE, len(f.list))
	data, err := json.Marshal(f.list[start:min(start+api.ZKILL_PAGE_SIZE, len(f.list))])
	if err != nil {
		return nil, err
	}
	var gzipped bytes.Buffer
	writer := gzip.NewWriter(&gzipped)
	writer.Write(data)
	writer.Close()
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&gzipped)}, nil
}

func (f *fakeZkill) UpdateHeaders(map[string]string) {}

// asked returns the pages asked for since it was last called
func (f *fakeZkill) asked() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	pages := f.pages
	f.pages = nil
	return pages
}

// prepend puts newer killmails at the top of the list, as zKillboard does when they happen
func (f *fakeZkill) prepend(killmails []api.ZkbResponse) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.list = append(append([]api.ZkbResponse{}, killmails...), f.list...)
}

func TestCachedZkillLists(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	// whole killmails, the newest first, an hour apart from hoursBack, with ids counting down from first
	killmails := func(first, count, hoursBack int) []api.ZkbResponse {
		list := []api.ZkbResponse{}
		for index := range count {
			list = append(list, WholeZkillmail(first-index, now.Add(-time.Duration(hoursBack+index)*time.Hour)))
		}
		return list
	}
	setup := func(list []api.ZkbResponse) (*fakeZkill, retrieval.IntelTarget, *RawESIClient) {
		zkill := &fakeZkill{list: list}
		raw := &RawESIClient{}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cache.db"))
		return zkill, retrieval.IntelTarget{IDNumber: 98000001, TypeOfEntity: types.CORPORATION,
			Clients: retrieval.CollectionClientPool{ZkillClient: api.ZkillConnection{Client: zkill}, ESIClient: cache}}, raw
	}
	collect := func(target retrieval.IntelTarget, limits retrieval.KillmailLimits) []int {
		collected, err := retrieval.CollectKillmails(target, limits)
		assert.NoError(t, err)
		ids := []int{}
		for _, km := range *collected {
			ids = append(ids, km.ESIKillmail.KillmailID)
		}
		return ids
	}
	ids := func(list []api.ZkbResponse) []int {
		listed := []int{}
		for _, km := range list {
			listed = append(listed, km.KillmailID)
		}
		return listed
	}

	t.Run("a second search reads only page 1, the cached list giving the rest", func(t *testing.T) {
		zkill, target, raw := setup(killmails(100000, 1000, 3)) // older than the three new ones added below
		all := retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: 10}

		assert.Len(t, collect(target, all), 1000)
		assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, zkill.asked(), "page 6 is empty, the end of the list")

		zkill.prepend(killmails(100003, 3, 0))
		got := collect(target, all)
		assert.Equal(t, []int{1}, zkill.asked())
		assert.Equal(t, ids(zkill.list), got, "the three new killmails, then the thousand known")
		assert.Zero(t, raw.calls["killmail"], "every killmail came from zKillboard's pages or the cache")
	})

	t.Run("a search needing more than the cached list has carries on paging below it", func(t *testing.T) {
		zkill, target, _ := setup(killmails(100000, 1000, 1))

		assert.Len(t, collect(target, retrieval.KillmailLimits{Max: 200}), 200)
		assert.Equal(t, []int{1}, zkill.asked())

		got := collect(target, retrieval.KillmailLimits{Max: 600})
		assert.Equal(t, []int{1, 2, 3}, zkill.asked(), "page 1 reaches the cached list, pages 2 and 3 carry on below it")
		assert.Equal(t, ids(zkill.list[:600]), got)
	})

	t.Run("a gap left between new killmails and the cached list is paged over, not skipped", func(t *testing.T) {
		zkill, target, _ := setup(killmails(100000, 1000, 500))
		collect(target, retrieval.KillmailLimits{Max: 200})
		zkill.prepend(killmails(100500, 500, 0)) // more than a page of new killmails
		collect(target, retrieval.KillmailLimits{Max: 200})
		zkill.asked()

		got := collect(target, retrieval.KillmailLimits{Max: 1000})
		assert.Equal(t, ids(zkill.list[:1000]), got, "the 300 new killmails the second search did not read are there, in order")
	})

	t.Run("paging stops at the page reaching the date limit", func(t *testing.T) {
		zkill, target, _ := setup(killmails(100000, 1000, 1))
		newerThan := now.Add(-300 * time.Hour)

		got := collect(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: 10, NewerThan: newerThan})
		assert.Equal(t, []int{1, 2}, zkill.asked(), "page 2 reaches 300 hours back")
		assert.Len(t, got, 299)
	})

	t.Run("with no killmail inside the date limit, dropping it reads the list further back", func(t *testing.T) {
		zkill, target, _ := setup(killmails(100000, 1000, 24*60))
		asked := 0
		limits := retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: 10, NewerThan: now.AddDate(0, 0, -7),
			ConfirmIgnoreDate: func(time.Time) bool { asked++; return true }}

		got := collect(target, limits)
		assert.Equal(t, 1, asked)
		assert.Len(t, got, 1000)
		assert.Equal(t, []int{1, 1, 2, 3, 4, 5, 6}, zkill.asked(), "page 1 reaches the date, then page 1 again reaches the cached list and the rest carry on below it")

		_, kept, _ := setup(killmails(100000, 1000, 24*60))
		limits.ConfirmIgnoreDate = func(time.Time) bool { return false }
		assert.Empty(t, collect(kept, limits), "kept the date limit")
	})

	t.Run("cached killmails past the limits are used only with IncludeCached", func(t *testing.T) {
		zkill, target, _ := setup(killmails(100000, 1000, 1))
		collect(target, retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: 10})
		zkill.asked()

		assert.Len(t, collect(target, retrieval.KillmailLimits{Max: 20}), 20)
		got := collect(target, retrieval.KillmailLimits{Max: 20, IncludeCached: true})
		assert.Equal(t, []int{1, 1}, zkill.asked())
		assert.Equal(t, ids(zkill.list), got, "the twenty, then every older killmail cached, newest first")
	})
}
