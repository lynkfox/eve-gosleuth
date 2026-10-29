# What GoSleuth can do

GoSleuth reads a target's killmails from zKillboard, fills in the details from EVE's own API (the ESI), and lays them out so you can see who they fly with, where, in what, and when. Everything runs on your own computer: there is no GoSleuth server, account, or login.

Killmails only record deaths, so keep in mind what they can't show: fights nobody died in, the other side of a gate, logi, objectives, and much more. GoSleuth collates; reading the results is up to you.

## Searching

- **One search bar for everything.** Type a pilot, corporation, alliance, or system name, or a corp or alliance ticker. If a name matches more than one thing, GoSleuth asks which you meant. You can even use an ID from the ESI or a link from zKill or EveWho. 
- **Everything is cached.** Killmails are kept for good, and names and details until they could have changed, so a repeat search is quick and kind to the APIs. The cache can be cleared, in whole or by age, from Preferences. The search bar even saves recent searches for quick returns, and you can adjust how many it shows in the preferences.
- **Search limits.** Choose how many killmails to collect, how far back to go, and whether to use killmails you've already downloaded. The limits button beside the search bar changes them for the current session; Preferences sets the defaults.
- **Stop at any time.** Stopping a search builds the report from what has been collected so far.


## What a report shows

Every search opens on **At a glance**: the target's profile (details, corporation history, member corps, or stargates) and a card for each of the report's tabs with its top entries. The tabs down the left depend on what you searched for.

| Tab | Pilot | Corp / Alliance | System |
|---|:-:|:-:|:-:|
| Killmails: every killmail collected, newest first | ✓ | ✓ | ✓ |
| Activity: when they're active, by weekday and hour | ✓ | ✓ | ✓ |
| Top Fleetmates: the pilots most often on killmails with them | ✓ | | |
| Top Members: the pilots on the most of their killmails | | ✓ | |
| Members: the current roster (corp) or member corps (alliance) | | ✓ | |
| Top Pilots and Top Corps: who is most active in the system | | | ✓ |
| Top Systems: where they fight | ✓ | ✓ | |
| Allies?: who shows up on their kills, and might be friendly | ✓ | ✓ | |
| Capital Pilots: who flies their capitals, and in what | | ✓ | |
| Structures: citadels and towers they use, destroyed, and lost | | ✓ | |
| Fits: the ships they lose, and how they fit them | ✓ | ✓ | |
| PI Pilots: haulers lost near the system's planets, or with PI aboard | | | ✓ |
| Ratters: ships lost to NPCs, and whether anyone else was there | | | ✓ |

A wormhole system also shows the last structure lost there, however long ago.

Every tab has a **?** that explains it, and most rows have a right-click menu: search that pilot, corp, alliance, or system, or open it on zKillboard, EveWho, or Dotlan.

## Fits

- **Every loss as a fit**, drawn as a fitting window: modules round the hull, with charges, drones, fighters, and capsule related killmail implants/boosters.
- **Identical fits merge**, whatever slots the modules were in and whichever ECM flavors were used, with a count of how often each was lost. Page through a merged fit's losses with the arrows.
- **Implants from the capsule.** When the pilot's pod died in the same fight, its implants are shown with the ship.
- **Tags** describe what a fit is for: Tackle, Logi, Links, Neut, EWAR, ECM, Roller, Cyno, Nano, Cloaky, Scanner, Bait, Firewaller, Ratting, its tank (Shield, Armor, Hull, Active, Buffer, Passive), and more. Abyssal marks a fit with a mutated module. Hover a tag in the fitting window to see exactly what earned it. The Tag key explains each one.
- **Filter** by hull, module, pilot, corp, or alliance, or by tag (`tags: logi`). Clicking a tag filters by it.
- **Copy EFT** puts a loss on the clipboard in the format Pyfa and the in-game fitting window import. If an associated capsule included implants, those are placed in the fit's original inventory.

## Allies and holdings

- **Allies?** lists the alliances and corps that turn up on a target's kills. They are only possible allies: a third party in the same fight looks the same on a killmail. Ignore one, and the next takes its place. Allies the target also killed can be hidden.
- **Holdings.** Link a corp or alliance to its holding or alt corps. A search of the main then includes the holdings' killmails, and a search of a holding goes to its main.
- **My Intel** shows every holding and ignored ally you've set, for editing in one place. Export them to a file to share with your corp, or import someone else's.

## Making it yours

- **Themes** after EVE's own UI themes, the factions, Triglavian, and Synthwave, each with its own icon.
- **Reorder the tabs** by dragging them, and shrink the sidebar to icons.
- **EVE time or local time** for every time shown.
- **Your own game data.** Fenris Creations may change the game more often than GoSleuth is released, so the built-in game data (which ships are capitals, how fit tags are read, wormhole statics, and so on) can be corrected or extended with small files of your own. See [Editing GoSleuth's game data](static-data-overrides.md).

## Help and privacy

- **Debug mode** (Preferences, Advanced) records a search in detail, for a bug report.
- **Report a bug** from the About window.
- GoSleuth only talks to zKillboard, the ESI, EveWho, and EVE's image server. It is free, open source, and has no ads or paid tiers. It collects no user data from your machine.
