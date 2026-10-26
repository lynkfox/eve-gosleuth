package retrieval_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestIgnoredAllies(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	friends := retrieval.IgnoredAlly{Type: "ALLIANCE", ID: 99000002, Name: "Friends"}
	loneCorp := retrieval.IgnoredAlly{Type: "CORPORATION", ID: 98000005, Name: "Lone Corp"}

	t.Run("a target with none saved has none", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))

		assert.Equal(t, []retrieval.IgnoredAlly{}, cache.IgnoredAlliesFor("PILOT", 1))
	})

	t.Run("they are kept per target, saved in the cache, and survive an SDE release", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SetIgnoredAllies("PILOT", 1, []retrieval.IgnoredAlly{friends, loneCorp})
		cache.SetIgnoredAllies("ALLIANCE", 1, []retrieval.IgnoredAlly{loneCorp})
		assert.NoError(t, cache.Close())

		loaded, err := retrieval.LoadCache(&RawESIClient{}, cache.Path)
		assert.NoError(t, err)
		loaded.UpdateSDERelease(time.Now().Add(time.Hour))

		assert.Equal(t, []retrieval.IgnoredAlly{friends, loneCorp}, loaded.IgnoredAlliesFor("PILOT", 1))
		assert.Equal(t, []retrieval.IgnoredAlly{loneCorp}, loaded.IgnoredAlliesFor("ALLIANCE", 1), "the same id of another type is another target")
	})

	t.Run("saving replaces them, and none clears them", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SetIgnoredAllies("PILOT", 1, []retrieval.IgnoredAlly{friends, loneCorp})
		cache.SetIgnoredAllies("PILOT", 1, []retrieval.IgnoredAlly{loneCorp})
		assert.Equal(t, []retrieval.IgnoredAlly{loneCorp}, cache.IgnoredAlliesFor("PILOT", 1))

		cache.SetIgnoredAllies("PILOT", 1, nil)
		assert.Equal(t, []retrieval.IgnoredAlly{}, cache.IgnoredAlliesFor("PILOT", 1))
	})

	t.Run("clears keep them unless told to include them", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SetIgnoredAllies("PILOT", 1, []retrieval.IgnoredAlly{friends})

		cache.ClearAll(false)
		cache.ClearOlderThan(time.Now().Add(time.Hour), false)
		assert.Equal(t, []retrieval.IgnoredAlly{friends}, cache.IgnoredAlliesFor("PILOT", 1))

		cache.ClearAll(true)
		assert.Equal(t, []retrieval.IgnoredAlly{}, cache.IgnoredAlliesFor("PILOT", 1))
	})
}
