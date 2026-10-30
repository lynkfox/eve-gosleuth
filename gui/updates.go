package gui

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// NoteSpan is a run of text in a note line, bold or not
type NoteSpan struct {
	Text string `json:"text"`
	Bold bool   `json:"bold,omitempty"`
}

// NoteBlock is one line of a release's notes as the app draws it
type NoteBlock struct {
	Kind  string     `json:"kind"` // "heading" (### New), "para", or "item" (- ...)
	Spans []NoteSpan `json:"spans"`
}

// ReleaseNotes is one release as the popups show it
type ReleaseNotes struct {
	Version   string      `json:"version"`   // "1.1.0", without the v
	Published string      `json:"published"` // "Oct 30th, 2026"
	Tagline   string      `json:"tagline"`   // empty when the release has none
	Blocks    []NoteBlock `json:"blocks"`    // the summary then the changes, the install steps left out
	URL       string      `json:"url"`       // the release page
}

// UpdateStatus is what the launch check and About's button get back
type UpdateStatus struct {
	Current string         `json:"current"`
	Latest  *ReleaseNotes  `json:"latest,omitempty"` // a release newer than this one, nil when up to date
	Notice  string         `json:"notice"`           // "popup", "chip", or "none": how to show Latest
	Unseen  []ReleaseNotes `json:"unseen,omitempty"` // What's New: releases after the last seen one up to this one, newest first; empty unless it should open
}

// the values of UpdateStatus.Notice
const (
	noticePopup = "popup"
	noticeChip  = "chip"
	noticeNone  = "none"
)

// errGitHubUnreachable is what the update check says when GitHub did not answer, the cause going to the log
var errGitHubUnreachable = errors.New("Couldn't reach GitHub to check for updates")

// releaseNotesLimit is how many releases About's What's New link shows
const releaseNotesLimit = 5

// notesEndMarker ends the notes in a release body, the install steps after it stay on the release page only
const notesEndMarker = "<!-- end of notes -->"

// installingHeading starts the install steps in a release body that has no notesEndMarker, as 1.0.0 and 1.0.1 have
const installingHeading = "## Installing"

// versionedRelease is a GitHub release with its version read from the tag
type versionedRelease struct {
	version [3]int
	release api.GitHubRelease
}

/*
	parseVersion reads a version number

Parameters:
  - s: ie "1.2.3" or "v1.2.3"

Returns:
  - [3]int: major, minor, and patch
  - bool: s is such a version, anything else (a dev build's name, a pre-release suffix) is not
*/
func parseVersion(s string) ([3]int, bool) {
	var version [3]int
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return version, false
	}
	for i, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return version, false
			}
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return version, false
		}
		version[i] = number
	}
	return version, true
}

// newerThan is whether version a is after version b
func newerThan(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

var (
	noteLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	noteBoldPattern = regexp.MustCompile(`\*\*(.+?)\*\*`)
	noteHeading     = regexp.MustCompile(`^#{1,6}\s+`)
)

// noteSpans reads a line's inline markdown: **bold** becomes bold spans, links keep only their text, backticks go, anything else is plain
func noteSpans(text string) []NoteSpan {
	text = noteLinkPattern.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "`", "")

	spans := []NoteSpan{}
	add := func(part string, bold bool) {
		if part != "" {
			spans = append(spans, NoteSpan{Text: part, Bold: bold})
		}
	}
	last := 0
	for _, match := range noteBoldPattern.FindAllStringSubmatchIndex(text, -1) {
		add(text[last:match[0]], false)
		add(text[match[2]:match[3]], true)
		last = match[1]
	}
	add(text[last:], false)
	return spans
}

// italicLine returns the text of a line that is all italic (*text* or _text_), not bold
func italicLine(line string) (string, bool) {
	if len(line) < 3 || strings.HasPrefix(line, "**") {
		return "", false
	}
	for _, mark := range []string{"*", "_"} {
		if strings.HasPrefix(line, mark) && strings.HasSuffix(line, mark) {
			return strings.TrimSpace(line[1 : len(line)-1]), true
		}
	}
	return "", false
}

