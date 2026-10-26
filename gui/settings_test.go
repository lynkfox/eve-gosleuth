// loading and saving settings is unexported, so this tests from inside package gui
package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSettings(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("missing or unreadable settings use the defaults", func(t *testing.T) {
		dir := t.TempDir()
		assert.Equal(t, defaultSettings(), loadSettings(filepath.Join(dir, "settings.json")))

		corrupt := filepath.Join(dir, "corrupt.json")
		assert.NoError(t, os.WriteFile(corrupt, []byte(`{"theme": `), 0o600))
		assert.Equal(t, defaultSettings(), loadSettings(corrupt))
	})

	t.Run("the defaults collect 20 killmails of any age, without labels or extra cached killmails, in EVE time", func(t *testing.T) {
		assert.Equal(t, Preferences{MaxKillmails: 20, TimeZone: "eve"}, defaultSettings().Preferences)
	})

	t.Run("settings saved before the killmail preferences existed keep their choices and get the default limits", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		assert.NoError(t, os.WriteFile(path, []byte(`{"theme": "amarr", "includeZkillLabels": true}`), 0o600))

		assert.Equal(t, Settings{Theme: "amarr", Preferences: Preferences{IncludeZkillLabels: true, MaxKillmails: 20, TimeZone: "eve"}}, loadSettings(path))
	})

	t.Run("a preference edited out of range in the file resets the preferences", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		assert.NoError(t, os.WriteFile(path, []byte(`{"theme": "amarr", "maxKillmails": 99999}`), 0o600))

		assert.Equal(t, Settings{Theme: "amarr", Preferences: defaultSettings().Preferences}, loadSettings(path))
	})

	t.Run("a chosen theme is saved and loaded again", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "eve-gosleuth", "settings.json")
		app := &App{settingsPath: path, settings: defaultSettings()}

		assert.NoError(t, app.SetTheme("minmatar"))

		assert.Equal(t, "minmatar", app.GetSettings().Theme)
		assert.Equal(t, "minmatar", loadSettings(path).Theme)
		assert.NoFileExists(t, path+".tmp")
	})

	t.Run("the default theme is saved as empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: Settings{Theme: "amarr", Preferences: defaultSettings().Preferences}}

		assert.NoError(t, app.SetTheme(""))

		assert.Equal(t, defaultSettings(), loadSettings(path))
	})

	t.Run("an over long theme name is refused", func(t *testing.T) {
		app := &App{settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: Settings{Theme: "amarr"}}

		assert.Error(t, app.SetTheme(strings.Repeat("x", 65)))
		assert.Equal(t, "amarr", app.GetSettings().Theme)
	})

	t.Run("preferences are saved together and keep the theme", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: loadSettings(path)}
		assert.NoError(t, app.SetTheme("amarr"))

		chosen := Preferences{IncludeZkillLabels: true, MaxKillmails: 2000, KillmailsNewerThanDays: 30, UseCachedKillmails: true, TimeZone: "local"}
		assert.NoError(t, app.SavePreferences(chosen))

		assert.Equal(t, Settings{Theme: "amarr", Preferences: chosen}, loadSettings(path))
		assert.Equal(t, chosen, app.GetSettings().Preferences)
	})

	t.Run("preferences out of range are refused and change nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: defaultSettings()}

		for _, bad := range []Preferences{{MaxKillmails: 0, TimeZone: "eve"}, {MaxKillmails: 2001, TimeZone: "eve"}, {MaxKillmails: 20, KillmailsNewerThanDays: 14, TimeZone: "eve"}, {MaxKillmails: 20, TimeZone: "mars"}} {
			assert.Error(t, app.SavePreferences(bad))
		}

		assert.Equal(t, defaultSettings(), app.GetSettings())
		assert.NoFileExists(t, path)
	})

	t.Run("preferences become killmail limits", func(t *testing.T) {
		now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

		assert.Equal(t, retrieval.KillmailLimits{Max: 20}, defaultSettings().KillmailLimits(now))
		assert.Equal(t, retrieval.KillmailLimits{Max: 500, NewerThan: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), IncludeCached: true},
			Preferences{MaxKillmails: 500, KillmailsNewerThanDays: 30, UseCachedKillmails: true}.KillmailLimits(now))
	})
}
