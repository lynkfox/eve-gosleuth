package retrieval

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// IntelCheck is a pilot, corporation, or alliance from a My Intel import still to be checked with the ESI, saved so a check the app closed during carries on at the next start
type IntelCheck struct {
	Type string `json:"type"` // PILOT, CORPORATION, or ALLIANCE, as the file says
	ID   int    `json:"id"`
}

// intelCheckKey is a check's key in PendingChecks, ie ALLIANCE:99013330
func intelCheckKey(check IntelCheck) string {
	return fmt.Sprintf("%s:%d", check.Type, check.ID)
}

// loadIntelChecks reads every saved check into PendingChecks the first time they are wanted. The caller must hold c.mu.
func (c *CachedEveIDs) loadIntelChecks() {
	if c.checksLoaded || c.db == nil {
		return
	}
	c.checksLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionIntelChecks))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the imported entities still to check")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			continue
		}
		check, err := decodeValue[IntelCheck](sectionIntelChecks, data)
		if err != nil || check.ID == 0 {
			continue
		}
		if _, found := c.PendingChecks[key]; !found {
			c.PendingChecks[key] = &CacheEntry{Value: check, CachedAt: timeOf(cachedAt), Permanent: true}
		}
	}
}

// AddIntelChecks saves pilots, corporations, and alliances to check with the ESI, each once
func (c *CachedEveIDs) AddIntelChecks(checks []IntelCheck) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadIntelChecks()

	now := time.Now().UTC()
	for _, check := range checks {
		put(c, sectionIntelChecks, c.PendingChecks, intelCheckKey(check), &CacheEntry{Value: &check, CachedAt: now, Permanent: true})
	}
}

// IntelChecks returns the pilots, corporations, and alliances still to check with the ESI, by type then id
func (c *CachedEveIDs) IntelChecks() []IntelCheck {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadIntelChecks()

	checks := []IntelCheck{}
	for _, entry := range c.PendingChecks {
		if check, ok := entry.Value.(*IntelCheck); ok {
			checks = append(checks, *check)
		}
	}
	slices.SortFunc(checks, func(a, b IntelCheck) int {
		if byType := strings.Compare(a.Type, b.Type); byType != 0 {
			return byType
		}
		return a.ID - b.ID
	})
	return checks
}

// RemoveIntelChecks forgets checks that are done, from this session and the database
func (c *CachedEveIDs) RemoveIntelChecks(checks []IntelCheck) {
	done := map[string]bool{}
	for _, check := range checks {
		done[intelCheckKey(check)] = true
	}
	for batch := range slices.Chunk(checks, 500) {
		keys := []any{string(sectionIntelChecks)}
		for _, check := range batch {
			keys = append(keys, intelCheckKey(check))
		}
		c.removeWhere(
			func(s section, entry *CacheEntry) bool {
				check, ok := entry.Value.(*IntelCheck)
				return s == sectionIntelChecks && ok && done[intelCheckKey(*check)]
			},
			"section = ? AND key IN ("+strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")+")", keys...)
	}
}
