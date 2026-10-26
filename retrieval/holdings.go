package retrieval

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// HoldingLink ties a corporation or alliance to a holding corporation or alliance it keeps assets or capitals in. The user sets them by hand.
type HoldingLink struct {
	MainType    string    `json:"mainType"` // CORPORATION or ALLIANCE
	MainID      int       `json:"mainId"`
	MainName    string    `json:"mainName"`
	HoldingType string    `json:"holdingType"` // CORPORATION or ALLIANCE
	HoldingID   int       `json:"holdingId"`
	HoldingName string    `json:"holdingName"`
	Note        string    `json:"note"`
	SetAt       time.Time `json:"setAt"`
}

// holdingKey is a link's key in Holdings: the main, then the holding, ie ALLIANCE:99000001>CORPORATION:98000001
func holdingKey(mainType string, mainID int, holdingType string, holdingID int) string {
	return fmt.Sprintf("%s:%d>%s:%d", mainType, mainID, holdingType, holdingID)
}

/*
loadHoldings reads every saved holding link into Holdings the first time they are wanted. The caller must hold c.mu.
Like recent searches they are read all at once, as a link is looked up from either end.
*/
func (c *CachedEveIDs) loadHoldings() {
	if c.holdingsLoaded || c.db == nil {
		return
	}
	c.holdingsLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionHoldings))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the holding links")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			comm.Logger.WithError(err).Warn("Unable to read a holding link")
			continue
		}
		link, err := decodeValue[HoldingLink](sectionHoldings, data)
		if err != nil || link.MainID == 0 || link.HoldingID == 0 {
			comm.Logger.WithError(err).Warnf("Holding link %s can not be read, it is skipped", key)
			continue
		}
		if _, found := c.Holdings[key]; !found {
			c.Holdings[key] = &CacheEntry{Value: link, CachedAt: timeOf(cachedAt), Permanent: true}
		}
	}
}

/*
	HoldingLinks returns the links an entity is on either end of

Parameters:
  - entityType: CORPORATION or ALLIANCE
  - entityID: its eve id

Returns:
  - holdings: the links where it is the main, so its holdings, by holding name
  - heldFor: the links where it is the holding, so the mains it holds for, by main name
*/
func (c *CachedEveIDs) HoldingLinks(entityType string, entityID int) (holdings []HoldingLink, heldFor []HoldingLink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadHoldings()

	holdings, heldFor = []HoldingLink{}, []HoldingLink{}
	for _, entry := range c.Holdings {
		link := *entry.Value.(*HoldingLink)
		if link.MainType == entityType && link.MainID == entityID {
			holdings = append(holdings, link)
		}
		if link.HoldingType == entityType && link.HoldingID == entityID {
			heldFor = append(heldFor, link)
		}
	}
	slices.SortFunc(holdings, func(a, b HoldingLink) int { return cmp.Compare(a.HoldingName, b.HoldingName) })
	slices.SortFunc(heldFor, func(a, b HoldingLink) int { return cmp.Compare(a.MainName, b.MainName) })
	return holdings, heldFor
}

/*
	SaveHoldingLink adds a holding link, or replaces the one between the same main and holding, ie to change its note.
	Holding links are the user's own, so cache clears leave them unless asked to clear them too.

Parameters:
  - link: the link, with both ends' names as EVE spells them. Its SetAt is set here.
*/
func (c *CachedEveIDs) SaveHoldingLink(link HoldingLink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadHoldings()

	link.SetAt = time.Now().UTC()
	put(c, sectionHoldings, c.Holdings, holdingKey(link.MainType, link.MainID, link.HoldingType, link.HoldingID),
		&CacheEntry{Value: &link, CachedAt: link.SetAt, Permanent: true})
}

/*
	RemoveHoldingLink removes the link between a main and a holding, from this session and the database. One that is not saved changes nothing.

Parameters:
  - mainType, mainID: the main's CORPORATION or ALLIANCE and eve id
  - holdingType, holdingID: the holding's CORPORATION or ALLIANCE and eve id
*/
func (c *CachedEveIDs) RemoveHoldingLink(mainType string, mainID int, holdingType string, holdingID int) {
	key := holdingKey(mainType, mainID, holdingType, holdingID)
	c.mu.Lock()
	c.loadHoldings()
	entry := c.Holdings[key]
	c.mu.Unlock()

	c.removeWhere(
		func(s section, candidate *CacheEntry) bool { return s == sectionHoldings && candidate == entry },
		"section = ? AND key = ?", string(sectionHoldings), key)
}
