package retrieval

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	_ "modernc.org/sqlite" // the pure Go SQLite driver, so the app still builds without a C compiler
)

/*
	The cache is kept in a SQLite database: one row per cached value, read only when it is first wanted and written in batches by SaveCache.
	The cache's maps hold what this session has used or added, and dirty holds what SaveCache has still to write.
*/

// schemaVersion is bumped whenever the table layout changes. A database with another version is emptied and rebuilt, since everything in it can be fetched again.
const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS cache (
	section   TEXT    NOT NULL, -- which of the cache's maps the value belongs to, see section
	key       TEXT    NOT NULL, -- the eve id, or the names searched for a name lookup
	type      TEXT    NOT NULL, -- the Go type of the value, ie *api.ESICharacter, so pilot, corp, and alliance data can be cleared on its own
	cached_at INTEGER NOT NULL, -- unix milliseconds, UTC
	permanent INTEGER NOT NULL DEFAULT 0,
	value     BLOB    NOT NULL, -- json, gzipped for killmails
	PRIMARY KEY (section, key)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS cache_age ON cache (cached_at);
CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

// section names one of the cache's maps. Each is a table section, as their keys can clash.
type section string

const (
	sectionKnown                section = "known"
	sectionExpanded             section = "expanded"
	sectionKillmails            section = "killmail"
	sectionNameLookups          section = "name_lookup"
	sectionEntityNames          section = "entity_name"
	sectionAllianceCorporations section = "alliance_corporations"
	sectionCorporationHistories section = "corporation_history"
	sectionAllianceHistories    section = "alliance_history"
	sectionAffiliations         section = "affiliation"
	sectionRecentSearches       section = "recent_search"
	sectionIgnoredAllies        section = "ignored_ally"
	sectionHoldings             section = "holding"
	sectionZkillLists           section = "zkill_list"
	sectionIntelChecks          section = "intel_check"
	sectionImages               section = "image"
	sectionAppState             section = "app_state"
)

// compressed reports whether a section's values are gzipped. Killmails and zKillboard lists are most of the cache and shrink several times, the rest are small.
func (s section) compressed() bool {
	return s == sectionKillmails || s == sectionZkillLists
}

// rowKey is a value's place in the table
type rowKey struct {
	section section
	key     string
}

// keyText writes a map key as the table's key column
func keyText[K comparable](key K) string {
	return fmt.Sprint(key)
}

// msOf and timeOf convert the cached_at column, unix milliseconds
func msOf(t time.Time) int64    { return t.UnixMilli() }
func timeOf(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

/*
	openStore opens the cache database at path, creating it and its folder if needed, and makes sure it has the current schema

Returns:
  - *sql.DB: the open database, with a single connection so the cache's reads and writes never wait on each other's locks
  - error: if it could not be opened or is not a usable SQLite database
*/
func openStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("unable to create cache directory %s: %w", filepath.Dir(path), err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// one connection keeps the pragmas below, and SQLite allows only one writer anyway
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{"PRAGMA journal_mode = WAL", "PRAGMA synchronous = NORMAL", "PRAGMA busy_timeout = 5000"} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("unable to set up cache database %s: %w", path, err)
		}
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, fmt.Errorf("cache %s is not a usable database: %w", path, err)
	}
	if version != schemaVersion {
		if version != 0 {
			comm.Logger.Infof("Cache %s is schema version %d, expected %d - starting fresh", path, version, schemaVersion)
		}
		if _, err := db.Exec("DROP TABLE IF EXISTS cache; DROP TABLE IF EXISTS meta;"); err != nil {
			db.Close()
			return nil, fmt.Errorf("unable to reset cache %s: %w", path, err)
		}
	}
	if _, err := db.Exec(schema + fmt.Sprintf("PRAGMA user_version = %d;", schemaVersion)); err != nil {
		db.Close()
		return nil, fmt.Errorf("unable to create the cache tables in %s: %w", path, err)
	}

	return db, nil
}

// removeStoreFiles deletes a cache database and the write-ahead log files SQLite keeps beside it
func removeStoreFiles(path string) {
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(file); err != nil && !errors.Is(err, os.ErrNotExist) {
			comm.Logger.WithError(err).Warnf("Unable to remove %s", file)
		}
	}
}

