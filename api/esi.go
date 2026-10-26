package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type ESIClient interface {
	GetEveIDsFromNames(names []string) (map[int]EntityIDAndName, error)
	GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error)
	GetCharacterAffiliations(characterIDs []int) (map[int]ESIAffiliation, error)
	GetCharacterInfo(idNumber int) (*ESICharacter, error)
	GetCorporationInfo(idNumber int) (*ESICorporation, error)
	GetAllianceInfo(idNumber int) (*ESIAlliance, error)
	GetSystemInfo(idNumber int) (*ESISystem, error)
	GetItemInfo(idNumber int) (*ESIItem, error)
	GetStargateInfo(idNumber int) (*ESIStargate, error)
	GetStarInfo(idNumber int) (*ESIStar, error)
	GetAllianceCorporations(allianceID int) (*ESIAllianceCorporations, error)
	GetCorporationHistory(characterID int) (*ESICorporationHistory, error)
	GetAllianceHistory(corporationID int) (*ESIAllianceHistory, error)
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
				"User-Agent":      USER_AGENT,
			},
			MaxRetries: 3,
			Backoff:    1 * time.Second,
			Guard:      NewRateGuard(ESI_MAX_IN_FLIGHT),
		},
	}

	return connection
}

/*
	GetEveIDsFromNames resolves exact names to their eve IDs with the bulk ESI universe/ids endpoint. Names are sent in batches of 500, the ESI limit per request.

Parameters:
  - names: the exact names to look up (case insensitive)

Returns:
  - map[int]EntityIDAndName: every match keyed by ID, with Category set to the ESI group it was found in (such as "characters" or "corporations"). Names with no match are left out
  - error: any request or decode error
*/
func (e ESIConnection) GetEveIDsFromNames(names []string) (map[int]EntityIDAndName, error) {
	extracted := make(map[int]EntityIDAndName)

	for batch := range slices.Chunk(names, 500) {
		namesAndIds := ESIUniverseIDsResponse{}

		resp, err := e.Client.Request("POST", "universe/ids/", batch)

		if err != nil {
			if errors.Is(err, ErrRequestFailedToSend) {
				comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			}
			return nil, err
		}

		err = json.NewDecoder(resp.Body).Decode(&namesAndIds)
		resp.Body.Close()

		if err != nil {
			comm.Logger.WithError(err).Error("Could not decode universe/ids response from ESI")
			return nil, err
		}

		maps.Copy(extracted, combineEntities(namesAndIds))
	}

	return extracted, nil

}

/*
	GetNamesFromEveIDs resolves eve IDs to their names with the bulk ESI universe/names endpoint. IDs are sent in batches of 1000, the ESI limit per request.

Parameters:
  - idsNumbers: the eve IDs to look up

Returns:
  - map[int]string: ID to name
  - error: any request or decode error
*/
func (e ESIConnection) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	combined := make(map[int]string)

	for batch := range slices.Chunk(idsNumbers, ESI_BULK_IDS) {
		namesAndIds := []EntityIDAndName{}

		resp, err := e.Client.Request("POST", "universe/names/", batch)

		if err != nil {
			if errors.Is(err, ErrRequestFailedToSend) {
				comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			}
			return nil, err
		}

		err = json.NewDecoder(resp.Body).Decode(&namesAndIds)
		resp.Body.Close()

		if err != nil {
			comm.Logger.WithError(err).Error("Could not decode universe/names response from ESI")
			return nil, err
		}

		for i := 0; i < len(namesAndIds); i++ {
			combined[namesAndIds[i].ID] = namesAndIds[i].Name
		}
	}

	return combined, nil

}

/*
	GetCharacterAffiliations looks up characters' current corporations and alliances in bulk, ESI_BULK_IDS at a time, with the characters/affiliation endpoint.

Parameters:
  - characterIDs: the characters to look up. One invalid id fails its whole batch, deleted characters are fine.

Returns:
  - map[int]ESIAffiliation: by character id
  - error: any request or decode error, the batches before it are still returned
*/
func (e ESIConnection) GetCharacterAffiliations(characterIDs []int) (map[int]ESIAffiliation, error) {
	combined := make(map[int]ESIAffiliation)

	for batch := range slices.Chunk(characterIDs, ESI_BULK_IDS) {
		affiliations := []ESIAffiliation{}

		resp, err := e.Client.Request("POST", "characters/affiliation/", batch)
		if err != nil {
			return combined, err
		}

		err = json.NewDecoder(resp.Body).Decode(&affiliations)
		resp.Body.Close()
		if err != nil {
			comm.Logger.WithError(err).Error("Could not decode characters/affiliation response from ESI")
			return combined, err
		}

		for _, affiliation := range affiliations {
			combined[affiliation.CharacterID] = affiliation
		}
	}

	return combined, nil
}

// GetCharacterInfo takes a character ID and returns an ESICharacter object
func (e ESIConnection) GetCharacterInfo(idNumber int) (*ESICharacter, error) {
	return typedEsiResponse[ESICharacter](e, idNumber, types.PILOT)
}

