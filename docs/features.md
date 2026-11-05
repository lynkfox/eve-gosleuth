# What GoSleuth can do

GoSleuth reads a target's killmails from zKillboard, fills in the details from EVE's own API (the ESI), and lays them out so you can see who they fly with, where, in what, and when. Everything runs on your own computer: there is no GoSleuth server, account, or login.

Killmails only record deaths, so keep in mind what they can't show: fights nobody died in, the other side of a gate, logi, objectives, and much more. GoSleuth collates; reading the results is up to you.

## Searching

- **One search bar for everything.** 
  - *Type a pilot, corporation, alliance, or system name, or a corp or alliance ticker. If a name matches more than one thing, GoSleuth asks which you meant. You can even use an ID from the ESI or a link from zKill or EveWho.*
- **Everything is cached.** 
  - *Killmails are kept for good, and names and details until they could have changed, so a repeat search is quick and kind to the APIs. The cache can be cleared, in whole or by age, from Preferences. The search bar even saves recent searches for quick returns, and you can adjust how many it shows in the preferences. Its list also offers your coalitions and affiliate groups, so you can search them without opening My Intel, or type `a:` and a name for an affiliate group or `c:` and a name for a coalition, ie `a:ursa`.*
- **Search limits.** 
  - *Choose how many killmails to collect, how far back to go, and whether to use killmails you've already downloaded. The limits button beside the search bar changes them for the current session; Preferences sets the defaults.*
- **Stop at any time.** 
  - *Stopping a search builds the report from what has been collected so far.*


## What a report shows

Every search opens on **At a glance**: the target's profile (details, corporation history, member corps, or stargates) and a card for each of the report's tabs with its top entries. The tabs down the left depend on what you searched for.

| Tab | Pilot | Corp / Alliance | System |
|---|:-:|:-:|:-:|
| **Killmails:** *every killmail collected, newest first* | ✓ | ✓ | ✓ |
| **Activity:** *when they're active, by weekday and hour* | ✓ | ✓ | ✓ |
| **Top Fleetmates/Pilots/Corps:** *the pilots (or corps, for a system) most often on its killmails* | ✓ | ✓ | ✓ |
| **Top Systems:** *where they fight* | ✓ | ✓ | |
| **Battles:** *fights of 10 or more pilots, with links to their battle reports on zKillboard, EVE Tools, and Warbeacon* | ✓ | ✓ | ✓ |
| **Capital Pilots:** *who flies their capitals* | | ✓ | |
| **Structures:** *citadels and towers they use, destroyed, and lost* | | ✓ | |
| **Fits:** *the ships they lose, and how they fit them, potential classifications, and some stats* | ✓ | ✓ | |
| **PI Pilots:** *haulers lost near the system's planets, or with PI aboard* | | | ✓ |
| **Ratters:** *ships lost to Rat NPCs, and whether anyone else was there* | | | ✓ |
| **Members:** *the current roster (corp) or member corps (alliance)* | | ✓ | |
| **Allies?:** *who shows up on their kills, and might be friendly* | ✓ | ✓ | |

A wormhole system also shows the last structure lost there, however long ago.

Every tab has a **?** that explains it, and most rows have a right-click menu: search that pilot, corp, alliance, or system, or open it on zKillboard, EveWho, or Dotlan.


## Save that Report as a PDF to Share

Every report run on a Corporation or Alliance can also be saved as a PDF with many notable discoveries for sharing with your Alliance or allies.

- **Summary**
  - *A summary of the target, its affiliates, what range of killmails and how many were gathered*
- **Activity Chart**
  - *An EVE time chart of activity, displaying kills found by day and hour, and over weeks.*
- **Battles**
  - *The 10 latest battles found, then the 10 biggest by pilots, with when and where they happened and their size*
  - *Each links to the battle report on zKillboard, EVE Tools, and Warbeacon. Battles are estimated from the killmails found, so the sites show the whole fight*
- **Potential Fleet Doctrines**
  - *A brief summary of anything that might be a Fleet Doctrine - as determined by GoSleuth based on rules regarding number of kills and similar ships in a battle (rolling 15 minute period) and usually in the presence of logi*
  - *Contains an at a glance fit, major resist hole, and its sensor strength. Specialized fits may contain other information, like Neut Pressure or Remote Rep ehp/s*
  - *Links to the EFT version of the fit in the appendix*
- **Capital Pilots**
  - *A listing of all seen pilots flying capitals*
- **Appendix: EFT Fits**
  - *For easy copy paste into the game or Pyfa*
  - *Fleet Doctrine summaries link here, and there is a link directly back*


## Fits

- **Every loss as a fit** 
  - *drawn as a fitting window: modules round the hull, with charges, drones, fighters, and capsule related killmail implants/boosters.*
  - *Identical fits are merged together (positioning is ignored, and some modules like ECM are counted as the same thing)*
  - *Still able to browse through the individual killmail fits*
  - *Any fit can be copied to EFT to import into Pyfa, the Fitting window in game, or something else*
