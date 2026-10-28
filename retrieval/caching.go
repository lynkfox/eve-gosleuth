package retrieval

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

const cacheDirName = "eve-gosleuth"
const cacheFileName = "cache.db"

// legacyCacheFileName is the json file the cache was kept in before it moved to SQLite. Its killmails are imported once, then it is removed.
const legacyCacheFileName = "cached_data.json"

// volatileTTL is how long pilot, corporation, and alliance data is trusted before it is refetched, since membership changes
const volatileTTL = 24 * time.Hour

// CacheEntry is a cached value and when it was cached. Values are always their model type, ie *api.ESICharacter.
// Pilots, corporations, and alliances are trusted for volatileTTL, Permanent entries (killmails) always, and everything else until a newer SDE is released.
type CacheEntry struct {
	Value     any       `json:"value"`
	CachedAt  time.Time `json:"cached_at"`
	Permanent bool      `json:"permanent,omitempty"`
}

// stale returns true if the entry was cached before the latest SDE release, or is older than ttl when ttl is set, and so should be refetched
func (e *CacheEntry) stale(sdeRelease time.Time, ttl time.Duration) bool {
	if e.Permanent {
		return false
	}
	return e.CachedAt.Before(sdeRelease) || (ttl > 0 && time.Since(e.CachedAt) > ttl)
}

// ttlFor returns how long a cached *T is trusted regardless of SDE releases: volatileTTL for pilots, corporations, and alliances (raw or expanded), otherwise 0 for no limit
func ttlFor[T any]() time.Duration {
	switch any(new(T)).(type) {
	case *api.ESICharacter, *api.ESICorporation, *api.ESIAlliance, *api.ESIAllianceCorporations, *api.ESICorporationHistory, *api.ESIAllianceHistory, *api.ESIAffiliation, *Pilot, *Corporation, *Alliance:
		return volatileTTL
	}
	return 0
}

func newEntry(value any) *CacheEntry {
	return &CacheEntry{Value: value, CachedAt: time.Now().UTC()}
}

/*
	CachedEveIDs caches every ESI response, kept in a SQLite database between runs (see store.go).

Known holds raw ESI responses by eve ID (such as *api.ESICharacter), Expanded holds the human readable models built from them (such as *Pilot).
Killmails, name searches, id to name lookups, alliance corp lists, corp and alliance histories, and character affiliations are kept apart, since their keys can clash with eve IDs.
The maps only hold what this session has used or added: anything else is read from the database when first wanted, and SaveCache writes what was added.

It is also an api.ESIClient: each call is answered from the cache when it can be, otherwise it is fetched with Client and cached.
Pilots, corporations, alliances, alliance corp lists, corp and alliance histories, and affiliations are refetched after 24 hours, killmails never, and everything else once a newer SDE than SDERelease is seen.

Safe for concurrent use.
*/
type CachedEveIDs struct {
	SDERelease           time.Time // the latest SDE release seen, data cached before it is refetched
	Known                map[int]*CacheEntry
	Expanded             map[int]*CacheEntry
	Killmails            map[int]*CacheEntry    // never expire
	NameLookups          map[string]*CacheEntry // universe/ids results, keyed by the names searched
	EntityNames          map[int]*CacheEntry    // universe/names results, eve id to name
	AllianceCorporations map[int]*CacheEntry    // alliance member corp lists, by alliance id
	CorporationHistories map[int]*CacheEntry    // characters' corp histories, by character id
	AllianceHistories    map[int]*CacheEntry    // corporations' alliance histories, by corporation id
	Affiliations         map[int]*CacheEntry    // characters' current corp and alliance, by character id
	Searched             map[string]*CacheEntry // the search bar's recent searches, by recentSearchKey. Never expire, see AddRecentSearch.
	IgnoredAllies        map[string]*CacheEntry // each target's allies left out of its Allies tab, by ignoredAlliesKey. Never expire, see SetIgnoredAllies.
	Holdings             map[string]*CacheEntry // the user's holding links, by holdingKey. Never expire, see SaveHoldingLink.
	PendingChecks        map[string]*CacheEntry // imported pilots, corporations, and alliances still to check with the ESI, by intelCheckKey, see AddIntelChecks
	ZkillLists           map[string]*CacheEntry // entities' zKillboard killmail lists, by zkillListKey. Never expire, see SaveKnownZkillList.
	Client               api.ESIClient          // the uncached client that cache misses are fetched with
	Path                 string                 // the database file
	mu                   sync.Mutex
	counts               CacheCounts // lookups since the app started, for progress messages

	searchedLoaded bool // Searched has every saved recent search, see loadRecentSearches. Guarded by mu.
	holdingsLoaded bool // Holdings has every saved holding link, see loadHoldings. Guarded by mu.
	ignoredLoaded  bool // IgnoredAllies has every target's saved ignored allies, see AllIgnoredAllies. Guarded by mu.
	checksLoaded   bool // PendingChecks has every saved check, see IntelChecks. Guarded by mu.

	db      *sql.DB                // nil if the database could not be opened, the cache then only lasts the session
	dirty   map[rowKey]*CacheEntry // added since the last SaveCache, guarded by mu
	storeMu sync.Mutex             // held while writing to or replacing the database, so a removal never races a save. Taken before mu.
}

