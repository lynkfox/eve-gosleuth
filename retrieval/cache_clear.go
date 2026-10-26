package retrieval

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
)

// CachePathIn returns where the cache database is kept in a folder the user chose
func CachePathIn(dir string) string {
	return filepath.Join(dir, cacheFileName)
}

// CurrentPath returns the cache's database file
func (c *CachedEveIDs) CurrentPath() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Path
}

/*
	MoveTo saves the cache into a new database file, switches to it, and removes the old one. A database already at newPath is replaced.
	If the copy can not be made or opened the cache stays where it was.

Parameters:
  - newPath: the new database file, ie CachePathIn(dir)

Returns:
  - error: why it could not be moved
*/
func (c *CachedEveIDs) MoveTo(newPath string) error {
	if err := c.flush(); err != nil {
		return err
	}

	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	oldPath := c.Path
	if filepath.Clean(oldPath) == filepath.Clean(newPath) {
		return nil
	}
	if c.db == nil {
		return errors.New("the cache database is not open")
	}

	// VACUUM INTO writes a compact copy, needing the new folder and no file in the way
	newDB, err := openStore(newPath)
	if err != nil {
		return err
	}
	newDB.Close()
	removeStoreFiles(newPath)
	if _, err := c.db.Exec("VACUUM INTO ?", newPath); err != nil {
		removeStoreFiles(newPath)
		return fmt.Errorf("unable to copy the cache to %s: %w", newPath, err)
	}

	newDB, err = openStore(newPath)
	if err != nil {
		removeStoreFiles(newPath)
		return err
	}

	if err := c.db.Close(); err != nil {
		comm.Logger.WithError(err).Warnf("Unable to close the old cache %s", oldPath)
	}
	c.db = newDB
	c.Path = newPath
	removeStoreFiles(oldPath)
	return nil
}

// Entries counts everything cached, saving anything new first so it is counted
func (c *CachedEveIDs) Entries() int {
	if err := c.flush(); err != nil {
		comm.Logger.WithError(err).Warn("Unable to save the cache before counting it")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		count := len(c.NameLookups)
		for _, entries := range c.idMaps() {
			count += len(entries.entries)
		}
		return count
	}

	var count int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM cache").Scan(&count); err != nil {
		comm.Logger.WithError(err).Warn("Unable to count the cache")
	}
	return count
}

// sectionMap pairs one of the maps keyed by eve id with its section
type sectionMap struct {
	section section
	entries map[int]*CacheEntry
}

// idMaps returns every map keyed by eve id, which is all of them but NameLookups. The caller must hold c.mu.
func (c *CachedEveIDs) idMaps() []sectionMap {
	return []sectionMap{
		{sectionKnown, c.Known}, {sectionExpanded, c.Expanded}, {sectionKillmails, c.Killmails}, {sectionEntityNames, c.EntityNames},
		{sectionAllianceCorporations, c.AllianceCorporations}, {sectionCorporationHistories, c.CorporationHistories},
		{sectionAllianceHistories, c.AllianceHistories}, {sectionAffiliations, c.Affiliations},
	}
}

/*
	removeWhere removes entries from both this session's maps and the database, saving anything new first so nothing removed comes back

Parameters:
  - match: whether an entry in the maps is removed
  - where: the same choice as a SQL condition on the cache table, with its args

Returns:
  - int: how many entries were removed
*/
func (c *CachedEveIDs) removeWhere(match func(section, *CacheEntry) bool, where string, args ...any) int {
	if err := c.flush(); err != nil {
		comm.Logger.WithError(err).Warn("Unable to save the cache before removing from it")
	}

	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := map[rowKey]struct{}{}
	for _, entries := range c.idMaps() {
		for id, entry := range entries.entries {
			if match(entries.section, entry) {
				delete(entries.entries, id)
				removed[rowKey{entries.section, keyText(id)}] = struct{}{}
			}
		}
	}
	for name, entry := range c.NameLookups {
		if match(sectionNameLookups, entry) {
			delete(c.NameLookups, name)
			removed[rowKey{sectionNameLookups, name}] = struct{}{}
		}
	}
	for key := range removed {
		delete(c.dirty, key)
	}

	if c.db != nil {
		if err := deleteRows(c, removed, where, args...); err != nil {
			comm.Logger.WithError(err).Error("Unable to remove entries from the cache database")
		}
	}
	return len(removed)
}

