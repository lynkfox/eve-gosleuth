package retrieval

import (
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// The api.ESIClient methods of CachedEveIDs. Each is answered from the cache when it can be, otherwise fetched with the uncached Client and cached.

// cachedCall gets a cached or freshly fetched ESI response as its model type
func cachedCall[T any](c *CachedEveIDs, idNumber int, typeOfEntity types.EntityType) (*T, error) {
	response, err := c.GetIfNotCached(idNumber, typeOfEntity)
	if err != nil {
		return nil, err
	}
	return (*response).(*T), nil
}

func (c *CachedEveIDs) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	return cachedCall[api.ESICharacter](c, idNumber, types.PILOT)
}

func (c *CachedEveIDs) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return cachedCall[api.ESICorporation](c, idNumber, types.CORPORATION)
}

func (c *CachedEveIDs) GetAllianceInfo(idNumber int) (*api.ESIAlliance, error) {
	return cachedCall[api.ESIAlliance](c, idNumber, types.ALLIANCE)
}

func (c *CachedEveIDs) GetSystemInfo(idNumber int) (*api.ESISystem, error) {
	return cachedCall[api.ESISystem](c, idNumber, types.SYSTEM)
}

func (c *CachedEveIDs) GetItemInfo(idNumber int) (*api.ESIItem, error) {
	return cachedCall[api.ESIItem](c, idNumber, types.ITEM)
}

func (c *CachedEveIDs) GetStargateInfo(idNumber int) (*api.ESIStargate, error) {
	return cachedCall[api.ESIStargate](c, idNumber, types.STARGATE)
}

func (c *CachedEveIDs) GetStarInfo(idNumber int) (*api.ESIStar, error) {
	return cachedCall[api.ESIStar](c, idNumber, types.STAR)
}

func (c *CachedEveIDs) GetPlanetInfo(idNumber int) (*api.ESIPlanetInfo, error) {
	return cachedCall[api.ESIPlanetInfo](c, idNumber, types.PLANET)
}

func (c *CachedEveIDs) GetCategoryInfo(idNumber int) (*api.ESICategory, error) {
	return cachedCall[api.ESICategory](c, idNumber, types.CATEGORY)
}

func (c *CachedEveIDs) GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error) {
	response, err := c.GetIfNotCached(idNumber, typeOfEntity)
	if err != nil {
		return nil, err
	}
	return *response, nil
}

// CachedKillmail returns a killmail if the cache already holds it, without fetching it, or nil
func (c *CachedEveIDs) CachedKillmail(killmailID int) *api.ESIKillmail {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached := cachedAs[api.ESIKillmail](c, sectionKillmails, c.Killmails, killmailID)
	if cached == nil || missingID(cached) {
		return nil
	}
	return cached
}

// StoreKillmail caches a killmail that came from elsewhere, ie zKillboard's copy of the ESI's. Killmails never change, so it is kept like a fetched one.
func (c *CachedEveIDs) StoreKillmail(killmail *api.ESIKillmail) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := newEntry(killmail)
	entry.Permanent = true
	put(c, sectionKillmails, c.Killmails, killmail.KillmailID, entry)
}

// GetKillmail returns a killmail from the cache, or fetches and caches it. Killmails never change, so they are never refetched.
func (c *CachedEveIDs) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	c.mu.Lock()
	cached := cachedAs[api.ESIKillmail](c, sectionKillmails, c.Killmails, zkillmail.KillmailID)
	if cached != nil {
		c.counts.Cached++
	}
	c.mu.Unlock()

	if cached != nil && !missingID(cached) {
		comm.Logger.Debugf("Found killmail %d in esi cache", zkillmail.KillmailID)
		return cached, nil
	}

	killmail, err := c.Client.GetKillmail(zkillmail)
	c.countFetched(1, err)
	if err != nil {
		// keep using a cached copy that is only missing its id, the next lookup tries the ESI again
		if cached != nil {
			return cached, nil
		}
		return nil, err
	}

	c.mu.Lock()
	entry := newEntry(killmail)
	entry.Permanent = true
	put(c, sectionKillmails, c.Killmails, zkillmail.KillmailID, entry)
	c.mu.Unlock()

	return killmail, nil
}

