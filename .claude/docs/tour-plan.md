# New user tour plan

Built 2026-10-30: see features.md, New user tour, for how it works now. This plan is kept for the reasoning behind it.

A guided tour for a first launch: the window darkens except for the one control being explained, a dialogue box in a corner says what it is, and the tour types a search, "runs" it, and pages through the report it builds. An alliance report is the main walk; a short switch to a system report shows how that one differs. Nothing in the tour touches the network, the cache, or the user's settings.

## How it looks

- **Overlay**: one fixed `#tour` layer over the whole window. The highlight is a positioned box the size of the current target plus a few px of padding, with `box-shadow: 0 0 0 100vmax rgba(0,0,0,.65)` doing the darkening, so the hole is the highlight itself and no clip-path math is needed. It moves with a short transition between steps. A transparent click-catcher under it stops clicks reaching the app; a step that wants the user to press the highlighted control lets clicks through the hole.
- **Dialogue box**: fixed in a corner, the corner farthest from the highlight (worked out each step), so it never covers what it describes. Title, a paragraph or two, then a footer: Skip tour at the left, the progress in the middle, Back and Next (Finish on the last step) at the right. Back is hidden on the first step. Themed colors only (`--panel`, `--accent`, ...), as the rest of the chrome.
- **Progress**: a row of dots, one per step, under a `Step 4 of 19` label. A step already viewed is a filled dot, the current one a larger accent dot, the rest hollow. Going Back keeps the later steps filled, as they have been viewed, so the row shows both where the user is and how far they've been. Clicking a filled dot jumps to that step (the steps' `before` run in order to rebuild the screen up to it); a hollow one does nothing. On a narrow window the dots give way to the label alone.
- **Driven by Next**: the tour never moves on by itself or asks the user to click the app. Next plays a step's action (typing the search, the scripted run) and stops on the result; Back undoes it by replaying the steps before it from a clean stage, so every step must be rebuildable from the ones before (no step depends on what the user clicked).
- **Keys**: Right/Enter next, Left back. Esc asks first: the box's body is swapped for "Leave the tour?" with Leave and Keep going (focused); Enter or Leave ends the tour, Esc again or Keep going returns to the step. The Skip tour button needs no confirming, as pressing it is the choice.
- **Placement** is recomputed on window resize and when the target moves (a `ResizeObserver` on the target and `#tab-main`), since the sidebar and target panel animate.

## The script

Steps are data, one object each, so the copy can be edited without touching the engine:

```js
{ id: "search-bar", target: "#search-input", title: "Search", text: "...",
  before: async (tour) => {},   // set the stage: type, render, select a tab; run again when Back or a dot returns past it
  after: (tour) => {},          // optional: tidy what before opened, ie close the limits menu
  quick: true,                  // optional: on a replay (Back, a dot), skip the animation, ie type the name at once
}
```

Back from step N clears the stage and runs steps 1 to N-1's `before` (and `after`) with `quick` set, so a replay is instant and the screen is exactly what a forward walk would have shown.

Proposed order (copy to be written with you; each tab line comes from its `TAB_HELP` so it stays in step):

1. Welcome (no highlight, box centered).
2. Search bar: names, zKillboard and EveWho links, tickers.
3. Recent searches toggle.
4. Search limits button (opens the limits menu, highlights it, closes it after).
5. The tour types the demo alliance's name into the bar letter by letter and highlights Search: "Next runs the search."
6. The "search" runs: busy state, the timer, scripted ticker lines (`Fetching zKillboard page 1`, `Processed killmail ...`, about 3 seconds), the Stop button highlighted while it runs. Next is disabled until it finishes.
7. Target panel: the alliance, its executor, holdings chips; the pull bar.
8. At a glance: the Profile, then the cards (each opens its tab).
9. Sidebar: dragging tabs into another order, the « shrink.
10. One step per tab the alliance report shows, in the sidebar's order: highlight the tab button, select it, then highlight the panel. The busy ones (Killmails, Battles, Fits, Allies?) get a second step on one feature: the kill/loss filter, a battle's span, opening a fit, ignoring an ally.
    One line here notes that a pilot's report swaps Top Pilots and Members for Top Fleetmates; there is no pilot walk.
11. Right-click menus (opens one on a pilot row, closes it).
12. The ? help and color key in the panel header.
13. Generate Report (highlighted, not run).
14. System switch: the bar's text changes to a system name, a short scripted run (1 second), and the system report renders: its Profile (security, stargates, statics for a wormhole), and its different tab set. Two or three steps, no full walk.
15. My Intel, Theme, Preferences buttons, one step for the three.
16. The bear: About, where Take the tour again sits. Finish.

Skip or Finish at any point ends the tour cleanly (below).

## Demo data

The tour renders its reports with the app's own code (`renderTarget`, `renderIntel`), so it can never drift from the real UI. It needs what those take:

- `frontend/dist/tour/alliance.json`: `{ target: <SearchTarget's result>, report: <CollectIntel's IntelReport>, members: [<GetMembers pages>], allies: <RankAllies result> }`
- `frontend/dist/tour/system.json`: `{ target, report }`

These are the backend's JSON exactly, so there is no new format to keep in step, only these files to regenerate when a report gains a field. They are made by a dev tool, `cmd/tourgen`, which runs a real search through the same code as `App.CollectIntel` and writes the JSON:

- **Fictional players.** Every pilot, corp, and alliance is renamed to a made-up name from a word list, with a made-up id, consistently across the file (one map, applied to every name and id field), and its portrait or logo URL cleared or pointed at a bundled placeholder. Ships, items, systems, and structures keep their real ids and names, as they are game data. The tour then shows no real group's intel, and its pictures of ships and systems still load.
- **Trimmed**: at most about 60 killmails, enough to fill every tab, keeping a few battles, a capital, a structure loss, and some fits.
- Links (`zkillUrl`, `evewhoUrl`) in the fictional entries are removed, so nothing in the tour opens a real page about a renamed entity.
- A test in `cmd/tourgen` (or a `go test` over the JSON) checks every player id in the output is one of the fake ones, so a regeneration can't leak a real name.

`tourgen` is a dev tool under `cmd/`, so its config (which alliance and system to search) may sit in Go.

## Keeping the tour harmless

- **A stand-in backend.** `const backend = () => window.go.gui.App;` becomes `() => tourBackend || window.go.gui.App`. While the tour runs, `tourBackend` is an object that answers `SearchTarget`, `CollectIntel`, `GetMembers`, `RankAllies`, `GetHoldings` from the demo files, and turns every other method (saves, removes, exports, `SetTabOrder`, `AddIgnoredAlly`, ...) into a no-op that resolves empty, logging the name at debug. A `Proxy` does the no-op part, so a method added later is harmless by default.
- **Stash and restore.** Starting the tour saves what's on screen: the search box, `intelReport`, `intelTarget`, the target display, the selected tab, the sidebar and target panel states. Ending it clears the demo (`clearIntel`) and re-renders the saved report if there was one, else returns to the empty page. Recent searches are untouched because `SearchTarget` never reached the backend.
- **No real search during the tour**: the search form's submit goes to the tour while `tourBackend` is set, and Search is disabled except at the step that wants it pressed.
- Tab order drags during the tour are undone at the end (`arrangeTabs` with the saved order).

## When it opens

- **First launch**: `App.newUser` already marks a fresh install (see features.md, Updates: "The planned new user tour should use the same flag"). A new bound method `App.IsNewUser() bool` (no `a.mu`, it reads a field set at startup; waits on `waitReady`) tells the page. Launch order becomes: caveat warning, then for a new user a small popup "New to GoSleuth? Take a two-minute tour." with Take the tour / Not now, then the update check. Not now is final; the tour stays in About.
- **Any time**: a Take the tour again button (`#about-tour`, `secondary`) in About's footer, centered between Check for new updates and Report a bug. The footer becomes a three-column grid (`1fr auto 1fr`, Report a bug `justify-self: end`) so the button stays centered while the update status beside Check for new updates grows and shrinks. It closes About and starts the tour. The empty page also gets a line, "New here? Take the tour".
- A search running when the tour is asked for: the About button is disabled while busy (`setBusy`), so the tour never starts over a live search.
- Nothing is saved about the tour: a new user is new only on that first launch.

## Files

| File | Change |
| --- | --- |
| `frontend/dist/tour.js` | new: the engine (overlay, box, placement, keys), the stand-in backend, stash/restore, the scripted run, the steps |
| `frontend/dist/index.html` | `#tour` markup (box, progress dots, Skip/Back/Next), the first-launch popup, `#about-tour` in About's footer, the empty page's line, `<script src="tour.js">` after main.js |
| `frontend/dist/style.css` | a Tour section at the end; `.about-actions` as a three-column grid |
| `frontend/dist/main.js` | `tourBackend` in `backend()`, the launch order hook, wiring the buttons; small exported helpers the tour needs (ie a `renderSearchResult(target, report)` pulled out of `search()` so the tour and a real search share it) |
| `frontend/dist/tour/*.json` | new: demo data, generated |
| `cmd/tourgen/` | new: dev tool + its leak test |
| `gui/app.go` (or `gui/tour.go`) | `IsNewUser`, with a test |
| `.claude/docs/features.md` | a New user tour section; the Updates note about the flag updated |
| `wails.json`, `.github/release-notes/1.2.0.md` | minor bump with notes |

No `static_data` schema changes, so no major bump.

## Agent split

Worth it: the demo data and the overlay engine are independent and each is a fair chunk of work. Two agents on `sonnet`, neither commits.

**Contract first (me)**:
- `tourBackend` hook in `main.js`, `renderSearchResult` pulled out of `search()`, `App.IsNewUser` with its test.
- The demo file shape above, and placeholder `tour/alliance.json` / `tour/system.json` made by saving one real search's JSON by hand (unscrubbed, local only, not committed) so the engine has something to render while `tourgen` is built.
- `tour.js` skeleton: the step object format, `startTour()` / `endTour()` signatures, an empty `TOUR_STEPS` array.
- `#tour` markup in index.html.

**Agent A: demo data** (`cmd/tourgen/`, `frontend/dist/tour/*.json`): the generator, the renaming and trimming, the leak test, then generate both files. Runs live searches.

**Agent B: tour engine** (`frontend/dist/tour.js` engine part, the Tour section of `style.css`): overlay, highlight, dialogue box placement, the progress dots (viewed vs current, jumping back), Skip/Back/Next and keys, replaying steps for Back, the stand-in backend Proxy, stash/restore, the scripted run with ticker lines, and the first ~6 steps as a working example. Codes against the placeholder JSON. Checks its file under node with a stub DOM, as main.js is (memory: frontend load check).

**Me after**: the remaining steps and their copy with you, the first-launch popup and About button, features.md, a Windows build, and walking the whole tour in the app, both themes, with the sidebar shrunk and the window narrow.

## Decided (2026-10-30)

- Made-up players, real ships and systems.
- Driven by Next, with Back; no step waits on a click in the app.
- Alliance walk plus the short system switch; a pilot report gets one line, no walk.
- Progress dots showing steps viewed, Skip tour always on the box, Esc confirms before leaving, Take the tour again centered in About's footer.