/*
	parseReleaseNotes turns a release's markdown body into what the app draws. The notes end at notesEndMarker, or failing that at the Installing heading.

Parameters:
  - body: the release body from GitHub

Returns:
  - string: the tag line, the italic first line, empty if there is none
  - []NoteBlock: the lines after it, headings, items, and paragraphs, with blank lines dropped
*/
func parseReleaseNotes(body string) (tagline string, blocks []NoteBlock) {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(body, "\n")
	hasMarker := false
	for _, line := range lines {
		if strings.TrimSpace(line) == notesEndMarker {
			hasMarker = true
		}
	}

	blocks = []NoteBlock{}
	first := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if hasMarker && line == notesEndMarker || !hasMarker && strings.HasPrefix(line, installingHeading) {
			break
		}
		if first {
			first = false
			if text, ok := italicLine(line); ok {
				tagline = text
				continue
			}
		}

		kind, text := "para", line
		switch {
		case noteHeading.MatchString(line):
			kind, text = "heading", noteHeading.ReplaceAllString(line, "")
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			kind, text = "item", strings.TrimSpace(line[2:])
		}
		if spans := noteSpans(text); len(spans) > 0 {
			blocks = append(blocks, NoteBlock{Kind: kind, Spans: spans})
		}
	}
	return tagline, blocks
}

// releaseNotesOf is a release as the popups show it
func releaseNotesOf(release api.GitHubRelease) ReleaseNotes {
	tagline, blocks := parseReleaseNotes(release.Body)
	return ReleaseNotes{
		Version:   strings.TrimPrefix(release.TagName, "v"),
		Published: api.FormatShortDate(release.PublishedAt),
		Tagline:   tagline,
		Blocks:    blocks,
		URL:       release.HTMLURL,
	}
}

// versioned keeps the releases whose tag is a version, newest version first
func versioned(releases []api.GitHubRelease) []versionedRelease {
	found := []versionedRelease{}
	for _, release := range releases {
		if version, ok := parseVersion(release.TagName); ok {
			found = append(found, versionedRelease{version: version, release: release})
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return newerThan(found[i].version, found[j].version) })
	return found
}

// fetchReleases asks GitHub for the releases and keeps them for GetReleaseNotes
func (a *App) fetchReleases() ([]versionedRelease, error) {
	if a.github == nil {
		return nil, errGitHubUnreachable
	}
	releases, err := a.github.GetReleases()
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the releases from GitHub")
		return nil, errGitHubUnreachable
	}

	a.releasesMu.Lock()
	defer a.releasesMu.Unlock()
	a.releases, a.releasesFetched = releases, true
	return versioned(releases), nil
}

// knownReleases is the releases of the last check, asking GitHub if there was none yet
func (a *App) knownReleases() ([]versionedRelease, error) {
	a.releasesMu.Lock()
	releases, fetched := a.releases, a.releasesFetched
	a.releasesMu.Unlock()

	if fetched {
		return versioned(releases), nil
	}
	return a.fetchReleases()
}

// appState reads what the app remembers about itself, empty without a cache
func (a *App) appState() retrieval.AppState {
	if a.cache == nil {
		return retrieval.AppState{}
	}
	return a.cache.AppState()
}

// changeAppState changes what the app remembers about itself and saves the cache, which has no meaning without one
func (a *App) changeAppState(change func(*retrieval.AppState)) {
	if a.cache == nil {
		return
	}
	a.appStateMu.Lock()
	defer a.appStateMu.Unlock()

	state := a.cache.AppState()
	change(&state)
	a.cache.SetAppState(state)
	a.saveCache()
}

/*
	CheckForUpdates asks GitHub for the app's releases, once, and works out what to show: the update popup or chip for a newer release, and What's New after an update.
	It takes no a.mu, as it touches only settings, the cache, and GitHub.

Parameters:
  - manual: About's Check for new updates button, which ignores a skip and Later (Notice is always popup when there is a Latest) and never fills Unseen

Returns:
  - UpdateStatus: what to show
  - error: a user readable message when GitHub can't be reached, in which case nothing is marked seen
*/
func (a *App) CheckForUpdates(manual bool) (status UpdateStatus, err error) {
	defer recoverAsError(&err)

	status = UpdateStatus{Current: a.version, Notice: noticeNone}
	releases, err := a.fetchReleases()
	if err != nil {
		return status, err
	}
	current, known := parseVersion(a.version)
	if !known {
		// a build with no version number can not be older or newer than anything
		return status, nil
	}

	if len(releases) > 0 && newerThan(releases[0].version, current) {
		latest := releaseNotesOf(releases[0].release)
		status.Latest = &latest
		status.Notice = a.updateNotice(latest.Version, manual)
	}

	if !manual && !a.newUser {
		status.Unseen = a.unseenReleases(releases, current)
	}
	return status, nil
}

