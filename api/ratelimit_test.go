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

	t.Run("a 429 is not sent again, and holds its group for 15 minutes", func(t *testing.T) {
		reported := []time.Time{}
		comm.SetRateLimitReporter(func(until time.Time) { reported = append(reported, until) })
		defer comm.SetRateLimitReporter(nil)
		connection, calls := guarded(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Ratelimit-Group", r.URL.Query().Get("group"))
			if r.URL.Query().Get("group") == "killmail" {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
			}
		})
		killmail := "killmails/1/0123456789abcdef0123456789abcdef/?group=killmail"

		started := time.Now()
		_, err := connection.Request("GET", killmail, nil)

		assert.ErrorIs(t, err, api.ErrRateLimited)
		assert.Equal(t, int32(1), calls.Load())
		assert.Less(t, time.Since(started), 500*time.Millisecond, "no waiting for the Retry-After")
		if assert.Len(t, reported, 1) {
			assert.WithinDuration(t, started.Add(api.ESI_RATE_LIMIT_HOLD), reported[0], 5*time.Second, "held for 15 minutes, longer than the Retry-After")
		}

		_, err = connection.Request("GET", "killmails/2/0123456789abcdef0123456789abcdef/?group=killmail", nil)
		assert.ErrorIs(t, err, api.ErrRateLimited)
		assert.Equal(t, int32(1), calls.Load(), "the held group's requests are not sent")

		_, err = connection.Request("GET", "characters/1/?group=character", nil)
		assert.NoError(t, err, "other groups carry on")
		assert.Len(t, reported, 1)
	})

	t.Run("a halted guard refuses every request until resumed", func(t *testing.T) {
		connection, calls := guarded(func(w http.ResponseWriter, r *http.Request) {})

		connection.Guard.Halt()
		_, err := connection.Request("GET", "characters/1/", nil)
		assert.ErrorIs(t, err, api.ErrStopped)
		assert.Equal(t, int32(0), calls.Load())

		connection.Guard.Resume()
		_, err = connection.Request("GET", "characters/1/", nil)
		assert.NoError(t, err)
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