// encodeValue writes a value for the value column
func encodeValue(s section, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil || !s.compressed() {
		return data, err
	}

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// decodeValue reads the value column into a *T
func decodeValue[T any](s section, data []byte) (*T, error) {
	if s.compressed() {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if data, err = io.ReadAll(reader); err != nil {
			return nil, err
		}
	}

	value := new(T)
	if err := json.Unmarshal(data, value); err != nil {
		return nil, err
	}
	return value, nil
}

/*
	loadRow reads a value this session has not used yet from the database. The caller must hold c.mu.

Returns:
  - *CacheEntry: the entry with its value as a *T, or nil if it is not in the database or can not be read
*/
func loadRow[T any](c *CachedEveIDs, s section, key string) *CacheEntry {
	if c.db == nil {
		return nil
	}

	var cachedAt int64
	var permanent bool
	var data []byte
	err := c.db.QueryRow("SELECT cached_at, permanent, value FROM cache WHERE section = ? AND key = ?", string(s), key).Scan(&cachedAt, &permanent, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		comm.Logger.WithError(err).Warnf("Unable to read cached %s %s", s, key)
		return nil
	}

	value, err := decodeValue[T](s, data)
	if err != nil {
		comm.Logger.WithError(err).Warnf("Cached %s %s is not a %T, it will be fetched again", s, key, *new(T))
		return nil
	}
	return &CacheEntry{Value: value, CachedAt: timeOf(cachedAt), Permanent: permanent}
}

// put caches an entry in one of the maps and marks it to be written by the next SaveCache. The caller must hold c.mu.
func put[K comparable](c *CachedEveIDs, s section, entries map[K]*CacheEntry, key K, entry *CacheEntry) {
	entries[key] = entry
	c.dirty[rowKey{section: s, key: keyText(key)}] = entry
}

// storedRow is a dirty entry ready to be written
type storedRow struct {
	rowKey
	typeName  string
	cachedAt  int64
	permanent bool
	value     []byte
}

/*
flush writes every entry added since the last flush to the database, in one transaction.
The values are encoded while holding c.mu, so nothing changes them mid write, and written after letting it go, so lookups carry on meanwhile.
If the write fails the entries stay dirty for the next flush.
*/
func (c *CachedEveIDs) flush() error {
	c.mu.Lock()
	if c.db == nil || len(c.dirty) == 0 {
		c.mu.Unlock()
		return nil
	}
	pending := c.dirty
	c.dirty = map[rowKey]*CacheEntry{}

	rows := make([]storedRow, 0, len(pending))
	var encodeErrs []error
	for key, entry := range pending {
		value, err := encodeValue(key.section, entry.Value)
		if err != nil {
			encodeErrs = append(encodeErrs, fmt.Errorf("%s %s: %w", key.section, key.key, err))
			continue
		}
		rows = append(rows, storedRow{rowKey: key, typeName: fmt.Sprintf("%T", entry.Value), cachedAt: msOf(entry.CachedAt), permanent: entry.Permanent, value: value})
	}
	db := c.db
	c.mu.Unlock()

	if err := writeRows(db, rows); err != nil {
		// keep them for the next flush, unless they were replaced meanwhile
		c.mu.Lock()
		for key, entry := range pending {
			if _, replaced := c.dirty[key]; !replaced {
				c.dirty[key] = entry
			}
		}
		c.mu.Unlock()
		return err
	}

	if len(encodeErrs) > 0 {
		comm.Logger.WithError(errors.Join(encodeErrs...)).Warn("Some cached values could not be saved, they will be fetched again next time")
	}
	return nil
}

// writeRows upserts rows in one transaction
func writeRows(db *sql.DB, rows []storedRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statement, err := tx.Prepare(`INSERT INTO cache (section, key, type, cached_at, permanent, value) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (section, key) DO UPDATE SET type = excluded.type, cached_at = excluded.cached_at, permanent = excluded.permanent, value = excluded.value`)
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, row := range rows {
		if _, err := statement.Exec(string(row.section), row.key, row.typeName, row.cachedAt, row.permanent, row.value); err != nil {
			return fmt.Errorf("unable to save cached %s %s: %w", row.section, row.key, err)
		}
	}
	return tx.Commit()
}

// readMeta and writeMeta keep values about the cache itself, ie the latest SDE release seen
func readMeta(db *sql.DB, key string) string {
	var value string
	if err := db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&value); err != nil && !errors.Is(err, sql.ErrNoRows) {
		comm.Logger.WithError(err).Warnf("Unable to read cache setting %s", key)
	}
	return value
}

func writeMeta(db *sql.DB, key, value string) error {
	_, err := db.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

const metaSDERelease = "sde_release"

// sectionPlaceholders writes "?, ?, ?" for an IN list, with the sections as its arguments
func sectionPlaceholders(sections ...section) (string, []any) {
	args := make([]any, len(sections))
	for index, s := range sections {
		args[index] = string(s)
	}
	return sectionPlaceholdersFor(len(sections)), args
}

// sectionPlaceholdersFor writes "?, ?, ?" for an IN list of count values
func sectionPlaceholdersFor(count int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", count), ", ")
}
