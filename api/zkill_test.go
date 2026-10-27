package api_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
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
	      "totalDroppableValue": 2500,
	      "points": 1,
	      "npc": false,
	      "solo": false,
	      "awox": false,
	      "labels": ["tz:use", "cat:6", "#:5+", "pvp", "loc:w-space", "isk:under1b"]
	    }
	  }
	]`

	// Expected result
	expected := []api.ZkbResponse{
		{
			KillmailID: 122631871,
			Zkb: api.Zkb{
				LocationID:          40441022,
				Hash:                "8c7585924b08e5ec0c3671506b847f990af68f1b",
				FittedValue:         10000,
				DroppedValue:        0,
				DestroyedValue:      10000,
				TotalValue:          10000,
				TotalDroppableValue: 2500,
				Points:              1,
				NPC:                 false,
				Solo:                false,
				Awox:                false,
				Labels:              []string{"tz:use", "cat:6", "#:5+", "pvp", "loc:w-space", "isk:under1b"},
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

	t.Run("a page short of 200 ends the list", func(t *testing.T) {

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

	t.Run("a nearly full page 1 goes on, as zKillboard holds back the newest killmails, but a nearly full page 2 does not", func(t *testing.T) {
		result, err := api.ZkillConnection{Client: CreateMockZkillResponse(198)}.GetZkillPage("characterID/12345", 1, &[]api.ZkbResponse{})
		assert.NoError(t, err)
		assert.Equal(t, 2, result)

		result, err = api.ZkillConnection{Client: CreateMockZkillResponse(198)}.GetZkillPage("characterID/12345", 2, &[]api.ZkbResponse{})
		assert.NoError(t, err)
		assert.Equal(t, 9999, result)
	})

	t.Run("an empty page ends the list", func(t *testing.T) {
		allZkillmails := &[]api.ZkbResponse{}

		result, err := api.ZkillConnection{Client: CreateMockZkillResponse(0)}.GetZkillPage("characterID/12345", 3, allZkillmails)

		assert.NoError(t, err)
		assert.Empty(t, *allZkillmails)
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
	if numberOfKillmails == 0 {
		return mockZkillPage("[]")
	}
	manyKillmails := strings.Repeat(strings.Join([]string{singleKillmail, ", "}, ""), numberOfKillmails-1)
	if numberOfKillmails > 1 {
		zkillMockResponse = strings.Join([]string{"[", manyKillmails, singleKillmail, "]"}, " ")
	} else {
		zkillMockResponse = strings.Join([]string{"[", singleKillmail, "]"}, " ")
	}
	return mockZkillPage(zkillMockResponse)
}

// gzipped encodes a body the way zkill responses are
func gzipped(body string) io.ReadCloser {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	gzipWriter.Write([]byte(body))
	gzipWriter.Close()
	return io.NopCloser(&buffer)
}

// mockZkillPage answers one request with the page as zKillboard would send it
func mockZkillPage(page string) *MockApiClient {
	return &MockApiClient{
		Method:   "GET",
		Endpoint: "zkill.com/api",
		Body:     nil,
		Called:   0,
		response: &http.Response{
			StatusCode: 200,
			Body:       gzipped(page),
		},
		err: nil,
	}
}

// pagesClient answers each request with the next of its pages, then with empty pages. failAt is a request, from 1, answered with an error.
type pagesClient struct {
	api.APIClient
	pages  []string
	calls  int
	failAt int
}

func (p *pagesClient) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	if p.calls+1 == p.failAt {
		p.calls++
		return nil, fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "zkillboard.com", Status: http.StatusBadGateway})
	}
	page := "[]"
	if p.calls < len(p.pages) {
		page = p.pages[p.calls]
	}
	p.calls++
	return &http.Response{StatusCode: 200, Body: gzipped(page)}, nil
}

func TestGetZkillResults(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// idPage is a page of zKillboard entries with the ids from first, counting down
	idPage := func(first, count int) string {
		entries := []string{}
		for id := first; id > first-count; id-- {
			entries = append(entries, fmt.Sprintf(`{"killmail_id": %d}`, id))
		}
		return "[" + strings.Join(entries, ",") + "]"
	}

	t.Run("a nearly full page 1 goes on to page 2, which ends it short, keeping the first of a killmail listed twice", func(t *testing.T) {
		client := &pagesClient{pages: []string{idPage(1000, 190), idPage(811, 30)}}

		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10})

		assert.NoError(t, err)
		assert.Equal(t, 2, client.calls, "the short page 2 ends it, without asking for page 3")
		assert.Len(t, killmails.Killmails, 219, "killmail 811 is on both pages")
	})

	t.Run("a short list is one request", func(t *testing.T) {
		client := &pagesClient{pages: []string{idPage(1000, 30)}}

		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10})

		assert.NoError(t, err)
		assert.Equal(t, 1, client.calls)
		assert.Len(t, killmails.Killmails, 30)
	})

	t.Run("the page after a short page 1 failing is the end of the list, not a partial pull", func(t *testing.T) {
		client := &pagesClient{pages: []string{idPage(1000, 190)}, failAt: 2}

		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10})

		assert.NoError(t, err)
		assert.Len(t, killmails.Killmails, 190)
	})

	t.Run("a page after a full one failing is a partial pull", func(t *testing.T) {
		client := &pagesClient{pages: []string{idPage(1000, 200)}, failAt: 2}

		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10})

		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.Len(t, killmails.Killmails, 200)
	})

	fullPages := func() *pagesClient {
		page := func(first int) string {
			entries := []string{}
			for id := first; id < first+api.ZKILL_PAGE_SIZE; id++ {
				entries = append(entries, fmt.Sprintf(`{"killmail_id": %d}`, id))
			}
			return "[" + strings.Join(entries, ",") + "]"
		}
		return &pagesClient{pages: []string{page(1), page(201), page(401), page(601)}}
	}

	t.Run("stops once the killmails asked for are listed", func(t *testing.T) {
		client := fullPages()
		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{Killmails: 250, MaxPages: 100})

		assert.NoError(t, err)
		assert.Equal(t, 2, client.calls)
		assert.Len(t, killmails.Killmails, 400)
	})

	t.Run("starts at StartPage", func(t *testing.T) {
		client := fullPages()
		client.calls = 2 // the client answers its pages in order, so this serves page 3 first
		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{StartPage: 3, MaxPages: 3})

		assert.NoError(t, err)
		assert.Equal(t, 3, client.calls, "only page 3")
		assert.Equal(t, 401, killmails.Killmails[0].KillmailID)
		assert.False(t, killmails.Ended, "a full last page stopped by MaxPages is not the end")
	})

	t.Run("stops after the page listing a known killmail", func(t *testing.T) {
		client := fullPages()
		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10,
			Known: func(killmailID int) bool { return killmailID == 250 }})

		assert.NoError(t, err)
		assert.Equal(t, 2, client.calls)
		assert.Len(t, killmails.Killmails, 400)
		assert.False(t, killmails.Ended)
	})

	t.Run("stops after the page reaching the date, and a list ending short has Ended", func(t *testing.T) {
		now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
		timedPage := func(first, count, hoursBack int) string {
			entries := []string{}
			for index := range count {
				at := now.Add(-time.Duration(hoursBack+index) * time.Hour).Format(time.RFC3339)
				entries = append(entries, fmt.Sprintf(`{"killmail_id": %d, "killmail_time": %q}`, first-index, at))
			}
			return "[" + strings.Join(entries, ",") + "]"
		}
		pages := []string{timedPage(1000, 200, 0), timedPage(800, 200, 200), timedPage(600, 50, 400)}

		dated := &pagesClient{pages: pages}
		killmails, err := api.ZkillConnection{Client: dated}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10, NewerThan: now.Add(-300 * time.Hour)})
		assert.NoError(t, err)
		assert.Equal(t, 2, dated.calls, "page 2 reaches 300 hours back")
		assert.False(t, killmails.Ended)

		all := &pagesClient{pages: pages}
		killmails, err = api.ZkillConnection{Client: all}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 10})
		assert.NoError(t, err)
		assert.Len(t, killmails.Killmails, 450)
		assert.True(t, killmails.Ended, "the short page 3 is the end of the list")
	})

	t.Run("progress says whose list a page is, when told", func(t *testing.T) {
		messages := []string{}
		comm.SetProgressReporter(func(message string) { messages = append(messages, message) })
		defer comm.SetProgressReporter(nil)

		_, err := api.ZkillConnection{Client: fullPages()}.GetZkillResults(98000002, types.CORPORATION, api.ZkillPaging{MaxPages: 1, For: "holding Holding Corp"})

		assert.NoError(t, err)
		assert.Equal(t, []string{"Fetching zKillboard page 1 for holding Holding Corp (live, 0 killmails so far)"}, messages)
	})

	t.Run("never pages past MaxPages", func(t *testing.T) {
		client := fullPages()
		killmails, err := api.ZkillConnection{Client: client}.GetZkillResults(12345, types.PILOT, api.ZkillPaging{MaxPages: 2})

		assert.NoError(t, err)
		assert.Equal(t, 2, client.calls)
		assert.Len(t, killmails.Killmails, 400)
	})
}

func TestZkbResponseKillmail(t *testing.T) {
	entry := `{"killmail_id": 139460150, "killmail_time": "2026-10-26T21:50:01Z", "solar_system_id": 30000142,
		"victim": {"character_id": 90000001, "corporation_id": 98000001, "damage_taken": 4200, "ship_type_id": 587,
			"items": [{"flag": 27, "item_type_id": 2873, "quantity_dropped": 1, "singleton": 0}], "position": {"x": 1, "y": 2, "z": 3}},
		"attackers": [{"character_id": 90000002, "damage_done": 4200, "final_blow": true, "security_status": -2.5, "ship_type_id": 11202, "weapon_type_id": 2873}],
		"zkb": {"hash": "b592e632f080697e9e3718a3ddd62348fb6ac5e8"}}`

	t.Run("an entry with the whole killmail gives the ESI killmail", func(t *testing.T) {
		var zkillmail api.ZkbResponse
		assert.NoError(t, json.Unmarshal([]byte(entry), &zkillmail))

		killmail := zkillmail.Killmail()

		assert.NotNil(t, killmail)
		assert.Equal(t, 139460150, killmail.KillmailID)
		assert.Equal(t, time.Date(2026, 10, 26, 21, 50, 1, 0, time.UTC), killmail.KillmailTime)
		assert.Equal(t, 30000142, killmail.SolarSystemID)
		assert.Equal(t, 587, killmail.Victim.ShipTypeID)
		assert.Equal(t, 90000001, *killmail.Victim.CharacterID)
		assert.Len(t, killmail.Victim.Items, 1)
		assert.Equal(t, 3.0, killmail.Victim.Position.Z)
		assert.Len(t, killmail.Attackers, 1)
		assert.Equal(t, 2873, *killmail.Attackers[0].WeaponTypeID)
	})

	t.Run("an entry missing part of the killmail gives none, so the ESI's is fetched", func(t *testing.T) {
		missing := map[string]string{
			"no body":      `{"killmail_id": 1, "zkb": {"hash": "abc"}}`,
			"no attackers": `{"killmail_id": 1, "killmail_time": "2026-10-26T21:50:01Z", "solar_system_id": 30000142, "victim": {"ship_type_id": 587}, "attackers": []}`,
			"no ship":      `{"killmail_id": 1, "killmail_time": "2026-10-26T21:50:01Z", "solar_system_id": 30000142, "victim": {}, "attackers": [{"ship_type_id": 11202}]}`,
			"no system":    `{"killmail_id": 1, "killmail_time": "2026-10-26T21:50:01Z", "victim": {"ship_type_id": 587}, "attackers": [{"ship_type_id": 11202}]}`,
		}
		for name, body := range missing {
			var zkillmail api.ZkbResponse
			assert.NoError(t, json.Unmarshal([]byte(body), &zkillmail))
			assert.Nil(t, zkillmail.Killmail(), name)
		}
	})
}

// failingFirstClient answers its first request with err, and every later one with the wrapped client
type failingFirstClient struct {
	api.APIClient
	err   error
	calls int
}

func (f *failingFirstClient) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	f.calls++
	if f.calls == 1 {
		return nil, f.err
	}
	return f.APIClient.Request(method, endpoint, body)
}

func TestGetZkillPageRateLimit(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	defer api.SetZkillRateLimitPause(time.Millisecond)()

	failed := func(status int) error {
		return fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "zkillboard.com", Status: status})
	}

	t.Run("a rate limited page is tried once more after a pause", func(t *testing.T) {
		client := &failingFirstClient{APIClient: CreateMockZkillResponse(1), err: failed(http.StatusTooManyRequests)}
		allZkillmails := &[]api.ZkbResponse{}

		_, err := api.ZkillConnection{Client: client}.GetZkillPage("characterID/12345", 1, allZkillmails)

		assert.NoError(t, err)
		assert.Equal(t, 2, client.calls)
		assert.Len(t, *allZkillmails, 1)
	})

	t.Run("other failures are not tried again, and the shared error values are left alone", func(t *testing.T) {
		client := &failingFirstClient{APIClient: CreateMockZkillResponse(1), err: failed(http.StatusBadGateway)}

		_, err := api.ZkillConnection{Client: client}.GetZkillPage("characterID/12345", 1, &[]api.ZkbResponse{})

		assert.Error(t, err)
		assert.Equal(t, 1, client.calls)
		assert.Equal(t, "request failed to send", api.ErrRequestFailedToSend.Error())
	})
}
