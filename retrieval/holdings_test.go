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

func TestHoldingLinks(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	main := func(holdingType string, holdingID int, holdingName string) retrieval.HoldingLink {
		return retrieval.HoldingLink{MainType: "ALLIANCE", MainID: 99000001, MainName: "Main Alliance", HoldingType: holdingType, HoldingID: holdingID, HoldingName: holdingName}
	}
	holdingAlliance := main("ALLIANCE", 99000009, "Main Holding")
	holdingCorp := main("CORPORATION", 98000009, "Capital Holdings")
	ids := func(links []retrieval.HoldingLink) []int {
		found := []int{}
		for _, link := range links {
			found = append(found, link.HoldingID)
		}
		return found
	}

	t.Run("a main can have several holdings, found from either end", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SaveHoldingLink(holdingAlliance)
		cache.SaveHoldingLink(holdingCorp)

		holdings, heldFor := cache.HoldingLinks("ALLIANCE", 99000001)
		assert.Equal(t, []int{98000009, 99000009}, ids(holdings), "by holding name")
		assert.Empty(t, heldFor)

		holdings, heldFor = cache.HoldingLinks("CORPORATION", 98000009)
		assert.Empty(t, holdings)
		if assert.Len(t, heldFor, 1) {
			assert.Equal(t, "Main Alliance", heldFor[0].MainName)
		}

		holdings, _ = cache.HoldingLinks("CORPORATION", 99000001)
		assert.Empty(t, holdings, "the same id of another type is another entity")
	})

	t.Run("saving the same link again replaces its note, and they are saved and loaded", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SaveHoldingLink(holdingCorp)
		noted := holdingCorp
		noted.Note = "supers parked here"
		cache.SaveHoldingLink(noted)
		assert.NoError(t, cache.Close())

		loaded, err := retrieval.LoadCache(&RawESIClient{}, cache.Path)
		assert.NoError(t, err)
		loaded.UpdateSDERelease(time.Now().Add(time.Hour))
		holdings, _ := loaded.HoldingLinks("ALLIANCE", 99000001)
		if assert.Len(t, holdings, 1) {
			assert.Equal(t, "supers parked here", holdings[0].Note)
			assert.False(t, holdings[0].SetAt.IsZero())
		}
	})

	t.Run("a link can be removed", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SaveHoldingLink(holdingAlliance)
		cache.SaveHoldingLink(holdingCorp)
		assert.NoError(t, retrieval.SaveCache(cache))

		cache.RemoveHoldingLink("ALLIANCE", 99000001, "CORPORATION", 98000009)

		holdings, _ := cache.HoldingLinks("ALLIANCE", 99000001)
		assert.Equal(t, []int{99000009}, ids(holdings))
	})

	t.Run("clears keep them unless told to include them", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		cache.SaveHoldingLink(holdingCorp)

		cache.ClearAll(false)
		cache.ClearOlderThan(time.Now().Add(time.Hour), false)
		holdings, _ := cache.HoldingLinks("ALLIANCE", 99000001)
		assert.Len(t, holdings, 1)

		cache.ClearOlderThan(time.Now().Add(time.Hour), true)
		holdings, _ = cache.HoldingLinks("ALLIANCE", 99000001)
		assert.Empty(t, holdings)
	})
}
