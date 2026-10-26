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

	t.Run("the defaults collect 20 killmails of any age, without labels or extra cached killmails, in EVE time, keeping 5 recent searches", func(t *testing.T) {
		assert.Equal(t, Preferences{MaxKillmails: 20, TimeZone: "eve", MaxRecentSearches: 5}, defaultSettings().Preferences)
	})

	t.Run("settings saved before the killmail preferences existed keep their choices and get the default limits", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		assert.NoError(t, os.WriteFile(path, []byte(`{"theme": "amarr", "includeZkillLabels": true}`), 0o600))

		assert.Equal(t, Settings{Theme: "amarr", Preferences: Preferences{IncludeZkillLabels: true, MaxKillmails: 20, TimeZone: "eve", MaxRecentSearches: 5}}, loadSettings(path))
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

		chosen := Preferences{IncludeZkillLabels: true, MaxKillmails: 2000, KillmailsNewerThanDays: 30, UseCachedKillmails: true, TimeZone: "local", MaxRecentSearches: 25}
		assert.NoError(t, app.SavePreferences(chosen))

		assert.Equal(t, Settings{Theme: "amarr", Preferences: chosen}, loadSettings(path))
		assert.Equal(t, chosen, app.GetSettings().Preferences)
	})

	t.Run("preferences out of range are refused and change nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: defaultSettings()}

		good := defaultSettings().Preferences
		for _, change := range []func(*Preferences){
			func(p *Preferences) { p.MaxKillmails = 0 },
			func(p *Preferences) { p.MaxKillmails = -2 },
			func(p *Preferences) { p.MaxKillmails = 2001 },
			func(p *Preferences) { p.KillmailsNewerThanDays = 14 },
			func(p *Preferences) { p.TimeZone = "mars" },
			func(p *Preferences) { p.MaxRecentSearches = 0 },
			func(p *Preferences) { p.MaxRecentSearches = 26 },
		} {
			bad := good
			change(&bad)
			assert.Error(t, app.SavePreferences(bad), "%+v", bad)
		}

		assert.Equal(t, defaultSettings(), app.GetSettings())
		assert.NoFileExists(t, path)
	})

	t.Run("a max killmail count of -1 is all killmails", func(t *testing.T) {
		app := &App{settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings()}
		chosen := defaultSettings().Preferences
		chosen.MaxKillmails = -1
		chosen.KillmailsNewerThanDays = 7

		assert.NoError(t, app.SavePreferences(chosen))
		now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
		assert.Equal(t, retrieval.KillmailLimits{Max: retrieval.AllKillmails, NewerThan: now.AddDate(0, 0, -7)}, chosen.KillmailLimits(now))
	})

	t.Run("a tab order is saved and loaded again, keeping only ids that look like tabs, each once", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: defaultSettings()}

		assert.NoError(t, app.SetTabOrder([]string{"tab-pi", "tab-killmails", "tab-pi", "<script>", "tab-activity"}))

		want := []string{"tab-pi", "tab-killmails", "tab-activity"}
		assert.Equal(t, want, app.GetSettings().TabOrder)
		assert.Equal(t, want, loadSettings(path).TabOrder)
	})

	t.Run("an empty tab order goes back to the default", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		app := &App{settingsPath: path, settings: defaultSettings()}
		assert.NoError(t, app.SetTabOrder([]string{"tab-pi"}))

		assert.NoError(t, app.SetTabOrder([]string{}))
		assert.Nil(t, loadSettings(path).TabOrder)
	})

	t.Run("preferences become killmail limits", func(t *testing.T) {
		now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

		assert.Equal(t, retrieval.KillmailLimits{Max: 20}, defaultSettings().KillmailLimits(now))
		assert.Equal(t, retrieval.KillmailLimits{Max: 500, NewerThan: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC), IncludeCached: true},
			Preferences{MaxKillmails: 500, KillmailsNewerThanDays: 30, UseCachedKillmails: true}.KillmailLimits(now))
	})
}
