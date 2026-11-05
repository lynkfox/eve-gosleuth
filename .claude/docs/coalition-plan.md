# Coalition grouping and searches

Plan of 2026-11-04, decisions settled with the user the same night. This is the Readme's "Custom Collation searches" item; the name in everything users see is **Coalition**.

## What it is

A named group of corps and alliances, made in My Intel, searched as one target. Unlike affiliates it has no main.

- Its report counts every member's killmails, and each member's own affiliates' (one level, following Include Affiliates), as the target's own, one killmail once.
- Searching a member on its own is unchanged: it never pulls in the other members. An affiliate search that crosses into a coalition stays as it is today, since `IntelTarget.Holdings` only ever comes from holding links.
- Corps and alliances only.
- **Structures are opt-in per coalition, off by default** (a checkbox on its card): off, the report has no Structures tab and makes none of the structure list calls. Their cost grows with members times structure groups.

## Warnings (user's rule)

The Coalitions tab always says each member is paged on its own, so a large coalition takes long and members reach back to different dates.

A further warning shows when the limits the next search uses (`GetSearchLimits`) are large: `killmailsNewerThanDays == 0` (any age, the only choice past 90 days), or more than 2000 killmails, where that count is `maxKillmails`, or `killmailPages * 200` when `maxKillmails` is -1 (so 10 pages is the line). It shows in the Coalitions tab, refreshed when the tab opens, and in the status line when a coalition search starts by any route (magnifying glass, search bar, recent search). It warns only, never blocks.

## My Intel window (user's layout)

- **Outside the tabs**, at the top: description, warning, filter, Import, add form, undo, bulk buttons (Check all, Export, Delete). A collapse button folds this down to just the Filter row. It starts open; the choice is kept only while the app runs.
- **Two tabs below it: Affiliates and Coalitions.** Only the cards scroll, as now (`.myintel-body` is a flex column, `.myintel-list` scrolls). The top part applies to the shown tab: the description and warning are that tab's; the filter, Check all, Export, and Delete act on its cards.
- The add form on Coalitions is a name and a Create button (no type select).
- Import stays one button, as one file can hold both.

## Contract

### comm/types

`COALITION` is added last in `EntityType`, `String()` gives `"COALITION"`.

### retrieval (agent A)

New section `sectionCoalitions = "coalition"`, in `userSections`. `retrieval/coalitions.go`, loaded and written like holdings (`retrieval/holdings.go`):

```go
type CoalitionMember struct {
    Type string `json:"type"` // CORPORATION or ALLIANCE
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type Coalition struct {
    ID                int               `json:"id"`   // negative, local to this cache, never an eve id
    Name              string            `json:"name"`
    Note              string            `json:"note"`
    Members           []CoalitionMember `json:"members"`
    IncludeStructures bool              `json:"includeStructures"`
    SetAt             time.Time         `json:"setAt"`
}

func (c *CachedEveIDs) Coalitions() []Coalition                    // by name, ignoring case
func (c *CachedEveIDs) CoalitionByID(id int) (Coalition, bool)
func (c *CachedEveIDs) CoalitionNamed(name string) (Coalition, bool) // ignoring case
func (c *CachedEveIDs) SaveCoalition(coalition Coalition) Coalition // ID 0 gets a new one: -time.Now().UnixMilli(), moved down until unused, never reused by a later coalition. Sets SetAt.
func (c *CachedEveIDs) RemoveCoalition(id int)
func (t *IntelTarget) IsGroupTarget() bool                          // CORPORATION, ALLIANCE, or COALITION
```

`IntelTarget` for a coalition: `{TypeOfEntity: COALITION, IDNumber: <negative>, Name, Holdings: members plus their affiliates, deduped by type and id, Expanded: nil}`. `IsGroup` goes straight to the `Holdings` loop. `TargetGroup` gains `Member bool` (true for a member, false for an affiliate of one), used only for labels.

- `listKillmails`: no list of its own; the ticker says `member X` or `affiliate X`. `RawKillmail.Via` is set on every killmail.
- `CollectStructureKillmails`: entities start from `Holdings` only.
- `listEntity` returns an error for an id below 0, so a slip can't reach zKill. So does `GetIfNotCached`, and anything else that would reach the ESI with one.
- aggregate: every `TypeOfEntity` switch treats `COALITION` as a group target, with no `Expanded` to assert on. `HoldingKillsTogether` returns nothing. One member killing another is `internal`: a loss, never a kill.

### gui (agent B)

```go
type CoalitionCard struct {
    ID                int         `json:"id"`
    Name              string      `json:"name"`
    Note              string      `json:"note"`
    IncludeStructures bool        `json:"includeStructures"`
    Members           []EntityRef `json:"members"`       // by name
    Affiliates        []EntityRef `json:"affiliates"`    // members' own affiliates, which its search also pulls, by name
    IgnoredAllies     []EntityRef `json:"ignoredAllies"`
}
```

All skip `a.mu` like the other My Intel methods, use `recoverAsError`, and return user readable errors. A name must be non-empty after trimming, at most 100 characters, and unique ignoring case.

