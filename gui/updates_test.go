package gui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func readNotes(t *testing.T, name string) string {
	data, err := os.ReadFile(filepath.Join("testdata", name))
	assert.NoError(t, err)
	return string(data)
}

func plain(text string) []NoteSpan { return []NoteSpan{{Text: text}} }

func TestParseVersion(t *testing.T) {
	for text, want := range map[string][3]int{"1.2.3": {1, 2, 3}, "v1.2.3": {1, 2, 3}, "v10.0.12": {10, 0, 12}} {
		got, ok := parseVersion(text)
		assert.True(t, ok, text)
		assert.Equal(t, want, got, text)
	}
	for _, text := range []string{"", "dev", "1.2", "1.2.3.4", "v1.2.3-rc1", "1.x.3", "1.-2.3", "+1.2.3", "vv1.2.3"} {
		_, ok := parseVersion(text)
		assert.False(t, ok, text)
	}

	assert.True(t, newerThan([3]int{1, 1, 0}, [3]int{1, 0, 9}))
	assert.True(t, newerThan([3]int{2, 0, 0}, [3]int{1, 9, 9}))
	assert.True(t, newerThan([3]int{1, 0, 10}, [3]int{1, 0, 9}))
	assert.False(t, newerThan([3]int{1, 0, 0}, [3]int{1, 0, 0}))
	assert.False(t, newerThan([3]int{1, 0, 0}, [3]int{1, 0, 1}))
}

func TestParseReleaseNotes(t *testing.T) {
	t.Run("1.0.0 is hand written: its heading and paragraphs stop at Installing", func(t *testing.T) {
		tagline, blocks := parseReleaseNotes(readNotes(t, "release_1.0.0.md"))

		assert.Empty(t, tagline)
		assert.Len(t, blocks, 4)
		assert.Equal(t, NoteBlock{Kind: "heading", Spans: plain("EVE Online GoSleuth")}, blocks[0])
		assert.Equal(t, "para", blocks[1].Kind)
		// **Py**Fa
		assert.Equal(t, []NoteSpan{{Text: "In the grand tradition of eve tools like "}, {Text: "Py", Bold: true}, {Text: "Fa being coded in "}, {Text: "Py", Bold: true}}, blocks[2].Spans[:4])
		assert.Equal(t, NoteBlock{Kind: "para", Spans: plain("Click Here for a Full Feature List")}, blocks[3])
	})

	t.Run("1.0.1 has headings and items with a bold area, and no tag line", func(t *testing.T) {
		tagline, blocks := parseReleaseNotes(readNotes(t, "release_1.0.1.md"))

		assert.Empty(t, tagline)
		assert.Equal(t, []NoteBlock{
			{Kind: "heading", Spans: plain("New")},
			{Kind: "item", Spans: []NoteSpan{{Text: "fits", Bold: true}, {Text: ": fleet doctrines in the report show the implants seen on their losses"}}},
			{Kind: "heading", Spans: plain("Fixed")},
			{Kind: "item", Spans: []NoteSpan{{Text: "fits", Bold: true}, {Text: ": report doctrine tiles no longer cut their text short"}}},
		}, blocks)
	})

	t.Run("a new style body has a tag line, a summary, and stops at the marker", func(t *testing.T) {
		tagline, blocks := parseReleaseNotes(readNotes(t, "release_1.1.0.md"))

		assert.Equal(t, "Battles in the PDF report, and the app now tells you when it's out of date", tagline)
		assert.Len(t, blocks, 6)
		assert.Equal(t, NoteBlock{Kind: "para", Spans: []NoteSpan{{Text: "The report's "}, {Text: "PDF", Bold: true}, {Text: " now lists the latest battles with battle report links."}}}, blocks[0])
		assert.Equal(t, "heading", blocks[1].Kind)
		assert.Equal(t, "item", blocks[2].Kind)
		assert.Equal(t, NoteBlock{Kind: "heading", Spans: plain("Fixed")}, blocks[4])
		assert.Equal(t, "item", blocks[5].Kind)
	})

	t.Run("small cases", func(t *testing.T) {
		for name, test := range map[string]struct {
			body    string
			tagline string
			blocks  []NoteBlock
		}{
			"underscore tag line":           {"_Short_\n\nhello", "Short", []NoteBlock{{Kind: "para", Spans: plain("hello")}}},
			"bold first line is no tagline": {"**Loud**", "", []NoteBlock{{Kind: "para", Spans: []NoteSpan{{Text: "Loud", Bold: true}}}}},
			"star items":                    {"* one\n- two", "", []NoteBlock{{Kind: "item", Spans: plain("one")}, {Kind: "item", Spans: plain("two")}}},
			"## heading":                    {"## Big", "", []NoteBlock{{Kind: "heading", Spans: plain("Big")}}},
			"the marker wins over Installing": {
				"a\n## Installing\nb\n<!-- end of notes -->\nc", "", []NoteBlock{{Kind: "para", Spans: plain("a")}, {Kind: "heading", Spans: plain("Installing")}, {Kind: "para", Spans: plain("b")}},
			},
			"empty": {"", "", []NoteBlock{}},
		} {
			tagline, blocks := parseReleaseNotes(test.body)
			assert.Equal(t, test.tagline, tagline, name)
			assert.Equal(t, test.blocks, blocks, name)
		}
	})
}

