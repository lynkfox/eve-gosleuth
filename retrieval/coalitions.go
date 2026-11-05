package retrieval

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// CoalitionMember is a corporation or alliance in a Coalition
type CoalitionMember struct {
	Type string `json:"type"` // CORPORATION or ALLIANCE
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Coalition is a named group of corporations and alliances the user made in My Intel, searched as one target. Unlike affiliates it has no main.
type Coalition struct {
	ID                int               `json:"id"` // negative, local to this cache, never an eve id
	Name              string            `json:"name"`
	Note              string            `json:"note"`
	Members           []CoalitionMember `json:"members"`
	IncludeStructures bool              `json:"includeStructures"`
	SetAt             time.Time         `json:"setAt"`
}

// coalitionKey is a coalition's key in SavedCoalitions: its id
func coalitionKey(id int) string {
	return fmt.Sprint(id)
}

/*
loadCoalitions reads every saved coalition into SavedCoalitions the first time they are wanted. The caller must hold c.mu.
Like holding links they are read all at once, as they are listed and looked up by name.
*/
func (c *CachedEveIDs) loadCoalitions() {
	if c.coalitionsLoaded || c.db == nil {
		return
	}
	c.coalitionsLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionCoalitions))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the coalitions")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			comm.Logger.WithError(err).Warn("Unable to read a coalition")
			continue
		}
		coalition, err := decodeValue[Coalition](sectionCoalitions, data)
		if err != nil || coalition.ID >= 0 || coalitionKey(coalition.ID) != key {
			comm.Logger.WithError(err).Warnf("Coalition %s can not be read, it is skipped", key)
			continue
		}
		if _, found := c.SavedCoalitions[key]; !found {
			c.SavedCoalitions[key] = &CacheEntry{Value: coalition, CachedAt: timeOf(cachedAt), Permanent: true}
		}
		c.coalitionFloor = min(c.coalitionFloor, coalition.ID)
	}
}

// copyOf is a coalition with its own members, so a caller changing them leaves the cache's copy alone
func (coalition Coalition) copyOf() Coalition {
	coalition.Members = slices.Clone(coalition.Members)
	if coalition.Members == nil {
		coalition.Members = []CoalitionMember{}
	}
	return coalition
}

// Coalitions returns every saved coalition, by name ignoring case
func (c *CachedEveIDs) Coalitions() []Coalition {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadCoalitions()

	all := []Coalition{}
	for _, entry := range c.SavedCoalitions {
		all = append(all, entry.Value.(*Coalition).copyOf())
	}
	slices.SortFunc(all, func(a, b Coalition) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return all
}

// CoalitionByID returns a saved coalition by its id, false when there is none
func (c *CachedEveIDs) CoalitionByID(id int) (Coalition, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadCoalitions()

	if entry, found := c.SavedCoalitions[coalitionKey(id)]; found {
		return entry.Value.(*Coalition).copyOf(), true
	}
	return Coalition{}, false
}

// CoalitionNamed returns a saved coalition by its name, ignoring case, false when there is none
func (c *CachedEveIDs) CoalitionNamed(name string) (Coalition, bool) {
	for _, coalition := range c.Coalitions() {
		if strings.EqualFold(coalition.Name, name) {
			return coalition, true
		}
	}
	return Coalition{}, false
}

/*
	SaveCoalition adds a coalition, or replaces the one with its id, ie to rename it or change its members.
	Coalitions are the user's own, so cache clears leave them unless asked to clear them too.

Parameters:
  - coalition: the coalition. An ID of 0 (or any that is not negative) gets a new one, -time.Now().UnixMilli() moved down until unused.
    A removed coalition's id is not given again while the app runs. A negative ID is kept, ie to restore a removed coalition. Its SetAt is set here.

Returns:
  - Coalition: what was saved, with its ID
*/
func (c *CachedEveIDs) SaveCoalition(coalition Coalition) Coalition {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadCoalitions()

	if coalition.ID >= 0 {
		coalition.ID = min(-int(time.Now().UnixMilli()), c.coalitionFloor-1)
		for c.SavedCoalitions[coalitionKey(coalition.ID)] != nil {
			coalition.ID--
		}
	}
	c.coalitionFloor = min(c.coalitionFloor, coalition.ID)
	coalition = coalition.copyOf()
	coalition.SetAt = time.Now().UTC()
	put(c, sectionCoalitions, c.SavedCoalitions, coalitionKey(coalition.ID), &CacheEntry{Value: &coalition, CachedAt: coalition.SetAt, Permanent: true})
	return coalition.copyOf()
}

// RemoveCoalition removes a coalition from this session and the database. One that is not saved changes nothing.
func (c *CachedEveIDs) RemoveCoalition(id int) {
	key := coalitionKey(id)
	c.mu.Lock()
	c.loadCoalitions()
	entry := c.SavedCoalitions[key]
	c.mu.Unlock()

	c.removeWhere(
		func(s section, candidate *CacheEntry) bool { return s == sectionCoalitions && candidate == entry },
		"section = ? AND key = ?", string(sectionCoalitions), key)
}

// memberEntityType is the entity type of a CoalitionMember's or holding link's Type text, and false for any but a corporation or alliance
func memberEntityType(typeName string) (types.EntityType, bool) {
	switch typeName {
	case types.CORPORATION.String():
		return types.CORPORATION, true
	case types.ALLIANCE.String():
		return types.ALLIANCE, true
	}
	return 0, false
}

/*
	CoalitionGroups lists the corporations and alliances a coalition's search collects the killmails of, which become its IntelTarget's Holdings:
	its members, then each member's own affiliates (HoldingLinks, one level, not the mains it holds for), each once by type and id. A member
	that is also another's affiliate is a member.

Parameters:
  - coalition: the coalition
  - includeAffiliates: also pull each member's affiliates, the Include Affiliates preference

Returns:
  - []TargetGroup: Member true for a member, false for an affiliate. A member of an unknown type is left out.
*/
func (c *CachedEveIDs) CoalitionGroups(coalition Coalition, includeAffiliates bool) []TargetGroup {
	groups := []TargetGroup{}
	seen := map[string]bool{}
	for _, member := range coalition.Members {
		entityType, ok := memberEntityType(member.Type)
		if !ok || seen[coalitionKey(member.ID)+member.Type] {
			continue
		}
		seen[coalitionKey(member.ID)+member.Type] = true
		groups = append(groups, TargetGroup{Type: entityType, ID: member.ID, Name: member.Name, Member: true})
	}
	if !includeAffiliates {
		return groups
	}
	for _, member := range slices.Clone(groups) {
		holdings, _ := c.HoldingLinks(member.Type.String(), member.ID)
		for _, link := range holdings {
			entityType, ok := memberEntityType(link.HoldingType)
			if !ok || seen[coalitionKey(link.HoldingID)+link.HoldingType] {
				continue
			}
			seen[coalitionKey(link.HoldingID)+link.HoldingType] = true
			groups = append(groups, TargetGroup{Type: entityType, ID: link.HoldingID, Name: link.HoldingName})
		}
	}
	return groups
}
