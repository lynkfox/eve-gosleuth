package api_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type MockApiClient struct {
	Method   string
	Endpoint string
	Body     interface{}
	Called   int
	response *http.Response
	err      error
}

func (m MockApiClient) WasCalled() int {
	return m.Called
}

// Mock the Request method
func (m *MockApiClient) Request(method, endpoint string, body interface{}) (*http.Response, error) {
	m.Called = m.Called + 1
	m.Method = method
	m.Endpoint = endpoint
	m.Body = body

	return m.response, m.err
}

func CreateMockResposne(jsonData string, method string, endpoint string, statuscode int) *MockApiClient {

	r := io.NopCloser(bytes.NewReader([]byte(jsonData)))
	return &MockApiClient{
		Method:   method,
		Endpoint: endpoint,
		Body:     nil,
		Called:   0,
		response: &http.Response{
			StatusCode: statuscode,
			Body:       r,
		},
		err: nil,
	}
}

func TestRequestRetryResendsBody(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	receivedBodies := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBodies = append(receivedBodies, string(body))
		if len(receivedBodies) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	connection := api.APIConnection{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Headers:    map[string]string{},
		MaxRetries: 2,
		Backoff:    time.Millisecond,
	}

	resp, err := connection.Request("POST", "universe/ids/", []string{"CCP Zoetrope"})

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{`["CCP Zoetrope"]`, `["CCP Zoetrope"]`}, receivedBodies)
}

func TestFailureReason(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	connectTo := func(server *httptest.Server) api.APIConnection {
		return api.APIConnection{BaseURL: server.URL, HTTPClient: server.Client(), Headers: map[string]string{}, MaxRetries: 2, Backoff: time.Millisecond}
	}

	t.Run("an error status says what the server answered and why", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Character not found"}`))
		}))
		defer server.Close()

		_, err := connectTo(server).Request("GET", "characters/1/", nil)

		assert.ErrorIs(t, err, api.ErrRequestFailedToComplete)
		assert.Equal(t, strings.TrimPrefix(server.URL, "http://")+" returned 404 Not Found: Character not found", api.FailureReason(err))
	})

	t.Run("an error status without an explanation still gives the status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()

		_, err := connectTo(server).Request("GET", "characters/1/", nil)

		assert.Equal(t, strings.TrimPrefix(server.URL, "http://")+" returned 502 Bad Gateway", api.FailureReason(err))
	})

	t.Run("a server that can not be reached says so", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		connection := connectTo(server)
		server.Close()

		_, err := connection.Request("GET", "characters/1/", nil)

		assert.Equal(t, "could not reach "+strings.TrimPrefix(server.URL, "http://"), api.FailureReason(err))
	})

	t.Run("a slow server times out", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
		defer server.Close()
		defer close(release)

		connection := connectTo(server)
		connection.HTTPClient = &http.Client{Timeout: 20 * time.Millisecond}

		_, err := connection.Request("GET", "characters/1/", nil)

		assert.Equal(t, "timed out reaching "+strings.TrimPrefix(server.URL, "http://"), api.FailureReason(err))
	})

	t.Run("other errors are given as they are", func(t *testing.T) {
		assert.Equal(t, "unexpected EOF", api.FailureReason(io.ErrUnexpectedEOF))
	})
}

func TestUserAgent(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// every service sees the same app and contact, never the user's own name
	headersOf := func(client api.APIClient) map[string]string { return client.(api.APIConnection).Headers }
	for name, headers := range map[string]map[string]string{
		"ESI":        headersOf(api.NewESIClient().Client),
		"zKillboard": headersOf(api.NewZkillClient().Client),
		"EveWho":     headersOf(api.NewEveWhoClient().Client),
		"SDE":        headersOf(api.NewSDEClient().Client),
	} {
		assert.Equal(t, "eve-gosleuth/"+comm.VERSION+" (+https://github.com/lynkfox/eve-gosleuth)", headers["User-Agent"], name)
	}
}

