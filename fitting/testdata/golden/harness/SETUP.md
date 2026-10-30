# Regenerating the golden fits

These scripts ran Pyfa without its window to write the `*.eft` and `*.json` beside this folder (2026-10-29, Pyfa v2.69.3, commit d2722a4c). Pyfa is GPLv3: it is run here to get numbers, and none of its code is in the app.

- `fits_def.py` holds every fit as EFT text with its file name. Add a fit there.
- `run_golden.py` imports each fit with Pyfa's `service.port.eft.importEft`, uses an All 5 character, forces every module that can be active to active (never overheated), marks an active Assault Damage Control `"state": "active"` in the input (the app leaves it online), sets the Uniform damage pattern before computing (so the Reactive Armor Hardener settles against it), sets each tactical destroyer mode in turn, and writes `fitting.Input` and `fitting.Stats` json. It prints "no problems" only when every line resolved, every module and implant was kept, and every charge loaded.
- `pyfa_env.py` starts Pyfa headless, and `stubs/sitecustomize_wx.py` hands back a stand-in for any `wx`, `gui`, or `graphs` module Pyfa's own folder doesn't supply.

Setup, in this folder (the checkout, venv, and database are not committed):

1. `git clone https://github.com/pyfa-org/Pyfa pyfa_build && git -C pyfa_build checkout d2722a4c` (or a newer Pyfa; record its version in `run_golden.py`'s `PYFA` and `PYFA_DATA`).
2. `python3 -m venv venv && venv/bin/pip install -r pyfa_build/requirements.txt` (skip wxPython if it won't build: it is never imported).
3. `(cd pyfa_build && ../venv/bin/python db_update.py)` builds `pyfa_build/eve.db` from Pyfa's bundled data.
4. `venv/bin/python run_golden.py [fit-name ...]`, then `go test ./fitting -run Golden`.

Pyfa's data may be older than our `dogma.json`: a golden file's `pyfa_data` says which build it used. A number that differs from ours on purpose goes in that file's `known_differences`, with the reason; regenerating keeps them.
