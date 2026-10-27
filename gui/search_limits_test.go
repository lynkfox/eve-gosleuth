package gui

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSearchLimits(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	appIn := func(t *testing.T) (*App, string) {
		path := filepath.Join(t.TempDir(), "settings.json")
		return &App{settingsPath: path, settings: defaultSettings()}, path
	}
	wide := SearchLimits{MaxKillmails: 500, KillmailPages: 10, KillmailsNewerThanDays: 90, UseCachedKillmails: true, IncludeHoldings: false}

	t.Run("with none set, searches use the saved preferences", func(t *testing.T) {
		app, _ := appIn(t)
		assert.Equal(t, defaultSettings().Preferences.searchLimits(), app.GetSearchLimits())
		assert.Equal(t, defaultSettings().Preferences, app.searchPreferences())
	})

	t.Run("set limits are what searches use, and are not saved", func(t *testing.T) {
		app, path := appIn(t)

		assert.NoError(t, app.SetSearchLimits(wide))

		assert.Equal(t, wide, app.GetSearchLimits())
		assert.Equal(t, 500, app.searchPreferences().MaxKillmails)
		assert.False(t, app.searchPreferences().IncludeHoldings)
		assert.Equal(t, defaultSettings().Preferences, app.GetSettings().Preferences)
		assert.NoFileExists(t, path)
	})

	t.Run("limits out of range are refused and change nothing", func(t *testing.T) {
		app, _ := appIn(t)
		bad := wide
		bad.MaxKillmails = 0

		assert.Error(t, app.SetSearchLimits(bad))
		assert.Error(t, app.SaveSearchLimits(bad))
		assert.Equal(t, defaultSettings().Preferences.searchLimits(), app.GetSearchLimits())
	})

	t.Run("reset goes back to the saved preferences", func(t *testing.T) {
		app, _ := appIn(t)
		assert.NoError(t, app.SetSearchLimits(wide))

		app.ResetSearchLimits()

		assert.Equal(t, defaultSettings().Preferences.searchLimits(), app.GetSearchLimits())
	})

	t.Run("saving makes them the defaults, keeping the other preferences, and drops the session's", func(t *testing.T) {
		app, path := appIn(t)
		assert.NoError(t, app.SetTheme("amarr"))
		assert.NoError(t, app.SetSearchLimits(SearchLimits{MaxKillmails: 20, KillmailPages: 10}))

		assert.NoError(t, app.SaveSearchLimits(wide))

		assert.Nil(t, app.searchLimits)
		assert.Equal(t, wide, loadSettings(path).Preferences.searchLimits())
		assert.Equal(t, "amarr", loadSettings(path).Theme)
		assert.Equal(t, "eve", loadSettings(path).TimeZone)
	})
}
