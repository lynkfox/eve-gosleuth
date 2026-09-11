package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func (m MockApiClient) UpdateHeaders(newHeaders map[string]string) {

}

// TestZkbResponse ensures that the ZkbResponse struct correctly unmarshals JSON data
func TestZkbResponse(t *testing.T) {
	// Example JSON input
	jsonData := `
	[
	  {
	    "killmail_id": 122631871,
	    "zkb": {
	      "locationID": 40441022,
	      "hash": "8c7585924b08e5ec0c3671506b847f990af68f1b",
	      "fittedValue": 10000,
	      "droppedValue": 0,
	      "destroyedValue": 10000,
	      "totalValue": 10000,
	      "points": 1,
	      "npc": false,
	      "solo": false,
	      "awox": false
	    }
	  }
	]`

	// Expected result
	expected := []api.ZkbResponse{
		{
			KillmailID: 122631871,
			Zkb: api.Zkb{
				LocationID:     40441022,
				Hash:           "8c7585924b08e5ec0c3671506b847f990af68f1b",
				FittedValue:    10000,
				DroppedValue:   0,
				DestroyedValue: 10000,
				TotalValue:     10000,
				Points:         1,
				NPC:            false,
				Solo:           false,
				Awox:           false,
			},
		},
	}

	// Unmarshal JSON into ZkbResponse slice
	var actual []api.ZkbResponse
	err := json.Unmarshal([]byte(jsonData), &actual)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Assert the result matches expected
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Unmarshaled result does not match expected:\nExpected: %+v\nActual: %+v", expected, actual)
	}
}

func TestGetZillPage(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("less than 200 killmails returned on a page", func(t *testing.T) {

		// Create a mocked ZkillConnection
		mock := CreateMockZkillResponse(1)
		zkillClient := api.ZkillConnection{
			Client: mock,
		}
		allZkillmails := &[]api.ZkbResponse{}

		// Mock the Request calls to simulate API responses
		result, err := zkillClient.GetZkillPage("characterID/12345/page", 1, allZkillmails)

		// Validate the results
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, len(*allZkillmails))
		assert.Equal(t, 122312802, (*allZkillmails)[0].KillmailID)
		assert.Equal(t, 9999, result)
	})

	t.Run("200 kms returns the next page", func(t *testing.T) {

		zkillClient := api.ZkillConnection{
			Client: CreateMockZkillResponse(200),
		}
		allZkillmails := &[]api.ZkbResponse{}

		// Mock the Request calls to simulate API responses
		result, err := zkillClient.GetZkillPage("characterID/12345/page", 1, allZkillmails)

		// Validate the results
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 200, len(*allZkillmails))
		assert.Equal(t, 122312802, (*allZkillmails)[0].KillmailID)
		assert.Equal(t, 2, result)

		mock2 := CreateMockZkillResponse(200)
		zkillClient = api.ZkillConnection{
			Client: mock2,
		}

		result, err = zkillClient.GetZkillPage("characterID/12345/page", result, allZkillmails)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 400, len(*allZkillmails))
		assert.Equal(t, 122312802, (*allZkillmails)[0].KillmailID)
		assert.Equal(t, 122312802, (*allZkillmails)[201].KillmailID)
		assert.Equal(t, 3, result)
	})

}

func CreateMockZkillResponse(numberOfKillmails int) *MockApiClient {
	singleKillmail := `{ "killmail_id": 122312802, "zkb": { "locationID": 40251268, "hash": "3d9d091bbd20eb655bf2c57c3f6524237d86a3b2", "fittedValue": 21919076.03, "droppedValue": 5919.74, "destroyedValue": 32100856.29, "totalValue": 32106776.03, "points": 1, "npc": false, "solo": false, "awox": false, "labels": [ "cat:6", "#:5+", "pvp", "loc:nullsec" ] } }`
	var zkillMockResponse string
	manyKillmails := strings.Repeat(strings.Join([]string{singleKillmail, ", "}, ""), numberOfKillmails-1)
	if numberOfKillmails > 1 {
		zkillMockResponse = strings.Join([]string{"[", manyKillmails, singleKillmail, "]"}, " ")
	} else {
		zkillMockResponse = strings.Join([]string{"[", singleKillmail, "]"}, " ")
	}
	r := io.NopCloser(bytes.NewReader([]byte(zkillMockResponse)))
	// Create a mocked ZkillConnection
	return &MockApiClient{
		Method:   "GET",
		Endpoint: "zkill.com/api",
		Body:     nil,
		Called:   0,
		response: &http.Response{
			StatusCode: 200,
			Body:       r,
		},
		err: nil,
	}

}
