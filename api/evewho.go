package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ErrNoEveWhoData is returned when EveWho has no record for the entity
var ErrNoEveWhoData = errors.New("no EveWho data found")

type EveWhoClient interface {
	GetCharacter(characterID int) (*EveWhoCharacterResponse, error)
	GetMemberCount(entityID int, typeOfEntity types.EntityType) (int, error)
	GetCorpJoined(corporationID int, page int) (*EveWhoJoinList, error)
}

type EveWhoConnection struct {
	Client APIClient
}

// NewEveWhoClient initizes the connection data for an ApiConnection as an EveWhoClient interface
func NewEveWhoClient() *EveWhoConnection {

	connection := &EveWhoConnection{
		Client: APIConnection{
			BaseURL: EVE_WHO_API_URL,
			HTTPClient: &http.Client{
				Timeout: 5 * time.Second,
			},
			Headers:    map[string]string{"User-Agent": USER_AGENT},
			MaxRetries: 2, // EveWho is only used for the target panel, so a failure should not hold up the search for long
			Backoff:    500 * time.Millisecond,
		},
	}

	return connection
}

/*
	GetCharacter pulls a pilot's current info and corp history from EveWho

Parameters:
  - characterID: the eve id of the pilot

Returns:
  - *EveWhoCharacterResponse: the pilot's info and corp history, oldest first
  - error: any request or decode error, or ErrNoEveWhoData if EveWho does not know the pilot
*/
func (e EveWhoConnection) GetCharacter(characterID int) (*EveWhoCharacterResponse, error) {
	response := &EveWhoCharacterResponse{}

	if err := e.get(fmt.Sprintf("character/%d", characterID), response); err != nil {
		return nil, err
	}
	if len(response.Info) == 0 {
		return nil, ErrNoEveWhoData
	}

	return response, nil
}

/*
	GetMemberCount pulls how many pilots EveWho has in a corporation or alliance

Parameters:
  - entityID: the eve id of the corporation or alliance
  - typeOfEntity: types.CORPORATION or types.ALLIANCE

Returns:
  - int: the member count
  - error: any request or decode error, or ErrNoEveWhoData if EveWho does not know the entity
*/
func (e EveWhoConnection) GetMemberCount(entityID int, typeOfEntity types.EntityType) (int, error) {
	endpoint, ok := memberListEndpoints[typeOfEntity]
	if !ok {
		return 0, fmt.Errorf("EveWho has no member list for %s", typeOfEntity)
	}

	response := &EveWhoGroupResponse{}
	if err := e.get(fmt.Sprintf("%s/%d", endpoint, entityID), response); err != nil {
		return 0, err
	}
	if len(response.Info) == 0 {
		return 0, ErrNoEveWhoData
	}

	return response.Info[0].MemberCount, nil
}

// memberListEndpoints are EveWho's member lists for each kind of group
var memberListEndpoints = map[types.EntityType]string{
	types.CORPORATION: "corplist",
	types.ALLIANCE:    "allilist",
}

/*
	GetCorpJoined pulls one page of a corporation's join records from EveWho, newest join first, EVEWHO_PAGE_SIZE to a page. It lists everyone who has ever joined, those who have since left have an end date.

Parameters:
  - corporationID: the eve id of the corporation
  - page: the page, from 1

Returns:
  - *EveWhoJoinList: the page's join records and the paging
  - error: any request or decode error
*/
func (e EveWhoConnection) GetCorpJoined(corporationID int, page int) (*EveWhoJoinList, error) {
	response := &EveWhoJoinList{}
	if err := e.get(fmt.Sprintf("corpjoined/%d/page/%d", corporationID, page), response); err != nil {
		return nil, err
	}
	return response, nil
}

// get requests an EveWho api endpoint and decodes the json response into model
func (e EveWhoConnection) get(endpoint string, model interface{}) error {
	resp, err := e.Client.Request("GET", endpoint, nil)
	if err != nil {
		comm.Logger.WithError(err).Warnf("[EveWho] Request for %s failed", endpoint)
		return err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(model); err != nil {
		comm.Logger.WithError(err).Errorf("[EveWho] Could not decode response for %s", endpoint)
		return err
	}

	return nil
}