// GetCorporationInfo takes a character ID and returns an ESICorporation object
func (e ESIConnection) GetCorporationInfo(idNumber int) (*ESICorporation, error) {
	return typedEsiResponse[ESICorporation](e, idNumber, types.CORPORATION)
}

// GetAllianceInfo takes a character ID and returns an ESIAlliance object
func (e ESIConnection) GetAllianceInfo(idNumber int) (*ESIAlliance, error) {
	return typedEsiResponse[ESIAlliance](e, idNumber, types.ALLIANCE)
}

// GetSystemInfo takes a system ID and returns an ESISystem object
func (e ESIConnection) GetSystemInfo(idNumber int) (*ESISystem, error) {
	return typedEsiResponse[ESISystem](e, idNumber, types.SYSTEM)
}

// GetItemInfo takes a system ID and returns an ESIItem object
func (e ESIConnection) GetItemInfo(idNumber int) (*ESIItem, error) {
	return typedEsiResponse[ESIItem](e, idNumber, types.ITEM)
}

// typedEsiResponse gets an ESI response as its model type, returning the error rather than panicking on a nil response when the request failed
func typedEsiResponse[T any](e ESIConnection, idNumber int, typeOfEntity types.EntityType) (*T, error) {
	response, err := e.GetEsiResponse(idNumber, typeOfEntity)
	if err != nil {
		return nil, err
	}
	return response.(*T), nil
}

// GetStargateInfo takes a stargate ID and returns an ESIStargate object, which says which system the gate leads to
func (e ESIConnection) GetStargateInfo(idNumber int) (*ESIStargate, error) {
	stargate := &ESIStargate{}
	if err := e.getEsiResponseFromEndpointWithID(idNumber, stargate, "universe/stargates"); err != nil {
		return nil, err
	}
	return stargate, nil
}

// GetAllianceCorporations takes an alliance ID and returns the IDs of its member corporations
func (e ESIConnection) GetAllianceCorporations(allianceID int) (*ESIAllianceCorporations, error) {
	resp, err := e.Client.Request("GET", fmt.Sprintf("alliances/%d/corporations/", allianceID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	members := &ESIAllianceCorporations{AllianceID: allianceID}
	if err := json.NewDecoder(resp.Body).Decode(&members.CorporationIDs); err != nil {
		return nil, err
	}
	return members, nil
}

// GetCorporationHistory takes a character ID and returns every corporation they have been in
func (e ESIConnection) GetCorporationHistory(characterID int) (*ESICorporationHistory, error) {
	resp, err := e.Client.Request("GET", fmt.Sprintf("characters/%d/corporationhistory/", characterID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	history := &ESICorporationHistory{CharacterID: characterID}
	if err := json.NewDecoder(resp.Body).Decode(&history.Records); err != nil {
		return nil, err
	}
	return history, nil
}

// GetAllianceHistory takes a corporation ID and returns every alliance it has been in
func (e ESIConnection) GetAllianceHistory(corporationID int) (*ESIAllianceHistory, error) {
	resp, err := e.Client.Request("GET", fmt.Sprintf("corporations/%d/alliancehistory/", corporationID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	history := &ESIAllianceHistory{CorporationID: corporationID}
	if err := json.NewDecoder(resp.Body).Decode(&history.Records); err != nil {
		return nil, err
	}
	return history, nil
}

// GetStarInfo takes a star ID and returns an ESIStar object, whose type id is used for the star's image
func (e ESIConnection) GetStarInfo(idNumber int) (*ESIStar, error) {
	star := &ESIStar{}
	if err := e.getEsiResponseFromEndpointWithID(idNumber, star, "universe/stars"); err != nil {
		return nil, err
	}
	return star, nil
}

// GetKillmail takes a zkillmail response that includes an id number and a hash and retrieves the same killmail from the esi
func (e ESIConnection) GetKillmail(zkillmail ZkbResponse) (*ESIKillmail, error) {
	url := fmt.Sprintf("killmails/%d/%s", zkillmail.KillmailID, zkillmail.Zkb.Hash)
	killmail := ESIKillmail{}

	resp, err := e.Client.Request("GET", url, nil)

	if err != nil {
		if errors.Is(err, ErrRequestFailedToSend) {
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
		if errors.Is(err, ErrRequestFailedToSend) {
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
	CombineEntities takes responses from the ESI Universe IDs endpoint and constructs it into a map of ID: EntityIDAndName

The universe/ids response groups entities by type rather than labeling each one, so Category is set from the group name.

Parameters:
  - response: A response from the ESI containing fields that are slices of EntityIDtoName structs

Returns:
  - a map[int]EntityIDAndName of ID number to entity
*/
func combineEntities(response ESIUniverseIDsResponse) map[int]EntityIDAndName {
	combined := make(map[int]EntityIDAndName)

	groups := map[string]*[]EntityIDAndName{
		"alliances":       response.Alliances,
		"characters":      response.Characters,
		"corporations":    response.Corporations,
		"inventory_types": response.InventoryTypes,
		"systems":         response.Systems,
	}

	for category, entities := range groups {
		if entities == nil {
			continue
		}
		for _, entity := range *entities {
			entity.Category = category
			combined[entity.ID] = entity
		}
	}

	return combined
}
