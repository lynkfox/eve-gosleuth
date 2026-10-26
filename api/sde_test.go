package api_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetLatestSDERelease(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("reads the release date from the sde line", func(t *testing.T) {
		mock := CreateMockResposne(`{"_key": "sde", "buildNumber": 3646012, "releaseDate": "2026-10-23T11:09:07Z"}`+"\n", "GET", "", 200)

		release, err := api.SDEConnection{Client: mock}.GetLatestSDERelease()

		assert.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 23, 11, 9, 7, 0, time.UTC), release)
		assert.Equal(t, "latest.jsonl", mock.Endpoint)
	})

	t.Run("finds the sde line among other lines", func(t *testing.T) {
		lines := `{"_key": "other", "releaseDate": "2020-01-01T00:00:00Z"}` + "\n" + `{"_key": "sde", "buildNumber": 1, "releaseDate": "2026-10-23T11:09:07Z"}`
		release, err := api.SDEConnection{Client: CreateMockResposne(lines, "GET", "", 200)}.GetLatestSDERelease()

		assert.NoError(t, err)
		assert.Equal(t, 2026, release.Year())
	})

	t.Run("no sde line is ErrNoSDERelease", func(t *testing.T) {
		_, err := api.SDEConnection{Client: CreateMockResposne(`{"_key": "other"}`, "GET", "", 200)}.GetLatestSDERelease()

		assert.ErrorIs(t, err, api.ErrNoSDERelease)
	})
}
