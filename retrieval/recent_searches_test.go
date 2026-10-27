package retrieval_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRecentSearches(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// searched adds the searches oldest first, a little apart so each has its own time, as the database keeps milliseconds
	const keep = retrieval.DefaultRecentSearches
	searched := func(cache *retrieval.CachedEveIDs, searches ...retrieval.RecentSearch) {
		for _, search := range searches {
			cache.AddRecentSearch(search, keep)
			time.Sleep(2 * time.Millisecond)
		}
	}
	reloaded := func(t *testing.T, cache *retrieval.CachedEveIDs) *retrieval.CachedEveIDs {
		assert.NoError(t, cache.Close())
		loaded, err := retrieval.LoadCache(&RawESIClient{}, cache.Path)
		assert.NoError(t, err)
		return loaded
	}
	jita := retrieval.RecentSearch{Name: "Jita", Type: "SYSTEM"}
	pilot := retrieval.RecentSearch{Name: "Luxxianna Seraph", Type: "PILOT"}

	t.Run("an empty cache has none", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))

		assert.Equal(t, []retrieval.RecentSearch{}, cache.RecentSearches(keep))
	})

	t.Run("they are newest first, and a repeated search moves to the front with its new spelling", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, retrieval.RecentSearch{Name: "jita", Type: "SYSTEM"}, pilot, jita)

		assert.Equal(t, []retrieval.RecentSearch{jita, pilot}, cache.RecentSearches(keep))
	})

	t.Run("the same name of another type is another search", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita, retrieval.RecentSearch{Name: "Jita", Type: "PILOT"})

		assert.Len(t, cache.RecentSearches(keep), 2)
	})

	t.Run("they are saved in the cache and loaded again", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita, pilot)

		assert.Equal(t, []retrieval.RecentSearch{pilot, jita}, reloaded(t, cache).RecentSearches(keep))
	})

	t.Run("a corp's or alliance's ticker is saved with it", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		ursa := retrieval.RecentSearch{Name: "URSA.", Type: "ALLIANCE", Ticker: "URSA."}
		searched(cache, ursa)

		assert.Equal(t, []retrieval.RecentSearch{ursa}, reloaded(t, cache).RecentSearches(keep))
	})

	t.Run("past the limit the oldest are dropped, on disk too", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		for i := range keep + 3 {
			searched(cache, retrieval.RecentSearch{Name: fmt.Sprintf("Pilot %d", i), Type: "PILOT"})
		}

		for _, recent := range [][]retrieval.RecentSearch{cache.RecentSearches(keep), reloaded(t, cache).RecentSearches(keep)} {
			assert.Len(t, recent, keep)
			assert.Equal(t, fmt.Sprintf("Pilot %d", keep+2), recent[0].Name)
			assert.Equal(t, "Pilot 3", recent[len(recent)-1].Name)
		}
	})

	t.Run("lowering the number kept drops the oldest, on disk too", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita, pilot)

		assert.Equal(t, []retrieval.RecentSearch{pilot}, cache.RecentSearches(1), "a lower limit only shows the newest")
		cache.TrimRecentSearches(1)
		assert.Equal(t, []retrieval.RecentSearch{pilot}, reloaded(t, cache).RecentSearches(keep), "and once trimmed the rest are gone")
	})

	t.Run("a removed search is gone, on disk too, and removing one not there changes nothing", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita, pilot)
		assert.NoError(t, retrieval.SaveCache(cache))

		cache.RemoveRecentSearch(retrieval.RecentSearch{Name: "JITA", Type: "SYSTEM"})
		cache.RemoveRecentSearch(retrieval.RecentSearch{Name: "Jita", Type: "PILOT"})

		assert.Equal(t, []retrieval.RecentSearch{pilot}, cache.RecentSearches(keep))
		assert.Equal(t, []retrieval.RecentSearch{pilot}, reloaded(t, cache).RecentSearches(keep))
	})

	t.Run("a new SDE and clearing pilot data keep them, clearing everything removes them", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita)

		cache.UpdateSDERelease(time.Now().Add(time.Hour))
		cache.ClearEntities()
		assert.Equal(t, []retrieval.RecentSearch{jita}, cache.RecentSearches(keep))

		cache.ClearAll(false)
		assert.Equal(t, []retrieval.RecentSearch{}, cache.RecentSearches(keep))
		assert.Equal(t, []retrieval.RecentSearch{}, reloaded(t, cache).RecentSearches(keep))
	})

	t.Run("clearing older data removes searches made before the cutoff", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		searched(cache, jita)
		cutoff := time.Now()
		searched(cache, pilot)

		cache.ClearOlderThan(cutoff, false)
		assert.Equal(t, []retrieval.RecentSearch{pilot}, reloaded(t, cache).RecentSearches(keep))
	})
}
