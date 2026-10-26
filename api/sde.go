package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const SDE_URL = "https://developers.eveonline.com/static-data/tranquility"

// ErrNoSDERelease is returned when the latest.jsonl response has no sde entry
var ErrNoSDERelease = errors.New("no sde release found")

type SDEClient interface {
	GetLatestSDERelease() (time.Time, error)
}

type SDEConnection struct {
	Client APIClient
}

// sdeRelease is one line of latest.jsonl, the line keyed "sde" describes the current static data export
type sdeRelease struct {
	Key         string    `json:"_key"`
	BuildNumber int       `json:"buildNumber"`
	ReleaseDate time.Time `json:"releaseDate"`
}

// NewSDEClient initizes the connection data for an ApiConnection as an SDEClient interface
func NewSDEClient() *SDEConnection {
	return &SDEConnection{
		Client: APIConnection{
			BaseURL: SDE_URL,
			HTTPClient: &http.Client{
				Timeout: 3 * time.Second,
			},
			Headers:    map[string]string{"User-Agent": USER_AGENT},
			MaxRetries: 2, // checked at startup and before searches, so a failure should be quick, the cache keeps the last known release
			Backoff:    500 * time.Millisecond,
		},
	}
}

/*
	GetLatestSDERelease reads when the current EVE static data export was released. ESI data cached before it is refetched.

Returns:
  - time.Time: the release date of the latest SDE
  - error: any request or decode error, or ErrNoSDERelease if latest.jsonl has no sde line
*/
func (s SDEConnection) GetLatestSDERelease() (time.Time, error) {
	resp, err := s.Client.Request("GET", "latest.jsonl", nil)
	if err != nil {
		return time.Time{}, err
	}

	defer resp.Body.Close()

	lines := bufio.NewScanner(resp.Body)
	for lines.Scan() {
		var release sdeRelease
		if err := json.Unmarshal(lines.Bytes(), &release); err != nil {
			return time.Time{}, fmt.Errorf("could not decode latest.jsonl line: %w", err)
		}
		if release.Key == "sde" {
			return release.ReleaseDate, nil
		}
	}
	if err := lines.Err(); err != nil {
		return time.Time{}, err
	}

	return time.Time{}, ErrNoSDERelease
}