// CacheCounts tallies lookups the cache answered itself and lookups it had to fetch from the ESI, and which of those failed
type CacheCounts struct {
	Cached  int
	Fetched int // includes the failed lookups
	Failed  int
	Reason  string // why the latest failed lookup failed, ie esi.evetech.net returned 502 Bad Gateway
}

// Since returns the lookups made after start
func (c CacheCounts) Since(start CacheCounts) CacheCounts {
	since := CacheCounts{Cached: c.Cached - start.Cached, Fetched: c.Fetched - start.Fetched, Failed: c.Failed - start.Failed}
	if since.Failed > 0 {
		since.Reason = c.Reason
	}
	return since
}

// Source describes where lookups came from for progress messages, ie "95 hits from cache, 25 from the ESI", with any failures and why. Empty if there were none.
func (c CacheCounts) Source() string {
	parts := []string{}
	if c.Cached > 0 {
		parts = append(parts, fmt.Sprintf("%s from cache", countOfHits(c.Cached)))
	}
	if c.Fetched > 0 {
		parts = append(parts, fmt.Sprintf("%d from the ESI", c.Fetched))
	}
	if c.Failed > 0 {
		failed := fmt.Sprintf("%d failed", c.Failed)
		if c.Reason != "" {
			failed += ": " + c.Reason
		}
		parts = append(parts, failed)
	}
	return strings.Join(parts, ", ")
}

// countOfHits writes a cache hit count, ie 1 hit or 95 hits
func countOfHits(count int) string {
	if count == 1 {
		return "1 hit"
	}
	return fmt.Sprintf("%d hits", count)
}

// Annotate adds where lookups came from to a progress message, ie "Killmail details 3/20 (1 hit from cache)". The message is unchanged if there were none.
func (c CacheCounts) Annotate(message string) string {
	if source := c.Source(); source != "" {
		return fmt.Sprintf("%s (%s)", message, source)
	}
	return message
}

// Counts returns the lookups made so far. Take one before and after some work and use Since to see where that work's data came from.
func (c *CachedEveIDs) Counts() CacheCounts {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts
}

// CountsOf returns the cache counts of an ESI client that is the cache, or zero counts for an uncached client
func CountsOf(client api.ESIClient) CacheCounts {
	if cache, ok := client.(*CachedEveIDs); ok {
		return cache.Counts()
	}
	return CacheCounts{}
}

// the cache answers every ESI call, so it can stand in for the uncached client anywhere
var _ api.ESIClient = (*CachedEveIDs)(nil)

/*
NewCache opens the cache database at path, creating it if needed. client must be the uncached ESI client, never another cache.
If the database can not be opened the cache still works, for this session only. LoadCache also recovers a corrupt database and imports the old json cache.
*/
func NewCache(client api.ESIClient, path string) *CachedEveIDs {
	cache := &CachedEveIDs{Client: client, Path: path}
	cache.ensureMaps()

	db, err := openStore(path)
	if err != nil {
		comm.Logger.WithError(err).Error("Unable to open the cache database, the cache only lasts this session")
		return cache
	}
	cache.attach(db)
	return cache
}

