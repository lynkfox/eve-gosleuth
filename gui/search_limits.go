package gui

import "errors"

// SearchLimits are the choices of the search limits button beside the search bar: what a search collects, overriding the saved Preferences for this session
type SearchLimits struct {
	MaxKillmails           int  `json:"maxKillmails"`           // as Preferences.MaxKillmails
	KillmailPages          int  `json:"killmailPages"`          // as Preferences.KillmailPages
	KillmailsNewerThanDays int  `json:"killmailsNewerThanDays"` // as Preferences.KillmailsNewerThanDays
	UseCachedKillmails     bool `json:"useCachedKillmails"`     // as Preferences.UseCachedKillmails
	IncludeHoldings        bool `json:"includeHoldings"`        // as Preferences.IncludeHoldings
}

// searchLimits are the preferences' search limits
func (p Preferences) searchLimits() SearchLimits {
	return SearchLimits{MaxKillmails: p.MaxKillmails, KillmailPages: p.KillmailPages, KillmailsNewerThanDays: p.KillmailsNewerThanDays,
		UseCachedKillmails: p.UseCachedKillmails, IncludeHoldings: p.IncludeHoldings}
}

// withSearchLimits are the preferences with their search limits replaced
func (p Preferences) withSearchLimits(limits SearchLimits) Preferences {
	p.MaxKillmails, p.KillmailPages, p.KillmailsNewerThanDays = limits.MaxKillmails, limits.KillmailPages, limits.KillmailsNewerThanDays
	p.UseCachedKillmails, p.IncludeHoldings = limits.UseCachedKillmails, limits.IncludeHoldings
	return p
}

// searchPreferences are the preferences a search uses: the saved ones, with the search limits button's for this session in their place when it set any
func (a *App) searchPreferences() Preferences {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if a.searchLimits == nil {
		return a.settings.Preferences
	}
	return a.settings.Preferences.withSearchLimits(*a.searchLimits)
}

/*
	GetSearchLimits returns the limits the next search uses, for the search limits button

Returns:
  - SearchLimits: the session's limits, or the saved Preferences' when it has none
*/
func (a *App) GetSearchLimits() SearchLimits {
	return a.searchPreferences().searchLimits()
}

/*
	SetSearchLimits sets the limits searches use until the app closes, without saving them. Like the theme, it does not wait behind a running search, and applies from the next one.

Parameters:
  - limits: every limit in the search limits button

Returns:
  - error: a user readable message if one is out of range, in which case nothing changes
*/
func (a *App) SetSearchLimits(limits SearchLimits) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	if err := a.settings.Preferences.withSearchLimits(limits).validate(); err != nil {
		return err
	}
	a.searchLimits = &limits
	return nil
}

// ResetSearchLimits drops the session's search limits, so searches use the saved Preferences again
func (a *App) ResetSearchLimits() {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()

	a.searchLimits = nil
}

/*
	SaveSearchLimits saves the search limits button's limits as the Preferences, so they are the defaults from now on, and drops the session's

Parameters:
  - limits: every limit in the search limits button

Returns:
  - error: a user readable message if one is out of range, in which case nothing changes, or if they could not be saved, in which case they still apply for this session
*/
func (a *App) SaveSearchLimits(limits SearchLimits) error {
	preferences := a.GetSettings().Preferences.withSearchLimits(limits)
	if err := preferences.validate(); err != nil {
		return err
	}
	saveErr := a.updateSettings(func(settings *Settings) {
		settings.Preferences = settings.Preferences.withSearchLimits(limits)
		a.searchLimits = nil
	})
	if saveErr != nil {
		return errors.New("Could not save the limits as your defaults, they will reset when the app restarts")
	}
	return nil
}
