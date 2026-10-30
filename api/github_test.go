package api_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetReleases(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("drops drafts and pre-releases and returns the rest newest first", func(t *testing.T) {
		body := `[
			{"tag_name": "v1.0.0", "name": "EVE GoSleuth 1.0.0", "body": "old", "html_url": "https://github.com/x/releases/tag/v1.0.0", "published_at": "2026-10-20T10:00:00Z"},
			{"tag_name": "v1.2.0-rc1", "published_at": "2026-10-29T10:00:00Z", "prerelease": true},
			{"tag_name": "v1.1.0", "name": "EVE GoSleuth 1.1.0", "body": "new", "html_url": "https://github.com/x/releases/tag/v1.1.0", "published_at": "2026-10-25T10:00:00Z"},
			{"tag_name": "v1.3.0", "published_at": "2026-10-30T10:00:00Z", "draft": true}
		]`
		mock := CreateMockResposne(body, "GET", "", 200)

		releases, err := api.GitHubConnection{Client: mock}.GetReleases()

		assert.NoError(t, err)
		assert.Len(t, releases, 2)
		assert.Equal(t, "v1.1.0", releases[0].TagName)
		assert.Equal(t, "new", releases[0].Body)
		assert.Equal(t, "v1.0.0", releases[1].TagName)
		assert.Equal(t, "repos/lynkfox/eve-gosleuth/releases?per_page=20", mock.Endpoint)
		assert.Equal(t, "GET", mock.Method)
	})

	t.Run("no releases is an empty list", func(t *testing.T) {
		releases, err := api.GitHubConnection{Client: CreateMockResposne(`[]`, "GET", "", 200)}.GetReleases()

		assert.NoError(t, err)
		assert.Empty(t, releases)
	})

	t.Run("an unreadable answer is an error", func(t *testing.T) {
		_, err := api.GitHubConnection{Client: CreateMockResposne(`{"message": "rate limited"}`, "GET", "", 200)}.GetReleases()

		assert.Error(t, err)
	})

	t.Run("the client sends the app's user agent and the GitHub media type", func(t *testing.T) {
		headers := api.NewGitHubClient().Client.(api.APIConnection).Headers

		assert.Equal(t, api.USER_AGENT, headers["User-Agent"])
		assert.Equal(t, "application/vnd.github+json", headers["Accept"])
	})
}
