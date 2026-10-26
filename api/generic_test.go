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

func TestDotlanSystemURL(t *testing.T) {
	assert.Equal(t, "https://evemaps.dotlan.net/system/Jita", api.DotlanSystemURL("Jita"))
	assert.Equal(t, "https://evemaps.dotlan.net/system/Old_Man_Star", api.DotlanSystemURL("Old Man Star"), "spaces are underscores")
	assert.Equal(t, "https://evemaps.dotlan.net/system/J105443", api.DotlanSystemURL("J105443"))
}
