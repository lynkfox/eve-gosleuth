package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// clearOlderThanDays is how old cached data must be for the "older than" choice of ClearCache
const clearOlderThanDays = 30

// errSearchRunning is returned by the cache actions while a search is using the cache
var errSearchRunning = errors.New("Wait for the search to finish before changing the cache")

// CacheInfo describes the cache for the Preferences window
type CacheInfo struct {
	Path      string `json:"path"`      // the cache file
	IsDefault bool   `json:"isDefault"` // kept in the default folder, under the user's local app data
	Entries   int    `json:"entries"`
	SizeBytes int64  `json:"sizeBytes"` // of the database and its write-ahead log
}

// cachePathFor returns where the settings say the cache is kept: the folder the user chose, or the default
func cachePathFor(settings Settings) string {
	if settings.CacheDir == "" {
		return retrieval.DefaultCachePath()
	}
	return retrieval.CachePathIn(settings.CacheDir)
}

// GetCacheInfo returns where the cache is kept and how much it holds
func (a *App) GetCacheInfo() CacheInfo {
	path := a.cache.CurrentPath()
	info := CacheInfo{Path: path, IsDefault: filepath.Clean(path) == filepath.Clean(retrieval.DefaultCachePath()), Entries: a.cache.Entries()}
	for _, file := range []string{path, path + "-wal"} {
		if stat, err := os.Stat(file); err == nil {
			info.SizeBytes += stat.Size()
		}
	}
	return info
}

/*
	ClearCache removes cached data, then saves the cache

Parameters:
  - scope: "all" for everything, "entities" for pilot, corporation, and alliance data only, or "older" for everything cached more than clearOlderThanDays days ago

Returns:
  - CacheInfo: the cache after clearing
  - error: a user readable message if a search is running, the scope is unknown, or the cache could not be saved. The data is still cleared from memory if only the save failed.
*/
func (a *App) ClearCache(scope string) (CacheInfo, error) {
	if !a.mu.TryLock() {
		return a.GetCacheInfo(), errSearchRunning
	}
	defer a.mu.Unlock()

	var removed int
	var what string
	switch scope {
	case "all":
		removed, what = a.cache.ClearAll(), "everything"
	case "entities":
		removed, what = a.cache.ClearEntities(), "pilot, corporation, and alliance data"
	case "older":
		removed, what = a.cache.ClearOlderThan(time.Now().UTC().AddDate(0, 0, -clearOlderThanDays)), fmt.Sprintf("data older than %d days", clearOlderThanDays)
	default:
		return a.GetCacheInfo(), fmt.Errorf("Unknown cache clearing choice %q", scope)
	}

	comm.ReportProgress("Cleared %s from the cache, %d entries", what, removed)
	if err := retrieval.SaveCache(a.cache); err != nil {
		comm.Logger.WithError(err).Error("Failed to save the cleared cache")
		return a.GetCacheInfo(), errors.New("Cleared, but the cache file could not be saved, the old data comes back when the app restarts")
	}
	return a.GetCacheInfo(), nil
}

// ChooseCacheFolder asks the user for a folder and moves the cache there. Cancelling the folder picker changes nothing.
func (a *App) ChooseCacheFolder() (CacheInfo, error) {
	current := a.cache.CurrentPath()
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "Choose a folder for the GoSleuth cache",
		DefaultDirectory:     filepath.Dir(current),
		CanCreateDirectories: true,
	})
	if err != nil {
		return a.GetCacheInfo(), fmt.Errorf("Could not open the folder picker: %w", err)
	}
	if dir == "" {
		return a.GetCacheInfo(), nil
	}

	// choosing the default folder is the same as going back to the default
	if filepath.Clean(dir) == filepath.Clean(filepath.Dir(retrieval.DefaultCachePath())) {
		dir = ""
	}
	return a.moveCache(dir)
}

// ResetCacheFolder moves the cache back to its default folder, under the user's local app data
func (a *App) ResetCacheFolder() (CacheInfo, error) {
	return a.moveCache("")
}

/*
	moveCache copies the cache into a new folder and switches to it, removing the old database, then records the folder in the settings.
	A cache already in the new folder is replaced. If any step fails the cache ends up where it was.

Parameters:
  - dir: the new folder, or empty for the default

Returns:
  - CacheInfo: the cache after moving
  - error: a user readable message if it could not be moved
*/
func (a *App) moveCache(dir string) (CacheInfo, error) {
	if !a.mu.TryLock() {
		return a.GetCacheInfo(), errSearchRunning
	}
	defer a.mu.Unlock()

	oldPath := a.cache.CurrentPath()
	newPath := retrieval.DefaultCachePath()
	if dir != "" {
		newPath = retrieval.CachePathIn(dir)
	}
	if filepath.Clean(oldPath) == filepath.Clean(newPath) {
		return a.GetCacheInfo(), nil
	}

	if err := a.cache.MoveTo(newPath); err != nil {
		comm.Logger.WithError(err).Errorf("Failed to move the cache to %s", newPath)
		return a.GetCacheInfo(), fmt.Errorf("Could not save the cache in %s", filepath.Dir(newPath))
	}

	if err := a.updateSettings(func(settings *Settings) { settings.CacheDir = dir }); err != nil {
		// without the setting the app would look in the new folder's place next time, so move back
		_ = a.updateSettings(func(settings *Settings) { settings.CacheDir = cacheDirOf(oldPath) })
		if moveErr := a.cache.MoveTo(oldPath); moveErr != nil {
			comm.Logger.WithError(moveErr).Errorf("Failed to move the cache back to %s", oldPath)
		}
		return a.GetCacheInfo(), errors.New("Could not save the new cache location, the cache was not moved")
	}

	comm.ReportProgress("Moved the cache to %s", filepath.Dir(newPath))
	return a.GetCacheInfo(), nil
}

// cacheDirOf returns the CacheDir setting for a cache file: empty if it is the default
func cacheDirOf(path string) string {
	if filepath.Clean(path) == filepath.Clean(retrieval.DefaultCachePath()) {
		return ""
	}
	return filepath.Dir(path)
}
