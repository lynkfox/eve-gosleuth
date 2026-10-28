# Features

How each feature works, for when you are changing it. The rules that apply everywhere are in `CLAUDE.md`.

## Search bar

- The search bar has no type: `SearchTarget(name, "")` looks the name up with `retrieval.MatchTargets` (ESI universe/ids: exact names ignoring case, plus corps and alliances by ticker, ie Jita Holding Inc. (JITA) for Jita, kept as `ByTicker` since a player may only have seen a ticker; items left out) and searches a lone exact match, or with more than one or any by ticker returns `TargetResult.Choices`, ticker ones last, for `chooseTarget` (main.js, `#target-choice`) to ask which, then is called again with the chosen name and type. A typed lookup (`GetTargetInfoOfType`) prefers the exact name among its type.
- A name filled in by the app, ie a recent search or a menu's Search, carries its type and eve id as `searchTypeHint`/`searchIdHint`, cleared by typing, and `SearchTarget(name, type, id)` then looks it up by id (`GetTargetInfoByID`, `lookupTargetByID`), with no name lookup: the ESI's universe/ids can not find every name, ie the alliance L A Z E R H A W K S. Pass the id wherever a search is offered; recent searches save it (`RecentSearch.ID`) and the holding redirect uses the main's.
- A zKillboard or EveWho link typed into the search bar, My Intel, or the holding dialog is read by `api.ParseEntityLink` and looked up by its id, its own type winning over a dropdown's where that kind is allowed (`lookupEntity`).

## Recent searches

Recent searches live in the cache, not settings (retrieval/recent_searches.go): `SearchTarget` calls `AddRecentSearch` with each found target as EVE spells it, with a corp's or alliance's ticker (`TargetDisplay.Ticker`, also on `RelatedEntity`, shown EVE style, [CORP] and <ALLY>, by `formatTicker`; a typed ticker finds its recent search), one `Permanent` row per search in the `recent_search` section keyed by type and lower case name, keeping the number set in Preferences (`Preferences.MaxRecentSearches`, default `DefaultRecentSearches` 5, at most `MaxRecentSearches` 25; lowering it calls `TrimRecentSearches`); the × on a row calls `RemoveRecentSearch`. An SDE release keeps them; Clear everything and Clear older than remove them.

## Search limits, caveat warning, collapse target

- The search limits button beside the search bar (`#limits-button`, gui/search_limits.go) changes what searches collect (max, pages, newer than, cached, holdings) until the app closes: `SetSearchLimits` keeps them on `App.searchLimits` (under `settingsMu`, so it skips `a.mu`), `searchPreferences()` is the Preferences with them in place and is what `CollectIntel` and `holdingMain` read, and `SaveSearchLimits` makes them the defaults; the button is in the accent while it has its own.
- The Collapse target on search preference (`Preferences.CollapseTarget`) pulls the target panel shut at each search.
- The pull bar under the target panel steps it shut: for a target with holdings saved, the first click puts the holdings away and the next shuts the panel; without any, one click shuts it. A click from shut opens it all (`stepTargetPanel`).
- As the app launches, `showCaveatWarning()` opens the killmail caveat warning (`#caveat-warning`) while `Preferences.ShowCaveatWarning` (on by default) is on; its Don't show this again box calls `DismissCaveatWarning`.

## Stopping a search

`StopSearch` (gui/search_control.go) halts the ESI `RateGuard` (`api.ErrStopped` for every request), so killmail collection ends where it is and the report is built from what is cached; `SearchTarget` and `CollectIntel` resume it when they return, and the report's `stopped`/`rateLimited` flags show a warning above it. A search asks the frontend a question with `ask` (a Wails event) and waits for `AnswerPrompt`.

## Debug mode

Debug mode (`Preferences.DebugMode`, gui/debug.go) lasts until the app closes (`loadSettings` and `shutdown` turn it off) and puts an orange border round the window (`body.debug-mode`, `showDebugMode`): the log goes to Debug level, and `debugRecorder`, a logrus hook, writes each search's entries to `debug/search-<time>.log` beside the settings, from `SearchTarget` (`startDebug`) to the end of `CollectIntel` or a failed `SearchTarget` (`finishDebug`, deferred before `recoverAsError` so panics are in it). It is then zipped with `session.json` (build, OS, WebView user agent, preferences, effective search limits, the search, cache counts, static data overrides with hashes, memory), the newest 256 KB of `crash.log`, and `goroutines.txt` into `gosleuth-debug-<time>.zip`, the newest 10 kept; a log left by a search the app closed during is zipped at the next start. `crash.log` is trimmed to its newest 512 KB past 1 MB. The frontend's JS errors reach the log through `LogFrontendError`.