// deleteRows deletes the rows matching where, adding them to removed. The caller must hold c.mu.
func deleteRows(c *CachedEveIDs, removed map[rowKey]struct{}, where string, args ...any) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query("SELECT section, key FROM cache WHERE "+where, args...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key rowKey
		var s string
		if err := rows.Scan(&s, &key.key); err != nil {
			rows.Close()
			return err
		}
		key.section = section(s)
		removed[key] = struct{}{}
	}
	rows.Close()

	if _, err := tx.Exec("DELETE FROM cache WHERE "+where, args...); err != nil {
		return err
	}
	return tx.Commit()
}

// compact gives the space freed by a clear back to the disk
func (c *CachedEveIDs) compact() {
	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return
	}
	if _, err := c.db.Exec("VACUUM"); err != nil {
		comm.Logger.WithError(err).Warn("Unable to compact the cache database")
	}
}

// ClearAll empties the cache, returning how many entries were removed. The SDE release seen is kept.
func (c *CachedEveIDs) ClearAll() int {
	removed := c.removeWhere(func(section, *CacheEntry) bool { return true }, "1 = 1")
	c.compact()
	return removed
}

// entitySections hold nothing but pilot, corporation, and alliance data
var entitySections = []section{sectionAffiliations, sectionCorporationHistories, sectionAllianceHistories, sectionAllianceCorporations}

// entityTypes are the pilot, corporation, and alliance values in Known and Expanded, as the cache table's type column names them
var entityTypes = []any{
	fmt.Sprintf("%T", (*api.ESICharacter)(nil)), fmt.Sprintf("%T", (*api.ESICorporation)(nil)), fmt.Sprintf("%T", (*api.ESIAlliance)(nil)),
	fmt.Sprintf("%T", (*Pilot)(nil)), fmt.Sprintf("%T", (*Corporation)(nil)), fmt.Sprintf("%T", (*Alliance)(nil)),
}

/*
	ClearEntities removes the cached pilot, corporation, and alliance data: their details, readable models, current affiliations, corp and alliance histories, and alliance corp lists.
	Killmails, ships, systems, and names are kept, as they do not change.

Returns:
  - int: how many entries were removed
*/
func (c *CachedEveIDs) ClearEntities() int {
	sections, sectionArgs := sectionPlaceholders(entitySections...)
	mixed, mixedArgs := sectionPlaceholders(sectionKnown, sectionExpanded)
	typeList := sectionPlaceholdersFor(len(entityTypes))

	args := append(append(sectionArgs, mixedArgs...), entityTypes...)
	removed := c.removeWhere(
		func(s section, entry *CacheEntry) bool {
			for _, entitySection := range entitySections {
				if s == entitySection {
					return true
				}
			}
			return (s == sectionKnown || s == sectionExpanded) && isEntity(entry.Value)
		},
		fmt.Sprintf("section IN (%s) OR (section IN (%s) AND type IN (%s))", sections, mixed, typeList), args...)
	c.compact()
	return removed
}

// ClearOlderThan removes everything cached before cutoff, killmails and deleted characters included, returning how many entries were removed
func (c *CachedEveIDs) ClearOlderThan(cutoff time.Time) int {
	removed := c.removeWhere(func(_ section, entry *CacheEntry) bool { return entry.CachedAt.Before(cutoff) }, "cached_at < ?", msOf(cutoff))
	c.compact()
	return removed
}

// isEntity reports whether a cached value is a pilot, corporation, or alliance, raw or readable
func isEntity(value any) bool {
	switch value.(type) {
	case *api.ESICharacter, *api.ESICorporation, *api.ESIAlliance, *Pilot, *Corporation, *Alliance:
		return true
	}
	return false
}
