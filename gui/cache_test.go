package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCacheActions(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// newApp has a saved cache holding one character, in its own folder rather than the default
	newApp := func(t *testing.T) *App {
		path := retrieval.CachePathIn(t.TempDir())
		cache := retrieval.NewCache(api.NewESIClient(), path)
		cache.Update(2121139046, &api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph"})
		assert.NoError(t, retrieval.SaveCache(cache))
		return &App{cache: cache, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: Settings{CacheDir: filepath.Dir(path)}}
	}

	t.Run("moving saves the cache in the new folder, remembers it, and removes the old file", func(t *testing.T) {
		app := newApp(t)
		oldPath := app.cache.CurrentPath()
		dir := filepath.Join(t.TempDir(), "new cache")

		info, err := app.moveCache(dir)

		assert.NoError(t, err)
		assert.Equal(t, retrieval.CachePathIn(dir), info.Path)
		assert.False(t, info.IsDefault)
		assert.Equal(t, 1, info.Entries)
		assert.Positive(t, info.SizeBytes)
		assert.NoFileExists(t, oldPath)
		assert.Equal(t, dir, loadSettings(app.settingsPath).CacheDir)
		assert.Equal(t, retrieval.CachePathIn(dir), cachePathFor(loadSettings(app.settingsPath)), "the next start loads it from there")
	})

	t.Run("a folder the cache can not be saved in leaves it where it was", func(t *testing.T) {
		app := newApp(t)
		oldPath := app.cache.CurrentPath()
		blocker := filepath.Join(t.TempDir(), "a file")
		assert.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

		_, err := app.moveCache(filepath.Join(blocker, "cache"))

		assert.Error(t, err)
		assert.Equal(t, oldPath, app.cache.CurrentPath())
		assert.FileExists(t, oldPath)
		assert.Equal(t, filepath.Dir(oldPath), app.GetSettings().CacheDir)
	})

	t.Run("clearing saves the cleared cache", func(t *testing.T) {
		app := newApp(t)

		info, err := app.ClearCache("all", false)

		assert.NoError(t, err)
		assert.Equal(t, 0, info.Entries)
		reloaded, _ := retrieval.LoadCache(api.NewESIClient(), app.cache.CurrentPath())
		assert.Equal(t, 0, reloaded.Entries())
	})

	t.Run("an unknown choice clears nothing", func(t *testing.T) {
		app := newApp(t)

		_, err := app.ClearCache("some", false)

		assert.Error(t, err)
		assert.Equal(t, 1, app.cache.Entries())
	})

	t.Run("nothing changes while a search is running", func(t *testing.T) {
		app := newApp(t)
		app.mu.Lock()
		defer app.mu.Unlock()

		_, clearErr := app.ClearCache("all", false)
		_, moveErr := app.moveCache(t.TempDir())

		assert.ErrorIs(t, clearErr, errSearchRunning)
		assert.ErrorIs(t, moveErr, errSearchRunning)
		assert.Equal(t, 1, app.cache.Entries())
	})
}
