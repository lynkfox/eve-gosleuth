package gui

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// IntelLink is the other end of a holding link on a My Intel card, with the link's note
type IntelLink struct {
	EntityRef
	Note string `json:"note"`
}

// IntelGroup is one My Intel card: a corp, alliance, or pilot the user saved holdings or ignored allies for
type IntelGroup struct {
	Entity        EntityRef   `json:"entity"`
	Holdings      []IntelLink `json:"holdings"`      // the links where it is the main, by name
	HeldFor       []IntelLink `json:"heldFor"`       // the links where it is the holding, so the mains it holds for, for a group with a card of its own
	IgnoredAllies []EntityRef `json:"ignoredAllies"` // in the order they were ignored
}

// intelGroupKey names a group, ie ALLIANCE:99000001
func intelGroupKey(typeName string, id int) string {
	return fmt.Sprintf("%s:%d", typeName, id)
}

/*
	GetMyIntel returns everything the user saved by hand, for the My Intel window: a card for each corp or alliance with holdings, and each pilot, corp,
	or alliance with ignored allies, by name. Like GetHoldings it does not wait for a running search, the cache has its own lock.

Returns:
  - []IntelGroup: the cards
  - error: a user readable message if something went wrong
*/
func (a *App) GetMyIntel() (groups []IntelGroup, err error) {
	defer recoverAsError(&err)

	byKey := map[string]*IntelGroup{}
	groupFor := func(typeName string, id int, name string) *IntelGroup {
		key := intelGroupKey(typeName, id)
		if group, found := byKey[key]; found {
			return group
		}
		group := &IntelGroup{Entity: *entityRef(searchTypes[typeName], id, name), Holdings: []IntelLink{}, HeldFor: []IntelLink{}, IgnoredAllies: []EntityRef{}}
		byKey[key] = group
		return group
	}

	links := a.cache.AllHoldingLinks()
	for _, link := range links {
		main := groupFor(link.MainType, link.MainID, link.MainName)
		main.Holdings = append(main.Holdings, IntelLink{EntityRef: *entityRef(searchTypes[link.HoldingType], link.HoldingID, link.HoldingName), Note: link.Note})
	}

	// ignored allies are saved without the target's name, which is looked up for all of them at once
	ignored := a.cache.AllIgnoredAllies()
	unnamed := []int{}
	for _, target := range ignored {
		if _, found := byKey[intelGroupKey(target.TargetType, target.TargetID)]; !found {
			unnamed = append(unnamed, target.TargetID)
		}
	}
	names := map[int]string{}
	if len(unnamed) > 0 {
		if found, lookupErr := a.clients.ESIClient.GetNamesFromEveIDs(unnamed); lookupErr == nil {
			names = found
		}
	}
	for _, target := range ignored {
		name := names[target.TargetID]
		if name == "" {
			name = fmt.Sprintf("%s %d", strings.ToLower(target.TargetType), target.TargetID)
		}
		group := groupFor(target.TargetType, target.TargetID, name)
		for _, ally := range target.Allies {
			group.IgnoredAllies = append(group.IgnoredAllies, *entityRef(searchTypes[ally.Type], ally.ID, ally.Name))
		}
	}

	// a holding with a card of its own also lists the mains it holds for
	for _, link := range links {
		if group, found := byKey[intelGroupKey(link.HoldingType, link.HoldingID)]; found {
			group.HeldFor = append(group.HeldFor, IntelLink{EntityRef: *entityRef(searchTypes[link.MainType], link.MainID, link.MainName), Note: link.Note})
		}
	}

	groups = []IntelGroup{}
	for _, group := range byKey {
		groups = append(groups, *group)
	}
	slices.SortFunc(groups, func(a, b IntelGroup) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Entity.Name), strings.ToLower(b.Entity.Name)), cmp.Compare(a.Entity.Type, b.Entity.Type))
	})
	return groups, nil
}

/*
	lookupEntity finds a pilot, corporation, or alliance by its exact name, spelled as EVE spells it

Parameters:
  - name: the name, ignoring case
  - typeName: PILOT, CORPORATION, or ALLIANCE

Returns:
  - EntityRef: what was found
  - error: a user readable message if nothing of that type has the name
*/
func (a *App) lookupEntity(name string, typeName string) (EntityRef, error) {
	entityType, ok := searchTypes[typeName]
	if !ok || entityType == types.SYSTEM {
		return EntityRef{}, errors.New("Only pilots, corporations, and alliances can be looked up here")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return EntityRef{}, errors.New("Enter a name")
	}

	found := retrieval.IntelTarget{Name: name, Clients: a.clients}
	found.GetTargetInfoOfType(entityType)
	if found.Error != nil {
		return EntityRef{}, errors.New(found.DisplayMessage)
	}
	switch entityType {
	case types.ALLIANCE:
		if alliance, err := a.clients.ESIClient.GetAllianceInfo(found.IDNumber); err == nil && alliance != nil {
			name = alliance.Name
		}
	case types.CORPORATION:
		if corp, err := a.clients.ESIClient.GetCorporationInfo(found.IDNumber); err == nil && corp != nil {
			name = corp.Name
		}
	case types.PILOT:
		if pilot, err := a.clients.ESIClient.GetCharacterInfo(found.IDNumber); err == nil && pilot != nil {
			name = pilot.Name
		}
	}
	return *entityRef(entityType, found.IDNumber, name), nil
}

