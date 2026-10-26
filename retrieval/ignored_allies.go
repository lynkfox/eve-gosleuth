package retrieval

import (
	"fmt"
	"time"
)

// IgnoredAlly is an alliance, or a corporation in none, the user chose to leave out of a target's Allies
type IgnoredAlly struct {
	Type string `json:"type"` // ALLIANCE or CORPORATION
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// IgnoredAllies is what a target's ignored_ally row holds
type IgnoredAllies struct {
	Allies []IgnoredAlly `json:"allies"`
}

// ignoredAlliesKey is a target's key in IgnoredAllies, ie ALLIANCE:99000001
func ignoredAlliesKey(targetType string, targetID int) string {
	return fmt.Sprintf("%s:%d", targetType, targetID)
}

/*
	IgnoredAlliesFor returns the allies the user chose to leave out of a target's Allies

Parameters:
  - targetType: the target's PILOT, CORPORATION, or ALLIANCE
  - targetID: the target's eve id

Returns:
  - []IgnoredAlly: in the order they were saved, empty if there are none
*/
func (c *CachedEveIDs) IgnoredAlliesFor(targetType string, targetID int) []IgnoredAlly {
	c.mu.Lock()
	defer c.mu.Unlock()

	if saved := cachedAs[IgnoredAllies](c, sectionIgnoredAllies, c.IgnoredAllies, ignoredAlliesKey(targetType, targetID)); saved != nil && saved.Allies != nil {
		return saved.Allies
	}
	return []IgnoredAlly{}
}

/*
	SetIgnoredAllies replaces the allies left out of a target's Allies. Like recent searches they never expire with the SDE,
	but are removed by ClearAll and ClearOlderThan like any other entry.

Parameters:
  - targetType: the target's PILOT, CORPORATION, or ALLIANCE
  - targetID: the target's eve id
  - allies: every ally to leave out, empty to leave none out
*/
func (c *CachedEveIDs) SetIgnoredAllies(targetType string, targetID int, allies []IgnoredAlly) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if allies == nil {
		allies = []IgnoredAlly{}
	}
	put(c, sectionIgnoredAllies, c.IgnoredAllies, ignoredAlliesKey(targetType, targetID),
		&CacheEntry{Value: &IgnoredAllies{Allies: allies}, CachedAt: time.Now().UTC(), Permanent: true})
}
