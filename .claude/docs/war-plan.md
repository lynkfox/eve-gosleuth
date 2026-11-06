# War tracker and group search

Plan of 2026-11-04, decisions from the user the same night. Stages 1 to 5 built.

The model is the user's hand-kept 2024 Coalition vs Lazerhawks sheet (id `11i7aE7hB7ES0Q-mtVmYRaO8dBj6DX7B6IL6GGNQQLP8`): gid 722070133 is one row per fight, gid 1013484469 the totals by side and day.

## Stages

| Stage | What ships |
| --- | --- |
| 1 | Group search from the search bar |
| 2 | War core: setup, saved wars, collection, classification, Overview, Battles, Systems |
| 3 | Station trash and asset denial, and the Timeline with every category |
| 4 | Merging, splitting, and ignoring battles, then notes and system nicknames |
| 5 | Pilots & Ships and Participants tabs |
| 6 | Last: the war's export, a folder of HTML pages and a JSON file, in place of a war PDF |

Each stage is usable when it lands. **After these, in a later version:** membership changes during a war, importing zKillboard killmail dumps, who joined in, fleet doctrines, the tour, right-click "Add to a war", multi-system battle reports, user assigned markers, coloring k-space Timeline bubbles by sov.

## Settled rules

### Sides

A side is one of: a coalition, a main with its affiliates, or a single corp or alliance. Pilots and systems are not allowed. Sides are saved as references and resolved at each run, so a change to a coalition carries into its wars. A side that no longer exists stops the run with a message naming it. An entity listed directly on both sides is refused at setup.

Everything works on the corp and alliance **at the time of the kill**. A side listing the corp wins over a side listing its alliance, as the more specific.

**Later version, nothing shown to the user for now:** membership that changes during a war, ie "Noob Corp Switches Sides!!!" in the sheet. It will be optional from and until dates on a side's member. Keep `WarSide` easy to extend for it, and build no UI.

### What counts

**A battle counts when one side killed the other on at least one of its killmails** (user, 2026-11-05, in place of anyone of both sides appearing: on the Apes Strong Together vs MSF war, one 1,012 pilot fight with a lone pilot of each side put 14.57B on a side and about 199 groups under Others). Every killmail in such a battle counts in the totals, including those of third parties. Any other battle is forgotten, whatever else happened in it.

- **Battles** are `FindBattles`' rules unchanged.
- **Battles and skirmishes:** a counted fight of `battleMinPilots` (10) or more pilots is a battle. Anything smaller is a **skirmish**, shown too, always (the toggle to hide them was removed, user, 2026-11-05), including a lone killmail of one side killing the other, which counts in the totals. The Overview counts both, ie "41 battles, 96 skirmishes".
- **A killmail outside any counted battle** with no one from the other side does not count, except asset denial (Stage 3).
- **Third parties:** a third party group in a counted battle counts for the side it fought beside: per battle, the opposite of whichever side it shot at or was shot by more. Even or unknown, it shows under Others, and does not count for a side. The group is the alliance, or the corp if it has none. An added group before its date is on no side, but the others on its killmails still count toward the side they shot at (user, 2026-11-05).
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
- **Update** lists only from the last run's newest killmail, less 5 days' overlap, so a tracked war rarely meets the cap. A change of sides or start date does a full run, as does a change in the groups its sides resolve to, ie a coalition gaining a member or a member gaining an affiliate (`RunKeyFor`).
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

**The report takes the whole window (user, 2026-11-05):** only the top bar and the ticker stay; the sidebar, target panel, and width limit go, and nothing is left from the previous report. A slim strip under the top bar holds the two sides, the dates, the last run, and Update, with the tabs across beneath it. A run lands on Overview. **The killmails are not listed (user, 2026-11-05):** a war has far too many, so there is no Killmails tab and the frontend gets only their count. Side colors are fixed, blue and orange, since red and green mean loss and kill.

- **Overview:** the Measures per side, the totals by side as in the sheet's top rows, battles and skirmishes counted, the reach warning, the caveat.
- **Battles:** one row per fight as in the sheet: time, duration, system, class, each side's named group, ISK and ships lost per side, Others, structures, and the battle report links. Battles and skirmishes together, always (user, 2026-11-05), a skirmish marked beside its date.
- **Systems:** the count by class, then each system's killmails and ISK per side.