// attach starts using an open database, reading the SDE release it has seen
func (c *CachedEveIDs) attach(db *sql.DB) {
	c.db = db
	if release, err := time.Parse(time.RFC3339, readMeta(db, metaSDERelease)); err == nil {
		c.SDERelease = release
	}
}

// ensureMaps creates any maps missing, ie after a clear
func (c *CachedEveIDs) ensureMaps() {
	if c.Known == nil {
		c.Known = make(map[int]*CacheEntry)
	}
	if c.Expanded == nil {
		c.Expanded = make(map[int]*CacheEntry)
	}
	if c.Killmails == nil {
		c.Killmails = make(map[int]*CacheEntry)
	}
	if c.NameLookups == nil {
		c.NameLookups = make(map[string]*CacheEntry)
	}
	if c.EntityNames == nil {
		c.EntityNames = make(map[int]*CacheEntry)
	}
	if c.AllianceCorporations == nil {
		c.AllianceCorporations = make(map[int]*CacheEntry)
	}
	if c.CorporationHistories == nil {
		c.CorporationHistories = make(map[int]*CacheEntry)
	}
	if c.AllianceHistories == nil {
		c.AllianceHistories = make(map[int]*CacheEntry)
	}
	if c.Affiliations == nil {
		c.Affiliations = make(map[int]*CacheEntry)
	}
	if c.Searched == nil {
		c.Searched = make(map[string]*CacheEntry)
	}
	if c.IgnoredAllies == nil {
		c.IgnoredAllies = make(map[string]*CacheEntry)
	}
	if c.Holdings == nil {
		c.Holdings = make(map[string]*CacheEntry)
	}
	if c.PendingChecks == nil {
		c.PendingChecks = make(map[string]*CacheEntry)
	}
	if c.ZkillLists == nil {
		c.ZkillLists = make(map[string]*CacheEntry)
	}
	if c.dirty == nil {
		c.dirty = make(map[rowKey]*CacheEntry)
	}
}

// DefaultCachePath returns the per user cache file location (%LocalAppData%\eve-gosleuth on Windows), falling back to the working directory
func DefaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		comm.Logger.WithError(err).Warn("No user cache directory, using the working directory for the cache")
		return cacheFileName
	}

	return filepath.Join(dir, cacheDirName, cacheFileName)
}

/*
	LoadCache opens the cache database at path. It always returns a usable cache: if the database is missing it is created, and if it is unreadable it starts empty.
	The first time, the killmails in the old json cache beside it are imported, as they are the costly part to fetch again, and the json file is removed.

Parameters:
  - client: the ESI client used to fill cache misses
  - path: the database location, normally DefaultCachePath()

Returns:
  - *CachedEveIDs: the cache
  - error: nil, or why the saved cache could not be used. A corrupt database is moved aside to <path>.corrupt
*/
func LoadCache(client api.ESIClient, path string) (*CachedEveIDs, error) {
	cache := &CachedEveIDs{Client: client, Path: path}
	cache.ensureMaps()

	db, err := openStore(path)
	if err != nil {
		corruptPath := path + ".corrupt"
		removeStoreFiles(corruptPath)
		renameErr := os.Rename(path, corruptPath)
		removeStoreFiles(path)
		if db, err2 := openStore(path); err2 == nil {
			cache.attach(db)
		}
		if renameErr != nil {
			return cache, fmt.Errorf("cache %s is unusable and could not be moved aside: %w", path, errors.Join(err, renameErr))
		}
		return cache, fmt.Errorf("cache %s is unusable, moved to %s: %w", path, corruptPath, err)
	}
	cache.attach(db)

	if err := cache.importLegacy(filepath.Join(filepath.Dir(path), legacyCacheFileName)); err != nil {
		return cache, err
	}
	return cache, nil
}

