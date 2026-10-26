package retrieval_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestClearCache(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// filled builds a cache holding a pilot, corp, alliance, item, system, killmail, name, and affiliation, all cached old, reloaded from disk when asked
	filled := func(t *testing.T, reload bool) *retrieval.CachedEveIDs {
		path := filepath.Join(t.TempDir(), "cache.json")
		cache := retrieval.NewCache(&RawESIClient{}, path)
		cache.Update(2121139046, &api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph"})
		cache.Update(98744491, &api.ESICorporation{CorporationID: 98744491, Name: "URSA PRIMUS"})
		cache.Update(99013330, &api.ESIAlliance{AllianceID: 99013330, Name: "URSA."})
		cache.Update(29988, &api.ESIItem{ItemId: 29988, Name: "Proteus"})
		cache.Update(31001861, &api.ESISystem{SystemID: 31001861, Name: "J105443"})
		cache.UpdateExpanded(2121139046, &retrieval.Pilot{CharacterID: 2121139046, Name: "Luxxianna Seraph"})
		cache.UpdateExpanded(98744491, &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS"})
		cache.UpdateExpanded(99013330, &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."})
		cache.UpdateExpanded(29988, &retrieval.Item{ItemID: 29988, Name: "Proteus"})
		_, _ = cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})
		_, _ = cache.GetNamesFromEveIDs([]int{2121139046})
		_, _ = cache.GetCharacterAffiliations([]int{2121139046})
		if !reload {
			return cache
		}
		assert.NoError(t, retrieval.SaveCache(cache))
		loaded, err := retrieval.LoadCache(&RawESIClient{}, path)
		assert.NoError(t, err)
		return loaded
	}

	t.Run("everything", func(t *testing.T) {
		cache := filled(t, false)
		assert.Equal(t, 12, cache.Entries())

		assert.Equal(t, 12, cache.ClearAll(false))
		assert.Equal(t, 0, cache.Entries())
	})

	for _, reload := range []bool{false, true} {
		t.Run("pilot, corp, and alliance data only, reloaded from disk "+map[bool]string{false: "no", true: "yes"}[reload], func(t *testing.T) {
			cache := filled(t, reload)

			// 3 raw, 3 expanded, and the affiliation
			assert.Equal(t, 7, cache.ClearEntities())

			assert.Equal(t, 5, cache.Entries(), "the item, system, killmail, and name are kept")
			assert.NotNil(t, cache.CachedKillmail(139416795))
			assert.NotNil(t, retrieval.FetchExpanded[retrieval.Item](cache, 29988))
		})
	}

	t.Run("older than a cutoff, killmails included", func(t *testing.T) {
		cache := filled(t, false)

		assert.Equal(t, 0, cache.ClearOlderThan(time.Now().UTC().Add(-time.Hour), false), "nothing is that old")
		assert.Equal(t, 12, cache.ClearOlderThan(time.Now().UTC().Add(time.Second), false))
		assert.Nil(t, cache.CachedKillmail(139416795))
	})
}
