# War tracker and group search

Plan of 2026-11-04, decisions from the user the same night. Stage 1 under way.

The model is the user's hand-kept 2024 Coalition vs Lazerhawks sheet (id `11i7aE7hB7ES0Q-mtVmYRaO8dBj6DX7B6IL6GGNQQLP8`): gid 722070133 is one row per fight, gid 1013484469 the totals by side and day.

## Stages

| Stage | What ships |
| --- | --- |
| 1 | Group search from the search bar |
| 2 | War core: setup, saved wars, collection, classification, Overview, Battles, Systems, Killmails |
| 3 | Station trash and asset denial, and the Timeline with every category |
| 4 | Notes and flags per battle, merging and splitting battles |
| 5 | Pilots and Ships tabs |
| 6 | Last: data export, then the PDF report |

Each stage is usable when it lands. **After these, in a later version:** membership changes during a war, importing zKillboard killmail dumps, who joined in, fleet doctrines, the tour, right-click "Add to a war".

## Settled rules

### Sides

A side is one of: a coalition, a main with its affiliates, or a single corp or alliance. Pilots and systems are not allowed. Sides are saved as references and resolved at each run, so a change to a coalition carries into its wars. A side that no longer exists stops the run with a message naming it. An entity listed directly on both sides is refused at setup.

Everything works on the corp and alliance **at the time of the kill**. A side listing the corp wins over a side listing its alliance, as the more specific.

**Later version, nothing shown to the user for now:** membership that changes during a war, ie "Noob Corp Switches Sides!!!" in the sheet. It will be optional from and until dates on a side's member. Keep `WarSide` easy to extend for it, and build no UI.

### What counts

**A battle counts when anyone of both sides appears on its killmails, as victim or attacker.** Every killmail in such a battle counts in the totals, including those of third parties. A battle without anyone of one side is forgotten, whatever else happened in it.

- **Battles** are `FindBattles`' rules unchanged.
- **Battles and skirmishes:** a counted fight of `battleMinPilots` (10) or more pilots is a battle, shown by default. Anything smaller is a **skirmish**, behind a toggle, including a lone killmail of one side killing the other, which counts in the totals. The Overview counts both, ie "41 battles, 96 skirmishes".
- **A killmail outside any counted battle** with no one from the other side does not count, except asset denial (Stage 3).
- **Third parties:** a third party group in a counted battle counts for the side it fought beside: per battle, the opposite of whichever side it shot at or was shot by more. Even or unknown, it shows under Others, and does not count for a side. The group is the alliance, or the corp if it has none.
- **Adding Others to a side:** the user can put a group under Others onto a side from the report. Adding it asks whether it counts **from the beginning of the war**, the default, or **from that point onward** (user, 2026-11-04). It is saved on the war with that date, and its killmails before the date stay under Others. The date is on the added group only; dates on a side's own members wait for the later membership feature. Stage 2.
- **No winners.** The report shows each side's ISK and ships, and never calls a battle won. Efficiency is shown only for the whole war and per day.
- The report keeps the caveat that killmails show neither objectives nor survivors.

**Known limit, accepted by the user:** the lists read are the named groups' only, so a killmail with no named group on it is never seen. Others still show when they die to a named group or kill one. Adding a group to a side brings its own list in at the next full run.

### Reach

Past wars are not chased. A run reads what zKillboard's lists reach within the 100 page cap, plus killmails already in the cache. A list stopped short of the start date is named in the report's reach warning with the date it got to. More history waits for importing zKillboard killmail dumps.

Saved wars soften the cap, see Saved wars.

### Measures

The categories are the sheet's, per side:

- **ISK lost:** total, then fights, station trash and asset denial, structures. These three add up to the total.
- **Ships lost:** total, then in fights, station trash and asset denial.
- **Structures on the report: All Citadels and All POS.** Citadels are every `upwell_structure_groups` type. POS means `control_tower_groups`. Hovering or opening the figure gives the count per structure type, POS as one figure. POCOs count under POS, as in the sheet.
- **Who fought:** each battle names each side's largest group in it, by pilots.
- **Systems:** the count, then by class as a separate figure, ie "25 systems: 13 C5, 12 C4", from `comm.WormholeClass`, and `api.SecurityBand` outside wormhole space.

### Saved wars and reports

Wars are saved, and so is each report: it opens again across sessions with no network, and a run adds to it.

- **New user sections:** `war` for the definition, notes, and flags; `war_killmail` for the killmails a war holds, one row each, gzipped like `killmails`. Both join `userSections`, so a cache clear leaves them unless the user ticks "Also clear what you set by hand". They are separate from the `killmail` section because Clear everything empties that.
- **A war keeps the killmails it counted.** Once held, a killmail stays held even after zKillboard's cap has slid past it.
- **Update** lists only from the last run's newest killmail, less a day's overlap, so a tracked war rarely meets the cap. A change of sides or start date does a full run.
- Names are looked up through the usual cache.

## Stage 1: group search

The search bar's dropdown (`#recent-list`) gains **Coalitions** and **Affiliate groups** sections under Recent, filtered as typed. Picking one searches it by type and id, through the same path as `pickRecent`. These rows have no ×.

New bound method `GetSearchGroups() []SearchGroup` (`{name, type, id, ticker, count}`), from `Coalitions()` and `AllHoldingLinks()`, cache only and without `a.mu`, so it answers during a search.

## Stage 2: war core

### Setup

A **War** button in the search form, between the limits button and Search, opens `#war-setup`:

```
┌ WAR ─────────────────────────────────────────────── × ┐
│ Saved wars                                            │
│   Coalition vs Hawks    Mar 25th, 2024 → [Open][Update][×]
│ ───────────────────────────────────────────────────── │
│ Side A [ coalition, affiliate group, or name      ▾ ] │
│ Side B [ coalition, affiliate group, or name      ▾ ] │
│ Name   [ filled in as A vs B, editable              ] │
│ From   [ Oct 1st, 2026 ▾ ]  W  30  90  ∞              │
│ Until  [ Still going    ▾ ]                           │
│ ⚠ large search warning, overlap errors                │
│                                       [Save and run]  │
└───────────────────────────────────────────────────────┘
```

The side boxes use the Stage 1 list plus any corp or alliance typed or linked (`lookupEntity`). From reuses `.age-picker` and `wireAgePicker`; Until is the same date field without buttons. A run joins recent searches as type `WAR`, and a typed name matching a saved war joins the search's choices.

### Collection

1. **List:** `CollectKillmails` on a stand-in `COALITION` target whose `Holdings` are both sides' groups. `Max: AllKillmails, Pages: AllPages, IncludeCached: true`, the start as `NewerThan`, no `ConfirmIgnoreDate`. It ignores the search limits button. Killmails after Until are dropped.
2. **Merge** with the killmails the war already holds.
3. **Classify on raw killmails:** `ESIKillmail` has the ids needed. `FindBattles` is split so its core takes `{id, time, system, pilots}`, fed by raw or expanded killmails. Two large sides list many killmails that have nothing to do with the war, and this drops them before any ESI lookup.
4. **Keep** what counts, then `PrefetchKillmailEntities` on it and save it to `war_killmail`.
5. **Build** the report. `OnListed func(group, oldest, complete)` on `KillmailLimits` supplies the reach warning.

### Report

The target panel shows the two sides as a versus card and the sidebar only the war tabs, hidden on other reports. Side colors are fixed, blue and orange, since red and green mean loss and kill.

- **Overview:** the Measures per side, the totals by side as in the sheet's top rows, battles and skirmishes counted, the reach warning, the caveat.
- **Battles:** one row per fight as in the sheet: time, duration, system, class, each side's named group, ISK and ships lost per side, Others, structures, and the battle report links. Battles of `battleMinPilots` or more first, the rest behind the skirmishes toggle.
- **Systems:** the count by class, then each system's killmails and ISK per side.
- **Killmails:** the existing cards and paging, filtered by side.