/*
	importLegacy copies the killmails from the old json cache into the database, then removes the json file. Everything else in it is left to be fetched again, as it is cheap to.

Returns:
  - error: if the json file could not be read or the killmails saved. The json file is kept then, to try again next time.
*/
func (c *CachedEveIDs) importLegacy(legacyPath string) error {
	file, err := os.Open(legacyPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to open the old cache %s: %w", legacyPath, err)
	}

	legacy := struct {
		Version   int                 `json:"version"`
		Killmails map[int]*CacheEntry `json:"killmails"`
	}{}
	err = json.NewDecoder(file).Decode(&legacy)
	file.Close()

	imported := 0
	if err == nil && legacy.Version == 2 {
		c.mu.Lock()
		for killmailID, entry := range legacy.Killmails {
			killmail, castErr := CastExpanded[api.ESIKillmail](entry.Value)
			if castErr != nil || killmail == nil || killmail.KillmailID == 0 {
				continue
			}
			put(c, sectionKillmails, c.Killmails, killmailID, &CacheEntry{Value: killmail, CachedAt: entry.CachedAt, Permanent: true})
			imported++
		}
		c.mu.Unlock()

		if err := c.flush(); err != nil {
			return fmt.Errorf("unable to import the killmails from the old cache %s: %w", legacyPath, err)
		}
		// they are read back from the database when wanted, rather than all held now
		c.mu.Lock()
		c.Killmails = make(map[int]*CacheEntry)
		c.mu.Unlock()
	}

	comm.ReportProgress("Moved %d killmails from the old cache file into the new cache", imported)
	if err := os.Remove(legacyPath); err != nil {
		comm.Logger.WithError(err).Warnf("Unable to remove the old cache %s", legacyPath)
	}
	return nil
}

// SaveCache writes everything added to the cache since the last save to its database, in one transaction
func SaveCache(c *CachedEveIDs) error {
	if err := c.flush(); err != nil {
		return fmt.Errorf("unable to save cache to %s: %w", c.CurrentPath(), err)
	}
	return nil
}

// Close saves the cache and closes its database. The cache only lasts the session after.
func (c *CachedEveIDs) Close() error {
	saveErr := SaveCache(c)

	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return saveErr
	}
	closeErr := c.db.Close()
	c.db = nil
	return errors.Join(saveErr, closeErr)
}

/*
	UpdateSDERelease records the latest SDE release. If it is newer than the last one seen, every cached entry from before it except killmails is dropped, so it is refetched when next used.

Parameters:
  - release: the release date of the latest SDE

Returns:
  - int: how many entries were dropped, 0 if the release was not newer
*/
func (c *CachedEveIDs) UpdateSDERelease(release time.Time) int {
	c.mu.Lock()
	newer := release.After(c.SDERelease)
	if newer {
		c.SDERelease = release
	}
	db := c.db
	c.mu.Unlock()

	if !newer {
		return 0
	}
	if db != nil {
		if err := writeMeta(db, metaSDERelease, release.UTC().Format(time.RFC3339)); err != nil {
			comm.Logger.WithError(err).Warn("Unable to save the SDE release seen, it will be checked again")
		}
	}

	return c.removeWhere(
		func(_ section, entry *CacheEntry) bool { return entry.stale(release, 0) },
		"permanent = 0 AND cached_at < ?", msOf(release))
}

/*
	cachedAs returns the current cached value for a key as a *T, or nil if it is missing, stale, or the wrong type. The caller must hold c.mu.
	A value this session has not used yet is read from the database.

Whether it is stale depends on T: see ttlFor.

A value of another type, ie one a test cached directly, is converted once and the typed value is stored back for later lookups.
*/
func cachedAs[T any, K comparable](c *CachedEveIDs, s section, entries map[K]*CacheEntry, key K) *T {
	entry, found := entries[key]
	if !found {
		if entry = loadRow[T](c, s, keyText(key)); entry == nil {
			return nil
		}
		entries[key] = entry
	}
	if entry.Value == nil || entry.stale(c.SDERelease, ttlFor[T]()) || otherType[T](entry.Value) {
		return nil
	}

	casted, err := CastExpanded[T](entry.Value)
	if err != nil {
		comm.Logger.Errorf("Cached value for %v is not a %T: %v", key, *new(T), err)
		return nil
	}

	entry.Value = casted
	return casted
}