```go
func (a *App) GetCoalitions() ([]CoalitionCard, error)
func (a *App) CreateCoalition(name string) (CoalitionCard, error)
func (a *App) RenameCoalition(id int, name string) (CoalitionCard, error)
func (a *App) SetCoalitionNote(id int, note string) (CoalitionCard, error)
func (a *App) SetCoalitionStructures(id int, include bool) (CoalitionCard, error)
func (a *App) AddCoalitionMember(id int, name string, memberType string) (CoalitionCard, error) // name or a zKill/EveWho link, as My Intel's lookupEntity; corps and alliances only; a member twice is refused
func (a *App) RemoveCoalitionMember(id int, memberType string, memberID int) (CoalitionCard, error)
func (a *App) RemoveCoalitions(ids []int) error
func (a *App) RestoreCoalitions(cards []CoalitionCard) error // the undo of RemoveCoalitions, keeping their ids
```

Ignored allies use the existing `AddIgnoredAlly`/`RemoveIgnoredAlly` with type `"COALITION"` and the negative id, so those must accept it.

- `searchTypes` gains COALITION. `SearchTarget(name, "COALITION", id)` builds the result from the cache with no ESI call: `TargetResult.Coalition *CoalitionCard` (json `coalition`, omitempty), `Display.Name`, `Holdings` nil. A missing id or name gives "That coalition no longer exists". A typed name with no type: an exact coalition name joins the choices (`TargetChoice.Type` "COALITION"), and is searched directly when the ESI finds nothing else or can't be reached.
- Recent searches record it like any target.
- `CollectIntel(id, "COALITION")`: as an alliance, except `report.MemberList` false, no `newRoster`, no `HoldingActivity`, and `report.Structures` and the structure list calls only when `IncludeStructures`.
- `canHold` stays false for it. `GetMyIntel` must skip type COALITION in ignored ally rows, so it gets no Affiliates card and no name lookup with a negative id.
- `holdingAllies`: every member and affiliate is left out of Allies. `buildScope`: no main row, a row per member and affiliate. `buildKillmailCards` Via shown as with affiliates.
- `fits.go`'s affiliate helper and `pdf_report.go` gates allow COALITION. The PDF's report works for a coalition, the Scope card listing its sources, with no Structures section unless opted in.
- `myintel_share.go`: `intelFile` gains `coalitions` (omitempty; name, note, includeStructures, members), format still version 1, as older copies ignore it. `ExportMyIntel(cards)` takes keys `COALITION:<id>` beside the others. `planImport`/`ApplyMyIntelImport`: matched by name ignoring case, else created with a new id; only ever adds members; `ImportPreview` gains `coalitions` and `coalitionMembers` counts; malformed entries skipped and counted, names cleaned like the others, members added to `AddIntelChecks`. `correctIntel` and the removal side of `runIntelChecks` fix or drop coalition members too.

### frontend (agent C)

Calls the methods above as `backend().X`. See "My Intel window" and "Warnings". Plus:

- A coalition card: the magnifying glass left of the card (`searchFromMyIntel`, with `{type: "COALITION", id, name}`), tick, name, member count, ×; body with Members (rows and add row like affiliates, Note like theirs), the members' Affiliates read only, Ignored allies, the Structures checkbox with a line on its cost, and rename.
- Filter matches a coalition's name, members, and affiliates.
- Delete and × confirm, and offer undo through `RestoreCoalitions`, as cards do.
- Target panel for a coalition: the name, a generic emblem (the sprite's `icon-corps`), and its members as chips with icons, each opening `offerSearch`; no zKill or EveWho links.
- `searchTypeLabel` gives "Coalition". `setExportPDF` and the gate at main.js:4658 allow it. The search limits warning goes in the status line for any coalition search.
- An Allies row (corp or alliance, any target) gets "Add to Coalition…" in its section, opening a small picker of coalitions with a new name field. One line in that tab's right click help.
- `tourBackend` in tour.js gets whatever stand-ins it needs, so the tour still walks My Intel-free as before.

## Tests

- `IsGroup` with members, and that a coalition's own id never matches.
- Merged lists: no list for the coalition itself, one killmail once, `Via` names the member; a member's affiliate is pulled.
- A member searched alone does not pull in the others; an affiliate of a member goes to its main as before.
- Allies leave out every member; members' killmails on each other are internal.
- Structures: no structure list calls when opted out.
- Save, rename, remove, survives `ClearAll` without the tick; ids not reused; export then import adds by name; a bad member is dropped by the intel check.
- `CollectIntel` through mock clients fails on any ESI or zKill call with a negative id.
- The frontend load check in headless Edge, with both tabs, the collapse, and a coalition target rendered.

## Build order and agents

Sonnet agents, one file set each. The contract above is the interface; each reports what the next needs.

| Agent | Files | Runs |
| --- | --- | --- |
| A, backend core | `comm/types`, `retrieval/*`, `aggregate/*` | first |
| B, app layer | `gui/*` | after A, beside C |
| C, frontend | `frontend/dist/*` | after A, beside B |

Then the main session joins them, runs the end to end check, updates `docs/features.md` and `.claude/docs/features.md`, and rebuilds. `1.4.0` follows after the review cycle. No `static_data` change, so no major bump. The Readme is the user's.

## Later, not in this plan

- Small scope searches over a coalition (All MSF + Machariel).
- Systems as members, which belongs with the Affiliates systems item.