## Killmail collection (`CollectKillmails`)

- Fetches the zKill list newest first, pages of 200 up to zKill's 100; a page short of 200 ends it, except a nearly full page 1 (`zkillNearlyFullPage`), since zKill holds back the newest killmails, under five minutes old, and the page after it failing is then the end of the list; a killmail listed twice is kept once.
- zKill sends the whole ESI killmail with each entry (`ZkbResponse.Killmail()`), so each killmail is the cache's copy, else zKill's (then cached with `StoreKillmail`), else fetched from the ESI; the ESI is only asked for an entry missing its time, system, victim's ship, or attackers, at most `MaxESIKillmails` (1700) a search, the rest skipped with `ErrTooManyESIKillmails`. Only the entries within Max are looked up in the cache, all of them with `IncludeCached`.
- Paging also stops at the page reaching `NewerThan`; when even the newest killmail is older, `ConfirmIgnoreDate` is asked up front and a yes lists again further back.
- Each entity's list is cached (retrieval/zkill_lists.go, section `zkill_list`, gzipped, `Permanent`): `KnownZkillList` keeps zKill's data for each killmail and how many from the top are `Unbroken` (no gap), and every killmail zKill sends, past the limits too, is cached with `StoreKillmail` as it is listed. `listEntity` stops paging at the first page reaching the cached list's unbroken part (`ZkillPaging.Known`) and splices the rest in; when the search needs more than is unbroken it carries on from there (`ZkillPaging.StartPage`). Entries past the limits that only the cached list has are used only with `IncludeCached`. Late-posted old killmails deep in a list are not picked up by a search that stops early.
- A zKill list can be narrowed with `ZkillPaging.Filter` (modifiers put before the entity, ie `losses/groupID/1657`; an entity with a `groupID` means that entity's own ship or structure is in the group), cached apart by `zkillListKey` with the filter.
- This produces `[]RawKillmail{Zkillmail, ESIKillmail}`, and the names, pilots, corps, and the rest the killmails refer to are looked up in bulk afterwards (`PrefetchKillmailEntities`).
- `KillmailLimits` decides how many: a max count (default `DefaultKillmailLimit`, 20, up to 20,000; zKill is paged only until that many are listed), or `AllKillmails` (-1) for every killmail on the first `Pages` pages (default `DefaultKillmailPages`, 10), an optional cutoff date, and whether to also use cached killmails beyond those limits. The GUI takes these from the Preferences window (`gui/settings.go`); the CLI uses `DefaultKillmailLimits()`.

## Allies

- `GetTopAllies` (aggregate/allies.go) feeds the Allies tab (labelled "Allies?", with a note that they are potential allies, not confirmed) of pilot, corp, and alliance targets: the alliances, then corps in no alliance, on the target's kills (not its losses) outside the alliance it was in at each kill, each noting how many of the target's kills its own pilots were the victim of.
- The user can ignore allies per target from a row's right click menu, saved at once by gui/allies.go `SaveIgnoredAllies` in the cache's `ignored_ally` section (retrieval/ignored_allies.go); the × on an ignored one under the table brings it back. `GetTopAllies` skips them so the next ones take their places. The ignored tags show the kills each shared (`AllyRanking.IgnoredKillmails`), most first.
- With the Hide killed allies preference (`Preferences.HideKilledAllies`) every ally the target also killed is skipped too; `RankAllies` re-ranks the shown search after that preference changes. While it is on and the target killed any (`AllyRanking.KilledAllies`), a toggle tag in the theme's accent leads the left-out tags under the table and re-ranks with `showKilled` to show or hide them for that search only (kept on `alliesSource` until the next search).
- Allies leaves out holdings linked to the target's own groups, and skips `internal` killmails.

## Right click menus

Every row's menu is built by `withMenu` in main.js from sections (`openSectionsMenu`): a heading per entity on the row, ie the pilot, their corp, and alliance, each with Search and site links, so rows send corps and alliances as `EntityRef`s. An Allies row's section has its own items above them: Add to Holding, Ignore, and Open shared killmails on zKillboard (`AllySummary.TogetherURL`, `api.ZkillTogetherURL`: one zKill advanced search, the target and every holding searched in Attackers as or, the ally in Either, so its kills with them and their kills of it). A killmail card's menu has only the part right-clicked (`killmailCardSections`): the victim, the final blow, or else the system.

## Holdings

- Holding links (retrieval/holdings.go, section `holding`, gui/holdings.go) tie a corp or alliance to holding corps or alliances by hand; the target panel shows them from both ends (`TargetResult.Holdings`).
- With the Include holdings preference (`Preferences.IncludeHoldings`, on by default) a corp or alliance search sets `IntelTarget.Holdings` to its own holdings (one level, not the mains it holds for): `CollectKillmails` (`listKillmails`) pulls each holding's zKill list with the same paging and merges them newest first, a killmail on two lists kept once as the main's, before Max and the date limit apply. `RawKillmail.Via`/`Killmail.Via` name the holding a killmail came from, shown as a "via" tag at the foot of its card's victim corp and alliance column, on the date's line.
- Searching a holding, while that preference is on, searches its main instead (`holdingMain` in gui/app.go, the first main by name; `TargetResult.SearchedHolding` names the holding for the status line).
- The Allies tab of a corp or alliance opens with a line of its holdings and the kills each shared with it (`aggregate.HoldingKillsTogether`: kills with pilots of both the main itself and the holding).
- Holdings count as the target everywhere through `IntelTarget.IsGroup(corpID, allianceID)`: losses, members, capital pilots, and allies; the Members tab's roster stays the main corp's.
- A killmail whose victim and every attacker are the target or its holdings (`aggregate.IsInternal`, any outside attacker or NPC makes it a normal loss) has the outcome `internal`: still a loss, never a kill.

## My Intel

- My Intel (app menu, `#myintel`, gui/myintel.go) shows and edits every holding and ignored ally: `GetMyIntel` builds a card (`IntelGroup`) per main and per target with ignored allies (`AllHoldingLinks`, `AllIgnoredAllies`, the targets' names, which ignored ally rows do not keep, looked up in bulk; corp and alliance tickers only from the cache, `CachedTicker`, blank until looked up, so the window opens at once).
- `LookupEntity`, `AddIgnoredAlly`, `RemoveIgnoredAlly`, `SaveHolding`, `RemoveHolding`, and `RemoveMyIntelGroup`/`RemoveMyIntelGroups` (a card's holdings and ignored allies, not links where it is another main's holding; the ticked cards' Delete, or a card's ×, after a confirmation, with an undo) change them without a search and skip `a.mu`; after a change the shown target's holdings and Allies are refreshed (`syncShownTarget`).
- Ticked cards (unticked by default; Check all ticks those the filter shows) export to a JSON file (gui/myintel_share.go, `intelFile`, format `gosleuth-intel` version 1: a `cards` list, each its entity, holdings with notes, and ignored allies, by type and id) for other users.
- An import is read at once, without the ESI, so it is fast and can not trip its rate limits: `planImport` skips malformed entries (counted once per entry), takes the file's names cleaned of control and formatting characters (`cleanImportedName`), and previews what it would add before `ApplyMyIntelImport`, which only adds, checked again against what is saved then, keeping the user's notes unless they take the file's.
- Every entity an import names is then saved to check (`retrieval.AddIntelChecks`, section `intel_check`, kept with the user's data) and `runIntelChecks` (gui/intel_checks.go) checks them in the background with no sign to the user: `universe/names` in batches of 1000 (`GetEntitiesFromEveIDs`, uncached), a batch the ESI 404s for one bad id split in half until it is found, correcting names to EVE's and removing holdings and ignored allies naming an entity that does not exist as the type the file said, with one ticker line only if it removed any. One runs at a time (`checkMu`); a failure keeps the checks; it runs after an import, at start, and first thing in `SearchTarget`, and shutdown stops it between calls (`closing`). Nothing claims a link is verified, as no check can prove one.

## Structures

- Structures (retrieval/structures.go, aggregate/structures.go, gui/structures.go) are pulled with the killmails in `CollectIntel`, before the report, and prefetched with them.
- Structure groups are sections of `structures.json`: `upwell_structure_groups` (citadels, engineering complexes, refineries), `control_tower_groups`, and control tower batteries `pos_battery_groups`, since a tower is never a killmail attacker, only its batteries. Structures with no guns, Metenox moon drills and customs offices, are `minor_structure_groups`: never a last structure lost, never on kills, but listed among previous losses and Destroyed. A killmail's structure is matched to these by its type's group in `fit_types.json` (`StaticValues.ContainsGroupOf`), which is why sdegen keeps control towers and the unpublished customs offices.
- A j-space system target gets `IntelReport.LastStructureLoss` and `PreviousStructureLosses` (`retrieval.StructureLosses`, page 1 of each structure and minor structure group's losses in the system, no limits: the newest non-minor loss, and the `PreviousStructureLosses` (5) newest others), the box above its Activity with the others collapsed under it.
- A corp or alliance target gets the Structures tab (`IntelReport.Structures`): Used in Kills is `CollectStructureKillmails`, each structure group's kills (not towers) and losses, and each battery group's kills, for the target and its holdings, to Max or Pages but ignoring the date limit, then `aggregate.StructuresUsed` groups them by type (towers as one kind), system, and owner corp, as killmails give no structure name or id, grayed by a loss of the same after its newest kill (which also names a tower seen only through batteries). Destroyed is `aggregate.StructuresDestroyed`, the structure and minor structure losses on the search's own killmails that the target's pilots were on.

## Capital Pilots

- `GetCapitalPilots` (aggregate/capitalPilots.go) feeds a corp or alliance target's Capital Pilots tab: the target's non-NPC pilots on its killmails, victim or attacker, flying a capital, by ship type. A capital is a ship in `capital_groups` of `ships.json` (titans, dreadnoughts and lancers, carriers, supercarriers, force auxiliaries, capital industrials, freighters, jump freighters), matched by its type's group in `fit_types.json`, or one of `capital_types` (none built in, for edited copies).

## Fits

The full plan and the decisions behind it are in `.claude/docs/fits-plan.md`.

- **Building a fit.** `RawKillmail.Expand` builds each victim's `Fit` with `BuildFit` (retrieval/fits.go):
  - Each item's killmail flag is placed through `inventory_flags.json` (`comm.FitSlotOf`).
  - Each type is read from `fit_types.json` (`comm.FitTypeOf`). That file is generated from the SDE by `cmd/sdegen`, so type names cost no ESI call. Only victim types missing from it are prefetched from the ESI during the search, and they are listed in `Fit.MissingTypes`.
  - Charges pair with the module on the same flag.
  - Cargo keeps only `cargo_kept` types: charges, scripts, boosters and nanite paste.
  - Items nested under another item are dropped when the killmail is read.
- **Fits tab.** Pilot, corp and alliance searches get `IntelReport.Fits` (`buildFits`, gui/fits.go): the target's losses (holdings included) as `FitGroup`s, by ship name, then newest loss first.
  - Non-combat hulls are left out by category, group, or type (`ships.json` sections `fit_ignored_categories`, `fit_ignored_groups`, and `fit_ignored_types`: capsules, shuttles, corvettes, haulers and transports, freighters, mining and ORE ships, the Zephyr, yachts, structures, deployables, starbases), as is any loss with nothing fitted (`Fit.HasModules`).
  - Identical fits are one group (`fitKey`): the same hull with the same module types in each rack, in any order. Charges, drones and cargo don't count, since they are easily used up or forgotten before the ship dies. For a corp or alliance this merges across pilots.
  - A capsule lost with implants is not a fit, but its implants go to the ship the pilot lost before it in the same battle, the latest if several (`podsWithShips`, with `aggregate.CollectedData.BattleOfKillmail`). A capsule can outlive its ship by a long while, even through a gate. zKill's API doesn't link the two. Implants and boosters are in slot order, from the extract's `head_slot`.
  - The loss carries `PodKillmailID` and `PodURL`, the zKillboard button's right-click menu opens the capsule, and Show fit on the capsule's killmail selects the ship. A fit with such a loss is marked `Implants`, listed first among its hull's fits, and tagged C, as is the loss when shown. Its losses with the most implants come first, so picking a merged fit opens on one of those. Implants don't decide which fits merge.
  - A group with a mutated module is marked `Abyssal`, from the `abyssal` flag `cmd/sdegen` sets on types in the Abyssal meta group, and tagged A.
  - The tags are filled rounded squares, so they don't read as the ×N count: A always red, C in the theme's `--badge` (its accent, or steel blue in the default theme, whose accent is the highlight's gold). The tab's key explains them (`FIT_KEY`). Mutated drones don't count, as drones don't decide the fit.
  - Each loss in a group keeps its own `FitLayout` (slots, with nil for an empty one, and the drone, fighter, booster and implant lists) and its own EFT text (`Fit.EFT`, retrieval/eft.go, mirroring pyfa's export). The EFT text still includes the kept cargo, but the window does not show it.
  - **The list** (`renderFits`, main.js) has a heading for each hull with its losses counted, which folds its fits away. Over 12 fits, every hull starts folded except the selected one's. A row shows when and where the fit was last lost, the pilots who flew it when the list has more than one pilot, and a ×N count for a merged fit.
  - **The fitting window** (`renderFitWindow`) draws the selected fit: the hull's render in the centre, highs over the top, mids down the right, lows along the bottom, and rigs and subsystems inside on the left (`FIT_RACKS`). Each slot is placed with a CSS `--angle`. An empty slot is a hollow socket, and a loaded charge is a badge on its module; hovering a slot names the module and its charge. Arrows beside the ship's name (or ← → anywhere on the tab) page through the fit's losses, wrapping round at either end, each showing its own charges and drones. A fit's losses come most implants first, then newest (`buildFits`), and picking it opens on the first; the one shown is named under the title (pilot · date · system, and the C tag), and "2 of 7" says which. The zKillboard button beside Copy EFT opens it, and its right-click menu, like the name under the title's, also opens the capsule. Rigs and subsystems sit on the render's edge, half over it, spaced wider than the outer ring so a T3 cruiser's keep a gap. Sizes are CSS variables on `.fit-ring` (`--slot`, `--render`, `--outer`, `--inner`, `--far`), with the angle between slots in `FIT_SLOT_STEP`. Outside the ring's line, on a wider arc (`FIT_ARCS`), implants then boosters run down the left from the top left corner, and drones then fighters up the right from the bottom right, each an icon with its count on its corner, named on hover. The hull, each module and charge, and each of those icons has a right-click menu that opens the type on EVE Ref (`typeSection`, `api.EveRefTypeURL`). Copy EFT puts the chosen loss on the clipboard through the Wails runtime.
  - **Show fit**, on a killmail's right-click menu, appears when the killmail is in the Fits list (`hasFit`). It switches to the tab, selects that loss, and scrolls its row into view (`showFit`). A redraw, ie a time zone change, keeps the chosen loss and the folded hulls (`fitsShown`).
- The CLI prints the target's losses as EFT.

## System tabs

- `GetPIPilots` (aggregate/piPilots.go) feeds a system target's PI Pilots tab: hauler losses, T1 and T2 transports but no freighters (`hauler_groups` minus `hauler_exclusions` in `ships.json`) whose zKill `locationID` (`Killmail.LocationID`, the nearest celestial) is one of the system's planets, plus any loss with PI aboard wherever it died (the victims' item types are prefetched with `PrefetchItemTypes` first), with PI cargo found by item group (`pi_material_groups` in `items.json`) from the raw `Victim.Items`.
- `GetRatters` (aggregate/ratters.go) feeds its Ratters tab: any pilot loss with an NPC attacker (no character, in an NPC corp or none, so structures are not rats), each tagged by who else was on it: NPCs only, players using only smartbombs (weapon group in `smartbomb_groups` of `items.json`), or other players; pilots with the surest loss rank first.

## Static data overrides

- The user guide is `docs/static-data-overrides.md`; keep it in step with the files' sections and these rules.
- A json file in the `static_data` folder beside the exe (`staticdata.Overrides()`) is merged over the built-in file of the same name (`comm.loadMerged`, `mergeJSON`), so it need only hold what it adds or changes: objects merge key by key, an id or key set to `null` is removed, and anything else, lists included (`inventory_flags.json`'s slots, a j-space system's statics), is replaced whole. For example `{"capital_groups": {"513": null}}` as `ships.json` stops freighters counting as capitals, and `{"types": {"587": {"group": 26}}}` as `fit_types.json` changes only the Rifter's group.
- A copy that is not one json object (`mergeJSON` rejects `null`, lists, and trailing text), or whose merge its loader rejects (a section of a sectioned file removed, `fit_types.json` with no types, `jspace_statics.json` with no systems or a system given a code it does not list), is skipped with a warning and the built-in file is used. Debug mode's `session.json` lists the copies in the folder with their hashes.
- Preferences > Game data > Export game data (`App.ExportStaticData`, gui/static_data.go) writes every file as loaded, merged and indented (`comm.StaticDataFiles`), to `static_data_export` beside the settings with a `README.txt` saying editing them changes nothing, then removing any other json a previous export left (`os.ReadDir`, as a glob would read brackets in the user's folder names), and opens the folder with the readme selected.

## Other GUI details

- The window itself never scrolls (end of style.css): the header, target panel, and sidebar stay put and each `.tab-panel` scrolls on its own. A paged tab (`.paged-panel`, Killmails and Members) scrolls only its `.panel-scroll` list, keeping its pager in reach.
- The target panel is one row: the target, its related entities beside it, and its holdings at the right end. `api/esi_gui.go` builds its `TargetDisplay` per entity type via `TargetEntity.BuildDisplay`; the two detail panels it fills (`left-details-*`, `right-details-*`) are At a glance's Profile. `api.EntityImageURL` gives EVE image server portraits/logos.
- Themes are defined in `frontend/dist/main.js` (`THEMES`); only colors without a meaning are themed, kill/loss, security, and wormhole class colors stay fixed.

## Sidebar and At a glance

- **Sidebar.** The report's tabs run down its left (`#tab-sidebar`). Only the tabs a search type has are shown.
  - Each tab has an icon, its name, and a count where the list is not a fixed top 10: Killmails, PI Pilots, Ratters, Capital Pilots (each pilot once), Structures, and Fits (`setTabCount`). Counts over 999 are shown in thousands, ie 1.2k or 200k, so they fit the collapsed sidebar's badge; its tooltip has the full count.
  - The icons are `<symbol>`s in one sprite at the top of the report in `index.html`, drawn with `currentColor`. To change an icon, edit its symbol.
  - The panel header above the panels shows the selected tab's name, color key, and `?` help (`selectTab`).
- **Order.** At a glance is always first and can't be moved. The tabs below it (`#tab-list`) can be dragged, or moved with Alt+↑/↓.
  - The order in `index.html` is the default. A saved order (`Settings.TabOrder` via `SetTabOrder`) fills the slots of the tabs it names (`tabOrderFrom`), so a new tab keeps its default place. Preferences can reset it.
- **Collapsing.** The « button shrinks the sidebar to its icons, with counts as badges and names in tooltips.
  - The choice is saved (`Settings.SidebarCollapsed` via `SetSidebarCollapsed`), so a first run starts expanded and later runs open the way it was left.
  - A window under 760 px wide shows icons only whatever was chosen, without changing the saved choice.
- **At a glance** (`renderGlance`) is the tab shown after each search. Across its top is the target's Profile: its details and its history, member corps, or stargates, filled by `renderTarget`. Profile is not a tab of its own. A search that ends without a report (stopped early, or failed) still shows At a glance with the Profile alone (`showProfileOnly`).
  - Below it is a card per tab the search has, each with its top two rows (`GLANCE_ROWS`, `GLANCE_CARDS`) and an Open link to the full tab.
  - The cards keep a set order (`GLANCE_ORDER`, the tabs' default order), whatever order the sidebar is in, and are all the same size.
  - A j-space system also gets a Last Structure Lost card after Activity, opening the Activity tab.
  - PI Pilots and Ratters show about the hour each pilot most often loses a ship (`~19:00 EVE`), in EVE or local time as chosen.
  - A row for a pilot, corp, system, ally, or structure loss has the same right-click menu as its row in the tab.
  - It reads only the report already fetched, so it costs no API calls. It is redrawn when the allies or the time zone change.
  - Members has no card, as its rows load page by page.
  - It could later be the base for the PDF report export on the Readme list.