func TestZkillTogetherURL(t *testing.T) {
	t.Run("the target and its holdings are attackers, any of them, and the ally is in Either, as zKillboard writes the url", func(t *testing.T) {
		// URSA. and its holding High Bear Nation with 10mn, for all time, as built on zKillboard
		want := "https://zkillboard.com/asearch/#%7B%22buttons%22%3A%5B%22togglefilters%22%2C%22alltime%22%2C%22attackers-or%22%2C%22either-and%22%2C%22victims-and%22%2C%22items-and%22%2C%22sort-date%22%2C%22sort-desc%22%2C%22page1%22%2C%22allinvolved%22%5D%2C%22attackers%22%3A%5B%7B%22type%22%3A%22allianceID%22%2C%22id%22%3A99013330%7D%2C%7B%22type%22%3A%22allianceID%22%2C%22id%22%3A99013580%7D%5D%2C%22neutrals%22%3A%5B%7B%22type%22%3A%22allianceID%22%2C%22id%22%3A99014851%7D%5D%2C%22includeAssociates%22%3Atrue%7D"
		groups := []api.ZkillEntity{{Type: types.ALLIANCE, ID: 99013330}, {Type: types.ALLIANCE, ID: 99013580}}
		assert.Equal(t, want, api.ZkillTogetherURL(groups, api.ZkillEntity{Type: types.ALLIANCE, ID: 99014851}))
	})

	t.Run("corporations and pilots use their own search fields", func(t *testing.T) {
		url := api.ZkillTogetherURL([]api.ZkillEntity{{Type: types.PILOT, ID: 1}}, api.ZkillEntity{Type: types.CORPORATION, ID: 2})
		assert.Contains(t, url, "%22attackers%22%3A%5B%7B%22type%22%3A%22characterID%22%2C%22id%22%3A1%7D%5D")
		assert.Contains(t, url, "%22neutrals%22%3A%5B%7B%22type%22%3A%22corporationID%22%2C%22id%22%3A2%7D%5D")
	})
}

func TestParseEntityLink(t *testing.T) {
	for _, link := range []struct {
		text       string
		entityType types.EntityType
		id         int
	}{
		{"https://zkillboard.com/alliance/99007237/", types.ALLIANCE, 99007237},
		{"zkillboard.com/character/2121139046", types.PILOT, 2121139046},
		{"  https://www.zkillboard.com/corporation/98744491/losses/  ", types.CORPORATION, 98744491},
		{"https://zkillboard.com/system/30000142/", types.SYSTEM, 30000142},
		{"https://evewho.com/character/2121139046", types.PILOT, 2121139046},
		{"evewho.com/alliance/99013330", types.ALLIANCE, 99013330},
	} {
		entityType, id, ok := api.ParseEntityLink(link.text)
		assert.True(t, ok, link.text)
		assert.Equal(t, link.entityType, entityType, link.text)
		assert.Equal(t, link.id, id, link.text)
	}

	for _, text := range []string{"L A Z E R H A W K S", "Jita", "https://zkillboard.com/kill/123/", "https://evewho.com/system/30000142",
		"https://example.com/alliance/99007237/", "https://zkillboard.com/alliance/notanumber/", "https://zkillboard.com/alliance/"} {
		_, _, ok := api.ParseEntityLink(text)
		assert.False(t, ok, text)
	}
}

func TestDotlanSystemURL(t *testing.T) {
	assert.Equal(t, "https://evemaps.dotlan.net/system/Jita", api.DotlanSystemURL("Jita"))
	assert.Equal(t, "https://evemaps.dotlan.net/system/Old_Man_Star", api.DotlanSystemURL("Old Man Star"), "spaces are underscores")
	assert.Equal(t, "https://evemaps.dotlan.net/system/J105443", api.DotlanSystemURL("J105443"))
}

func TestRelatedURLs(t *testing.T) {
	at := time.Date(2026, 10, 30, 12, 47, 0, 0, time.UTC)
	assert.Equal(t, "https://zkillboard.com/related/30005206/202610301200/", api.ZkillRelatedURL(30005206, at), "minutes are dropped")
	assert.Equal(t, "https://br.evetools.org/related/30005206/202610301200", api.EveToolsRelatedURL(30005206, at))
	assert.Equal(t, "https://warbeacon.net/br/related/30005206/202610301200/", api.WarbeaconRelatedURL(30005206, at))
	local := time.FixedZone("EDT", -4*60*60)
	assert.Equal(t, "https://zkillboard.com/related/30005206/202610301200/", api.ZkillRelatedURL(30005206, at.In(local)), "always in EVE time")
}
