package gui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestShutdown(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// newApp has an empty cache in its own folder, returning its path to read it back after
	newApp := func(t *testing.T) (*App, string) {
		path := retrieval.CachePathIn(t.TempDir())
		cache := retrieval.NewCache(api.NewESIClient(), path)
		return &App{cache: cache, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings()}, path
	}
	character := &api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph"}
	// savedEntries is how many entries the cache file at path holds
	savedEntries := func(t *testing.T, path string) int {
		cache, err := retrieval.LoadCache(api.NewESIClient(), path)
		assert.NoError(t, err)
		defer cache.Close()
		return cache.Entries()
	}

	t.Run("with no search running it saves the cache and stops nothing", func(t *testing.T) {
		app, path := newApp(t)
		app.cache.Update(character.CharacterID, character)

		app.shutdown(context.Background())

		assert.False(t, app.stopRequested.Load())
		assert.Equal(t, 1, savedEntries(t, path))
	})

	t.Run("a running search is stopped and waited for, so what it cached on the way out is saved", func(t *testing.T) {
		app, path := newApp(t)
		started := make(chan struct{})
		go func() {
			app.mu.Lock()
			defer app.mu.Unlock()
			close(started)
			for !app.stopRequested.Load() {
				time.Sleep(time.Millisecond)
			}
			app.cache.Update(character.CharacterID, character)
		}()
		<-started

		app.shutdown(context.Background())

		assert.True(t, app.stopRequested.Load())
		assert.Equal(t, 1, savedEntries(t, path))
	})

	t.Run("a search that does not stop in time is left behind, and the cache saved without it", func(t *testing.T) {
		wait := shutdownSearchWait
		shutdownSearchWait = 50 * time.Millisecond
		defer func() { shutdownSearchWait = wait }()
		app, path := newApp(t)
		app.cache.Update(character.CharacterID, character)
		app.mu.Lock() // a search that never stops
		defer app.mu.Unlock()

		start := time.Now()
		app.shutdown(context.Background())

		assert.Less(t, time.Since(start), time.Second)
		assert.Equal(t, 1, savedEntries(t, path))
	})
}
