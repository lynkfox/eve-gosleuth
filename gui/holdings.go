package gui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// maxHoldingNote is the longest note a holding link keeps
const maxHoldingNote = 200

// HoldingEntity is one end of a holding link, as the target panel shows it
type HoldingEntity struct {
	Type      string `json:"type"` // CORPORATION or ALLIANCE
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	ZkillURL  string `json:"zkillUrl"`
	EveWhoURL string `json:"evewhoUrl"`
	Note      string `json:"note"`
}

// Holdings are a corp or alliance target's holding links: the holdings it keeps, and the mains it is a holding for
type Holdings struct {
	Holdings []HoldingEntity `json:"holdings"`
	HeldFor  []HoldingEntity `json:"heldFor"`
}

// holdingEntity describes one end of a link, with the link's note
func holdingEntity(typeName string, id int, name string, note string) HoldingEntity {
	entityType := searchTypes[typeName]
	return HoldingEntity{
		Type:      typeName,
		ID:        id,
		Name:      name,
		ImageURL:  api.EntityIconURL(entityType, id),
		ZkillURL:  api.ZkillEntityURL(entityType, id),
		EveWhoURL: api.EveWhoURL(entityType, id),
		Note:      note,
	}
}

// holdingsFor looks up an entity's holding links from both ends
func (a *App) holdingsFor(typeName string, id int) Holdings {
	holdings, heldFor := a.cache.HoldingLinks(typeName, id)
	result := Holdings{Holdings: []HoldingEntity{}, HeldFor: []HoldingEntity{}}
	for _, link := range holdings {
		result.Holdings = append(result.Holdings, holdingEntity(link.HoldingType, link.HoldingID, link.HoldingName, link.Note))
	}
	for _, link := range heldFor {
		result.HeldFor = append(result.HeldFor, holdingEntity(link.MainType, link.MainID, link.MainName, link.Note))
	}
	return result
}

// canHold reports whether an entity type can be on either end of a holding link
func canHold(typeName string) bool {
	return typeName == types.CORPORATION.String() || typeName == types.ALLIANCE.String()
}

/*
	GetHoldings returns a corp or alliance's holding links. Like GetRecentSearches it does not wait for a running search, the cache has its own lock.

Parameters:
  - targetID: the corp or alliance's eve id
  - targetType: CORPORATION or ALLIANCE

Returns:
  - Holdings: its holdings, and the mains it holds for
*/
func (a *App) GetHoldings(targetID int, targetType string) (holdings Holdings, err error) {
	defer recoverAsError(&err)
	if !canHold(targetType) {
		return holdings, errors.New("Only corporations and alliances have affiliates")
	}
	return a.holdingsFor(targetType, targetID), nil
}

/*
	SaveHolding links a corp or alliance to a holding corp or alliance by name, or changes the note of a link it already has.
	The name is looked up in the ESI, through the cache, so it is spelled as EVE spells it. It does not wait for a running search.

Parameters:
  - mainID, mainType, mainName: the corp or alliance target the holding belongs to
  - holdingName: the holding's exact in game name, or a zKillboard or EveWho link to it
  - holdingType: CORPORATION or ALLIANCE
  - note: a few words about it, ie supers parked here. Trimmed to maxHoldingNote characters.

Returns:
  - Holdings: the main's holding links after saving
  - error: a user readable message if the holding is not found, is the main itself, or a type can not hold
*/
func (a *App) SaveHolding(mainID int, mainType string, mainName string, holdingName string, holdingType string, note string) (holdings Holdings, err error) {
	defer recoverAsError(&err)
	if !canHold(mainType) || !canHold(holdingType) {
		return holdings, errors.New("Only corporations and alliances can be affiliates, or have them")
	}
	if strings.TrimSpace(holdingName) == "" {
		return holdings, errors.New("Enter the affiliate's name")
	}

	holding, err := a.lookupEntity(holdingName, holdingType, types.CORPORATION, types.ALLIANCE)
	if err != nil {
		return holdings, err
	}
	if holding.ID == mainID && holding.Type == mainType {
		return holdings, fmt.Errorf("%s can not be its own affiliate", mainName)
	}

	note = strings.TrimSpace(note)
	if len([]rune(note)) > maxHoldingNote {
		note = string([]rune(note)[:maxHoldingNote])
	}
	a.cache.SaveHoldingLink(retrieval.HoldingLink{
		MainType: mainType, MainID: mainID, MainName: mainName,
		HoldingType: holding.Type, HoldingID: holding.ID, HoldingName: holding.Name,
		Note: note,
	})
	a.saveCacheQuietly()
	return a.holdingsFor(mainType, mainID), nil
}

