# Editing GoSleuth's game data

GoSleuth ships with a set of EVE game data built in: which ships count as capitals, which items are PI materials, what each module is and where it fits, every wormhole system's statics, and so on. CCP changes the game more often than GoSleuth is released, so you can correct or extend this data yourself, without waiting for an update.

You don't copy and edit the whole file. You write a small file holding only your changes, and GoSleuth lays it over its own data each time it starts.

## Quick start

1. Close GoSleuth.
2. In the folder that holds `eve-gosleuth.exe`, make a folder named `static_data`.
3. In it, make a file with the same name as the built-in file you want to change (see [the files](#the-files)), for example `ships.json`.
4. Put only your changes in it, for example to stop freighters counting as capitals:

   ```json
   {
       "capital_groups": {
           "513": null
       }
   }
   ```

5. Start GoSleuth. Your changes are read at startup, so restart it after every edit.

```
GoSleuth\
    eve-gosleuth.exe
    static_data\
        ships.json        <- only your changes
```

To undo your changes, delete your file (or the whole `static_data` folder) and restart.

## Seeing the data in full

In GoSleuth, open **Preferences**, open **Advanced** at the bottom, and beside **Game data** choose **Export game data**. GoSleuth writes every file it is using, with your changes already merged in, to a `static_data_export` folder and opens it. Read these files, or copy pieces from them into your own files. Editing the exported files changes nothing. Export again after changing your files to check the result: if your change isn't in the export, GoSleuth couldn't use your file (see [the rules](#rules-for-writing-the-files)).

## How your changes are combined

Your file is matched against the built-in file of the same name, piece by piece:

| In your file | What happens |
|---|---|
| A key the built-in file has | Your value replaces it. If both values are `{ }` objects, they are matched key by key again, so you only need the fields you are changing. |
| A key the built-in file doesn't have | It is added. |
| A key set to `null` | It is removed. |
| A key you leave out | The built-in value is kept. |
| A list `[ ... ]` | Your list replaces the built-in one whole. Give the complete list. |

To empty a section, set each of its ids to `null`. Setting a whole section to `null` (or giving it as `{}`, which changes nothing) won't do it: a file missing one of its sections can't be used.

For example, the built-in `items.json` has:

```json
{
    "pi_material_groups": { "1032": "Planet Solid - Raw Resource", "1042": "Basic Commodities - Tier 1", "...": "..." },
    "smartbomb_groups":   { "72": "Smart Bomb" }
}
```

and your `static_data\items.json` is:

```json
{
    "pi_material_groups": {
        "1042": "Basic Commodities - Tier 1 (P1)",
        "1032": null
    }
}
```

The result keeps every PI group except 1032, with 1042 renamed. `smartbomb_groups` isn't in your file, so it stays as built in.

## Rules for writing the files

- **Files must be valid JSON.** Every id is in double quotes (`"34328"`, not `34328`), a comma goes between entries but never after the last one, and comments are not allowed. If you aren't sure about a file, paste it into any online JSON validator.
- **If GoSleuth can't use your file, it is ignored completely** and the built-in data is used. That happens when the file is not a single valid JSON object, or when it would leave the data broken: a section removed, `fit_types.json` left with no types, or a wormhole system given a static that isn't in the `wormholes` list. GoSleuth does not show a message when this happens. If your change doesn't seem to take effect, check the file first, then [export the data](#seeing-the-data-in-full) to see whether your change is in it.
- **Ids are what count.** In the id lists, the text after each id is a readable label. GoSleuth only uses it in progress messages.
- **Use groups where you can.** A group covers every ship or item in it, including ones CCP adds later. Use single types only for exceptions, such as `capital_types` or `hauler_exclusions`.

## Finding ids

- **zKillboard links:** a ship's page is `zkillboard.com/ship/<type id>/`, and its group's page is `zkillboard.com/group/<group id>/`.
- **CCP's ESI:** `https://esi.evetech.net/latest/universe/types/<type id>/` shows a type's `group_id`, and `https://esi.evetech.net/latest/universe/groups/<group id>/` shows the group's `category_id` and every type in it.

## The files

### `ships.json`

Each section is a list of `"id": "label"` entries.

| Section | Holds | Used for |
|---|---|---|
| `capital_groups` | ship groups | The Capital Pilots tab: titans, dreadnoughts, carriers, force auxiliaries, ... |
| `capital_types` | ship types | Single ships that count as capitals although their group doesn't (none built in) |
| `hauler_groups` | ship groups | The PI Pilots tab's haulers: T1 haulers and transports, not freighters |
| `hauler_exclusions` | ship types | Ships in those groups that aren't haulers (built in: the Noctis) |
| `fit_ignored_categories` | categories | Losses not shown on the Fits tab, by category, group, or single type: capsules, shuttles, haulers, mining ships, structures, ... |
| `fit_ignored_groups` | ship groups | |
| `fit_ignored_types` | ship types | |

### `structures.json`

All sections hold groups.

| Section | Used for |
|---|---|
| `upwell_structure_groups` | Citadels, engineering complexes, and refineries: the Structures tab and a wormhole system's last structure lost |
| `control_tower_groups` | Control towers, the same |
| `pos_battery_groups` | Control tower guns and ewar batteries, which appear on kills in place of their tower |
| `minor_structure_groups` | Structures with no guns, such as Metenox moon drills and customs offices: listed among structures destroyed, never as a system's last structure lost |

### `items.json`

All sections hold groups.

| Section | Used for |
|---|---|
| `pi_material_groups` | PI materials, which mark a loss as a PI hauler's on the PI Pilots tab |
| `smartbomb_groups` | Smartbombs, which the Ratters tab treats as "only bombed it" |
| `fit_merged_groups` | Modules whose types count as one when the Fits tab merges identical fits: built in, ECM, so fits that differ only in their jammers' flavors are one fit |

### `fit_types.json`

Every ship, module, charge, drone, implant, structure, and control tower, by type id, made from CCP's game data. The structure and capital checks above look up a ship's group here, so **a ship CCP has just added counts as a capital, structure, or anything else only once it is in this file.** Add it like this:

```json
{
    "types": {
        "99999": { "name": "New Dreadnought", "group": 485, "category": 6 }
    }
}
```

To show its fits, a new hull also needs its slots: `high_slots`, `mid_slots`, `low_slots`, `rig_slots`, `turret_slots`, and `launcher_slots`, plus `subsystem_slots` and `service_slots` where it has them. A new module needs a `slot`: `high`, `mid`, `low`, `rig`, `subsystem`, or `service`.

To fix one field of an existing type, give only that field: `{"types": {"587": {"group": 25}}}`.

The Fits tab's tags also read some fields from here: a type's `market` (its market group), and a hull's `rig_size` (1 small, 2 medium, 3 large, 4 capital) and `bonuses` (its role and skill bonuses as text).

### `fit_tag_rules.json`

How the Fits tab tags each fit: Ratting, Logi, Links, Cyno, Neut, Tackle, Shield, Active, and the rest. Each tag adds up weights from what is fitted, and applies once the total reaches its `threshold`.

```json
{
    "tags": {
        "Neut": {
            "meaning": "Energy neutralizers",
            "priority": 9,
            "role": true,
            "threshold": 3,
            "signals": [
                { "note": "neuts and nosferatus", "groups": [71, 68], "weight": 1,
                  "bonused_by": ["Energy Neutralizer", "Energy Nosferatu"], "bonused_weight": 1.5 }
            ]
        }
    }
}
```

A tag's fields:

| Field | Meaning |
|---|---|
| `meaning` | What it means, shown in the tab's key |
| `priority` | The order a fit shows its tags in, lowest first. A row shows its first role and the next three tags. Tags without one come after the rest. |
| `role` | `true` for a tag that can name a fit's row, as its first role by `priority` does |
| `threshold` | The total weight it needs to apply |
| `share_threshold` | The share of their racks its modules must fill: Firewaller's 0.75 needs smartbombs in three quarters of the high slots. A tag with both needs both. |
| `signals` | What gives it weight (below) |
| `requires` | Signals of which each must match something, or the tag doesn't apply. Their weights are ignored. |
| `forbids` | Signals of which none may match. Logi forbids guns, so a ship spider tanking with one remote repairer isn't logi. |
| `tag_weights` | Weight added for other tags the fit has, ie `{"Buffer": -5, "Passive": -5}` on Ratting, as a buffer or passive tank is rarely a ratting fit. Only tags without `tag_weights` of their own count. |
| `hidden` | `true` to never show it. The filter still finds it. The weapons, Close, and Long are hidden. |

A signal matches each fitted module and drone (or the hull, with `"hull": true`) by any of `types`, `groups`, `categories`, or `markets`. A market group also matches everything listed below it, so Pulse Lasers (558) holds every size. Each match adds `weight`, so three neutralizers add 3, and each drone in a stack counts. A signal can also have:

| Field | Meaning |
|---|---|
| `max` | The most this signal adds in all |
| `hull_sizes` | Counts only on hulls of these sizes, as in `rig_size` above |
| `bonused_by`, `bonused_weight` | On a hull with a bonus naming one of these words, each match adds `bonused_weight` instead. Matching ignores upper and lower case. Hovering the tag in the fitting window shows the extra on its own "hull bonus" line. |
| `note` | Your description. GoSleuth doesn't read it. |

Weights may be negative: Bait counts each gun against itself. As a guide, a weight of 1 is about one slot given over to the tag's job. Hovering a tag in the fitting window shows what gave it weight, each module's share, and the total against its threshold, which is the quickest way to see why a tag did or didn't apply.

At the top of the file:
- `bonus_ignores`: bonus lines containing one of these words never count for `bonused_by`. A "bonus to Sensor Dampener resistance" is about what the hull resists, not what it fits.
- `exclusive`: sets of tags of which only the first listed that the fit has applies. Built in, a fit is only one of Active, Passive, or Buffer, in that order: any repairer makes a tank active, however much buffer it has.

`signals`, `requires`, and `forbids` are lists, so your copy of one replaces it whole. To change only a tag's threshold, give just that: `{"tags": {"Neut": {"threshold": 2}}}`. To remove a tag, set it to `null`. Fleet Doctrine is not read from the fit; its numbers are under `fleet_doctrine`.

### `jspace_statics.json`

```json
{
    "wormholes": {
        "H296": { "origin": "C5", "destination": "C5", "total_mass": 3300000, "individual_mass": "X-Large", "mass_regen": 0, "time_to_live": "24" }
    },
    "systems": {
        "J001025": ["H296", "U210"]
    }
}
```

`wormholes` describes each wormhole type once, and `systems` gives each wormhole system's statics by code. A system's list is replaced whole, so give all of its statics. Every code a system lists must be in `wormholes`, whether built in or added by you. The `origin` of a system's statics sets the class GoSleuth shows for it.

### `space_classes.json`

| Section | Holds |
|---|---|
| `wormhole_region_classes` | The first letter of a wormhole region's name (`E` of `E-R00024`) to the class of every system in it. Used when a system's statics don't settle its class. |
| `region_space_labels` | Whole regions with their own kind of space, such as `"Pochven": "Triglavian"` |

### `inventory_flags.json`

Where each item on a killmail sat (high slot, cargo, drone bay, ...), as `slots`, a list of flag ranges. **`slots` is a list, so your copy replaces it whole:** copy the complete built-in list and edit that.

## Updated game data from a GoSleuth release

When a GoSleuth release has a newer `fit_types.json` but you don't want to update the app, you can drop the release's whole file into the `static_data` folder. A complete file works the same way as a partial one: every entry in it replaces or adds to the built-in data.

## Reporting a problem

If GoSleuth behaves oddly while you have files in `static_data`, say so in your report. If you send the debug zip (Preferences → Advanced → Debug mode), it lists the files in your `static_data` folder.
