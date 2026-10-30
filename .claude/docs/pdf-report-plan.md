# PDF report plan

A briefing a fleet commander can read without GoSleuth: what the target flies, which capitals it can field, and when it is active. It is not a copy of the tabs. Decided with the user 2026-10-29.

## Decisions

- **Audience: FCs.** The first page answers "what will we face, when, and with what". Detail comes after.
- **Corp and alliance targets first.** Pilot and system reports come later, as cut-down versions (phase 4).
- **Section picker.** A dialog of checkboxes before export, with the choice saved.
- **Doctrines are the fits tagged Fleet Doctrine.** The Readme item to tag a kill as a Fleet Doctrine from a battle will feed the same tag, so nothing waits on it.
- **EVE time always**, whatever the time zone preference, as the PDF is passed between people in other zones.
- **Built in Go** with `github.com/go-pdf/fpdf`: pure Go, so it cross-builds to Windows with no cgo, and it embeds TTF fonts for UTF-8 names. WebView printing was ruled out: Wails v2 doesn't expose WebView2's PrintToPdf, and `window.print()` gives the Windows print dialog and the screen's layout.
- **Print colours, not the theme.** White page, dark text, one accent. Kill/loss, security, and wormhole class colours stay as fixed as they are in the app.
- **No killmails, only counts.** No killmail rows, ids, or zKillboard links anywhere; every section reports totals (a fit's losses, a hull's pilots). The structures stay listed one per row, as they are intel in themselves (where the target has citadels and towers), but without links. A "last seen" date is an aggregate, not a killmail, and stays.
- **No API calls at export.** The PDF is built from the report the search already made, images included (see Images).

## Layout (corp or alliance)

Each section is one picker checkbox, except the Summary, which is always included.

1. **Summary** (always, page 1)
   - Header: logo, name and ticker, alliance, member count, and the affiliates (holdings) the search included.
   - Scope: oldest to newest killmail, the count, and any search limit or stop that cut it short (`Stopped`, `RateLimited`, the page or date limit). Then one line of the Readme's caveat: killmails show only deaths.
   - Kills and losses: counts and ISK.
   - **Key findings**: 4 to 6 generated lines, each pointing to its section, ie
     - *Flies 3 doctrines: Ferox (shield, Magnetometric), Guardian (armor logi), Sabre*
     - *14 capital pilots: 2 titans, 5 dreadnoughts, 3 force auxiliaries*
     - *Peak Tue to Thu, 19:00 to 23:00 EVE (USTZ)*
     - *Most active in Delve, 1DQ1-A the most*
2. **Activity**
   - The 7×24 heatmap as the tab draws it, with hourly totals as a bar row beneath, in EVE time.
   - A time zone label from the peak hours (EUTZ / USTZ / AUTZ, or "spread" when no band dominates). New, small: `aggregate.PrimeTimeZone(times)` with the bands' hours in static data or a documented rule in code (it is a reading rule, not game data).
   - Kills and losses per week across the range, as a small bar strip, to show whether activity is rising or falling.
3. **Fleet Doctrines** (the main section)
   - One block per fit with the Fleet Doctrine tag, most losses first: hull render, ship name, the fit's naming tag, losses and pilots, last seen (date and system).
   - Modules summarised by rack, collapsed to counts (ie "3× Heavy Missile Launcher II"), not the slot ring.
   - **Fight stats** (an Assault Damage Control counts as off: its burst is brief on a long cooldown, and FCs know it, so neither its EHP nor a note about it is shown; user, 2026-10-29) from `FitLoss.Stats` (the first loss with stats, or the one with most implants as the fits list chooses): total EHP; tank type (shield / armor / hull, from where the EHP is); the weakest resist as **Shoot: Thermal** (lowest average resonance across the tanked layer); sensor type and strength for ECM; neut pressure and remote repair where non-zero. A tactical destroyer shows its Defense mode only.
   - The Fleet Doctrine reason in one line, as counts ("seen with 7 Ferox and 2 logi on one side").
   - Grouped by hull class, largest first (Capitals, Battleships, ... Frigates), a divider at each, alphabetical by hull within one (user, 2026-10-29). Other notable hulls was built and then dropped the same day at the user's request.
