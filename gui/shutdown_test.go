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

	t.Run("closed before startup is ready, it waits for the cache rather than closing none", func(t *testing.T) {
		app, path := newApp(t)
		cache := app.cache
		app.cache = nil
		app.ready = make(chan struct{})

		done := make(chan struct{})
		go func() {
			defer close(done)
			app.shutdown(context.Background())
		}()
		select {
		case <-done:
			t.Fatal("shutdown went on before startup was ready")
		case <-time.After(50 * time.Millisecond):
		}

		cache.Update(character.CharacterID, character)
		app.cache = cache
		app.markReady()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown still waiting after startup was ready")
		}
		assert.Equal(t, 1, savedEntries(t, path))
	})

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

	t.Run("a search asked for once closing has begun is refused, so it can not undo the stop", func(t *testing.T) {
		app, _ := newApp(t)
		app.closing.Store(true)

		_, err := app.SearchTarget("Luxxianna Seraph", "PILOT", 0)
		assert.ErrorIs(t, err, errAppClosing)
		_, err = app.CollectIntel(2121139046, "PILOT")
		assert.ErrorIs(t, err, errAppClosing)
		assert.False(t, app.stopRequested.Load(), "nothing was reset or started")
	})

	t.Run("a check of imported entities is waited for too", func(t *testing.T) {
		app, path := newApp(t)
		started := make(chan struct{})
		go func() {
			app.checkMu.Lock()
			defer app.checkMu.Unlock()
			close(started)
			for !app.closing.Load() {
				time.Sleep(time.Millisecond)
			}
			app.cache.Update(character.CharacterID, character)
		}()
		<-started

		app.shutdown(context.Background())

		assert.Equal(t, 1, savedEntries(t, path))
	})

	t.Run("a search that does not stop in time is left behind, and the cache saved without it", func(t *testing.T) {
		wait := shutdownSearchWait
		shutdownSearchWait = 50 * time.Millisecond
		defer func() { shutdownSearchWait = wait }()
		app, path := newApp(t)
		app.cache.Update(character.CharacterID, character)
		app.mu.Lock()      // a search that never stops
		app.checkMu.Lock() // nor does a check
		defer app.mu.Unlock()
		defer app.checkMu.Unlock()

		start := time.Now()
		app.shutdown(context.Background())

		assert.Less(t, time.Since(start), time.Second)
		assert.Equal(t, 1, savedEntries(t, path))
	})
}