/*
	LookupEntity finds a pilot, corporation, or alliance by its exact name, for My Intel to start a card for. It does not wait for a running search.

Parameters:
  - name: the name, ignoring case
  - entityType: PILOT, CORPORATION, or ALLIANCE

Returns:
  - EntityRef: what was found, spelled as EVE spells it
  - error: a user readable message if nothing of that type has the name
*/
func (a *App) LookupEntity(name string, entityType string) (entity EntityRef, err error) {
	defer recoverAsError(&err)
	return a.lookupEntity(name, entityType)
}

/*
	AddIgnoredAlly leaves an alliance or corporation out of a target's Allies, from My Intel. It does not wait for a running search.

Parameters:
  - targetType, targetID: the pilot, corporation, or alliance whose Allies it is left out of
  - allyName: the ally's exact name, ignoring case
  - allyType: CORPORATION or ALLIANCE

Returns:
  - error: a user readable message if the ally is not found, or is the target itself
*/
func (a *App) AddIgnoredAlly(targetType string, targetID int, allyName string, allyType string) (err error) {
	defer recoverAsError(&err)
	if entityType, ok := searchTypes[targetType]; !ok || entityType == types.SYSTEM {
		return errors.New("Only pilots, corporations, and alliances have allies")
	}
	if !canHold(allyType) {
		return errors.New("Only corporations and alliances can be allies")
	}
	ally, err := a.lookupEntity(allyName, allyType)
	if err != nil {
		return err
	}
	if ally.ID == targetID && ally.Type == targetType {
		return fmt.Errorf("%s can not be its own ally", ally.Name)
	}

	ignored := a.cache.IgnoredAlliesFor(targetType, targetID)
	if slices.ContainsFunc(ignored, func(other retrieval.IgnoredAlly) bool { return other.Type == ally.Type && other.ID == ally.ID }) {
		return nil
	}
	a.cache.SetIgnoredAllies(targetType, targetID, append(slices.Clone(ignored), retrieval.IgnoredAlly{Type: ally.Type, ID: ally.ID, Name: ally.Name}))
	a.saveCacheQuietly()
	return nil
}

/*
	RemoveIgnoredAlly puts an ignored alliance or corporation back in a target's Allies, from My Intel. It does not wait for a running search.

Parameters:
  - targetType, targetID: the pilot, corporation, or alliance
  - allyType, allyID: the ally's CORPORATION or ALLIANCE and eve id
*/
func (a *App) RemoveIgnoredAlly(targetType string, targetID int, allyType string, allyID int) (err error) {
	defer recoverAsError(&err)
	ignored := slices.DeleteFunc(slices.Clone(a.cache.IgnoredAlliesFor(targetType, targetID)), func(ally retrieval.IgnoredAlly) bool {
		return ally.Type == allyType && ally.ID == allyID
	})
	a.cache.SetIgnoredAllies(targetType, targetID, ignored)
	a.saveCacheQuietly()
	return nil
}

/*
	RemoveMyIntelGroup removes a My Intel card: the holdings it keeps and its ignored allies. Links where it is a holding of another main
	belong to that main's card, so they are kept. It does not wait for a running search.

Parameters:
  - entityType, entityID: the card's PILOT, CORPORATION, or ALLIANCE and eve id
*/
func (a *App) RemoveMyIntelGroup(entityType string, entityID int) (err error) {
	defer recoverAsError(&err)
	if entityTypeOf, ok := searchTypes[entityType]; !ok || entityTypeOf == types.SYSTEM {
		return errors.New("Only pilots, corporations, and alliances have cards")
	}
	if canHold(entityType) {
		holdings, _ := a.cache.HoldingLinks(entityType, entityID)
		for _, link := range holdings {
			a.cache.RemoveHoldingLink(link.MainType, link.MainID, link.HoldingType, link.HoldingID)
		}
	}
	a.cache.SetIgnoredAllies(entityType, entityID, nil)
	a.saveCacheQuietly()
	return nil
}
