package retrieval

import (
	"fmt"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type TargetError struct {
	Message string
}

func (e *TargetError) Error() string {
	return e.Message
}

var ErrTooManyTargets = &TargetError{Message: "too many targets"}

type IntelTarget struct {
	IDNumber       int
	Name           string
	TypeOfEntity   types.EntityType
	Clients        CollectionClientPool
	Data           interface{}
	Error          error
	DisplayMessage string
	Expanded       interface{}
	Holdings       []TargetGroup // a corp or alliance target's holdings, whose killmails are collected and counted as the target's own
}

// TargetGroup is a corporation or alliance searched along with a target, ie one of its holdings
type TargetGroup struct {
	Type types.EntityType // CORPORATION or ALLIANCE
	ID   int
	Name string
}

/*
	DisplayName is the target's name as EVE spells it

Returns:
  - string: the name from its expanded model, else the name searched for, else "the target"
*/
func (t *IntelTarget) DisplayName() string {
	switch expanded := t.Expanded.(type) {
	case *Pilot:
		if expanded != nil && expanded.Name != "" {
			return expanded.Name
		}
	case *Corporation:
		if expanded != nil && expanded.Name != "" {
			return expanded.Name
		}
	case *Alliance:
		if expanded != nil && expanded.Name != "" {
			return expanded.Name
		}
	}
	if t.Name != "" {
		return t.Name
	}
	return "the target"
}

/*
	IsGroup reports whether a corporation or alliance on a killmail is a corp or alliance target, or one of its Holdings. Always false for pilot and system targets.

Parameters:
  - corporationID, allianceID: the corp and alliance on the killmail, 0 when there is none
*/
func (t *IntelTarget) IsGroup(corporationID int, allianceID int) bool {
	switch t.TypeOfEntity {
	case types.CORPORATION:
		if corporationID != 0 && corporationID == t.IDNumber {
			return true
		}
	case types.ALLIANCE:
		if allianceID != 0 && allianceID == t.IDNumber {
			return true
		}
	default:
		return false
	}
	for _, holding := range t.Holdings {
		if holding.Includes(corporationID, allianceID) {
			return true
		}
	}
	return false
}

// Includes reports whether a corporation or alliance on a killmail is this group: the corp itself, or any corp in the alliance. 0 is never a group.
func (g TargetGroup) Includes(corporationID int, allianceID int) bool {
	switch g.Type {
	case types.CORPORATION:
		return corporationID != 0 && corporationID == g.ID
	case types.ALLIANCE:
		return allianceID != 0 && allianceID == g.ID
	}
	return false
}

type CollectionClientPool struct {
	ZkillClient  api.ZkillClient
	ESIClient    api.ESIClient
	EveWhoClient api.EveWhoClient
}

// InitTarget looks up a target by name, making its ESI calls through the cache
func InitTarget(name string, cache *CachedEveIDs) IntelTarget {
	target := IntelTarget{
		Name: name,
		Clients: CollectionClientPool{
			ZkillClient: api.NewZkillClient(),
			ESIClient:   cache,
		},
	}

	target.GetTargetInfo()

	return target
}

// targetCategories maps the ESI name lookup categories to the entity type they represent
var targetCategories = map[string]types.EntityType{
	"alliances":       types.ALLIANCE,
	"characters":      types.PILOT,
	"corporations":    types.CORPORATION,
	"systems":         types.SYSTEM,
	"inventory_types": types.ITEM,
}

/*
GetTargetInfo looks up the target by Name on the ESI, setting its ID, type, and Data. If the name matches more than one kind of entity, Error is set to ErrTooManyTargets.
*/
func (t *IntelTarget) GetTargetInfo() {
	t.lookupTarget(nil)
}

/*
	GetTargetInfoOfType looks up the target by Name on the ESI, only accepting a match of the given type, then sets its ID and Data.

Parameters:
  - entityType: the type of entity the user searched for. Other entities that share the name are ignored.
*/
func (t *IntelTarget) GetTargetInfoOfType(entityType types.EntityType) {
	t.TypeOfEntity = entityType
	t.lookupTarget(&entityType)
}

func (t *IntelTarget) lookupTarget(onlyType *types.EntityType) {

	var (
		data interface{}
		err  error
	)

	names, err := t.Clients.ESIClient.GetEveIDsFromNames([]string{t.Name})

	if err != nil {
		t.Error = fmt.Errorf("Unable to find Target ID: %w", err)
		t.DisplayMessage = "Could not search the ESI: " + api.FailureReason(err)
		return
	}

	if len(names) == 0 {
		t.Error = fmt.Errorf("No ESI match for %s", t.Name)
		t.DisplayMessage = "Nothing found by that name. Check spelling?"
		return
	}

	if onlyType != nil {
		for _, value := range names {
			if targetCategories[value.Category] == *onlyType {
				t.IDNumber = value.ID
			}
		}

		if t.IDNumber == 0 {
			t.Error = fmt.Errorf("No %s match for %s", onlyType.String(), t.Name)
			t.DisplayMessage = fmt.Sprintf("No %s found by that name. Check spelling or search type?", strings.ToLower(onlyType.String()))
			return
		}
	} else if len(names) > 1 {

		if t.TypeOfEntity.String() != "UNKNOWN" {
			for _, value := range names {
				if targetCategories[value.Category] == t.TypeOfEntity {
					t.IDNumber = value.ID
				}
			}
		}

		if t.IDNumber == 0 {
			t.Error = ErrTooManyTargets
			t.DisplayMessage = "Multiple Targets Found"
			return
		}
	} else {
		for _, value := range names {
			t.TypeOfEntity = targetCategories[value.Category]
			t.IDNumber = value.ID
		}
	}

	switch t.TypeOfEntity {
	case types.ALLIANCE:
		data, err = t.Clients.ESIClient.GetAllianceInfo(t.IDNumber)
	case types.CORPORATION:
		data, err = t.Clients.ESIClient.GetCorporationInfo(t.IDNumber)
	case types.SYSTEM:
		data, err = t.Clients.ESIClient.GetSystemInfo(t.IDNumber)
	case types.PILOT:
		data, err = t.Clients.ESIClient.GetCharacterInfo(t.IDNumber)

	}
	if err != nil {
		t.Error = fmt.Errorf("Unable to retrieve Target info: %w", err)
		t.DisplayMessage = "Could not load details for that target: " + api.FailureReason(err)
		return
	}

	t.Data = data

}
