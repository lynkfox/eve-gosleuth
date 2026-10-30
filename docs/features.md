# What GoSleuth can do

GoSleuth reads a target's killmails from zKillboard, fills in the details from EVE's own API (the ESI), and lays them out so you can see who they fly with, where, in what, and when. Everything runs on your own computer: there is no GoSleuth server, account, or login.

Killmails only record deaths, so keep in mind what they can't show: fights nobody died in, the other side of a gate, logi, objectives, and much more. GoSleuth collates; reading the results is up to you.

## Searching

- **One search bar for everything.** 
  - *Type a pilot, corporation, alliance, or system name, or a corp or alliance ticker. If a name matches more than one thing, GoSleuth asks which you meant. You can even use an ID from the ESI or a link from zKill or EveWho.*
- **Everything is cached.** 
  - *Killmails are kept for good, and names and details until they could have changed, so a repeat search is quick and kind to the APIs. The cache can be cleared, in whole or by age, from Preferences. The search bar even saves recent searches for quick returns, and you can adjust how many it shows in the preferences.*
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
| **Top Fleetmates:** *the pilots most often on killmails with them* | ✓ | | |
| **Top Members:** *the pilots on the most of their killmails* | | ✓ | |
| **Members:** *the current roster (corp) or member corps (alliance)* | | ✓ | |
| **Top Pilots/Corps:** *who is most active in the system* | | | ✓ |
| **Top Systems:** *where they fight* | ✓ | ✓ | |
| **Allies?:** *who shows up on their kills, and might be friendly* | ✓ | ✓ | |
| **Capital Pilots:** *who flies their capitals* | | ✓ | |
| **Structures:** *citadels and towers they use, destroyed, and lost* | | ✓ | |
| **Fits:** *the ships they lose, and how they fit them, potential classifications, and some stats* | ✓ | ✓ | |
| **PI Pilots:** *haulers lost near the system's planets, or with PI aboard* | | | ✓ |
| **Ratters:** *ships lost to Rat NPCs, and whether anyone else was there* | | | ✓ |

A wormhole system also shows the last structure lost there, however long ago.

Every tab has a **?** that explains it, and most rows have a right-click menu: search that pilot, corp, alliance, or system, or open it on zKillboard, EveWho, or Dotlan.


## Save that Report as a PDF to Share

Every report run on a Corporation or Alliance can also be saved as a PDF with many notable discoveries for sharing with your Alliance or allies.

- **Summary**
  - *A summary of the target, its affiliates, what range of killmails and how many were gathered*
- **Activity Chart**
  - *An EVE time chart of activity, displaying kills found by day and hour, and over weeks.*
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
- **My Intel** 
  - *shows every affiliate and ignored ally you've set, for editing in one place. Export them to a file to share with your corp, or import someone else's.*


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
