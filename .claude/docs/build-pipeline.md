# Build pipeline

Initial design notes for building, releasing, and keeping static data current. Only the static data part is worked out so far. The rest lists the Readme's Release to-dos as placeholders, to be fleshed out later.

## Static data from the SDE

`static_data/fit_types.json` is generated from CCP's static data export (SDE) by `cmd/sdegen` (see `.claude/docs/fits-plan.md`). The file records the SDE build it came from in `sde_build`.

**Decision (2026-10-27): `cmd/sdegen` stays a dev tool and is not bundled with the app.**

- **Download size:** regenerating means downloading the 99 MB SDE zip and reading the 150 MB `types.jsonl`. New SDEs ship with most patches, so every user would repeat that download every few weeks to produce the same 0.63 MB file.
- **What it would save:** nothing that isn't already covered. Item types newer than the extract fall back to the ESI and are cached, so a stale file costs only a few calls, for new hulls and modules.
- **Risk:** if CCP changes the SDE format, a parser on users' machines breaks with no way to fix it in time. As a dev tool, it breaks on the developer's machine, where it can be fixed.

### Steps

1. **Now (manual):** when a patch adds hulls or modules, run `go run ./cmd/sdegen` and commit the new JSON. A user who can't wait for a release can drop the new file into the `static_data` folder beside the exe, which `staticdata.Overrides()` merges over the built-in one.
2. **With CI:** a scheduled GitHub Action reads `https://developers.eveonline.com/static-data/tranquility/latest.jsonl`. When its build number differs from `sde_build` in `fit_types.json`, it runs `cmd/sdegen` and opens a PR. Because the extract is one type per line in id order, that diff is easy to review.
3. **With the auto-updater:** when the app's `sde_build` is behind the latest release's, it downloads only the release's `fit_types.json` (not the SDE or the parser) into the override folder.
4. **Phase 4 (dogma engine):** `cmd/sdegen` also writes `static_data/dogma.json` (attributes, effects with their modifiers, each type's dogma, and the hull bonus skill table, about 1 MB), from the same SDE build. It is generated in CI and shipped as data beside `fit_types.json`, and the auto-updater fetches both.

## To design later

These are the Readme's Release to-dos.

- **Version number in the title bar.** It is currently `comm.VERSION` in `comm/const.go`.
- **A version that can be bumped outside the code.** For example, from a git tag or `wails.json` `info.productVersion`, injected at build time.
- **GitHub Actions to build and release.** It would run `wails build -platform windows/amd64 -skipbindings`, then `go test ./...` and `GOOS=windows go vet ./...`, and attach the exe and `fit_types.json` to a GitHub release.
- **An auto-updater and a changelog splash screen.** Together with step 3 above, these share one release check.
