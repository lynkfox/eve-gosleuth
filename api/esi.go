package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type ESIClient interface {
	GetEveIDsFromNames(names []string) (map[int]EntityIDAndName, error)
	GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error)
	GetCharacterInfo(idNumber int) (*ESICharacter, error)
	GetCorporationInfo(idNumber int) (*ESICorporation, error)
	GetAllianceInfo(idNumber int) (*ESIAlliance, error)
	GetSystemInfo(idNumber int) (*ESISystem, error)
	GetItemInfo(idNumber int) (*ESIItem, error)
	GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error)
	GetKillmail(zkillmail ZkbResponse) (*ESIKillmail, error)
	// GetSystemSovInfo()
	// GetCharacterCorpHistory()
}

type ESIConnection struct {
	Client APIClient
}

// NewESIClient initizes the connection data for an ApiConnection as a ESIClient interface
func NewESIClient() *ESIConnection {

	connection := &ESIConnection{
		Client: APIConnection{
			BaseURL: ESI_URL,
			HTTPClient: &http.Client{
				Timeout: 3 * time.Second,
			},
			Headers: map[string]string{
				"accept":          "application/json",
				"Accept-Language": "en",
				"Content-Type":    "application/json",
				"Cache-Control":   "no-cache",
			},
			MaxRetries: 3,
			Backoff:    1 * time.Second,
		},
	}

	return connection
}

/*
GetEveIDsFromNames takes a slice of names and attempts to return a map if ID:Names
*/
func (e ESIConnection) GetEveIDsFromNames(names []string) (map[int]EntityIDAndName, error) {
	url := fmt.Sprintf("%s/universe/ids", ESI_URL)

	namesAndIds := ESIUniverseIDsResponse{}

	resp, err := e.Client.Request("GET", url, nil)

	if err != nil {
		if errors.As(err, &ErrRequestFailedToSend) {
			comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			return nil, err
		} else {
			return nil, err
		}

	}

	err = json.NewDecoder(resp.Body).Decode(&namesAndIds)

	if err != nil {
		comm.Logger.WithError(err).Fatal("Could not decode response from ESI")
		return nil, err
	}

	extracted := combineEntities(namesAndIds)

	return extracted, nil

}

/*
GetNamesFromEveIDs takes a slice of ids and attempts to return a map if ID:Names
*/
func (e ESIConnection) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	url := fmt.Sprintf("%s/universe/names", ESI_URL)

	namesAndIds := []EntityIDAndName{}
	combined := make(map[int]string)

	resp, err := e.Client.Request("GET", url, nil)

	if err != nil {
		if errors.As(err, &ErrRequestFailedToSend) {
			comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			return nil, err
		} else {
			return nil, err
		}

	}

	err = json.NewDecoder(resp.Body).Decode(&namesAndIds)

	if err != nil {
		comm.Logger.WithError(err).Fatal("Could not decode response from ESI")
		return nil, err
	}

	for i := 0; i < len(namesAndIds); i++ {
		combined[namesAndIds[i].ID] = namesAndIds[i].Name

	}

	return combined, err

}

// GetCharacterInfo takes a character ID and returns an ESICharacter object
func (e ESIConnection) GetCharacterInfo(idNumber int) (*ESICharacter, error) {

	character, err := e.GetEsiResponse(idNumber, types.PILOT)

	return character.(*ESICharacter), err
}

// GetCorporationInfo takes a character ID and returns an ESICorporation object
func (e ESIConnection) GetCorporationInfo(idNumber int) (*ESICorporation, error) {

	character, err := e.GetEsiResponse(idNumber, types.CORPORATION)

	return character.(*ESICorporation), err
}

// GetAllianceInfo takes a character ID and returns an ESIAlliance object
func (e ESIConnection) GetAllianceInfo(idNumber int) (*ESIAlliance, error) {

	character, err := e.GetEsiResponse(idNumber, types.ALLIANCE)

	return character.(*ESIAlliance), err
}

// GetSystemInfo takes a system ID and returns an ESISystem object
func (e ESIConnection) GetSystemInfo(idNumber int) (*ESISystem, error) {

	character, err := e.GetEsiResponse(idNumber, types.SYSTEM)

	return character.(*ESISystem), err
}

// GetItemInfo takes a system ID and returns an ESIItem object
func (e ESIConnection) GetItemInfo(idNumber int) (*ESIItem, error) {

	character, err := e.GetEsiResponse(idNumber, types.ITEM)

	return character.(*ESIItem), err
}

