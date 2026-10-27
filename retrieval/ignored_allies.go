package retrieval

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
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

// TargetIgnoredAllies are the allies left out of one target's Allies
type TargetIgnoredAllies struct {
	TargetType string // PILOT, CORPORATION, or ALLIANCE
	TargetID   int
	Allies     []IgnoredAlly
}

/*
AllIgnoredAllies returns every target's ignored allies, for My Intel. Targets with none left are left out.
The first call reads every saved row, as rows are otherwise only read for the target shown.
*/
func (c *CachedEveIDs) AllIgnoredAllies() []TargetIgnoredAllies {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadIgnoredAllies()

	all := []TargetIgnoredAllies{}
	for key, entry := range c.IgnoredAllies {
		saved, ok := entry.Value.(*IgnoredAllies)
		targetType, idText, found := strings.Cut(key, ":")
		targetID, idErr := strconv.Atoi(idText)
		if !ok || saved == nil || len(saved.Allies) == 0 || !found || idErr != nil {
			continue
		}
		all = append(all, TargetIgnoredAllies{TargetType: targetType, TargetID: targetID, Allies: saved.Allies})
	}
	return all
}

// loadIgnoredAllies reads every saved target's ignored allies into IgnoredAllies the first time they are all wanted. The caller must hold c.mu.
func (c *CachedEveIDs) loadIgnoredAllies() {
	if c.ignoredLoaded || c.db == nil {
		return
	}
	c.ignoredLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionIgnoredAllies))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the ignored allies")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			comm.Logger.WithError(err).Warn("Unable to read a target's ignored allies")
			continue
		}
		// a row read, or changed, in this session is already there and may be newer
		if _, found := c.IgnoredAllies[key]; found {
			continue
		}
		saved, err := decodeValue[IgnoredAllies](sectionIgnoredAllies, data)
		if err != nil {
			comm.Logger.WithError(err).Warnf("Ignored allies %s can not be read, they are skipped", key)
			continue
		}
		c.IgnoredAllies[key] = &CacheEntry{Value: saved, CachedAt: timeOf(cachedAt), Permanent: true}
	}
}