/*
	RemoveHolding removes a holding link. Like GetHoldings it does not wait for a running search.

Parameters:
  - mainType, mainID: the main's CORPORATION or ALLIANCE and eve id
  - holdingType, holdingID: the holding's CORPORATION or ALLIANCE and eve id
*/
func (a *App) RemoveHolding(mainType string, mainID int, holdingType string, holdingID int) (err error) {
	defer recoverAsError(&err)
	a.cache.RemoveHoldingLink(mainType, mainID, holdingType, holdingID)
	a.saveCacheQuietly()
	return nil
}

// saveCacheQuietly saves the cache, for a change the user made by hand, so it is kept even if the app is closed uncleanly
func (a *App) saveCacheQuietly() {
	if err := retrieval.SaveCache(a.cache); err != nil {
		comm.Logger.WithError(err).Warn("Unable to save the cache after a change")
	}
}

// targetHoldings returns a corp or alliance target's holdings, to collect their killmails as its own. Pilots and systems have none.
func (a *App) targetHoldings(target *retrieval.IntelTarget) []retrieval.TargetGroup {
	if target.TypeOfEntity != types.CORPORATION && target.TypeOfEntity != types.ALLIANCE {
		return nil
	}
	holdings, _ := a.cache.HoldingLinks(target.TypeOfEntity.String(), target.IDNumber)
	groups := []retrieval.TargetGroup{}
	for _, link := range holdings {
		if holdingType, ok := searchTypes[link.HoldingType]; ok {
			groups = append(groups, retrieval.TargetGroup{Type: holdingType, ID: link.HoldingID, Name: link.HoldingName})
		}
	}
	return groups
}

/*
holdingAllies returns the holdings linked to a target's own groups, either way, which its Allies leave out as the same group.
Its own groups are, for a pilot their current corp and alliance, for a corp itself and its alliance, and for an alliance itself.
*/
func (a *App) holdingAllies(target *retrieval.IntelTarget) map[aggregate.AllyKey]bool {
	own := []aggregate.AllyKey{}
	addCorp := func(corp *retrieval.Corporation) {
		if corp == nil {
			return
		}
		own = append(own, aggregate.AllyKey{Type: types.CORPORATION, ID: corp.CorporationID})
		if corp.Alliance != nil {
			own = append(own, aggregate.AllyKey{Type: types.ALLIANCE, ID: corp.Alliance.AllianceID})
		}
	}
	switch expanded := target.Expanded.(type) {
	case *retrieval.Pilot:
		if expanded != nil {
			addCorp(expanded.Corporation)
		}
	case *retrieval.Corporation:
		own = append(own, aggregate.AllyKey{Type: types.CORPORATION, ID: target.IDNumber})
		addCorp(expanded)
	case *retrieval.Alliance:
		own = append(own, aggregate.AllyKey{Type: types.ALLIANCE, ID: target.IDNumber})
	}

	linked := map[aggregate.AllyKey]bool{}
	for _, key := range own {
		holdings, heldFor := a.cache.HoldingLinks(key.Type.String(), key.ID)
		for _, link := range holdings {
			linked[aggregate.AllyKey{Type: searchTypes[link.HoldingType], ID: link.HoldingID}] = true
		}
		for _, link := range heldFor {
			linked[aggregate.AllyKey{Type: searchTypes[link.MainType], ID: link.MainID}] = true
		}
	}
	return linked
}