// updateNotice decides how to show the newer release, and notes that the popup told the user of it
func (a *App) updateNotice(latest string, manual bool) string {
	if manual {
		return noticePopup
	}
	if a.currentSettings().SkippedVersion == latest {
		return noticeNone
	}
	if state := a.appState(); state.NotifiedVersion == latest && state.Snoozed {
		return noticeChip
	}
	// told but not answered, as when the app closes with the popup open, it shows again at the next launch
	a.changeAppState(func(state *retrieval.AppState) { state.NotifiedVersion, state.Snoozed = latest, false })
	return noticePopup
}

// unseenReleases are the releases What's New should show: those after the last seen version up to this one, newest first
func (a *App) unseenReleases(releases []versionedRelease, current [3]int) []ReleaseNotes {
	state := a.appState()
	last, lastKnown := parseVersion(state.LastSeenVersion)

	if !a.currentSettings().ShowChangelog {
		if !lastKnown || newerThan(current, last) {
			a.changeAppState(func(state *retrieval.AppState) { state.LastSeenVersion = a.version })
		}
		return nil
	}
	if lastKnown && !newerThan(current, last) {
		return nil
	}

	var unseen []ReleaseNotes
	for _, found := range releases {
		switch {
		case newerThan(found.version, current):
		case !lastKnown && found.version != current:
		case lastKnown && !newerThan(found.version, last):
		default:
			unseen = append(unseen, releaseNotesOf(found.release))
		}
	}
	return unseen
}

/*
	SnoozeUpdate is the update popup's Later: the release shows as the top bar chip from now on, until the user updates or skips it

Parameters:
  - version: the release's version, ie "1.1.0"
*/
func (a *App) SnoozeUpdate(version string) (err error) {
	defer recoverAsError(&err)

	if _, ok := parseVersion(version); !ok {
		return fmt.Errorf("%q is not a version", version)
	}
	a.changeAppState(func(state *retrieval.AppState) { state.NotifiedVersion, state.Snoozed = version, true })
	// Later after About's check of a skipped release takes the skip back, so the chip stays
	if a.currentSettings().SkippedVersion == version {
		if err := a.updateSettings(func(settings *Settings) { settings.SkippedVersion = "" }); err != nil {
			comm.Logger.WithError(err).Warn("Unable to save the settings after a Later took back a skip")
		}
	}
	return nil
}

/*
	SkipUpdate is the update popup's Skip this update: the check shows nothing for this release, though a later one shows again

Parameters:
  - version: the release's version, ie "1.1.0"

Returns:
  - error: a user readable message if the choice could not be saved, in which case it still applies for this session
*/
func (a *App) SkipUpdate(version string) (err error) {
	defer recoverAsError(&err)

	if _, ok := parseVersion(version); !ok {
		return fmt.Errorf("%q is not a version", version)
	}
	a.changeAppState(func(state *retrieval.AppState) { state.Snoozed = false })
	if err := a.updateSettings(func(settings *Settings) { settings.SkippedVersion = version }); err != nil {
		return errors.New("Could not save that choice, the update will show again when the app restarts")
	}
	return nil
}

/*
	MarkChangelogSeen records that What's New was shown up to a version, called as it opens

Parameters:
  - version: the newest version it showed, ie "1.1.0"
*/
func (a *App) MarkChangelogSeen(version string) (err error) {
	defer recoverAsError(&err)

	shown, ok := parseVersion(version)
	if !ok {
		return fmt.Errorf("%q is not a version", version)
	}
	a.changeAppState(func(state *retrieval.AppState) {
		if last, known := parseVersion(state.LastSeenVersion); !known || newerThan(shown, last) {
			state.LastSeenVersion = version
		}
	})
	return nil
}

/*
	GetReleaseNotes returns the notes for About's What's New link: this version and the four before it, newest first

Returns:
  - []ReleaseNotes: the releases
  - error: a user readable message when GitHub can't be reached
*/
func (a *App) GetReleaseNotes() (notes []ReleaseNotes, err error) {
	defer recoverAsError(&err)

	releases, err := a.knownReleases()
	if err != nil {
		return []ReleaseNotes{}, err
	}
	current, known := parseVersion(a.version)

	notes = []ReleaseNotes{}
	for _, found := range releases {
		if known && newerThan(found.version, current) {
			continue
		}
		if len(notes) == releaseNotesLimit {
			break
		}
		notes = append(notes, releaseNotesOf(found.release))
	}
	return notes, nil
}