- **Implants from the capsule.** 
  - *When the pilot's pod died in the same fight, its implants are shown with the ship.*
  - *Exporting such a kill to EFT automatically places the implants into the cargo.*
- **Tags and Filtering** 
  - *describe what a fit is for, such as Tackle, ECM, Roller. Hover a tag in the fitting window to see exactly what earned it. The Tag key explains each one.*
  - *Fits that contain Abyssals are marked*
  - *Fits where at least one had a corresponding capsule kill with implants are also marked*
  - *Filter the list by most things - hull, module, pilot or group, or even tags (`tags: logi`) - you can also just click a tag to filter by it*

## Allies and affiliates

- **Allies?** 
  - *lists the alliances and corps that turn up on a target's kills. They are only possible allies: a third party in the same fight looks the same on a killmail.*
  - *Ignore some or many if you already know they don't count to see others on the list*
- **Affiliates and Home systems** 
  - *Link a corp or alliance to its affiliates, its holding or alt corps, and if it's a Wormhole Corp its home system.* 
  - *A search of the main then includes the affiliates' killmails, and a search of an affiliate goes to its main.*
- **Coalitions** 
  - *Group any corps and alliances under a name and search them as one target, with no main. Each member's own affiliates come along.*
  - *Structures are a checkbox on each coalition, off by default, as they take many more calls. Each member is paged on its own, so a large coalition takes a while.*
- **My Intel** 
  - *shows every affiliate, ignored ally, and coalition you've set, for editing in one place, in an Affiliates tab and a Coalitions tab. A magnifying glass beside each one searches it straight away.*
  - *Pull Coalitions out to sit beside Affiliates, each time My Intel opens, and drag a corp, alliance, or affiliate from the left straight onto a coalition.*
  - *Export them to a file to share with your corp, or import someone else's.*


## Tracking a war

The **War** button sets up two sides and a start date, and GoSleuth follows the fighting between them. Each side can be a coalition, an affiliate group, or one corp or alliance. Wars are saved, and open again with no searching at all.

- **Update**
  - *Reads only what is new since the last run, unless the sides or the start date changed. Search limits don't apply: a war reads everything from its start date*
- **What counts**
  - *A fight counts when one side killed the other on at least one of its killmails. Every killmail in that fight then counts. 10 or more pilots is a battle, anything smaller a skirmish*
  - *A third party counts, per battle, for the side it fought beside. If that can't be told, it shows under Others, and from there you can right-click a group to add it to a side, from the start of the war or from a date, or to ignore it*
- **Station trash and asset denial**
  - *Estimates from killmails, kept apart from the fighting, and graded definite or probable. Station trash is empty ships shot around a station after it falls: one pilot's ships dying too fast to reship and any anchorable such as a mobile warp disruptor, though never a mobile tractor unit (definite), and anything else there in the 4 hours after the fight ends (probable), unless the owner came back and shot. If the pilot's capsule dies within a few minutes, they were in the ship, so it isn't trash*
  - *Asset denial is capital ships only. A capital with nothing in its high, mid, or low slots, or with its own side's station at top damage, is definite. Its own station lower on the kill is probable, as is a fitted one its own side killed in the 3 days (medium) or 4 days (large) before that side loses a station there. A smaller ship is denial only with nothing but its own station on the kill, which is definite. Capsules are never trash or denial, and one only its own station shot is left out of the war altogether*
- **Station ref and Ref attempt**
  - *A fight where a side lost Standup Fighters means its station was probably under attack. It is marked a Station ref if that side's structure then died there within 3 days for a medium or 4 for a large, and a Ref attempt until then*
- **The report takes the whole window**
  - *Overview: each side's ISK, ships, and structures lost, and how far back zKillboard's lists reached*
  - *Battles: one row per fight, with what each side lost and links to its battle reports, sortable by date, class, system, or side, and filtered by clicking classes in the table above it*
  - *Systems: where the war was fought, by class, sortable by any column, and filtered the same way*
  - *Timeline: what each side lost by day or by week, as that period's figures or running totals*

Killmails can't show objectives or survivors, so GoSleuth never calls a battle won.

## Making it yours

- **Themes** 
  - *based on EVE's own UI themes, adding a Triglavian and a Synthwave*
- **Reorder the report window categories**
  - *Click and drag them around, though the At a glance is always pinned to the top*
- **EVE time or local time.**
- **Your own game data.** 
  - *Fenris Creations may change the game more often than GoSleuth is released, so the built-in game data (which ships are capitals, how fit tags are read, wormhole statics, and so on) can be corrected or extended with small files of your own. See [Editing GoSleuth's game data](static-data-overrides.md).*

## Help and privacy

- **Debug mode** 
  - *(Preferences, Advanced) records a search in detail, for a bug report.*
  - *Even a handy button to take you right to it so you can upload it to the bug report*
- **Report a bug** 
  - *from the About Window.*
  - *if you find a fit whose tags are completely wrong and you can explain why the weighting shown in those tags is bad, use this + the link to the killmail to let us know - we'll see if we can adjust them!*