// FetchExpanded returns the current human readable model for an ID as a *T (such as *Pilot), or nil if it needs to be built
func FetchExpanded[T any](c *CachedEveIDs, idNumber int) *T {
	c.mu.Lock()
	defer c.mu.Unlock()

	expanded := cachedAs[T](c, sectionExpanded, c.Expanded, idNumber)
	if expanded == nil {
		return nil
	}
	// a model missing its id is rebuilt by the caller, which sets the id it was asked for
	if missingID(expanded) {
		comm.Logger.Warnf("Cached %T for %d has no id, rebuilding it", expanded, idNumber)
		return nil
	}
	c.counts.Cached++
	return expanded
}

// knownAs returns the current raw ESI response for an ID, returning an untyped nil rather than a typed nil pointer when missing
func knownAs[T any](c *CachedEveIDs, idNumber int) any {
	c.mu.Lock()
	defer c.mu.Unlock()

	if value := cachedAs[T](c, sectionKnown, c.Known, idNumber); value != nil {
		c.counts.Cached++
		return value
	}
	return nil
}

/* CastExpanded for an epxandable readable version of the data that is also a specific struct*/
/*
otherType reports whether a cached value is already a model of another type than *T, ie a corporation asked for as a pilot, from a wrong link or a doctored import.
Converting it would give a wrong *T and overwrite the right value, so it is treated as not cached.
*/
func otherType[T any](value any) bool {
	if _, same := value.(*T); same {
		return false
	}
	return reflect.ValueOf(value).Kind() == reflect.Pointer
}