## Stage 3: trash and asset denial

These are estimates from killmails and are labelled so, as the user sorted them by hand in the sheet.

Station trash is looked for after a structure dies, in its system, for `trashWindow` (3 hours):

1. **Repeat victims:** a pilot lost in that system at least twice less than 5 minutes apart after the structure died. All that pilot's losses in the window are trash.
2. **Battles are found again without those.** Otherwise a stream of trash kills keeps the battle open for hours under its 15 minute rule, and it never "ends".
3. **After the battle:** any later killmail in that system in the window is trash, when its victim is on the structure owner's side or on neither side. A fleet returning for revenge keeps its real losses out of trash that way.

Trash stays attached to the structure's battle, in its own columns, and doesn't count towards that battle's fight ISK or duration.

**Asset denial:** a side destroying its own ships in its station before the station dies, ie one High Bear pilot killing another's ship (user, 2026-11-04). A killmail in the war's dates with victim and all attackers on one side, in a system where that side later loses a structure, before that loss. A structure need not be among the attackers. These may fall outside any counted battle, the one exception to that rule, and join trash in one bucket, as in the sheet. An update can turn earlier same-side killmails into asset denial once the structure falls, so the war holds same-side killmails too. Every other same-side killmail is left out, as with `IsInternal`.

**Station being reffed:** a fight where one side's losses include Standup Fighters (user, 2026-11-04) is probably that side's station being reinforced, as only structures launch them. The battle gets an automatic **Station ref** mark on its row, naming that side, and the Overview counts them. Standup Fighters never count as ships lost, only in ISK lost, under structures (user, 2026-11-04). They are their own groups, 4777, 4778, and 4779 (`category` 87 in `fit_types.json`), apart from carrier fighters' 1537, 1652, and 1653. They go in a new `standup_fighter_groups` section of `structures.json`, read with `ContainsGroupOf`, so a minor bump at most. Fights are found on killmails, so a station reffed with none of its fighters dying is not marked. In Stage 4 the mark starts the For Timer flag ticked, and the user can clear it.

**Timeline:** the sheet's tab: for each day and each side, every category as running totals and as that day's own figures, with a chart over them drawn like the Activity one.

## Stage 4: notes, flags, and battle edits

Added after a report is generated, from a battle's row, and saved with the war.

- **Flags, the sheet's:** One Fight, Not war, 3rd Party, For Timer, Timer defended, Defence succeeded. Plus a free-text note.
- **Flags change no totals**, as in the sheet, which counts flagged rows too.
- **Merge or split battles** by hand. An edit is saved as the killmail ids it moved, and applied after `FindBattles` at each build.
- **Keeping them through updates:** a battle's id comes from its first killmail, which a later run can change. Notes and flags are saved with the battle's killmail ids, and go to whichever battle now holds most of those.

## Stage 5: Pilots and Ships

**Pilots:** top pilots per side, by kills and ISK destroyed. **Ships:** hulls lost and flown per side, trash left out.

## Stage 6: export and PDF

A JSON export of the report's data, as the sheet offered. Then the war report in the PDF, built like the existing one.

## Contract for Stage 2

