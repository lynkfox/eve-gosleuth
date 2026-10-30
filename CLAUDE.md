# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

An EVE Online intel tool written in Go (module `github.com/lynkfox/go_eve_sleuth`). It pulls a target's killmails from zKillboard, fills in the details from CCP's ESI API, and aggregates them into stats such as most frequent fleetmates, systems, ships, battles, and capital pilots. `Readme.md` is the feature to-do list for pilot, corp/alliance, and system intel.

**How each feature works (search bar, killmail paging, allies, holdings, My Intel, structures, system tabs, debug mode) is in `.claude/docs/features.md`. Read the relevant section before changing a feature, and update it when behaviour changes.**

**`docs/` is for users** (ie `docs/static-data-overrides.md`, how to edit the game data): keep it in step when a change affects what it describes. Developer notes and plans (`features.md`, `build-pipeline.md`, `fits-plan.md`) live in `.claude/docs/`, never in `docs/`.

## Commands

```bash
wails build -platform windows/amd64 -skipbindings   # GUI -> build/bin/eve-gosleuth.exe (wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0)
go run ./cmd/cli                        # CLI scratch driver (target is hardcoded in cmd/cli/main.go)
go run ./cmd/sdegen                     # regenerate static_data/fit_types.json and dogma.json from the latest SDE (-sde-dir DIR or -sde-zip FILE to use a local copy, -pyfa DIR to check the hull bonus table against Pyfa)
go run ./cmd/tagcheck 139498664 ...     # print the fit and tags of killmails (ids, zKill links, or ESI json) for tuning static_data/fit_tag_rules.json
go test ./...                           # all tests
go test ./aggregate -run TestName -v    # single test
GOOS=windows go vet ./...               # vet including the Wails windows code
```

- The GUI is a Wails v2 app targeting **Windows**, developed in WSL. WSL has no GTK/WebKit, so `wails dev` and Linux GUI builds don't work there; cross-build the Windows exe from WSL (Wails on Windows needs no cgo) and run it from Windows. `-skipbindings` is required, since generating bindings builds and runs a Linux binary.
- The version is `info.productVersion` in `wails.json` (three numbers): bump it there. Pushing a new version to main releases it: `.github/workflows/release.yml` tests, builds the exe, and publishes GitHub release `vX.Y.Z` (0.x as pre-release) with the zipped exe, `fit_types.json`, and `dogma.json`. A version already tagged is skipped; run the workflow by hand from the Actions tab to retry one. `main.go` embeds the file, so `App.Version()`, shown under the top bar's title and in About, follows it, and Wails puts it in the exe's file properties.
- A plain `go build` of the root package without Wails build tags produces a binary that refuses to run; always build the GUI with `wails build`.
- The cache lives at `retrieval.DefaultCachePath()`: `%LocalAppData%\eve-gosleuth\cache.db` (SQLite) on Windows, `~/.cache/eve-gosleuth/` on Linux, unless the user moved it in Preferences (`Settings.CacheDir`).

## GUI

- `main.go` runs Wails, embedding `frontend/dist`. The GUI backend is package `gui`: `App` (gui/app.go) is bound to the frontend, and each exported method is callable from JS as `window.go.gui.App.<Method>()` and returns a Promise. `startup`/`shutdown` are unexported and handed to Wails by `gui.Lifecycle`, since Wails offers every exported method to the frontend.
- Wails calls bound methods concurrently, so every `App` method takes `a.mu` and uses `defer recoverAsError(&err)` so a panic becomes a frontend error instead of crashing the window. Methods that only touch the cache or settings (both have their own locks) skip `a.mu` so they don't wait behind a search, as do `AnswerPrompt` and `StopSearch`, which must reach a search that holds it.
- Start every goroutine with `defer comm.RecoverGoroutine(where, then)`, passing `then` when a caller waits on its result. Panics go to `crash.log` beside the settings with their stack (`comm.RecordPanic`). Everything written to logs or debug files goes through `comm.RedactPaths` (the home folder holds the user name).
- `frontend/dist` is hand-written HTML/CSS/JS, **not build output**: there is no Node/Vite step (`frontend:install`/`frontend:build` are empty in wails.json). Wails injects its runtime scripts into `index.html` automatically. Set text via `textContent`, never `innerHTML`, since names come from ESI.