// GetKillmail takes a zkillmail response that includes an id number and a hash and retrieves the same killmail from the esi
func (e ESIConnection) GetKillmail(zkillmail ZkbResponse) (*ESIKillmail, error) {
	url := fmt.Sprintf("killmails/%d/%s", zkillmail.KillmailID, zkillmail.Zkb.Hash)
	killmail := ESIKillmail{}

	resp, err := e.Client.Request("GET", url, nil)

	if err != nil {
		if errors.As(err, &ErrRequestFailedToSend) {
			comm.Logger.WithError(err).Error("Failed to get killmail from ESI")
			return nil, err
		} else {
			return nil, err
		}

	}

	err = json.NewDecoder(resp.Body).Decode(&killmail)

	if err != nil {
		comm.Logger.WithError(err).Errorf("Failed to parse killmail from url %s from the ESI", url)
		return nil, err
	}

	return &killmail, nil
}

/*
	GetEsiResponse is a generic wrapper around several similar ESI calls that will return a struct object of the response

Parameters:
  - idNumber: the Eve Id Number to use in the call
  - typeOfEntity: the type of call to be made (types.EntityType)

Returns:
  - interface{}: an ESItype object such as ESICharacter or ESICorporations, with the ID number included
  - error: any potential error
*/
func (e ESIConnection) GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error) {
	endpointDispatch := map[types.EntityType]string{
		types.ALLIANCE:      "alliances",
		types.CORPORATION:   "corporations",
		types.PILOT:         "characters",
		types.SYSTEM:        "universe/systems",
		types.ITEM:          "universe/types",
		types.CONSTELLATION: "universe/constellations",
		types.REGION:        "universe/regions",
	}

	modelDispatch := map[types.EntityType]interface{}{
		types.ALLIANCE:      &ESIAlliance{AllianceID: idNumber, DateCached: time.Now().UTC()},
		types.CORPORATION:   &ESICorporation{CorporationID: idNumber, DateCached: time.Now().UTC()},
		types.PILOT:         &ESICharacter{CharacterID: idNumber, DateCached: time.Now().UTC()},
		types.SYSTEM:        &ESISystem{},
		types.ITEM:          &ESIItem{},
		types.CONSTELLATION: &ESIConstellation{},
		types.REGION:        &ESIRegion{},
	}

	err := e.getEsiResponseFromEndpointWithID(idNumber, modelDispatch[typeOfEntity], endpointDispatch[typeOfEntity])

	if err != nil {
		comm.Logger.WithError(err).Errorf("GetESI Response for %s failed on %d", typeOfEntity, idNumber)
		return nil, err
	}

	return modelDispatch[typeOfEntity], nil

}

/*
	GetRsponseFromEndpointWithID is a generic method that can be used for any of the ESI_URL/[method]/[idnumber] end points. It updates the model passed in reference.

Parameters:
  - idNumber: the eve ID number to look up (int).
  - modelType: an compatible json annotated struct such as ESICharacter that the response from the url will be decoded into (interface{})
  - url: the method from esi for this id based endpoint, such as characters or corporations (str)

Returns:
  - any potential HTTP error.
*/
func (e ESIConnection) getEsiResponseFromEndpointWithID(idNumber int, modelType interface{}, url string) error {
	endpoint := fmt.Sprintf("%s/%d/", url, idNumber)

	resp, err := e.Client.Request("GET", endpoint, nil)

	if err != nil {
		if errors.As(err, &ErrRequestFailedToSend) {
			comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			return err
		} else {
			return err
		}

	}

	err = json.NewDecoder(resp.Body).Decode(&modelType)

	return err

}

/*
	CombineEntities takes responses from the ESI Universe IDs endpoint and constructs it into a map of ID: Name

Parameters:
  - response: A response from the ESI containing fields that are slices of EntityIDtoName structs

Returns:
  - a map[int]string of ID number to name
*/
func combineEntities(response ESIUniverseIDsResponse) map[int]EntityIDAndName {
	combined := make(map[int]EntityIDAndName)

	// Use reflection to iterate over fields of the struct
	val := reflect.ValueOf(response)
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			// Dereference the pointer to get the slice
			slice := field.Elem()
			if slice.Kind() == reflect.Slice {
				for j := 0; j < slice.Len(); j++ {
					entity := slice.Index(j).Interface().(EntityIDAndName)
					combined[entity.ID] = entity
				}
			}
		}
	}

	return combined
}
