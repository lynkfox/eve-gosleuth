package retrieval

import (
	"fmt"

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
}

type CollectionClientPool struct {
	ZkillClient api.ZkillClient
	ESIClient   api.ESIClient
}

func InitTarget(name string) IntelTarget {
	target := IntelTarget{
		Name: name,
		Clients: CollectionClientPool{
			ZkillClient: api.NewZkillClient(),
			ESIClient:   api.NewESIClient(),
		},
	}

	target.GetTargetInfo()

	return target
}

func (t *IntelTarget) GetTargetInfo() {

	dispatch := map[string]types.EntityType{
		"alliances":       types.ALLIANCE,
		"characters":      types.PILOT,
		"corporations":    types.CORPORATION,
		"systems":         types.SYSTEM,
		"inventory_types": types.ITEM,
	}

	var (
		data interface{}
		err  error
	)

	names, err := t.Clients.ESIClient.GetEveIDsFromNames([]string{t.Name})

	if err != nil {
		t.Error = fmt.Errorf("Unable to find Target ID: %w", err)
		t.DisplayMessage = "Nothing found by that name. Check spelling?"
		return
	}

	if len(names) > 1 {

		if t.TypeOfEntity.String() != "UNKNOWN" {
			for _, value := range names {
				if dispatch[value.Category] == t.TypeOfEntity {
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
			t.TypeOfEntity = dispatch[value.Category]
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
		// handle error
	}

	t.Data = data

}