## Stage 3: trash and asset denial

These are estimates from killmails and are labelled so, as the user sorted them by hand in the sheet.

Station trash is looked for after a side's citadel, engineering complex, refinery, or control tower dies in a counted battle, in its system. Customs offices and moon drills are not stations. A trash killmail's victim is on the structure owner's side or on neither side, or the killmail is blue on blue on either side (user, 2026-11-05: Wormageddon's pilot lost 36 ships to its own side around Lethal Estates' dead Astrahus; it was no fight, and it goes on the victim's side). It is never itself a structure or a Standup Fighter. **Trash is empty ships (user, 2026-11-05):** killmails name whoever last flew them, so a pilot's capsule dying in that system during a run of their losses, or within `warPodGap` (5 minutes, my number) after, means they were in those ships, and that run is not trash. Runs split where a pilot's losses are `warPodGap` or more apart.

1. **Repeat victims:** a pilot who lost two ships in that system less than 2 minutes apart, after the structure died, too short to reship with session timers (user, 2026-11-05). All that pilot's losses there after the structure died are trash. **Capsules are never trash or asset denial (user, 2026-11-05)**, so they stay where they fall, and don't count towards the two. This is not held to the 4 hour window (user, 2026-11-05); it reaches `warRepeatReach` (7 days, my number, so a pilot's bad night months later is not trash). Not in a later battle where the owner killed the other side: on the Apes Strong Together vs MSF war, an Apocalypse Navy fleet whose pilots lost a battleship then a frigate 3 days after a POS died came to 15B of false trash. A killmail goes with the last station to die before it.
2. **Battles are found again without those.** Otherwise a stream of trash kills keeps the battle open for hours under its 15 minute rule, and it never "ends".
3. **After the battle:** any later killmail in that system, up to `warTrashWindow` (4 hours, user, 2026-11-05) after the battle the structure died in ends, is trash. Not a loss of the side that killed the structure, as those are real losses. **Not a killmail in a later battle where the structure's owner killed the other side** (mine, 2026-11-05): the owner's revenge fleet shot back, so its losses are a fight. **Nor a fitted ship with more than one attacker (user, 2026-11-05):** an escape fleet, unless it falls under repeat victims above.

Trash stays attached to the structure's battle, in its own columns, and doesn't count towards that battle's fight ISK or duration. A neutral victim counts for the structure's owner, as the side it stood beside; one held by an added group's date, or a pilot in an NPC corp, stays under Others.

**Asset denial is capital ships only (user, 2026-11-05),** those of `capital_groups` and `capital_types`, of a side, in the war's dates. **Asset denial ends when a station dies (user, 2026-11-05):** after that side lost a station in that system, a killmail is not denial, so it may be trash or a fight, unless the side loses another within its timer days (3 medium, 4 large). With no station lost in the timer days after it, a capital with nothing fitted stays denial at the probable grade, as the station may never die, and an update raises it if one does. Its own station at top damage on a capital, or alone on any kill, is always definite (user, 2026-11-05):

- **Definite when its station then dies: nothing in any high, mid, or low slot (user).** Rigs and holds can hold anything. Whoever killed it. It stays out of the battles.
- **Its own side's station on the kill, whoever else shot it (user, 2026-11-05: a high weight, more so as top damage, which is always definite).** An attacker's ship is an upwell structure, control tower, tower battery, or Standup Fighter, on the victim's side. A station does not shoot its own side's capital by accident, so being on the kill is enough. The user's example, killmail 116654321, is an Atrax Naglfar, with Atrax's side's Fortizar at top damage among the Hawks fleet, in the sheet's "Atrax asset denial and death of their system". It stays out of the battles.
- **Fitted, with victim and all attackers on one side,** only when that side loses a station in that system within its timer days after (the bound is mine, 2026-11-05, so a friendly kill months earlier is not denial).

These may fall outside any counted battle, the one exception to that rule, and join trash in one bucket, as in the sheet. An update can turn earlier same-side killmails into asset denial once the structure falls, so the war holds same-side killmails too. Every other same-side killmail is left out, as with `IsInternal`.

**Anchorables (user, 2026-11-05):** mobile warp disruptors, depots, and the like (`anchorable_categories`, deployables, which sdegen now keeps) are always definite trash when the war's sides shot them: the other side, or blue on blue. No station is needed, they make no battle, and they go with the last station their side lost in that system before them, up to `warRepeatReach`. One only a third party shot is not the war's. A neutral's counts in the 4 hour window as other trash does. **Mobile tractor units are never trash (user, 2026-11-05, `not_trash_groups`):** they can hold a fight's loot, 732M each on their war, so their ISK stays in the fights.

**A subcapital is asset denial only with nothing but its own side's station on the kill (user, 2026-11-05),** which is always definite.

**A capsule only its own side's stations shot is ignored and not held (user, 2026-11-05):** in EVE's Crimson event implants can drop, so a pilot gets their own station to pod them to fetch them.

**Grades (user, 2026-11-05; since shown as Probable for definite and Uncertain for probable, the code keeping its names):** the bucket shows how much of it is **definite** and how much **probable** (`warTrashMark.definite`), on the Overview, a battle row's hover, and a Timeline Trash figure's hover. Definite: a repeat victim's empty ships, an anchorable, a capital with its own side's station at top damage, ties counting, anything with only its own station on the kill, or a capital with nothing fitted with a station lost in the timer days after. Probable: anything else only in the 4 hour window, a capital with nothing fitted and no station lost after, a capital with its own station lower on the kill, and a fitted capital's blue on blue before the station's loss.

**Held for this:** besides counted battles and same-side killmails, a war holds every trash killmail, and every structure lost by a side or a named third party even outside a counted battle, as asset denial and Station ref look at them. Those count for nothing themselves. `CurrentRunKey` gained a rules mark so a saved war reads its lists once more for them.

**Station being reffed:** a fight where one side lost or used Standup Fighters (user, 2026-11-04), or a station or battery of its fired without that station dying in the fight (user, 2026-11-05, on reviewing docs/war_report.md), is probably that side's station under attack, as only structures launch them. A carrier's fighters don't count. The battle gets an automatic mark on its row, naming that side, and the Overview counts them:

- **Station ref** when that side's upwell structure in that system then dies within 3 days for a medium structure, 4 days for a large (user, 2026-11-05). The fight it dies in counts. `medium_structure_types` in `structures.json` lists the mediums, as size has no group of its own; everything else, Keepstars and Sotiyos too, takes 4 days.
- **Ref attempt** otherwise, so a mark can move from one to the other at a later Update.

Standup Fighters never count as ships lost, only in ISK lost, under structures (user, 2026-11-04). They are their own groups, 4777, 4778, and 4779 (`category` 87 in `fit_types.json`), apart from carrier fighters' 1537, 1652, and 1653. They go in a new `standup_fighter_groups` section of `structures.json`, read with `ContainsGroupOf`, so a minor bump at most. Fights are found on killmails, so a station reffed that neither fires on a killmail nor has a fighter on one is not marked.

**Structure lookups in a war (user, 2026-11-05):** only Citadel, POS, and POCO losses and Standup Fighters are looked for. A war does not run the full structure lookup.

**Timeline:** the sheet's tab: for each day in EVE time and each side, every category, with toggle buttons on the left for By Day (By Week while on weeks) or Running Total, and on the right for Days or Weeks, weeks starting on a Monday. Above it a strip drawn like the Activity chart, one cell per period in the side's color, the more ISK lost the stronger. The strip and switches stay pinned while the table scrolls; past 14 cells the strip scrolls sideways, starting at the newest (user, 2026-11-05).

## Stage 4: battle edits, notes, and nicknames

Added after a report is generated, from a battle's row. Reordered by the user on 2026-11-05: merge, split, and ignore first, then notes and system nicknames. Settled with the user on 2026-11-05.

**Saving them.** All of it has to outlast closing the app, an Update, and a new version, so it is saved as the user's own data and applied again at every build: a `war_edit` section with one row per war (`retrieval.WarEdits`), one of `userSections` so a clear leaves it, removed with the war. Not in the `War` record, which every save rewrites.

**Replay.** The edits are an ordered list, applied after `findWarTrash` at each build (`aggregate/war_edits.go`). Each holds its killmail ids and goes to the battle now holding most of them. One that matches nothing, as its battle is outside the war's dates, stays saved and does nothing. Undo removes that edit and replays the rest.

### Merge and split

**They change no ISK, ship, Timeline, or Systems ISK figure:** tested. Only the row level moves: the battle and skirmish counts, marks, and each system's battle count.

- **A third party's side stays as the automatic battles decided it (user):** worked out per automatic battle and kept on each killmail, before any edit, as regrouping would otherwise move its ISK. So a split does not correct a group that really changed sides partway through.
- **A piece of a split keeps counting** though it may have no direct kill, as the battle it came from counted.
- **A new killmail** joins a merged battle by its usual battle. In a split one it joins the piece with the nearest killmail in time in its system, else the nearest overall.

**Merge with…** on a battle's menu lists the others, nearest in time first, to tick. **Split…** lists the battle's killmails (`GetWarBattleKillmails`) to tick by hand, by system, or from a line down, then moves them to a new battle. Each makes two, and can be repeated. The row carries a chit and an Undo on its menu. A row's id comes from its first killmail, with that killmail's id added if two pieces would share one.

### Ignore a battle

It leaves the war's totals and stays on the lists grayed out, with its own figures, to be taken back (user, 2026-11-05). **The Ahbazon tower is the first case, and it takes a split first:** its row is a real 185 pilot fight of 36.6B in J121959 that spread to Ahbazon on one killmail, a 14K shuttle. Ticking Ahbazon in Split and ignoring that piece takes only the shuttle and the tower's mark, checked on a copy of the user's cache on 2026-11-05.

- **What the row shows leaves with it (user):** its killmails and any trash or asset denial attached to its stations, from the Overview, Timeline, Systems, Others, and marks.
- **Trash is found again without them,** so a station in it stops confirming a Ref'd mark or ending asset denial elsewhere.
- **The killmails stay held,** and the Overview counts it apart.
- **Known limit, warned of (user, 2026-11-05):** if an Update joins an ignored fight to a real one, the whole joined battle is ignored; its chit then says how many joined since, ie "Ignored, +3". It can be taken back and the part that doesn't belong split off.
- **Ignoring a piece of a split can move a third party's ISK (user, 2026-11-05):** what is left is found again as its own battle, so each third party's side is worked out without the ignored killmails, as they should influence nothing. Merge and split alone move no total. The piece left loses its Split chit, and the split is undone from the ignored row.

Stages 5 and 6 must leave ignored battles out.

### Notes and system nicknames

- **A notes field** on any battle, saved with its killmail ids, so it follows the battle. Merged battles keep every note.
- **A nickname for a system** that follows it, shown beside its name. **Its scope is chosen when it is named (user, 2026-11-05):** for the whole app, as "Valhalla, Ursa home", or this war only, as an op name like "Mango". The war's wins inside that war. Changing a name to this war only moves it, unless Keep it everywhere else is ticked (user, 2026-11-05). Links, sorting, and copied text keep the real name.

### Held back

**User assigned markers** (a mark added or dismissed by hand): wanted, in a future version, and **they replace the sheet's flags entirely (user, 2026-11-05)**, so One Fight, Not war, 3rd Party, For Timer and the rest will not be built. When built, one added by hand should say so in its hover, as killmails don't show everything. Until then a wrong mark like the Ahbazon tower's is dealt with by ignoring its battle.

## Stage 5: Pilots & Ships and Participants

Settled with the user 2026-11-05. Both tabs read the war's battles as edited, so a merged battle is one battle and ignored ones count nowhere. Sides are per battle as everywhere (`warKill.effective`), so the tallies are made in `buildWar`, as `WarKillmail` keeps only the victim's side.

- **Pilots & Ships:** three columns that each scroll alone, stacking in a narrow window: side A's pilots, side B's, then ships flown above Others' pilots. Each list is sorted by ISK destroyed or killmails on, 50 a page, paged, with a filter for several alliances at once. Every pilot on a killmail gets the kill and its full ISK, as on zKillboard, with no line explaining it. A capsule counts in ISK but not as a kill.
- **Ships flown:** for sides A and B, each ship flown and the number seen, a pilot counting once per battle per hull. Only ships count, so no capsules, structures, or fighters.
- **Participants:** sides A and B beside each other, stacked with A on top in a narrow window, with no Others: an accordion of Alliance, then Corp, then Pilots, the pilots condensed into several columns. A corp with no alliance sits at the alliance level. A pilot who changed corps is under each, as everything goes by the group at the kill, with a small chit noting the change and their previous corp and alliance on hover.
- Trash stays out of both tabs. A pilot who counts for a side anywhere in the war has their killmails from time in an NPC corp left out of both tabs, so they show only on their side (user, 2026-11-05).

## Stage 6: the war's export

Settled with the user 2026-11-05: **HTML pages replace a war PDF**, as they keep hover and click through, and printing does not matter. Sharing is by zip or upload. The corp and alliance PDF stays as it is. The model is the user's own 2024 pages (totals, type_totals, timeline at lynkfox.github.io/eve_battle_report_timeline, now only an archive, so nothing exports to that repo).

- **Button:** where Generate Report is in the top bar, which on a war generates this instead.
- **Folder:** each war gets its own folder under a parent folder that is picked once and remembered in Preferences. Exporting again replaces its files.
- **Files, in order:** `01_Summary`, `02_Totals`, `03_Ship_counts`, `04_Timeline`, `05_Battles`, `06_<war name>.json`. Each page works alone, with Plotly's basic bundle in it (2.35.2 measured 1.07 MB), and links to the others. No network and no API calls. The app's dark look and side colors. Each page has the export date, app version, and the killmail caveat.
- **Totals:** ISK, ships, structures, and systems, each a running line and daily bars, as on the user's page, labeled as losses, with no "Bigger numbers are BAD".
- **Ship counts:** Stage 5's ships flown with those destroyed stacked on it, per side, in two panels A to Z.
- **Timeline:** a bubble per fight, skirmishes included and ignored battles not. Time across, system down, size by ISK, ringed where a structure is involved, click to its battle report. Notes are callouts cut to about 40 characters, in full on hover. A nicknamed system gets a dashed line with its name.
- **Timeline colors:** by the side whose citadels or POCOs killmails show in that system. **POS are ignored for this, as invaders often anchor them.** Nothing, or both sides, is grey. It can't be automated when both have one, as invaders will often drop an Astrahus, so that waits on a decision, perhaps Stage 6's user marks (undecided). K-space should go by sov, later.
- **Safety (no eval):** names, notes, and nicknames are data, written with `textContent`, escaped so they can't close a script, and with links and tags stripped from them and from Plotly's hover text, so they can't link or run. Only the battle report links the app makes itself work, `https` only. Each page has a Content-Security-Policy with no `unsafe-eval` and its script hashes, blocking injected script. The basic bundle's only `new Function` is webpack's fallback for browsers with no `globalThis`, never reached, but confirm it by rendering under the policy.
- **The JSON** holds these sections of the archive's jsons page, leaving out its two txt files and all_battle_reports: `alliance_appearances` and `corporation_appearances` (battles seen, per side and Others, corps under their alliance), and `system_appearances` (system, class, statics, battles). `major_players` was a judgement call, so it waits for user marks.
- **A "lost" system** is the first time a side loses a citadel or POCO there, POS ignored as for the colors.

## Later: multi-system battle reports

Its own piece of work after the war plan (user, 2026-11-05). Its `systems` list should replace `OtherSystemReports` (2026-11-05), which holds much the same.

The app never makes a battle report itself (user, 2026-11-05). The related links show one system each, and a fight that moved through a gate needs a report built by hand on the user's site of choice. EVE Tools (`/create`) and WarBeacon (`/br/create`) can build one; zKillboard has no way to. Their create endpoints are internal and undocumented, and using them would add public reports to someone else's site, so the user makes the report there and brings the link back.

**Getting the systems over:** on a battle's row, **Build a report** opens a small popup, for any battle but aimed at those in more than one system:

```
┌ BUILD A BATTLE REPORT ────────────────────────── × ┐
│ J123456  C5   Nov 4th, 2026 20:01 → 20:48          │
│ J654321  C5   Nov 4th, 2026 20:32 → 21:10          │
│                                                    │
│ [Copy systems and times]                           │
│ [Open EVE Tools] [Open WarBeacon]                  │
│ ────────────────────────────────────────────────── │
│ Report link [ paste it here              ] [Save]  │
└────────────────────────────────────────────────────┘
```

- Each system is listed in the order the battle reached it, with its own first and last killmail, as a report's time range for a system should cover only its part of the fight. Times follow the EVE/local preference on screen. The copied text is always EVE time, labelled so, as the sites work in it.
- **Copy** puts one line per system on the clipboard: name, system id, start, end.
- **Open** goes to the site's create page, in the browser. Whether either page can be filled from its URL, and what it takes pasted, is checked when this is built; if it can, the Open button fills it, and if not, the copied text is the whole help.
- This needs each system's id, name, start, and end on `WarBattle`: a new `systems` list, added then, with room left in the row beside `OtherSystems`.

**Including the report:** the pasted link is saved with the battle like a note: with its killmail ids, following the battle through updates, and cleared with the war. Only an `https` link is kept, and it opens in the browser as other links do. The row shows it first, named by its site where known, ahead of the per-system related links, which stay. It is also part of the export and the PDF's battles in Stage 6. A saved link changes no totals, and the app does not read anything from it.

## Contract for Stage 2

The contract is the code: `retrieval/wars.go`, `aggregate/war.go`, and `gui/war.go`, with `FindBattlesOf` and `BattleKillmail` in `aggregate/collate_data.go` and `KillmailLimits.OnListed`. Settled while writing it:

- **A war's start is a fixed date.** The age picker's buttons give a date on saving, as a rolling start would slide the report and force a full run each day.
- **`WarSide.WithAffiliates`:** a corp or alliance picked as a group brings its affiliates, one typed or linked is alone. A coalition always brings its members' affiliates.
- **`War.RunKey` against `War.CurrentRunKey()`** says whether a run is a full one. `War.Reach` keeps the reach warning across sessions.
- **`ClassifyRawWar` and `BuildWar` read sides and battles from raw killmails,** taking `WarSides` rather than two targets, so both find the same battles.
- **Same-side killmails are held from Stage 2** and count for nothing, leaving them out of battles too, so Stage 3 can look back at them.
- **`AddWarGroup` and `RemoveWarGroup`** change added groups and rebuild from what is held.
- **New sections:** `ship_categories` and `capsule_groups` in `ships.json`, `poco_groups` in `structures.json`, as customs offices were not in `control_tower_groups`.
- **An opened war:** recent searches and a typed name open it from what it holds; its Update button runs it.

## Tests

- **Counting:** both sides in a battle with no direct killmail forgotten, one direct killmail counting the whole battle, one side only forgotten, lone direct killmail, corp beats alliance, a corp moved between alliances, NPCs only, structures with no pilot.
- **Third parties:** beside A, beside B, even, and one added to a side by hand, from the beginning and from a date.
- **`FindBattles` split:** existing tests pass untouched; raw and expanded input give the same battles.
- **Saved wars:** round trip, a held killmail surviving a cache clear and the list no longer reaching it, update reading only from the last run, a side change doing a full run.
- **Station ref:** a Standup Fighter loss marks the battle for that side, a carrier fighter's does not, and it adds to ISK lost but not ships lost. A medium dying at 3 days confirms it and past that does not, a large at 4 days, and no kill leaves an attempt.
- **Trash:** repeat victim, a ship and its pod not being one, a repeat past the window, a battle ending once they are removed, the 4 hour edge, a returning fleet not counted, a revenge fight not counted, a neutral victim counted, a capsule never being trash, a pilot's capsule soon after their ships clearing those ships but not an earlier run, blue on blue trash on the victim's side, a capsule only its own station shot not being held, anchorables with and without a station and with only a third party, a subcapital only with nothing but its own station on the kill, the grades, a capital with nothing fitted being definite whoever killed it, a subcapital never being denial, a fitted capital with its own side's station on the kill among the other side's fleet being denial and the other side's station not, a fitted capital's blue on blue before the loss but not after it or with no loss. Stage 3's rules should also be compared by eye with a real structure fight.
- **Report totals** by hand on a small fixture, the three ISK buckets adding to the total.
- **Battle report links:** systems listed in the order reached with their own times, the copied text in EVE time whatever the preference, a saved link following its battle through an update, and a link not `https` refused.
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