4. **Capital Pilots**
   - Count line at the top by class, then a table per class in this order: Titans, Supercarriers, Dreadnoughts and Lancers, Force Auxiliaries, Carriers, then capital industrials, freighters, and jump freighters last.
   - Each row: pilot, current corp (and alliance if not the target's), last seen flying that hull (date, system), and a mark for pilots who have left (`CapitalPilot.Left`).
   - Needs `LastSeen` (time, system) added to `CapitalPilot` from `aggregate.GetCapitalPilots`; the class comes from the ship's group, its order from a list in `ships.json` (`capital_groups` order, or a new `capital_report_order` section).
5. **Where they live**
   - Top systems (security / wormhole class coloured, region), top regions rolled up from them.
   - Structures: Used in Kills and Lost as short tables, each structure with its type, system (and moon for a tower), and owner, as the tab lists them, without killmail links.
6. **Who they fly with**
   - Top allies, headed "potential allies, not confirmed" as the tab is.
   - Top members.
7. **Appendix: EFT** of each doctrine fit, monospaced, so readers can paste them into Pyfa. Built 2026-10-29 with phase 2: each doctrine links to its EFT and back, and the PDF has an outline.

Every page: footer with the target, "GoSleuth vX.Y.Z", the generation date (spoken format, "Oct 29th, 2026"), and page N of M. Nothing names the user.

## How it works

- **Keep the report.** `App` keeps the last finished `*IntelReport` and its target (`lastReport`, guarded by `mu`), set at the end of `CollectIntel`, cleared on a new search. A stopped or rate limited report can still be exported; its Summary says so.
- **Bound method** `ExportPDF(sections []string) (path string, err error)`: `runtime.SaveFileDialog` (as myintel_share.go does), default name `<ticker> intel <date>.pdf`, then builds and writes. Returns "" if the user cancels.
- **Package `report`** (new), no Wails imports, so it is testable and the CLI could use it:
  - `report.Build(in Input, sections []Section) (*fpdf.Fpdf, error)`, where `Input` is a plain struct the gui fills from `IntelReport` (keeps `report` free of the gui package).
  - One file per section (`summary.go`, `activity.go`, `doctrines.go`, `capitals.go`, `space.go`, `allies.go`, `appendix.go`), plus `layout.go` (page, fonts, colours, table and heading helpers) and `findings.go` (Key findings lines).
  - The heatmap and bar strips are drawn as filled rectangles; no chart library.
- **Fonts** (decided 2026-10-29): Noto Sans Regular and Bold, and Noto Sans Mono Regular for the EFT appendix, static TTFs embedded with `//go:embed` (about 1.5 MB on the exe). fpdf embeds only the glyphs a PDF uses, so the PDF stays small. Chosen for its likeness to the app's Segoe UI and its tabular digits (the report is mostly counts and percentages). EVE names are plain ASCII, so coverage is no concern. The fonts are under the SIL Open Font License: ship `OFL.txt` beside them and name them in About's credits. Rejected: Inter (fpdf can't turn on its tabular figures), Segoe UI read from `C:\Windows\Fonts` (needs a fallback for Linux), the PDF core fonts (cp1252 only, plain).
- **Section picker:** the Export PDF button (top bar, shown when a report is) opens a small dialog of checkboxes, Summary fixed on, with Export and Cancel. Only sections the report has data for are enabled. The ticked set is a user choice, so it goes in `Settings.PDFSections`.
- **Images** (decided 2026-10-29: fetch them in full): the target's logo, each Fleet Doctrine hull's render, and each capital class's hull icon come from images.evetech.net, the CDN behind the frontend's own images, which is not under the ESI's rate limits. They are fetched during the search, as every report's API data is, once `buildFits` and `GetCapitalPilots` have said which hulls are needed: a few dozen at most, in parallel (a small worker pool, the app's User-Agent), through `APIConnection` for its retries but not the ESI's `RateGuard`. Each is cached permanently in the SQLite cache (a new `image` section keyed by URL; renders only change with a new SDE, so `UpdateSDERelease` may expire them), so a repeat search fetches nothing. A failed image is logged and drawn as a plain box with the hull's name; it never fails the search or the export.
  - Sizes (measured 2026-10-29): render at 256 px about 14 KB (JPEG; 512 px is 41 KB, more than print at block size needs), icon at 64 px about 7 KB, logo at 128 px about 7 to 9 KB. A report needs about 150 to 250 KB the first time. Hulls are shared across targets and there are only a few hundred, so they level off under about 7 MB however much is searched; only logos grow, about 10 KB per new target (1000 targets, 10 MB). Small next to the killmail cache (the user's was 45 MB that day).

## Phases

1. **Skeleton** (built 2026-10-29, not yet tried with a real search). `report` package with layout helpers, fonts, footer; `lastReport`; `ExportPDF` with the save dialog; Summary page only (header, scope, kill/loss counts, no findings yet). Tests: builds a PDF from a fixture `Input` without error; page count; text extraction not needed.
2. **The FC sections** (built 2026-10-29 by five sonnet agents behind the `Input` contract, not yet tried with a real search; see features.md for the rules chosen). Activity (with `PrimeTimeZone`), Fleet Doctrines (with fight stats and other hulls), Capital Pilots (with `LastSeen`), then Key findings on the Summary from them. Images and their cache section.
3. **The rest and the picker.** Where they live, Who they fly with, EFT appendix, the section dialog and `Settings.PDFSections`. Update `features.md` and the user docs.
4. **Later: pilot and system.** A pilot: activity, ships flown and fits, fleetmates, systems. A system: activity, top pilots and corps, structures lost, ratters, PI pilots.

Agent split: not worth it for phase 1. In phase 2 the three sections are independent files behind the `Input` contract, so they could go to three sonnet agents once `layout.go` and `Input` exist; phase 3 is small enough to do directly.

## Open

- Whether the Key findings lines need a threshold before they appear (ie no "capital pilots" line for 1 freighter pilot).
