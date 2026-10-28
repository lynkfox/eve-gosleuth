package retrieval_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
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

// filteredZkill serves zKillboard lists by endpoint, ie losses/groupID/1657/systemID/31000001, a missing one empty, and records each page asked for
type filteredZkill struct {
	mu    sync.Mutex
	lists map[string][]api.ZkbResponse
	asked []string
}

func (f *filteredZkill) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, endpoint)
	list, page := endpoint, 0
	if at := strings.LastIndex(endpoint, "/page/"); at >= 0 {
		list = endpoint[:at]
		fmt.Sscanf(endpoint[at:], "/page/%d/", &page)
	}
	killmails := f.lists[list]
	start := min((page-1)*api.ZKILL_PAGE_SIZE, len(killmails))
	data, err := json.Marshal(killmails[start:min(start+api.ZKILL_PAGE_SIZE, len(killmails))])
	if err != nil {
		return nil, err
	}
	var gzipped bytes.Buffer
	writer := gzip.NewWriter(&gzipped)
	writer.Write(data)
	writer.Close()
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&gzipped)}, nil
}

func (f *filteredZkill) UpdateHeaders(map[string]string) {}

func TestStructureKillmails(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	setup := func(entityType types.EntityType, id int, lists map[string][]api.ZkbResponse) (*filteredZkill, retrieval.IntelTarget) {
		zkill := &filteredZkill{lists: lists}
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		return zkill, retrieval.IntelTarget{IDNumber: id, TypeOfEntity: entityType,
			Clients: retrieval.CollectionClientPool{ZkillClient: api.ZkillConnection{Client: zkill}, ESIClient: cache}}
	}

	t.Run("a system's last structure lost is the newest of the structure groups' losses, however old, the previous five any structure's", func(t *testing.T) {
		zkill, target := setup(types.SYSTEM, 31000001, map[string][]api.ZkbResponse{
			"losses/groupID/1657/systemID/31000001": {WholeZkillmail(10, now.AddDate(-3, 0, 0)), WholeZkillmail(7, now.AddDate(-6, 0, 0))},
			"losses/groupID/365/systemID/31000001":  {WholeZkillmail(11, now.AddDate(-2, 0, 0)), WholeZkillmail(9, now.AddDate(-4, 0, 0))},
			"losses/groupID/1025/systemID/31000001": {WholeZkillmail(20, now), WholeZkillmail(8, now.AddDate(-5, 0, 0)), WholeZkillmail(6, now.AddDate(-7, 0, 0))},
		})
		last, previous, err := retrieval.StructureLosses(target)
		assert.NoError(t, err)
		assert.Equal(t, 11, last.ESIKillmail.KillmailID, "the newer customs office lost is not the last structure")
		ids := []int{}
		for _, km := range previous {
			ids = append(ids, km.ESIKillmail.KillmailID)
		}
		assert.Equal(t, []int{20, 10, 9, 8, 7}, ids, "the five newest others, newest first")
		assert.Len(t, zkill.asked, len(comm.UpwellStructureGroups.IDNumbers())+len(comm.ControlTowerGroups.IDNumbers())+len(comm.MinorStructureGroups.IDNumbers()), "page 1 of each group")
	})

	t.Run("a system with no structure lost has none", func(t *testing.T) {
		_, target := setup(types.SYSTEM, 31000001, nil)
		last, previous, err := retrieval.StructureLosses(target)
		assert.NoError(t, err)
		assert.Nil(t, last)
		assert.Empty(t, previous)
	})

	t.Run("a corp's structure lists are each group's kills and losses, for it and its holdings, past the date limit but within Max", func(t *testing.T) {
		old := now.AddDate(-2, 0, 0)
		lists := map[string][]api.ZkbResponse{
			"kills/groupID/1657/corporationID/98000001":  {WholeZkillmail(30, now), WholeZkillmail(29, old), WholeZkillmail(28, old.AddDate(0, 0, -1))},
			"kills/groupID/426/corporationID/98000001":   {WholeZkillmail(30, now)}, // the same kill, a citadel and a battery on it
			"losses/groupID/1657/corporationID/98000002": {WholeZkillmail(40, old)},
		}
		zkill, target := setup(types.CORPORATION, 98000001, lists)
		target.Holdings = []retrieval.TargetGroup{{ID: 98000002, Type: types.CORPORATION, Name: "Holding"}}

		found, err := retrieval.CollectStructureKillmails(target, retrieval.KillmailLimits{Max: 2, NewerThan: now.AddDate(0, 0, -7)})
		assert.NoError(t, err)
		ids := func(raw []retrieval.RawKillmail) []int {
			listed := []int{}
			for _, km := range raw {
				listed = append(listed, km.ESIKillmail.KillmailID)
			}
			return listed
		}
		assert.Equal(t, []int{30, 29}, ids(found.Used), "Max keeps two, the date limit none, and a kill on two lists is kept once")
		assert.Equal(t, []int{40}, ids(found.Lost))
		assert.Equal(t, 98000002, found.Lost[0].Via.ID)

		asked := slices.ContainsFunc(zkill.asked, func(endpoint string) bool { return strings.HasPrefix(endpoint, "kills/groupID/365/") })
		assert.False(t, asked, "a control tower is never on a kill, its batteries are listed instead")
	})
}
