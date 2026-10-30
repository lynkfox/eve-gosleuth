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

func TestImagesRoundTrip(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	path := filepath.Join(t.TempDir(), "cache.db")
	cache, err := retrieval.LoadCache(&RawESIClient{}, path)
	assert.NoError(t, err)

	_, found := cache.GetImage("https://x/a.png")
	assert.False(t, found)

	data := []byte{0x89, 'P', 'N', 'G', 0, 1, 2, 255}
	cache.SaveImage("https://x/a.png", data)
	got, found := cache.GetImage("https://x/a.png")
	assert.True(t, found)
	assert.Equal(t, data, got)

	assert.NoError(t, retrieval.SaveCache(cache))
	loaded, err := retrieval.LoadCache(&RawESIClient{}, path)
	assert.NoError(t, err)
	got, found = loaded.GetImage("https://x/a.png")
	assert.True(t, found)
	assert.Equal(t, data, got)

	// no time limit, but a newer SDE drops it
	assert.Equal(t, 1, loaded.UpdateSDERelease(time.Now().Add(time.Hour)))
	_, found = loaded.GetImage("https://x/a.png")
	assert.False(t, found)
}

func TestImagesClearAll(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	path := filepath.Join(t.TempDir(), "cache.db")
	cache, err := retrieval.LoadCache(&RawESIClient{}, path)
	assert.NoError(t, err)
	cache.SaveImage("u", []byte{1, 2, 3})
	assert.NoError(t, retrieval.SaveCache(cache))

	assert.Equal(t, 1, cache.ClearAll(false))
	_, found := cache.GetImage("u")
	assert.False(t, found)
	loaded, _ := retrieval.LoadCache(&RawESIClient{}, path)
	_, found = loaded.GetImage("u")
	assert.False(t, found)
}
