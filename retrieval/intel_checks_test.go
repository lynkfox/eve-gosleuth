package retrieval_test

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestIntelChecks(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("checks are saved each once, kept across sessions, and forgotten when done", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cache.db")
		cache := retrieval.NewCache(&RawESIClient{}, path)
		ursa := retrieval.IntelCheck{Type: "ALLIANCE", ID: 99013330}
		primus := retrieval.IntelCheck{Type: "CORPORATION", ID: 98744491}
		cache.AddIntelChecks([]retrieval.IntelCheck{ursa, primus, ursa})
		assert.NoError(t, retrieval.SaveCache(cache))
		assert.NoError(t, cache.Close())

		loaded, err := retrieval.LoadCache(&RawESIClient{}, path)
		assert.NoError(t, err)
		assert.Equal(t, []retrieval.IntelCheck{ursa, primus}, loaded.IntelChecks())

		loaded.RemoveIntelChecks([]retrieval.IntelCheck{ursa})
		assert.Equal(t, []retrieval.IntelCheck{primus}, loaded.IntelChecks())
		assert.NoError(t, loaded.Close())

		again, err := retrieval.LoadCache(&RawESIClient{}, path)
		assert.NoError(t, err)
		assert.Equal(t, []retrieval.IntelCheck{primus}, again.IntelChecks(), "removed from the database too")
	})

	t.Run("clearing the cache keeps them, unless clearing what the user set by hand", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.AddIntelChecks([]retrieval.IntelCheck{{Type: "ALLIANCE", ID: 99013330}})

		cache.ClearAll(false)
		assert.Len(t, cache.IntelChecks(), 1)
		cache.ClearAll(true)
		assert.Empty(t, cache.IntelChecks())
	})
}
