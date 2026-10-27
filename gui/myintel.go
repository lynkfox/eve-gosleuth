package gui

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
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
		group.Entity.Ticker = a.cachedTicker(group.Entity)
		for index := range group.Holdings {
			group.Holdings[index].Ticker = a.cachedTicker(group.Holdings[index].EntityRef)
		}
		for index := range group.HeldFor {
			group.HeldFor[index].Ticker = a.cachedTicker(group.HeldFor[index].EntityRef)
		}
		for index := range group.IgnoredAllies {
			group.IgnoredAllies[index].Ticker = a.cachedTicker(group.IgnoredAllies[index])
		}
		groups = append(groups, *group)
	}
	slices.SortFunc(groups, func(a, b IntelGroup) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Entity.Name), strings.ToLower(b.Entity.Name)), cmp.Compare(a.Entity.Type, b.Entity.Type))
	})
	return groups, nil
}

// cachedTicker is a corp's or alliance's ticker if the cache has it, without asking the ESI, so My Intel opens at once. Empty for a pilot, or one not looked up yet.
func (a *App) cachedTicker(entity EntityRef) string {
	return a.cache.CachedTicker(searchTypes[entity.Type], entity.ID)
}

/*
	lookupEntity finds a pilot, corporation, or alliance by its exact name, or by a zKillboard or EveWho link to it, spelled as EVE spells it

Parameters:
  - text: the name, ignoring case, or a link, which is looked up by the id in it, as the ESI can not find every name
  - typeName: PILOT, CORPORATION, or ALLIANCE, what a name is looked up as. A link's own type is used instead.
  - allowed: the kinds that can be looked up here, ie only corporations and alliances for a holding

Returns:
  - EntityRef: what was found
  - error: a user readable message if nothing of that type has the name, or a link is to a kind not allowed
*/
func (a *App) lookupEntity(text string, typeName string, allowed ...types.EntityType) (EntityRef, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return EntityRef{}, errors.New("Enter a name")
	}

	found := retrieval.IntelTarget{Name: text, Clients: a.clients}
	if linkType, linkID, isLink := api.ParseEntityLink(text); isLink {
		if !slices.Contains(allowed, linkType) {
			return EntityRef{}, fmt.Errorf("That link is to a %s, which can not be used here", strings.ToLower(linkType.String()))
		}
		found.GetTargetInfoByID(linkType, linkID)
	} else {
		entityType, ok := searchTypes[typeName]
		if !ok || !slices.Contains(allowed, entityType) {
			return EntityRef{}, fmt.Errorf("A %s can not be used here", strings.ToLower(typeName))
		}
		found.GetTargetInfoOfType(entityType)
	}
	if found.Error != nil {
		return EntityRef{}, errors.New(found.DisplayMessage)
	}
	// both lookups set the name as EVE spells it
	return *entityRef(found.TypeOfEntity, found.IDNumber, found.Name), nil
}

/*
	LookupEntity finds a pilot, corporation, or alliance by its exact name, for My Intel to start a card for. It does not wait for a running search.

Parameters:
  - name: the name, ignoring case, or a zKillboard or EveWho link
  - entityType: PILOT, CORPORATION, or ALLIANCE, for a name

Returns:
  - EntityRef: what was found, spelled as EVE spells it
  - error: a user readable message if nothing of that type has the name
*/
func (a *App) LookupEntity(name string, entityType string) (entity EntityRef, err error) {
	defer recoverAsError(&err)
	return a.lookupEntity(name, entityType, types.PILOT, types.CORPORATION, types.ALLIANCE)
}

/*
	AddIgnoredAlly leaves an alliance or corporation out of a target's Allies, from My Intel. It does not wait for a running search.

Parameters:
  - targetType, targetID: the pilot, corporation, or alliance whose Allies it is left out of
  - allyName: the ally's exact name, ignoring case, or a zKillboard or EveWho link
  - allyType: CORPORATION or ALLIANCE, for a name

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
	ally, err := a.lookupEntity(allyName, allyType, types.CORPORATION, types.ALLIANCE)
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
	RemoveMyIntelGroups removes several My Intel cards at once, as RemoveMyIntelGroup does each. It does not wait for a running search.

Parameters:
  - cards: the cards' keys, ie ALLIANCE:99013330. Ones that are not cards are skipped.
*/
func (a *App) RemoveMyIntelGroups(cards []string) (err error) {
	defer recoverAsError(&err)
	for _, card := range cards {
		entityType, idText, _ := strings.Cut(card, ":")
		if id, convErr := strconv.Atoi(idText); convErr == nil {
			a.removeMyIntelGroup(entityType, id)
		}
	}
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
	a.removeMyIntelGroup(entityType, entityID)
	a.saveCacheQuietly()
	return nil
}

// removeMyIntelGroup removes a card's holdings and ignored allies, leaving saving to the caller
func (a *App) removeMyIntelGroup(entityType string, entityID int) {
	if entityTypeOf, ok := searchTypes[entityType]; !ok || entityTypeOf == types.SYSTEM {
		return
	}
	if canHold(entityType) {
		holdings, _ := a.cache.HoldingLinks(entityType, entityID)
		for _, link := range holdings {
			a.cache.RemoveHoldingLink(link.MainType, link.MainID, link.HoldingType, link.HoldingID)
		}
	}
	a.cache.SetIgnoredAllies(entityType, entityID, nil)
}