## Pipeline / architecture

Data moves through the packages in this order (see `cmd/cli/main.go` or `App.CollectIntel`):

1. **`comm`**: global state that must be set up first. `comm.InitLogger(level)` sets the package-level `comm.Logger` (logrus); code everywhere calls it directly, so tests must init it too. `comm.InitStaticData(staticdata.Files)` loads the JSON game data embedded by the `static_data` package; a copy of any file in a `static_data` folder beside the exe (`staticdata.Overrides()`) is merged over the embedded one (`comm.loadMerged`), so it need only hold what it adds, changes, or removes (null). **Game data such as type and group ids belongs in these files, not in Go constants.** Id lists are sections of `ships.json`, `structures.json`, and `items.json` (one `comm.StaticValues` each, a missing section falls back to the embedded file); list groups rather than single types where a group fits, matching a type with `StaticValues.ContainsGroupOf` (its group from `fit_types.json`, so the group's category must be one sdegen keeps). `jspace_statics.json` lists each wormhole type once and each system's statics by code. Dev tools under `cmd/` that the app never runs (ie `cmd/sdegen`'s extraction config) are exempt and may keep ids in Go. Rules for how codes are read (the NPC corp id range, `api.SecurityBand`, which the frontend gets as `securityBand`) stay in code. `dogma.json` (`comm/dogma.go`, loaded after `fit_types.json` and from the same SDE build) holds the attributes, effects, and type dogma the `fitting` engine reads. `comm.LogFields` helpers add fields only at Debug level.
2. **`api`**: HTTP clients. `APIConnection` (generic.go) implements `APIClient` with retries and exponential backoff; `ZkillConnection` and `ESIConnection` wrap it behind the `ZkillClient` / `ESIClient` interfaces. Tests replace the `APIClient` with `MockApiClient` (api/generic_test.go). A 429 from the ESI returns `api.ErrRateLimited`: the `RateGuard` (api/ratelimit.go) does not retry it, holds that rate limit group for at least 15 minutes (`ESI_RATE_LIMIT_HOLD`) so its requests fail at once, and calls `comm.ReportRateLimited` (a GUI popup); killmails that needed the ESI are skipped and the report is built with a warning. Long steps report progress with `comm.ReportProgress`.
3. **`retrieval`**: turns raw API data into readable models.
   - `IntelTarget` (target.go) is the subject being investigated: ID, `EntityType` (`comm/types`), a `CollectionClientPool`, `Expanded` (the readable `*Pilot`/`*Corporation`, type-asserted by `aggregate` on `TypeOfEntity`), and `Holdings`. `IntelTarget.IsGroup(corpID, allianceID)` is how anything asks "is this the target or one of its holdings".
   - `CollectKillmails` pages the zKill list (killmail bodies come from zKill, the ESI only for incomplete ones), caches each entity's list, and produces `[]RawKillmail`; the names they refer to are then looked up in bulk (`PrefetchKillmailEntities`). Fetch all of a report's API data during the search, never lazily per tab.
   - `CachedEveIDs` (caching.go) caches **every** ESI response and is itself an `api.ESIClient` (cached_esi.go): the app passes the cache, never the raw client, as `Clients.ESIClient`. `Known` (raw) and `Expanded` (readable) are keyed by eve ID as `CacheEntry{Value, CachedAt, Permanent}`; `Killmails`, `NameLookups`, and `EntityNames` are separate maps because their keys can clash with eve IDs. Expiry: pilots, corps, and alliances after 24 hours (`volatileTTL`, `ttlFor[T]`); killmails never; everything else when a newer SDE is released (`UpdateSDERelease`). Read with `GetIfNotCached` or `FetchExpanded[T]`; write with `Update`/`UpdateExpanded`.
   - The maps hold only what the session used: a miss in `cachedAs` reads the SQLite row (store.go, `modernc.org/sqlite`, one `cache` table keyed by `section` and key), and every write goes through `put`, marked dirty for `SaveCache`. **Always pass the map's matching `section` to `cachedAs`/`put`.** A value in memory as another type than asked for (`otherType`) is a miss, never converted and overwritten; id lookups check the record's own id (`ownID`); a cached value whose own id is 0 (`missingID`) is refetched once. **Bump `schemaVersion` (store.go) when the table layout changes**; a database with another version is emptied. `LoadCache` always returns a usable cache (a corrupt file is renamed to `.corrupt`).
   - User data sections (`userSections`: ignored allies, holding links, intel checks) survive `ClearAll`/`ClearOlderThan` unless the user ticks "Also clear what you set by hand". Recent searches live in the cache too, not settings.
   - `RawKillmail.Expand` and `cached.ExpandPilot/Corporation/Alliance/System/Item` build the readable `Killmail` model (killmail_models.go) through the cache.
4. **`aggregate`**: `CollateAllKillmailData` walks the killmails in order and fills `CollectedData` (collated_models.go). Killmails each within 15 minutes of the last in a battle are one battle, however long it runs, if they are in one of its systems or share pilots with it (a fight moving through a gate); several battles can be open at once, and a killmail linking two merges them (`FindBattles`). Query functions (`GetMostFrequentFleetmates`, `GetTopAllies`, `StructuresUsed`, `GetPIPilots`, `GetRatters`, ...) read from it. A killmail whose victim and every attacker are the target or its holdings is `internal` (`aggregate.IsInternal`): a loss, never a kill.
5. **`fitting`**: a dogma engine. `fitting.Compute(fit.FittingInput())` works out a fit's resists, EHP, repair, sensors, and ewar resistance from `dogma.json`, with all skills V and every module active but an Assault Damage Control, left online so its brief burst isn't counted (`brieflyActiveEffects`); `gui.fitStats` runs it for each lost fit during the search. It reads the SDE's modifiers generically, and codes by hand only what they can't express (`effects.go`, `rah.go`, `stats.go`). `go test ./fitting` checks it against Pyfa's numbers for the fits in `fitting/testdata/golden` (regenerated by the harness there). See `.claude/docs/fits-plan.md` Phase 4.

## Commit messages

Commits follow [Conventional Commits](https://www.conventionalcommits.org/), so the release notes are built from them: git-cliff (`cliff.toml`) lists the `feat`, `fix`, `perf`, and `revert` commits since the last tag.

- The first line is `<type>(<area>): <summary>`, ie `feat(fits): merge ECM flavors as one module`. The summary is written as before, plain words saying what changed for the user, lower case first.
- Types: `feat` (something new for the user), `fix` (a bug), `docs`, `refactor`, `test`, `perf`, `build` (wails, sdegen output, versions), `ci`, `chore` (tooling, hooks, agents), `revert`. A breaking change adds `!` after the area.
- Areas, optional but used where one fits: `search`, `killmails`, `allies`, `holdings`, `myintel`, `structures`, `capitals`, `systems`, `fits`, `tags`, `cache`, `static-data`, `gui`, `themes`, `about`, `debug`, `sdegen`, `cli`.
- One kind of change per commit where it can be split, so each lands under one heading in the changelog.
- `.githooks/commit-msg` checks the first line. Git uses it once `git config core.hooksPath .githooks` is set in the clone. Until 1.0 it only warns; at 1.0 set its `WARN_ONLY` to 0 so it refuses a message that doesn't follow these rules.

## Conventions

- Doc comments on exported functions use a `/* Name ... Parameters: ... Returns: ... */` block style.
- Errors are sentinel values on small custom types (`RequestError`, `TargetError`) wrapped with `fmt.Errorf("%w ...")`.
- Tests use testify `assert`. They live in external `_test` packages (e.g. `package api_test`), except the gui package's, which are `package gui`, as the App and its helpers they test are unexported.
- `settings.json` (beside the cache, gui/settings.go) holds only user choices: theme, Preferences, tab order. App data such as recent searches goes in the cache.
- Only colors without a meaning are themed (`THEMES` in main.js); kill/loss, security, and wormhole class colors stay fixed.
