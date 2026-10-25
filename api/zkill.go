package api

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/user"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ErrOnlyPartialZkillPull is returned if there is a failure when attempting to retrieve zkill pages, but some values may be in the return as well
var ErrOnlyPartialZkillPull = errors.New("zkill pull failed to complete all pages")

type ZkillClient interface {
	GetZkillResults(entityId int, entityType types.EntityType) (*[]ZkbResponse, error)
	addUserHeader()
}

type ZkillConnection struct {
	Client APIClient
}

// NewZkillClient initizes the connection data for an ApiConnection as a ZkillClient interface
func NewZkillClient() *ZkillConnection {

	connection := &ZkillConnection{
		Client: APIConnection{
			BaseURL: ZKILL_API_URL,
			HTTPClient: &http.Client{
				Timeout: 3 * time.Second,
			},
			Headers: map[string]string{
				"Accept-Encoding": "gzip",
			},
			MaxRetries: 3,
			Backoff:    1 * time.Second,
		},
	}

	connection.addUserHeader()

	return connection
}

/*
	GetZkillResults works through zkill responses pages until there are no more killmails or page 10 is recieved.

Parameters:
  - entityID:  the eve ID for the entity being checked on zkill (int)
  - typeOfEntity: the type, such as PILOT or ALLIANCE being checked (types.EntityType)

Returns:
  - responses: a slice containing all there killmails across all pages (*[]ZkbResponse)
  - error: nill, ErrOnlyPartialZkillPull if it failed to reach the end of the killmail lists - the killmails retrieved will be returned as well
*/
func (z ZkillConnection) GetZkillResults(entityId int, typeOfEntity types.EntityType) (*[]ZkbResponse, error) {
	var DispatchMap = map[types.EntityType]string{
		types.PILOT:       "characterID",
		types.SYSTEM:      "systemID",
		types.ALLIANCE:    "allianceID",
		types.CORPORATION: "corporationID",
	}

	endpoint := fmt.Sprintf("%s/%d", DispatchMap[typeOfEntity], entityId)
	allZkillmails := &[]ZkbResponse{}

	pageNumber := 1
	for pageNumber <= 10 {
		nextPage, err := z.GetZkillPage(endpoint, pageNumber, allZkillmails)

		pageNumber = nextPage

		if err != nil {
			return allZkillmails, fmt.Errorf("%w at page %d: %w", ErrOnlyPartialZkillPull, pageNumber-1, err)
		}
	}

	return allZkillmails, nil

}

/*
	GetZkillPage will pull the zkill page # provided for a given entity, combining all the zkill responses into the into the master list.

Parameters:
  - endpoint: the zkill endpoint, which includes the entity and entityID (string)
  - page: the page number, should start at 1, should not be more than 10 (int)
  - allZkillmails: the currently found killmails from zkillboard to be added too (*[]ZkbResponse)
*/
func (z ZkillConnection) GetZkillPage(endpoint string, page int, allZkillmails *[]ZkbResponse) (int, error) {

	thisPageKillmails := []ZkbResponse{}

	var nextPage int = 9999

	url := fmt.Sprintf("%s/page/%d/", endpoint, page)
	resp, err := z.Client.Request("GET", url, nil)

	if err != nil {
		if errors.As(err, &ErrRequestFailedToSend) {
			comm.Logger.WithError(err).Error("Failed to initiate request - check body")
			return nextPage, err
		}
		if errors.As(err, &ErrRequestFailedToComplete) {
			comm.Logger.WithError(err).Error("Request was blocked - 30 second pause")
			time.Sleep(30 * time.Second)
			resp, err = z.Client.Request("GET", url, nil)

			if err != nil {
				return nextPage, err
			}
		}
	}

	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		comm.Logger.WithError(err).Error("Could not decode response from Zkill (gzip decode)")
		return nextPage, err
	}

	defer gzipReader.Close()

	err = json.NewDecoder(gzipReader).Decode(&thisPageKillmails)

	if err != nil {
		comm.Logger.WithError(err).Error("Could not decode response from Zkill")
		return nextPage, err
	}

	if len(thisPageKillmails) >= 200 {
		nextPage = page + 1
	}

	*allZkillmails = append(*allZkillmails, thisPageKillmails...)

	return nextPage, nil
}

func (c *ZkillConnection) addUserHeader() {

	var username string = "Generic User"
	currentUser, err := user.Current()
	if err != nil {
		username = "Generic User"
	} else {
		username = currentUser.Username
	}

	c.Client.UpdateHeaders(map[string]string{"User-Agent": fmt.Sprintf("go-eve-sleuth/v%s (%s)", comm.VERSION, username)})
}