// GetEveIDsFromNames returns the eve IDs for names from the cache, or looks them up and caches them. Searches that find nothing are not cached, so a new name is found once it exists.
func (c *CachedEveIDs) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	key := strings.Join(names, "\n")

	c.mu.Lock()
	cached := cachedAs[map[int]api.EntityIDAndName](c, sectionNameLookups, c.NameLookups, key)
	if cached != nil {
		c.counts.Cached++
	}
	c.mu.Unlock()

	if cached != nil && !missingID(cached) {
		return *cached, nil
	}

	found, err := c.Client.GetEveIDsFromNames(names)
	c.countFetched(1, err)
	if cached != nil && (err != nil || len(found) == 0) {
		// keep using a cached copy that is only missing an id, the next lookup tries the ESI again
		return *cached, nil
	}
	if err != nil || len(found) == 0 {
		return found, err
	}

	c.mu.Lock()
	put(c, sectionNameLookups, c.NameLookups, key, newEntry(&found))
	c.mu.Unlock()

	return found, nil
}

// GetNamesFromEveIDs returns the names for eve IDs, looking up only the IDs not already cached
// GetAllianceCorporations returns an alliance's member corp ids, cached apart from Known as the alliance id is also the alliance's own key there
func (c *CachedEveIDs) GetAllianceCorporations(allianceID int) (*api.ESIAllianceCorporations, error) {
	return cachedApart(c, sectionAllianceCorporations, c.AllianceCorporations, allianceID, c.Client.GetAllianceCorporations)
}

// GetCorporationHistory returns a character's corp history, cached apart from Known as the character id is also the character's own key there
func (c *CachedEveIDs) GetCorporationHistory(characterID int) (*api.ESICorporationHistory, error) {
	return cachedApart(c, sectionCorporationHistories, c.CorporationHistories, characterID, c.Client.GetCorporationHistory)
}

// GetAllianceHistory returns a corporation's alliance history, cached apart from Known as the corporation id is also the corporation's own key there
func (c *CachedEveIDs) GetAllianceHistory(corporationID int) (*api.ESIAllianceHistory, error) {
	return cachedApart(c, sectionAllianceHistories, c.AllianceHistories, corporationID, c.Client.GetAllianceHistory)
}

// cachedApart answers a lookup from one of the cache's own maps, or fetches and caches it. A copy missing its id is refetched, and kept if that fails.
func cachedApart[T any](c *CachedEveIDs, s section, entries map[int]*CacheEntry, idNumber int, fetch func(int) (*T, error)) (*T, error) {
	c.mu.Lock()
	cached := cachedAs[T](c, s, entries, idNumber)
	if cached != nil {
		c.counts.Cached++
	}
	c.mu.Unlock()

	if cached != nil && !missingID(cached) {
		return cached, nil
	}

	fetched, err := fetch(idNumber)
	c.countFetched(1, err)
	if err != nil {
		if cached != nil {
			return cached, nil // only missing its id, the next lookup tries the ESI again
		}
		return nil, err
	}

	c.mu.Lock()
	put(c, s, entries, idNumber, newEntry(fetched))
	c.mu.Unlock()
	return fetched, nil
}

// GetCharacterAffiliations returns characters' current corp and alliance, looking up only those not already cached, in bulk
func (c *CachedEveIDs) GetCharacterAffiliations(characterIDs []int) (map[int]api.ESIAffiliation, error) {
	affiliations := map[int]api.ESIAffiliation{}
	missing := []int{}

	c.mu.Lock()
	for _, characterID := range characterIDs {
		if affiliation := cachedAs[api.ESIAffiliation](c, sectionAffiliations, c.Affiliations, characterID); affiliation != nil && !missingID(affiliation) {
			affiliations[characterID] = *affiliation
			c.counts.Cached++
		} else {
			missing = append(missing, characterID)
		}
	}
	c.mu.Unlock()

	if len(missing) == 0 {
		return affiliations, nil
	}

	found, err := c.Client.GetCharacterAffiliations(missing)
	c.countFetched(len(missing), err)

	// earlier batches are kept even when a later one failed
	c.mu.Lock()
	for characterID, affiliation := range found {
		affiliations[characterID] = affiliation
		put(c, sectionAffiliations, c.Affiliations, characterID, newEntry(&affiliation))
	}
	c.mu.Unlock()

	return affiliations, err
}

func (c *CachedEveIDs) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	names := map[int]string{}
	missing := []int{}

	c.mu.Lock()
	for _, idNumber := range idsNumbers {
		if name := cachedAs[string](c, sectionEntityNames, c.EntityNames, idNumber); name != nil {
			names[idNumber] = *name
			c.counts.Cached++
		} else {
			missing = append(missing, idNumber)
		}
	}
	c.mu.Unlock()

	if len(missing) == 0 {
		return names, nil
	}

	found, err := c.Client.GetNamesFromEveIDs(missing)
	c.countFetched(len(missing), err)
	if err != nil {
		return names, err
	}

	c.mu.Lock()
	for idNumber, name := range found {
		names[idNumber] = name
		put(c, sectionEntityNames, c.EntityNames, idNumber, newEntry(&name))
	}
	c.mu.Unlock()

	return names, nil
}