// fakeGitHub answers with a fixed list, or an error
type fakeGitHub struct {
	releases []api.GitHubRelease
	err      error
	calls    int
}

func (f *fakeGitHub) GetReleases() ([]api.GitHubRelease, error) {
	f.calls++
	return f.releases, f.err
}

func release(version string) api.GitHubRelease {
	return api.GitHubRelease{TagName: "v" + version, Body: "*Tag " + version + "*\n\n### New\n\n- thing", HTMLURL: "https://example.com/v" + version, PublishedAt: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)}
}

func versionsOf(notes []ReleaseNotes) []string {
	versions := []string{}
	for _, note := range notes {
		versions = append(versions, note.Version)
	}
	return versions
}

func TestUpdates(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	// newUpdateApp is an app at a version, with a real cache and the given releases on GitHub (given in any order)
	newUpdateApp := func(t *testing.T, version string, versions ...string) (*App, *fakeGitHub) {
		cache := retrieval.NewCache(api.NewESIClient(), retrieval.CachePathIn(t.TempDir()))
		t.Cleanup(func() { cache.Close() })
		github := &fakeGitHub{}
		for _, v := range versions {
			github.releases = append(github.releases, release(v))
		}
		return &App{version: version, cache: cache, github: github, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings()}, github
	}

	t.Run("a newer release pops up once, then shows as the chip after Later", func(t *testing.T) {
		app, github := newUpdateApp(t, "1.0.1", "1.1.0", "1.0.1", "1.0.0")

		status, err := app.CheckForUpdates(false)
		assert.NoError(t, err)
		assert.Equal(t, "1.0.1", status.Current)
		assert.Equal(t, noticePopup, status.Notice)
		assert.Equal(t, "1.1.0", status.Latest.Version)
		assert.Equal(t, "Tag 1.1.0", status.Latest.Tagline)
		assert.Equal(t, "Oct 24th, 2026", status.Latest.Published)
		assert.Equal(t, "https://example.com/v1.1.0", status.Latest.URL)
		assert.Equal(t, 1, github.calls)

		// closed without answering: it pops up again
		status, _ = app.CheckForUpdates(false)
		assert.Equal(t, noticePopup, status.Notice)

		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
		status, _ = app.CheckForUpdates(false)
		assert.Equal(t, noticeChip, status.Notice)
		assert.Equal(t, "1.1.0", status.Latest.Version)
	})

	t.Run("a snooze of an older release does not hold back a newer one", func(t *testing.T) {
		app, github := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
		github.releases = append(github.releases, release("1.2.0"))

		status, _ := app.CheckForUpdates(false)
		assert.Equal(t, noticePopup, status.Notice)
		assert.Equal(t, "1.2.0", status.Latest.Version)
	})

	t.Run("a skipped release shows nothing, a later one shows, and a manual check ignores the skip", func(t *testing.T) {
		app, github := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SkipUpdate("1.1.0"))
		assert.Equal(t, "1.1.0", loadSettings(app.settingsPath).SkippedVersion)

		status, _ := app.CheckForUpdates(false)
		assert.Equal(t, noticeNone, status.Notice)

		status, _ = app.CheckForUpdates(true)
		assert.Equal(t, noticePopup, status.Notice)
		assert.Equal(t, "1.1.0", status.Latest.Version)

		github.releases = append(github.releases, release("1.1.1"))
		status, _ = app.CheckForUpdates(false)
		assert.Equal(t, noticePopup, status.Notice)
		assert.Equal(t, "1.1.1", status.Latest.Version)
	})

	t.Run("a manual check ignores Later too", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))

		status, _ := app.CheckForUpdates(true)
		assert.Equal(t, noticePopup, status.Notice)
	})

	t.Run("Later after a manual check of a skipped release takes the skip back", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SkipUpdate("1.1.0"))
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
		assert.Empty(t, loadSettings(app.settingsPath).SkippedVersion)

		status, _ := app.CheckForUpdates(false)
		assert.Equal(t, noticeChip, status.Notice)
	})

	t.Run("Later on one release leaves the skip of another", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SkipUpdate("1.1.0"))
		assert.NoError(t, app.SnoozeUpdate("1.0.9"))
		assert.Equal(t, "1.1.0", app.currentSettings().SkippedVersion)
	})

	t.Run("skipping clears Later", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
		assert.NoError(t, app.SkipUpdate("1.1.0"))

		assert.False(t, app.cache.AppState().Snoozed)
	})

	t.Run("up to date, or a dev build ahead of the newest release, has no update", func(t *testing.T) {
		for _, version := range []string{"1.1.0", "1.2.0"} {
			app, _ := newUpdateApp(t, version, "1.1.0", "1.0.1")
			app.newUser = true

			status, err := app.CheckForUpdates(false)
			assert.NoError(t, err)
			assert.Nil(t, status.Latest, version)
			assert.Equal(t, noticeNone, status.Notice, version)
		}
	})

	t.Run("a build with no version number sees nothing", func(t *testing.T) {
		app, _ := newUpdateApp(t, "", "1.1.0")

		status, err := app.CheckForUpdates(false)
		assert.NoError(t, err)
		assert.Nil(t, status.Latest)
		assert.Empty(t, status.Unseen)
	})

	t.Run("offline is a readable error and marks nothing", func(t *testing.T) {
		app, github := newUpdateApp(t, "1.1.0", "1.1.0")
		github.err = errors.New("dial tcp: no route to host")

		status, err := app.CheckForUpdates(false)
		assert.EqualError(t, err, "Couldn't reach GitHub to check for updates")
		assert.Equal(t, noticeNone, status.Notice)
		assert.Equal(t, retrieval.AppState{}, app.cache.AppState())

		_, err = app.GetReleaseNotes()
		assert.Error(t, err)

		app.github = nil
		_, err = app.CheckForUpdates(false)
		assert.Error(t, err)
	})

	t.Run("What's New lists every release after the last seen one up to this one, newest first", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.3.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0", "1.0.1")
		app.cache.SetAppState(retrieval.AppState{LastSeenVersion: "1.0.1"})

		status, err := app.CheckForUpdates(false)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.3.0", "1.2.0", "1.1.0"}, versionsOf(status.Unseen))
		assert.Equal(t, "1.4.0", status.Latest.Version)

		// not marked until the page says it opened
		status, _ = app.CheckForUpdates(false)
		assert.Len(t, status.Unseen, 3)

		assert.NoError(t, app.MarkChangelogSeen("1.3.0"))
		status, _ = app.CheckForUpdates(false)
		assert.Empty(t, status.Unseen)
	})

	t.Run("an upgrade from 1.0.x, which never saved a last seen version, shows only this version", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.1.0", "1.1.0", "1.0.1", "1.0.0")

		status, _ := app.CheckForUpdates(false)
		assert.Equal(t, []string{"1.1.0"}, versionsOf(status.Unseen))
	})

	t.Run("this version missing from GitHub shows no What's New", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.1.0", "1.0.1")

		status, err := app.CheckForUpdates(false)
		assert.NoError(t, err)
		assert.Empty(t, status.Unseen)
	})

	t.Run("a fresh install and a manual check show no What's New", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.1.0", "1.1.0")
		app.newUser = true
		status, _ := app.CheckForUpdates(false)
		assert.Empty(t, status.Unseen)

		app.newUser = false
		status, _ = app.CheckForUpdates(true)
		assert.Empty(t, status.Unseen)
	})

	t.Run("with What's New turned off it is marked seen without showing", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.1.0", "1.1.0")
		app.settings.ShowChangelog = false
		app.cache.SetAppState(retrieval.AppState{LastSeenVersion: "1.0.1"})

		status, _ := app.CheckForUpdates(false)
		assert.Empty(t, status.Unseen)
		assert.Equal(t, "1.1.0", app.cache.AppState().LastSeenVersion)
	})

	t.Run("marking seen never goes back, and a bad version is refused", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.1.0")
		assert.NoError(t, app.MarkChangelogSeen("1.2.0"))
		assert.NoError(t, app.MarkChangelogSeen("1.1.0"))
		assert.Equal(t, "1.2.0", app.cache.AppState().LastSeenVersion)

		assert.Error(t, app.MarkChangelogSeen("soon"))
		assert.Error(t, app.SnoozeUpdate("soon"))
		assert.Error(t, app.SkipUpdate("soon"))
		assert.Empty(t, app.currentSettings().SkippedVersion)
	})

	t.Run("the app's state is kept in the cache, through every clear", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
		want := retrieval.AppState{NotifiedVersion: "1.1.0", Snoozed: true}
		assert.Equal(t, want, app.cache.AppState())

		app.cache.ClearAll(false)
		app.cache.ClearAll(true)
		app.cache.ClearOlderThan(time.Now().Add(time.Hour), true)
		assert.Equal(t, want, app.cache.AppState())

		// and in the database, not only this session
		assert.NoError(t, retrieval.SaveCache(app.cache))
		reloaded, err := retrieval.LoadCache(api.NewESIClient(), app.cache.CurrentPath())
		assert.NoError(t, err)
		t.Cleanup(func() { reloaded.Close() })
		assert.Equal(t, want, reloaded.AppState())
	})

	t.Run("without a cache the check still works", func(t *testing.T) {
		app, _ := newUpdateApp(t, "1.0.1", "1.1.0")
		app.cache = nil

		status, err := app.CheckForUpdates(false)
		assert.NoError(t, err)
		assert.Equal(t, noticePopup, status.Notice)
		assert.NoError(t, app.SnoozeUpdate("1.1.0"))
	})

	t.Run("release notes are this version and the four before it, from the last check", func(t *testing.T) {
		app, github := newUpdateApp(t, "1.3.0", "1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0", "1.0.1", "1.0.0")

		// no check yet: it asks GitHub, once
		notes, err := app.GetReleaseNotes()
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.3.0", "1.2.0", "1.1.0", "1.0.1", "1.0.0"}, versionsOf(notes))
		_, _ = app.GetReleaseNotes()
		_, _ = app.CheckForUpdates(true)
		assert.Equal(t, 2, github.calls)

		app.version = "dev"
		notes, _ = app.GetReleaseNotes()
		assert.Equal(t, []string{"1.5.0", "1.4.0", "1.3.0", "1.2.0", "1.1.0"}, versionsOf(notes))
	})
}
