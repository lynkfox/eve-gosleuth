package retrieval

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// DefaultRecentSearches is how many recent searches are kept unless the user sets another number in Preferences, and MaxRecentSearches the most they can set
const (
	DefaultRecentSearches = 5
	MaxRecentSearches     = 25
)

// RecentSearch is a target a search found, offered again by the search bar
type RecentSearch struct {
	Name   string `json:"name"`             // the target's name as EVE spells it
	Type   string `json:"type"`             // PILOT, CORPORATION, ALLIANCE, or SYSTEM
	Ticker string `json:"ticker,omitempty"` // a corp's or alliance's ticker, ie URSA., empty for others and searches saved before tickers were
}

// recentSearchKey is a search's key in Searched: its type and lower case name, so a name typed in another case is the same search
func recentSearchKey(search RecentSearch) string {
	return search.Type + ":" + strings.ToLower(search.Name)
}

/*
loadRecentSearches reads every saved recent search into Searched the first time they are wanted. The caller must hold c.mu.
Unlike the other sections they are read all at once, as the search bar lists them all.
*/
func (c *CachedEveIDs) loadRecentSearches() {
	if c.searchedLoaded || c.db == nil {
		return
	}
	c.searchedLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionRecentSearches))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the recent searches")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			comm.Logger.WithError(err).Warn("Unable to read a recent search")
			continue
		}
		search, err := decodeValue[RecentSearch](sectionRecentSearches, data)
		if err != nil || search.Name == "" {
			comm.Logger.WithError(err).Warnf("Recent search %s can not be read, it is skipped", key)
			continue
		}
		if _, found := c.Searched[key]; !found {
			c.Searched[key] = &CacheEntry{Value: search, CachedAt: timeOf(cachedAt), Permanent: true}
		}
	}
}

// newestSearched returns Searched's entries newest first. The caller must hold c.mu.
func (c *CachedEveIDs) newestSearched() []*CacheEntry {
	entries := make([]*CacheEntry, 0, len(c.Searched))
	for _, entry := range c.Searched {
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b *CacheEntry) int {
		if byTime := b.CachedAt.Compare(a.CachedAt); byTime != 0 {
			return byTime
		}
		return cmp.Compare(a.Value.(*RecentSearch).Name, b.Value.(*RecentSearch).Name)
	})
	return entries
}

/*
	RecentSearches returns the targets searches have found, newest first

Parameters:
  - keep: the most to return. The cache can hold more, ie saved under a higher limit, until TrimRecentSearches or the next search drops them.

Returns:
  - []RecentSearch: at most keep, empty if there are none
*/
func (c *CachedEveIDs) RecentSearches(keep int) []RecentSearch {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadRecentSearches()

	recent := []RecentSearch{}
	for _, entry := range c.newestSearched() {
		if len(recent) >= keep {
			break
		}
		recent = append(recent, *entry.Value.(*RecentSearch))
	}
	return recent
}

/*
	RemoveRecentSearch forgets a recent search, from this session and the database

Parameters:
  - search: the search to remove. A name differing only in case is the same search. One not in the list changes nothing.
*/
func (c *CachedEveIDs) RemoveRecentSearch(search RecentSearch) {
	c.mu.Lock()
	c.loadRecentSearches()
	c.mu.Unlock()

	c.removeRecentSearches([]string{recentSearchKey(search)})
}

// removeRecentSearches removes recent searches by their recentSearchKey. It takes the locks itself, through removeWhere, which also saves anything new first.
func (c *CachedEveIDs) removeRecentSearches(keys []string) {
	c.mu.Lock()
	removing := map[*CacheEntry]struct{}{}
	for _, key := range keys {
		if entry, found := c.Searched[key]; found {
			removing[entry] = struct{}{}
		}
	}
	c.mu.Unlock()

	args := []any{string(sectionRecentSearches)}
	for _, key := range keys {
		args = append(args, key)
	}
	c.removeWhere(
		func(s section, entry *CacheEntry) bool {
			_, remove := removing[entry]
			return s == sectionRecentSearches && remove
		},
		fmt.Sprintf("section = ? AND key IN (%s)", sectionPlaceholdersFor(len(keys))), args...)
}

/*
	AddRecentSearch records a target a search found, at the front of the recent searches. A search already in the list moves to the front, taking the new name's spelling.
	Recent searches never expire with the SDE, but are removed by ClearAll and ClearOlderThan like any other entry, and the oldest past keep are dropped.

Parameters:
  - search: the target's name and search type
  - keep: how many recent searches to keep
*/
func (c *CachedEveIDs) AddRecentSearch(search RecentSearch, keep int) {
	c.mu.Lock()
	c.loadRecentSearches()
	put(c, sectionRecentSearches, c.Searched, recentSearchKey(search), &CacheEntry{Value: &search, CachedAt: time.Now().UTC(), Permanent: true})
	c.mu.Unlock()

	c.TrimRecentSearches(keep)
}

/*
	TrimRecentSearches drops the oldest recent searches past keep, from this session and the database, ie when the user lowers the number kept

Parameters:
  - keep: how many of the newest to keep
*/
func (c *CachedEveIDs) TrimRecentSearches(keep int) {
	c.mu.Lock()
	c.loadRecentSearches()
	var dropped []string
	if newest := c.newestSearched(); len(newest) > keep {
		for _, entry := range newest[max(keep, 0):] {
			dropped = append(dropped, recentSearchKey(*entry.Value.(*RecentSearch)))
		}
	}
	c.mu.Unlock()

	if len(dropped) > 0 {
		c.removeRecentSearches(dropped)
	}
}
