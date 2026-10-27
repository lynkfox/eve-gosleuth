package retrieval

import (
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ZkillListEntry is one killmail on a cached zKillboard list: what zKillboard says of it. The killmail itself is in Killmails.
type ZkillListEntry struct {
	KillmailID   int       `json:"id"`
	KillmailTime time.Time `json:"time"`
	Zkb          api.Zkb   `json:"zkb"`
}

/*
KnownZkillList is an entity's zKillboard killmail list as far as it is known, newest first. Its first Unbroken entries are zKillboard's list
without a gap, from its newest killmail down. The rest are older killmails listed by earlier searches, which may have gaps between them.
*/
type KnownZkillList struct {
	Entries  []ZkillListEntry `json:"entries"`
	Unbroken int              `json:"unbroken"`
	Ended    bool             `json:"ended"` // the unbroken entries reach the oldest killmail zKillboard lists
}

// zkillListKey is an entity's key in ZkillLists, ie ALLIANCE:99000001
func zkillListKey(entityType string, entityID int) string {
	return fmt.Sprintf("%s:%d", entityType, entityID)
}

// KnownZkillListFor returns an entity's cached zKillboard list, or nil if none is cached
func (c *CachedEveIDs) KnownZkillListFor(entityType string, entityID int) *KnownZkillList {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cachedAs[KnownZkillList](c, sectionZkillLists, c.ZkillLists, zkillListKey(entityType, entityID))
}

/*
SaveKnownZkillList caches an entity's zKillboard list, replacing the one before. Like killmails it never expires with the SDE,
since the killmails on it never change, but cache clears remove it.
*/
func (c *CachedEveIDs) SaveKnownZkillList(entityType string, entityID int, list *KnownZkillList) {
	c.mu.Lock()
	defer c.mu.Unlock()
	put(c, sectionZkillLists, c.ZkillLists, zkillListKey(entityType, entityID), &CacheEntry{Value: list, CachedAt: time.Now().UTC(), Permanent: true})
}

// zkillListCache is a client that keeps entities' zKillboard lists, ie CachedEveIDs
type zkillListCache interface {
	KnownZkillListFor(entityType string, entityID int) *KnownZkillList
	SaveKnownZkillList(entityType string, entityID int, list *KnownZkillList)
}

// entityList is one entity's killmails listed for a search, newest first. The first within are inside the search's limits, the rest only from its cached list.
type entityList struct {
	killmails []api.ZkbResponse
	within    int
}

/*
	listEntity pulls one entity's zKillboard list as far as the paging asks, with its cached list filling in what is already known.
	Paging stops at the first page reaching a killmail the cached list has without a gap above it, the cached list giving the rest.
	When the search needs more than the cached list has without a gap, paging carries on from where that ends. Every killmail zKillboard sends
	is cached, so later searches can use those beyond their limits too, and the list is cached again.

Parameters:
  - target: the IntelTarget, for its clients
  - entityType, entityID: the target itself or one of its holdings
  - paging: how far to page, its Killmails, MaxPages, NewerThan, and For
  - need: how many killmails the search needs from the top of the list, its Max or its pages of killmails

Returns:
  - entityList: the listed killmails, and how many are within the search's limits
  - error: the zkill pull error, with what was listed
*/
func listEntity(target IntelTarget, entityType types.EntityType, entityID int, paging api.ZkillPaging, need int) (entityList, error) {
	zkill := target.Clients.ZkillClient
	lists, _ := target.Clients.ESIClient.(zkillListCache)
	bodies, _ := target.Clients.ESIClient.(killmailCache)

	var known *KnownZkillList
	if lists != nil {
		known = lists.KnownZkillListFor(entityType.String(), entityID)
	}
	knownUnbroken := map[int]bool{}
	if known != nil {
		for _, entry := range known.Entries[:min(known.Unbroken, len(known.Entries))] {
			knownUnbroken[entry.KillmailID] = true
		}
		if len(knownUnbroken) > 0 {
			paging.Known = func(killmailID int) bool { return knownUnbroken[killmailID] }
		}
	}

	pulled, err := zkill.GetZkillResults(entityID, entityType, paging)
	storeBodies(bodies, pulled.Killmails)
	merged, unbroken, ended := splice(pulled, known, knownUnbroken)

	// the search needs more than is known without a gap: carry on paging below it
	if err == nil && !ended && unbroken < need && !api.ReachesDate(merged[:unbroken], paging.NewerThan) {
		start := unbroken/api.ZKILL_PAGE_SIZE + 1
		if start <= paging.MaxPages {
			below := paging
			below.Known = nil
			below.StartPage = start
			if below.Killmails > 0 {
				below.Killmails = need - (start-1)*api.ZKILL_PAGE_SIZE
			}
			var more api.ZkillList
			more, err = zkill.GetZkillResults(entityID, entityType, below)
			storeBodies(bodies, more.Killmails)
			merged, unbroken, ended = extend(merged, unbroken, more)
		}
	}

	if lists != nil {
		lists.SaveKnownZkillList(entityType.String(), entityID, knownList(merged, unbroken, ended))
	}
	return entityList{killmails: merged, within: min(unbroken, need)}, err
}

// storeBodies caches the killmails zKillboard sent whole, so later searches have them, even those beyond this search's limits
func storeBodies(bodies killmailCache, listed []api.ZkbResponse) {
	if bodies == nil {
		return
	}
	for _, km := range listed {
		if body := km.Killmail(); body != nil {
			bodies.StoreKillmail(body)
		}
	}
}

/*
	splice merges a pull with the entity's cached list, newest first, each killmail once with the pull's copy kept

Returns:
  - []api.ZkbResponse: the merged list
  - int: how many from the top are zKillboard's list without a gap: the pull's, and when it reached the cached list's unbroken part, all of that too
  - bool: those reach the end of zKillboard's list
*/
func splice(pulled api.ZkillList, known *KnownZkillList, knownUnbroken map[int]bool) ([]api.ZkbResponse, int, bool) {
	cachedEntries := []api.ZkbResponse{}
	if known != nil {
		for _, entry := range known.Entries {
			cachedEntries = append(cachedEntries, api.ZkbResponse{KillmailID: entry.KillmailID, KillmailTime: entry.KillmailTime, Zkb: entry.Zkb})
		}
	}
	merged := combine(pulled.Killmails, cachedEntries)

	// the unbroken part ends at the oldest pulled killmail, or the cached list's unbroken oldest when the pull reached it
	unbroken, ended := lowest(merged, pulled.Killmails)+1, pulled.Ended
	reached := slices.ContainsFunc(pulled.Killmails, func(km api.ZkbResponse) bool { return knownUnbroken[km.KillmailID] })
	if reached && known.Unbroken > 0 {
		knownOldest := known.Entries[min(known.Unbroken, len(known.Entries))-1]
		if at := position(merged, knownOldest.KillmailID); at+1 > unbroken {
			unbroken, ended = at+1, known.Ended
		}
	}
	return merged, unbroken, ended
}

// lowest is the furthest down a merged list any of the killmails are, -1 for none
func lowest(merged []api.ZkbResponse, killmails []api.ZkbResponse) int {
	at := -1
	for _, km := range killmails {
		at = max(at, position(merged, km.KillmailID))
	}
	return at
}

// extend merges a pull carried on below the unbroken top of a list into it, the pull's oldest then ending the unbroken part
func extend(merged []api.ZkbResponse, unbroken int, more api.ZkillList) ([]api.ZkbResponse, int, bool) {
	if len(more.Killmails) == 0 {
		return merged, unbroken, more.Ended
	}
	extended := combine(merged[:unbroken], more.Killmails, merged[unbroken:])
	return extended, lowest(extended, more.Killmails) + 1, more.Ended
}

/*
combine merges killmail lists newest first, keeping the first copy of each. A single list keeps its own order, which is zKillboard's.
Should any have no time, they are sorted by killmail id instead, as ids are given out in nearly the order killmails happen.
*/
func combine(lists ...[]api.ZkbResponse) []api.ZkbResponse {
	seen := map[int]bool{}
	combined := []api.ZkbResponse{}
	untimed := false
	nonEmpty := 0
	for _, list := range lists {
		if len(list) > 0 {
			nonEmpty++
		}
		for _, km := range list {
			if !seen[km.KillmailID] {
				seen[km.KillmailID] = true
				combined = append(combined, km)
				untimed = untimed || km.KillmailTime.IsZero()
			}
		}
	}
	if nonEmpty <= 1 {
		return combined
	}
	if untimed {
		slices.SortStableFunc(combined, func(a, b api.ZkbResponse) int { return b.KillmailID - a.KillmailID })
	} else {
		slices.SortStableFunc(combined, func(a, b api.ZkbResponse) int { return b.KillmailTime.Compare(a.KillmailTime) })
	}
	return combined
}

// position is a killmail's index on a list, -1 if it is not on it
func position(list []api.ZkbResponse, killmailID int) int {
	return slices.IndexFunc(list, func(km api.ZkbResponse) bool { return km.KillmailID == killmailID })
}

// knownList is a merged list as cached: what zKillboard says of each killmail, at most api.ZKILL_MAX_KILLMAILS of them
func knownList(merged []api.ZkbResponse, unbroken int, ended bool) *KnownZkillList {
	kept := merged[:min(len(merged), api.ZKILL_MAX_KILLMAILS)]
	list := &KnownZkillList{Entries: make([]ZkillListEntry, 0, len(kept)), Unbroken: min(unbroken, len(kept)), Ended: ended && unbroken <= len(kept)}
	for _, km := range kept {
		list.Entries = append(list.Entries, ZkillListEntry{KillmailID: km.KillmailID, KillmailTime: km.KillmailTime, Zkb: km.Zkb})
	}
	if len(merged) > len(kept) {
		comm.Logger.Debugf("Cached zKillboard list kept the newest %d of %d killmails", len(kept), len(merged))
	}
	return list
}
