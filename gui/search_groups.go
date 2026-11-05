package gui

import (
	"cmp"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// SearchGroup is a saved group the search bar's list offers: a coalition, a main with affiliates, which its own search pulls along, or a war
type SearchGroup struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"` // COALITION, CORPORATION, ALLIANCE, or WAR
	ID     int      `json:"id"`
	Ticker string   `json:"ticker,omitempty"` // a corp's or alliance's ticker, from the cache only, empty until looked up
	Count  int      `json:"count"`            // a coalition's members, or a main's affiliates
	Sides  []string `json:"sides,omitempty"`  // a war's two sides, as the w: tag finds it by them too
}

/*
	GetSearchGroups returns the groups to offer in the search bar's list. It reads only the cache, which has its own lock, so it does not wait for a running search.
	The page asks as it loads, so it waits for startup to load the cache.

Returns:
  - []SearchGroup: the coalitions with members, by name, then the mains with affiliates, by name, then the wars
  - error: a user readable message if something went wrong
*/
func (a *App) GetSearchGroups() (groups []SearchGroup, err error) {
	defer recoverAsError(&err)
	a.waitReady()

	groups = []SearchGroup{}
	for _, coalition := range a.cache.Coalitions() {
		if len(coalition.Members) > 0 {
			groups = append(groups, SearchGroup{Name: coalition.Name, Type: types.COALITION.String(), ID: coalition.ID, Count: len(coalition.Members)})
		}
	}

	mains := []SearchGroup{}
	at := map[string]int{}
	for _, link := range a.cache.AllHoldingLinks() {
		key := intelGroupKey(link.MainType, link.MainID)
		index, found := at[key]
		if !found {
			index = len(mains)
			at[key] = index
			mains = append(mains, SearchGroup{Name: link.MainName, Type: link.MainType, ID: link.MainID,
				Ticker: a.cache.CachedTicker(searchTypes[link.MainType], link.MainID)})
		}
		mains[index].Count++
	}
	slices.SortStableFunc(mains, func(a, b SearchGroup) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	groups = append(groups, mains...)
	for _, war := range a.cache.Wars() {
		groups = append(groups, SearchGroup{Name: war.Name, Type: warType, ID: war.ID, Sides: []string{war.SideA.Name, war.SideB.Name}})
	}
	return groups, nil
}