func CastExpanded[T any](value any) (*T, error) {
	if value == nil {
		return nil, nil
	}

	if casted, ok := value.(*T); ok {
		return casted, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var data_casted T
	if err := json.Unmarshal(data, &data_casted); err != nil {
		return nil, err
	}

	return &data_casted, nil

}

/*
	missingID returns true if a cached value's own eve id is 0, which means it was cached badly (ie by an older version that did not save the id) and should be fetched again.

Name searches are missing an id if any of their matches is. Values with no id of their own, such as names, are never missing one.
*/
func missingID(value any) bool {
	switch typed := value.(type) {
	case *api.ESICharacter:
		return typed.CharacterID == 0
	case *api.ESICorporation:
		return typed.CorporationID == 0
	case *api.ESIAlliance:
		return typed.AllianceID == 0
	case *api.ESISystem:
		return typed.SystemID == 0
	case *api.ESIConstellation:
		return typed.ConstellationID == 0
	case *api.ESIRegion:
		return typed.RegionID == 0
	case *api.ESIStargate:
		return typed.StargateID == 0
	case *api.ESIAllianceCorporations:
		return typed.AllianceID == 0
	case *api.ESICorporationHistory:
		return typed.CharacterID == 0
	case *api.ESIAllianceHistory:
		return typed.CorporationID == 0
	case *api.ESIAffiliation:
		return typed.CharacterID == 0
	case *api.ESIStar:
		return typed.TypeID == 0 // a star response has no id of its own
	case *api.ESIPlanetInfo:
		return typed.PlanetID == 0
	case *api.ESIMoon:
		return typed.MoonID == 0
	case *api.ESICategory:
		return typed.CategoryID == 0
	case *api.ESIItem:
		return typed.ItemId == 0
	case *api.ESIKillmail:
		return typed.KillmailID == 0
	case *Pilot:
		return typed.CharacterID == 0
	case *Corporation:
		return typed.CorporationID == 0
	case *Alliance:
		return typed.AllianceID == 0
	case *System:
		return typed.SystemID == 0
	case *Item:
		return typed.ItemID == 0
	case *map[int]api.EntityIDAndName:
		for _, match := range *typed {
			if match.ID == 0 {
				return true
			}
		}
	}
	return false
}

// countFetched records lookups that had to go to the ESI, and if err is set, that they failed and why
func (c *CachedEveIDs) countFetched(lookups int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts.Fetched += lookups
	if err != nil {
		c.counts.Failed += lookups
		c.counts.Reason = api.FailureReason(err)
	}
}

/* Update caches a raw ESI response for an ID, replacing any existing entry */
func (c *CachedEveIDs) Update(idNumber int, item any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	put(c, sectionKnown, c.Known, idNumber, newEntry(item))
}

/* UpdateExpanded caches a human readable model for an ID, replacing any existing entry */
func (c *CachedEveIDs) UpdateExpanded(idNumber int, item any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	put(c, sectionExpanded, c.Expanded, idNumber, newEntry(item))
}

// GetIfNotCached takes an id number and a type and returns the raw ESI response for the IDnumber if cached and current, else calls the ESI and caches the response
func (c *CachedEveIDs) GetIfNotCached(idNumber int, typeOfEntity types.EntityType) (*interface{}, error) {

	var found any

	switch typeOfEntity {
	case types.PILOT:
		found = knownAs[api.ESICharacter](c, idNumber)
	case types.CORPORATION:
		found = knownAs[api.ESICorporation](c, idNumber)
	case types.ALLIANCE:
		found = knownAs[api.ESIAlliance](c, idNumber)
	case types.SYSTEM:
		found = knownAs[api.ESISystem](c, idNumber)
	case types.ITEM:
		found = knownAs[api.ESIItem](c, idNumber)
	case types.CONSTELLATION:
		found = knownAs[api.ESIConstellation](c, idNumber)
	case types.REGION:
		found = knownAs[api.ESIRegion](c, idNumber)
	case types.STARGATE:
		found = knownAs[api.ESIStargate](c, idNumber)
	case types.STAR:
		found = knownAs[api.ESIStar](c, idNumber)
	case types.PLANET:
		found = knownAs[api.ESIPlanetInfo](c, idNumber)
	case types.MOON:
		found = knownAs[api.ESIMoon](c, idNumber)
	case types.CATEGORY:
		found = knownAs[api.ESICategory](c, idNumber)
	}

	if found != nil && !missingID(found) {
		comm.Logger.Debugf("Found %d in esi cache", idNumber)
		return &found, nil
	}
	if found != nil {
		comm.Logger.Warnf("Cached %s %d has no id, fetching it again", typeOfEntity, idNumber)
	}

	var err error
	var output interface{}

	switch typeOfEntity {
	case types.PILOT:
		output, err = c.Client.GetCharacterInfo(idNumber)
	case types.CORPORATION:
		output, err = c.Client.GetCorporationInfo(idNumber)
	case types.ALLIANCE:
		output, err = c.Client.GetAllianceInfo(idNumber)
	case types.SYSTEM:
		output, err = c.Client.GetSystemInfo(idNumber)
	case types.ITEM:
		output, err = c.Client.GetItemInfo(idNumber)
	case types.CONSTELLATION, types.REGION:
		output, err = c.Client.GetEsiResponse(idNumber, typeOfEntity)
	case types.STARGATE:
		output, err = c.Client.GetStargateInfo(idNumber)
	case types.STAR:
		output, err = c.Client.GetStarInfo(idNumber)
	case types.PLANET:
		output, err = c.Client.GetPlanetInfo(idNumber)
	case types.MOON:
		output, err = c.Client.GetMoonInfo(idNumber)
	case types.CATEGORY:
		output, err = c.Client.GetCategoryInfo(idNumber)
	default:
		err = fmt.Errorf("%s data is not cached", typeOfEntity)
	}

	c.countFetched(1, err)

	// a character the ESI no longer has (biomassed) never comes back, so remember that rather than asking again on every killmail it is on
	if typeOfEntity == types.PILOT && api.IsNotFound(err) {
		comm.Logger.WithError(err).Infof("Character %d no longer exists, caching it as deleted", idNumber)
		output = &api.ESICharacter{CharacterID: idNumber, Name: api.DELETED_CHARACTER_NAME, Deleted: true}
		c.mu.Lock()
		// an id cached as something else, ie a corporation asked for as a pilot from a wrong link, is that, not a deleted pilot
		if existing, cached := c.Known[idNumber]; !cached || !otherType[api.ESICharacter](existing.Value) {
			entry := newEntry(output)
			entry.Permanent = true
			put(c, sectionKnown, c.Known, idNumber, entry)
		}
		c.mu.Unlock()
		return &output, nil
	}

	if err != nil {
		// keep using a cached copy that is only missing its id, the next lookup tries the ESI again
		if found != nil {
			comm.Logger.WithError(err).Warnf("Unable to refetch %s %d, using the cached copy", typeOfEntity, idNumber)
			return &found, nil
		}
		return &output, err
	}

	c.Update(idNumber, output)
	return &output, nil
}
