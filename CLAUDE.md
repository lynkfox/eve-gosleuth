# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

An EVE Online intel tool written in Go (module `github.com/lynkfox/go_eve_sleuth`). It pulls a target's killmails from zKillboard, fills in the details from CCP's ESI API, and aggregates them into stats such as most frequent fleetmates, systems, ships, battles, and capital pilots. `Readme.md` is the feature to-do list for pilot, corp/alliance, and system intel.

## Commands

```bash
go build ./...                          # build
go run .                                # run main.go (target is hardcoded in main.go)
go test ./...                           # all tests
go test ./aggregate -run TestName -v    # single test
go vet ./...
```

- Run from the repo root. `main.go` loads `./static_data` and reads/writes `./cached_data.json` relative to the working directory.
- `main.go` is a scratch driver: the target pilot ID/name is hardcoded there.
- `app.go` is leftover Wails GUI scaffold (`App`, `Greet`) and is not wired in yet. `api/esi_gui.go` builds `TargetDisplay` values for that planned GUI.

## Pipeline / architecture

Data moves through the packages in this order (see `main.go`):

1. **`comm`**: global state that must be set up first. `comm.InitLogger(level)` sets the package-level `comm.Logger` (logrus, JSON to stdout). Code everywhere calls `comm.Logger` directly, so tests must init it too. `comm.InitStaticData(path)` loads the JSON ID→name maps in `static_data/` into the globals `CapitalShips`, `ControlTowers`, and `UpwellStructures`. `comm.LogFields` helpers add fields only at Debug level.
2. **`api`**: HTTP clients. `APIConnection` (generic.go) implements `APIClient` with retries and exponential backoff. `ZkillConnection` and `ESIConnection` wrap an `APIClient` and are exposed through the `ZkillClient` / `ESIClient` interfaces. Tests replace the `APIClient` with `MockApiClient` (api/generic_test.go). zKill responses are gzip-decoded.
3. **`retrieval`**: turns raw API data into readable models.
   - `IntelTarget` (target.go) is the subject being investigated. It holds an ID, an `EntityType` (`comm/types`), a `CollectionClientPool` with both clients, and `Expanded` (the readable `*Pilot`/`*Corporation`). `aggregate` type-asserts `Expanded` based on `TypeOfEntity`.
   - `CollectKillmails` fetches the zKill list, then the ESI killmail for each one, producing `[]RawKillmail{Zkillmail, ESIKillmail}`. **There is a temporary cap that returns after index 10.**
   - `CachedEveIDs` (caching.go) is a two-layer cache saved to `cached_data.json`. `Known` holds raw ESI responses (characters, corps, and alliances expire after 15 days via `DateCached`). `Expanded` holds readable models. Use `GetIfNotCached` to read through to ESI. After a JSON reload, cached values are generic maps, so `CastExpanded[T]` converts them back into structs with a JSON round-trip.
   - `RawKillmail.Expand` and `cached.ExpandPilot/Corporation/Alliance/System/Item` build the readable `Killmail` model (killmail_models.go), filling nested corp/alliance data through the cache.
4. **`aggregate`**: `CollateAllKillmailData` walks the killmails in order and fills `CollectedData` (collated_models.go): maps of `PilotSeen`, `SystemVisted`, `ShipFlown`, `BattleFought`, `CapitalPilot`, `AllianceSeen`, and `CorporationSeen`. Consecutive killmails within 15 minutes in the same system are grouped into one battle. Query functions like `GetMostFrequentFleetmates` (pilotsSeen.go) read from `CollectedData`.

## Conventions

- Doc comments on exported functions use a `/* Name ... Parameters: ... Returns: ... */` block style.
- Errors are sentinel values on small custom types (`RequestError`, `TargetError`) wrapped with `fmt.Errorf("%w ...")`. Many call sites still have `//handle error` placeholders.
- Tests live in external `_test` packages (e.g. `package api_test`) and use testify `assert`.