```go
// comm/types: WAR added last in EntityType

// retrieval/wars.go
type WarSide struct { Type string; ID int; Name string } // COALITION, CORPORATION, or ALLIANCE
type WarAddedGroup struct { Type string; ID int; Name string; FromDate string } // FromDate YYYY-MM-DD, empty for from the beginning of the war
type War struct {
    ID           int    // negative, local to this cache, as a coalition's
    Name         string
    SideA, SideB WarSide
    FromDays     int    // as Preferences.KillmailsNewerThanDays
    FromDate     string // YYYY-MM-DD, used in place of FromDays
    UntilDate    string // YYYY-MM-DD, empty while it goes on
    AddedA, AddedB []WarAddedGroup // Others put on a side by hand
    LastRun      time.Time
    SetAt        time.Time
}
func (c *CachedEveIDs) Wars() []War
func (c *CachedEveIDs) WarByID(id int) (War, bool)
func (c *CachedEveIDs) WarNamed(name string) (War, bool)
func (c *CachedEveIDs) SaveWar(war War) War
func (c *CachedEveIDs) RemoveWar(id int)           // and its killmails
func (c *CachedEveIDs) WarKillmails(id int) []RawKillmail
func (c *CachedEveIDs) AddWarKillmails(id int, killmails []RawKillmail)

// aggregate/war.go
func ClassifyRawWar(raw []retrieval.RawKillmail, a, b *retrieval.IntelTarget) (keep []retrieval.RawKillmail)
func BuildWar(killmails []*retrieval.Killmail, a, b *retrieval.IntelTarget, minPilots int) *WarData

// gui/war.go, bound
func (a *App) GetSearchGroups() ([]SearchGroup, error) // Stage 1
func (a *App) GetWars() ([]WarCard, error)
func (a *App) SaveWar(card WarCard) (WarCard, error)   // checks the sides, overlap, dates
func (a *App) RemoveWar(id int) error
func (a *App) OpenWar(id int) (WarReport, error)       // from what it holds, no network beyond name lookups
func (a *App) CollectWar(id int) (WarReport, error)    // takes a.mu, stops and reports progress as CollectIntel does
```

`WarReport` is its own struct, not more `IntelReport` fields. Its fields are settled when the contract is written, with room for Stage 3's categories.

## Tests

- **Counting:** both sides in a battle with no direct killmail, one side only forgotten, lone direct killmail, corp beats alliance, a corp moved between alliances, NPCs only, structures with no pilot.
- **Third parties:** beside A, beside B, even, and one added to a side by hand, from the beginning and from a date.
- **`FindBattles` split:** existing tests pass untouched; raw and expanded input give the same battles.
- **Saved wars:** round trip, a held killmail surviving a cache clear and the list no longer reaching it, update reading only from the last run, a side change doing a full run.
- **Station ref:** a Standup Fighter loss marks the battle for that side, a carrier fighter's does not, and it adds to ISK lost but not ships lost.
- **Trash:** repeat victim, a battle ending once they are removed, the 3 hour edge, a returning fleet not counted, a neutral victim counted, asset denial before the loss but not after it or with no loss. Stage 3's rules should also be compared by eye with a real structure fight.
- **Report totals** by hand on a small fixture, the three ISK buckets adding to the total.
- **Frontend:** load check, and a stand-in report in the Edge harness.

## Build order and agents

Stage 1 is mine, as it is small and Stage 2 needs its list. For Stage 2, I write the contract: types, compiling stubs, the `FindBattles` split, `OnListed`, and `WarReport`. Then three sonnet agents, one file set each:

- **A, retrieval and aggregate:** `retrieval/wars.go`, `aggregate/war.go`, their tests.
- **B, gui:** `gui/war.go` and its tests, against A's stubs.
- **C, frontend:** the dialog, versus card, war tabs, and rendering, against a stand-in `WarReport`.

I join them and run it on two real sides, comparing a battle with zKillboard's own figures. Stages 3 to 6 are small enough to do without agents.

## Also to change

- `features.md` gets War and search group sections, `docs/features.md` the user's version.
- `war` joins the commit areas in CLAUDE.md and, if the hook lists them, `.githooks/commit-msg`.
- No `schemaVersion` bump: new sections don't change the table layout.
- Game data stays in `static_data`: `upwell_structure_groups` and `control_tower_groups` exist; POCOs and capsules are checked against them when built. A new section is a minor bump at most. The version and release notes are the user's call.
