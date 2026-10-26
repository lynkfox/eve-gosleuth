package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

const settingsFileName = "settings.json"

// maxThemeNameLength keeps a bad value from the frontend out of the settings file
const maxThemeNameLength = 64

// Settings are the user's app preferences, saved beside the cache
type Settings struct {
	Theme    string `json:"theme"`              // the frontend theme key, empty for the default theme
	CacheDir string `json:"cacheDir,omitempty"` // the folder the user moved the cache to, empty for the default. Changed with ChooseCacheFolder, not the Save button.
	Preferences
}

// Preferences are the choices in the Preferences window, saved together with its Save button
type Preferences struct {
	IncludeZkillLabels     bool   `json:"includeZkillLabels"`     // show zKillboard's labels, ie solo or ganked, on killmail cards. Off by default.
	MaxKillmails           int    `json:"maxKillmails"`           // the most killmails collected per search, 1 to api.ZKILL_MAX_KILLMAILS
	KillmailsNewerThanDays int    `json:"killmailsNewerThanDays"` // only collect killmails from the last 7, 30, or 90 days, 0 for any age
	UseCachedKillmails     bool   `json:"useCachedKillmails"`     // also use the target's cached killmails beyond the limits above
	TimeZone               string `json:"timeZone"`               // how killmail times are shown: "eve" for EVE time (UTC), the default, or "local"
}

// timeZoneChoices are the time zone options, EVE time (UTC) or the computer's local time
var timeZoneChoices = []string{"eve", "local"}

// killmailAgeChoices are the Killmails newer than options, in days, 0 for all
var killmailAgeChoices = []int{0, 7, 30, 90}

// defaultSettings are the settings before the user changes anything
func defaultSettings() Settings {
	return Settings{Preferences: Preferences{MaxKillmails: retrieval.DefaultKillmailLimit, TimeZone: "eve"}}
}

// validate returns a user readable error for a preference out of range
func (p Preferences) validate() error {
	if p.MaxKillmails < 1 || p.MaxKillmails > api.ZKILL_MAX_KILLMAILS {
		return fmt.Errorf("Max killmail count must be from 1 to %d", api.ZKILL_MAX_KILLMAILS)
	}
	if !slices.Contains(killmailAgeChoices, p.KillmailsNewerThanDays) {
		return fmt.Errorf("Killmails newer than must be 7, 30, or 90 days, or all")
	}
	if !slices.Contains(timeZoneChoices, p.TimeZone) {
		return fmt.Errorf("Times must be shown in EVE time or local time")
	}
	return nil
}

/*
	KillmailLimits turns the preferences into the limits killmail collection uses

Parameters:
  - now: the current time, which the age limit counts back from

Returns:
  - retrieval.KillmailLimits: the max count, the oldest killmail time (zero for any age), and whether to use cached killmails beyond them
*/
func (p Preferences) KillmailLimits(now time.Time) retrieval.KillmailLimits {
	limits := retrieval.KillmailLimits{Max: p.MaxKillmails, IncludeCached: p.UseCachedKillmails}
	if p.KillmailsNewerThanDays > 0 {
		limits.NewerThan = now.AddDate(0, 0, -p.KillmailsNewerThanDays)
	}
	return limits
}

// settingsPath returns where settings are saved, in the cache's default folder. They stay there when the cache is moved, as they say where it went.
func settingsPath() string {
	return filepath.Join(filepath.Dir(retrieval.DefaultCachePath()), settingsFileName)
}

// loadSettings reads the saved settings, returning the defaults if there are none or they can not be read
func loadSettings(path string) Settings {
	settings := defaultSettings()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return settings
	}
	if err == nil {
		err = json.Unmarshal(data, &settings)
	}
	if err != nil {
		comm.Logger.WithError(err).Warnf("Unable to read settings %s, using the defaults", path)
		return defaultSettings()
	}
	if err := settings.Preferences.validate(); err != nil {
		comm.Logger.WithError(err).Warnf("Settings %s have a preference out of range, using the default preferences", path)
		settings.Preferences = defaultSettings().Preferences
	}

	return settings
}

// saveSettings writes settings to a temporary file and renames it into place, so a failed save never leaves a half written file
func saveSettings(path string, settings Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("unable to create settings directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("unable to save settings to %s: %w", path, err)
	}
	return os.Rename(tmp, path)
}

// GetSettings returns the saved app settings, such as the theme, for the frontend to apply at startup
func (a *App) GetSettings() Settings {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	return a.settings
}

/*
	SetTheme saves the theme chosen in the Settings menu

Parameters:
  - theme: the frontend theme key, or an empty string for the default theme

Returns:
  - error: a user readable message if the theme could not be saved. The theme still applies for this session.
*/
func (a *App) SetTheme(theme string) error {
	theme = strings.TrimSpace(theme)
	if len(theme) > maxThemeNameLength {
		return fmt.Errorf("Theme name is too long")
	}

	if err := a.updateSettings(func(settings *Settings) { settings.Theme = theme }); err != nil {
		return errors.New("Could not save the theme, it will reset when the app restarts")
	}
	return nil
}

/*
	SavePreferences saves the Preferences window. The killmail limits apply from the next search.

Parameters:
  - preferences: every preference in the window

Returns:
  - error: a user readable message if a preference is out of range, in which case nothing changes, or if they could not be saved, in which case they still apply for this session
*/
func (a *App) SavePreferences(preferences Preferences) error {
	if err := preferences.validate(); err != nil {
		return err
	}
	if err := a.updateSettings(func(settings *Settings) { settings.Preferences = preferences }); err != nil {
		return errors.New("Could not save the preferences, they will reset when the app restarts")
	}
	return nil
}

// updateSettings changes the settings and saves them. The change is kept for this session even if saving fails.
func (a *App) updateSettings(change func(*Settings)) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	change(&a.settings)
	if err := saveSettings(a.settingsPath, a.settings); err != nil {
		comm.Logger.WithError(err).Error("Failed to save settings")
		return err
	}
	return nil
}
