# Update check and What's New plan

The app tells the user when a newer release is out and takes them to its page, and the first launch of a new version shows what changed. Each version can carry a tag line and a summary, written beside the version bump, that the release page and the app both show. Decided with the user 2026-10-30.

## Decisions

- **The notes live in the repo, one file per version:** `.github/release-notes/X.Y.Z.md`, committed with the bump of `info.productVersion` in `wails.json`. Each version has its own file, so an old summary never reaches a new release.
- **Without a notes file, the workflow warns on a patch bump and fails on a minor or major one.** The bump kind is worked out against the latest `vX.Y.Z` tag.
- **The tag line is a line right under the release title** ("EVE GoSleuth 1.1.0"), not part of the title.
- **The update notice is a popup the first time a version is found.** After **Later** it becomes a chip in the top bar, at this launch and the ones after, until the user updates or skips it. **Skip this update** shows nothing for that version. A later version shows again.
- **About gets a Check for new updates button.** It sits in the footer, all the way to the left, opposite Report a bug. It ignores a skip and always opens the popup when something newer exists, or says the app is up to date.
- **What's New opens at the first launch of a version newer than the last one seen.** It lists every release in between, newest first, so a jump from 1.0.1 to 1.3.0 shows all three. It has a "Don't show this again" box tied to a new `ShowChangelog` preference, also in Preferences, and About links to it.
- **Fresh install: no What's New.** No settings file and no cache file at startup means a new user, who gets the caveat warning instead. (Settings alone wasn't enough: settings.json is only written once a choice is made, so a 1.0.x user who changed nothing has none. Fixed after review 2026-10-30.)
- **No release title change, no ETag.** One call per launch is well under GitHub's 60 an hour without a login.
- **Only the update check.** The app doesn't download or install anything; Download opens the release page (see Not in this plan).

## The notes file and the release page

`.github/release-notes/1.1.0.md`:

```markdown
# Battles in the PDF report, and the app now tells you when it's out of date
Optional summary in markdown: paragraphs, `-` lists, **bold**.
```

- The first line must be `# ` and the tag line (the workflow fails otherwise). The tag line must be 120 characters or fewer, as it fits in a popup line.
- Everything after it is the summary, and it may be empty.

`release.yml` writes the release body in this order:

```markdown
*Battles in the PDF report, and the app now tells you when it's out of date*

Summary...

### New
- ...
### Fixed
- ...

<!-- end of notes -->
## Installing
...
```

- The italic first line is the tag line, which GitHub shows right under the title.
- `<!-- end of notes -->` is invisible on the page and tells the app where the notes stop. The install section stays on the page and is left out of the app.
- Releases from before this (1.0.0, 1.0.1) have no tag line and no marker: the app takes everything above `## Installing`.

### Workflow changes

A new step before "Write the release notes":

1. Find the previous tag (`git tag -l 'v*' --sort=-v:refname | head -1`) and compare it with the new version's numbers.
2. If `.github/release-notes/$version.md` is missing: `::error` and exit for a major or minor bump, `::warning` for a patch.
3. If it exists: check the `# ` first line and the length, then write `NOTES_HEAD.md` (the italic tag line, a blank line, the summary).
4. After git-cliff, put the body together: `NOTES_HEAD.md`, the cliff notes, the marker, the install section.

## Backend

### `api/github.go` (new)

- `GitHubClient` interface with `GetReleases() ([]GitHubRelease, error)`.
- `GitHubConnection`, built on `APIConnection` like `SDEConnection`. It fetches `GET https://api.github.com/repos/lynkfox/eve-gosleuth/releases?per_page=20` with `Accept: application/vnd.github+json`. The URL goes in `urls.go`.
- `GitHubRelease` keeps `tag_name`, `name`, `body`, `html_url`, `published_at`, `draft`, and `prerelease`. Drafts and pre-releases are dropped.
- Tests use `MockApiClient`, as `sde_test.go` does.

### `gui/updates.go` (new)

The contract the frontend codes against:

```go
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
```

Bound methods (none take `a.mu`, as they touch only settings, the cache, and GitHub; each has `defer recoverAsError(&err)`):

- `CheckForUpdates(manual bool) (UpdateStatus, error)`. The page calls it once after loading (`manual` false), and About's button calls it with `manual` true.
  - It makes one call to GitHub.
  - **`Notice`** is `popup` when `Latest` is a version not yet told about, `chip` when it was and Later was pressed, and `none` when it was skipped. With `manual`, it is always `popup` when there is a `Latest`.
  - **`Unseen`** is filled only at launch, and only when `ShowChangelog` is on, the version is newer than the last seen one, and this isn't a fresh install. When the last seen version is unknown (an upgrade from 1.0.x), it holds only this version.
  - **Offline:** it returns an error. The page shows nothing at launch, and the version is not marked as seen.
- `SnoozeUpdate(version string) error` (Later): the chip from now on.
- `SkipUpdate(version string) error`: `SkippedVersion` in settings.json, as the user's choice.
- `MarkChangelogSeen(version string) error`: called when What's New opens.
- `GetReleaseNotes() ([]ReleaseNotes, error)`: About's What's New link, this version and the four before it.

Stored:

- **Settings:** `Settings.SkippedVersion`, and `Preferences.ShowChangelog` (default true, in `defaultSettings`, so older settings files get it on).
- **Cache:** a new section `sectionAppState`, in `keptSections`, which no clear removes, holding `lastSeenVersion` and `notifiedVersion` (plus whether it was snoozed). It is app data, not a choice, so per the rule it lives in the cache.
- **New user:** `startup` sets `App.newUser` when neither `settings.json` nor the cache file existed, checked before `LoadCache` creates it.

Helpers, with table tests in `gui/updates_test.go`:

- `parseVersion("v1.2.3")` and `newerThan`. A dev build ahead of the latest release sees no update.
- `parseReleaseNotes(body) (tagline string, blocks []NoteBlock)`:
  - an italic first line is the tag line
  - it stops at `<!-- end of notes -->`, or at `## Installing`
  - `### ` gives a heading, `- ` an item, other lines a paragraph
  - `**x**` gives bold spans
  - `[text](url)` keeps only the text
  - any other markdown passes through as plain text

  Tests use the real 1.0.0 and 1.0.1 release bodies and a new-style one.

## Frontend

### `index.html`

- `#update-notice` dialog:
  - title: "Version X.Y.Z is out", fixed
  - body: the tag line, then the notes (scrolling)
  - actions: **Download** (opens `url`), **Later**, and a **Skip this update** checkbox
- `#update-chip`: a button in the top bar ("Update X.Y.Z"), hidden by default, that opens the popup.
- `#whats-new` dialog:
  - title: "What's New", fixed
  - body: per release, the version, date, tag line, and blocks (scrolling)
  - footer: a "Don't show this again" checkbox and Close
- About footer: `#about-check-updates` "Check for new updates" on the left, Report a bug on the right. `.window-actions` gets `justify-content: space-between` for About only. Beside the button, a short status: "Checking…", "Up to date", or "Couldn't reach GitHub".
- About gets a "What's New" text link near the version.
- Preferences gets a "Show What's New after an update" checkbox beside the caveat one.

### `main.js`

- At launch, after the caveat warning closes (or straight away when it is off), call `CheckForUpdates(false)`. Then open What's New if `unseen` isn't empty, and once it closes, show the notice as `notice` says. One modal at a time.
- Popup buttons:
  - **Download** opens the release page.
  - **Later** calls `SnoozeUpdate` and shows the chip.
  - **Closing with Skip ticked** calls `SkipUpdate` and hides the chip.
- Notes are drawn from blocks with `textContent` and `<strong>` per bold span, never `innerHTML`.
- `preferences.showChangelog` goes into the preferences object, load, and save.

## Docs

- `.claude/docs/features.md`: a new Updates section (the check, the notice states, What's New, the notes file).
- `.claude/docs/build-pipeline.md`: mark the check and changelog to-do as done, and point to the data update note below.
- CLAUDE.md: the version bump bullet says to write `.github/release-notes/X.Y.Z.md`, and when it is required.
- `.github/release-notes/1.1.0.md`: the first notes file, written with the user.
- Version: 1.1.0, a minor bump with no static data schema change.

## Agents

Worth a split: the backend and frontend are separate file sets that meet only at the contract above.

1. **Contract (me, first).**
   - The types and method stubs in `gui/updates.go`, compiling and returning empty values.
   - The `api.GitHubClient` interface and `GitHubRelease`.
   - The settings fields and `sectionAppState`.
   - The ids of the new HTML elements.
2. **Agent A, backend (sonnet):** `api/github.go` and its test, the bodies of `gui/updates.go`, `gui/updates_test.go`, and the settings default. It must not touch `frontend/`.
3. **Agent B, frontend (sonnet):** `frontend/dist/index.html`, `main.js`, and `style.css`, coding against the contract with a stubbed `window.go.gui.App`. It must run main.js under node with a stub DOM (`--check` alone has missed a load error before). It must not touch Go.
4. **Me, alongside:** `release.yml`, the docs, and the 1.1.0 notes file. Then I check the two agents' work together and fix loose ends, run `go test ./...` and `GOOS=windows go vet ./...`, build the exe, and try it against the live releases (a fake low version to see the popup, chip, skip, and What's New).

Neither agent commits; I commit in pieces (`feat(about)`, `ci`, `docs`) once it all works.

## Not in this plan: data-only updates

`fit_types.json` and `dogma.json` are embedded in the exe, so a user who updates never needs them. They are attached to each release only for a possible later data-only update: when CCP ships an SDE between app releases, the app could fetch the new files without a new exe. To explore later, as the user asked:

- **What it would save.** A stale `fit_types.json` costs only ESI calls for new types. A stale `dogma.json` gives wrong or missing stats for fits with new modules until a new exe. With the update check, a patch release (run sdegen, bump the patch number) already reaches users in one download, so a data-only path saves only that download and the user's click.
- **It must not use the override folder.** That folder is the user's. A full downloaded file there would mix with their edits and hide later embedded fixes. It needs its own layer: embedded, then downloaded, then the user's overrides.
- **Schema.** It should take only files from a release with the same major version (the schema rule in CLAUDE.md), and only when their `sde_build` is newer than the embedded one.
- Until then, an SDE-only change ships as a patch release, and the assets could be dropped from `release.yml` if they are never used.
