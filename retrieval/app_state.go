package retrieval

import "time"

// appStateKey is the one row of sectionAppState
const appStateKey = "app"

// AppState is what the app remembers about itself between launches, not a user choice: which versions it has shown the user
type AppState struct {
	LastSeenVersion string `json:"lastSeenVersion,omitempty"` // the newest version whose What's New was shown (or skipped as a fresh install), empty before 1.1.0
	NotifiedVersion string `json:"notifiedVersion,omitempty"` // the newer release the update popup last told the user about
	Snoozed         bool   `json:"snoozed,omitempty"`         // the user pressed Later on NotifiedVersion, so it shows as the top bar chip
}

/*
	AppState returns what the app remembers about itself, saved by SetAppState.

Returns:
  - AppState: the saved state, empty if none was saved
*/
func (c *CachedEveIDs) AppState() AppState {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state := cachedAs[AppState](c, sectionAppState, c.States, appStateKey); state != nil {
		return *state
	}
	return AppState{}
}

/*
	SetAppState replaces what the app remembers about itself. It is Permanent and one of userSections, so the SDE and Clear cache leave it.

Parameters:
  - state: the whole state
*/
func (c *CachedEveIDs) SetAppState(state AppState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	put(c, sectionAppState, c.States, appStateKey, &CacheEntry{Value: &state, CachedAt: time.Now().UTC(), Permanent: true})
}
