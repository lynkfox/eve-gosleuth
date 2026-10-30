package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// GitHubClient reads the app's releases, for the update check and What's New
type GitHubClient interface {
	GetReleases() ([]GitHubRelease, error)
}

// GitHubConnection is a GitHubClient over an APIClient, see NewGitHubClient
type GitHubConnection struct {
	Client APIClient
}

// GitHubRelease is one of the app's releases on GitHub, as the releases endpoint gives it
type GitHubRelease struct {
	TagName     string    `json:"tag_name"` // "v1.1.0"
	Name        string    `json:"name"`     // "EVE GoSleuth 1.1.0"
	Body        string    `json:"body"`     // the release notes in markdown
	HTMLURL     string    `json:"html_url"` // the release page
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

// NewGitHubClient initizes the connection data for an ApiConnection as a GitHubClient interface
func NewGitHubClient() *GitHubConnection {
	return &GitHubConnection{
		Client: APIConnection{
			BaseURL: GITHUB_API_URL,
			HTTPClient: &http.Client{
				Timeout: 3 * time.Second,
			},
			Headers:    map[string]string{"User-Agent": USER_AGENT, "Accept": "application/vnd.github+json"},
			MaxRetries: 2, // checked at launch, so a failure should be quick, the app just shows no update
			Backoff:    500 * time.Millisecond,
		},
	}
}

/*
	GetReleases reads the app's latest releases, newest first, leaving out drafts and pre-releases

Returns:
  - []GitHubRelease: up to the 20 newest published releases
  - error: any request or decode error
*/
func (g GitHubConnection) GetReleases() ([]GitHubRelease, error) {
	resp, err := g.Client.Request("GET", "repos/lynkfox/eve-gosleuth/releases?per_page=20", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var all []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&all); err != nil {
		return nil, fmt.Errorf("could not decode the releases: %w", err)
	}

	releases := []GitHubRelease{}
	for _, release := range all {
		if !release.Draft && !release.Prerelease {
			releases = append(releases, release)
		}
	}
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].PublishedAt.After(releases[j].PublishedAt) })
	return releases, nil
}
