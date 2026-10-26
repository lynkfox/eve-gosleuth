package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRateGuard(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	guarded := func(handler http.HandlerFunc) (api.APIConnection, *atomic.Int32) {
		calls := &atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			handler(w, r)
		}))
		t.Cleanup(server.Close)
		return api.APIConnection{BaseURL: server.URL, HTTPClient: server.Client(), Headers: map[string]string{}, MaxRetries: 3, Backoff: time.Millisecond, Guard: api.NewRateGuard(api.ESI_MAX_IN_FLIGHT)}, calls
	}

	t.Run("a 404 is not sent again", func(t *testing.T) {
		connection, calls := guarded(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

		_, err := connection.Request("GET", "characters/1/", nil)

		assert.True(t, api.IsNotFound(err))
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("a 429 waits for its Retry-After, then succeeds", func(t *testing.T) {
		answered := atomic.Int32{}
		connection, calls := guarded(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Ratelimit-Group", "killmail")
			if answered.Add(1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
			}
		})

		started := time.Now()
		_, err := connection.Request("GET", "killmails/1/0123456789abcdef0123456789abcdef/", nil)

		assert.NoError(t, err)
		assert.Equal(t, int32(2), calls.Load())
		assert.GreaterOrEqual(t, time.Since(started), 900*time.Millisecond)
	})

	t.Run("a group low on tokens spaces its requests to the refill rate", func(t *testing.T) {
		// 20 tokens a 2s window, a success costs 2, so one request every 200ms
		connection, _ := guarded(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Ratelimit-Group", "killmail")
			w.Header().Set("X-Ratelimit-Limit", "20/2s")
			w.Header().Set("X-Ratelimit-Remaining", "1")
		})

		started := time.Now()
		for id := range 4 {
			_, err := connection.Request("GET", fmt.Sprintf("killmails/%d/0123456789abcdef0123456789abcdef/", id), nil)
			assert.NoError(t, err)
		}

		// the first request learns the limit, the second starts at once, the last two wait 200ms each
		assert.GreaterOrEqual(t, time.Since(started), 380*time.Millisecond)
	})

	t.Run("a group with tokens to spare is not slowed", func(t *testing.T) {
		connection, _ := guarded(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Ratelimit-Group", "killmail")
			w.Header().Set("X-Ratelimit-Limit", "20/2s")
			w.Header().Set("X-Ratelimit-Remaining", "18")
		})

		started := time.Now()
		for id := range 4 {
			_, _ = connection.Request("GET", fmt.Sprintf("killmails/%d/0123456789abcdef0123456789abcdef/", id), nil)
		}

		assert.Less(t, time.Since(started), 150*time.Millisecond)
	})

	t.Run("nearly using up the error limit pauses every request until it resets", func(t *testing.T) {
		connection, _ := guarded(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Esi-Error-Limit-Remain", "5")
			w.Header().Set("X-Esi-Error-Limit-Reset", "1")
			w.WriteHeader(http.StatusNotFound)
		})

		_, _ = connection.Request("GET", "characters/1/", nil)
		started := time.Now()
		_, _ = connection.Request("GET", "corporations/2/", nil)

		assert.GreaterOrEqual(t, time.Since(started), 900*time.Millisecond)
	})
}
