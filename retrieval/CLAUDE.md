# retrieval

## Cache storage

The maps hold only what the session used: a miss in `cachedAs` reads the SQLite row (store.go, `modernc.org/sqlite`, one `cache` table keyed by `section` and key), and every write goes through `put`, marked dirty for `SaveCache`. **Always pass the map's matching `section` to `cachedAs`/`put`.** A value in memory as another type than asked for (`otherType`) is a miss, never converted and overwritten; id lookups check the record's own id (`ownID`); a cached value whose own id is 0 (`missingID`) is refetched once. **Bump `schemaVersion` (store.go) when the table layout changes**; a database with another version is emptied. `LoadCache` always returns a usable cache (a corrupt file is renamed to `.corrupt`).
