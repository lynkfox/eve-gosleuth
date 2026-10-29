#!/usr/bin/env python3
"""Regenerates fitting/testdata/golden/*.eft and *.json from a headless Pyfa.
Run: venv/bin/python run_golden.py [--out DIR] [name ...]   (see SETUP.md)
"""
import sys, os, json, math, re, argparse
HERE = os.path.dirname(os.path.abspath(__file__))
ap = argparse.ArgumentParser()
ap.add_argument("--out", default=os.path.dirname(HERE))  # the golden folder this harness sits in
ap.add_argument("names", nargs="*")
args = ap.parse_args()
from pyfa_env import *
import sqlite3
from fits_def import FITS
from eos.saveddata.mode import Mode

PYFA = "v2.69.3 (d2722a4c)"
PYFA_DATA = "game client build 3624425, dumped 2026-10-17 (Pyfa staticdata/phobos/metadata.0.json); our SDE is build 3660036"
gdb = sqlite3.connect("file:" + os.path.join(HERE, "pyfa_build/eve.db") + "?mode=ro", uri=True)
def exists(n): return gdb.execute("select 1 from invtypes where typeName=?", (n,)).fetchone() is not None

SLOTS = {FittingSlot.HIGH: "high", FittingSlot.MED: "mid", FittingSlot.LOW: "low", FittingSlot.RIG: "rig",
         FittingSlot.SUBSYSTEM: "subsystem", FittingSlot.SERVICE: "service"}
def r6(x):
    if x is None: return 0
    return float(f"{x:.6g}")
EWAR = {2253: "ECMResistance", 2112: "sensorDampenerResistance", 2113: "weaponDisruptionResistance",
        2114: "targetPainterResistance", 2115: "stasisWebifierResistance", 2045: "energyWarfareResistance",
        2116: "remoteRepairImpedance", 2135: "remoteAssistanceImpedance", 6463: "remoteCapacitorImpedance"}
EWAR_ATTR_NAMES = {}  # attrID -> pyfa attribute name
for aid, nm in EWAR.items():
    EWAR_ATTR_NAMES[aid] = nm

problems = []
def check_lines(name, eft):
    """every module/implant line's type names exist; returns count of expected items"""
    n = 0
    for ln in eft.splitlines()[1:]:
        ln = ln.strip()
        if not ln or ln.startswith("["): continue
        parts = [p.strip() for p in ln.split(",", 1)] if not exists(ln) else [ln]
        for p in parts:
            if not exists(p):
                problems.append(f"{name}: unknown item name {p!r}")
        n += 1
    return n

def build(eft, mode_idx):
    f = eft_import(eft)
    return f

def eft_import(eft):
    f = eft.importEft(eft_text.splitlines()) if False else None

def compute(name, eft, mode_idx):
    f = globals()["eft_mod"].importEft(eft.splitlines())
    if f is None:
        problems.append(f"{name}: Pyfa refused the EFT")
        return None
    f.character = Character.getAll5()
    f.damagePattern = DamagePattern.getDefaultBuiltin()
    notes = []
    mode_item = None
    if mode_idx is not None:
        modes = f.ship.modes
        f.mode = modes[mode_idx]
        mode_item = f.mode.item
    # activate everything that can be, never overheat
    for m in f.modules:
        if m.isEmpty: continue
        if m.isValidState(State.ACTIVE):
            m.state = State.ACTIVE
        else:
            if any(e.isType("active") for e in m.item.effects.values()):
                notes.append(f"{m.item.typeName}: cannot be active, left {m.state.name}")
    f.clear()
    f.calculateModifiedAttributes()
    ship = f.ship
    g = lambda a: ship.getModifiedItemAttr(a)
    hp = f.hp; ehp = f.ehp
    def res(prefix):
        return [r6(g(prefix + k + "DamageResonance")) for k in ("em", "thermal", "kinetic", "explosive")]
    shield = {"hp": r6(hp["shield"]), "resonance": [r6(g("shield" + k + "DamageResonance")) for k in ("Em", "Thermal", "Kinetic", "Explosive")], "ehp": r6(ehp["shield"])}
    armor = {"hp": r6(hp["armor"]), "resonance": [r6(g("armor" + k + "DamageResonance")) for k in ("Em", "Thermal", "Kinetic", "Explosive")], "ehp": r6(ehp["armor"])}
    hull = {"hp": r6(hp["hull"]), "resonance": res(""), "ehp": r6(ehp["hull"])}
    tank = f.tank
    # remote reps: sum over real modules (fit.getRemoteReps() trips over empty dummy slots here)
    rr = [0, 0, 0, 0]
    for m in f.modules:
        if m.isEmpty: continue
        for i, v in enumerate(m.getRemoteReps(spoolOptions=None)): rr[i] += v
    stype = f.scanType
    sens = {"type": str(stype), "strength": r6(f.scanStrength)}
    ew = {}
    for aid, an in EWAR.items():
        if aid in ship.item.attributes or an in ship.item.attributes:
            ew[an] = r6(g(an))
    stats = {
        "shield": shield, "armor": armor, "hull": hull,
        "ehp": r6(hp_sum(ehp)),
        "local_rep": {"shield": r6(tank["shieldRepair"]), "armor": r6(tank["armorRepair"]), "hull": r6(tank["hullRepair"])},
        "passive_regen": r6(tank["passiveShield"]),
        "remote_rep": {"shield": r6(rr[0]), "armor": r6(rr[1]), "hull": r6(rr[2]), "capacitor": r6(rr[3])},
        "sensor": sens, "ewar_resist": ew,
    }
    mods = []
    for m in f.modules:
        if m.isEmpty: continue
        d = {"type": m.item.ID, "slot": SLOTS[m.slot]}
        if m.charge is not None: d["charge"] = m.charge.ID
        mods.append(d)
    inp = {"hull": ship.item.ID, "modules": mods}
    imps = [i.item.ID for i in f.implants]
    if imps: inp["implants"] = imps
    if mode_item is not None: inp["mode"] = mode_item.ID
    info = {"nmods": len(mods), "nimp": len(imps), "notes": notes,
            "states": [(m.item.typeName, m.state.name) for m in f.modules if not m.isEmpty],
            "charges": [(m.item.typeName, m.charge.name if m.charge else None) for m in f.modules if not m.isEmpty and m.item.group.category.name == "Module"],
            "mode": mode_item.typeName if mode_item else None,
            "slots": [int(g(a)) for a in ("hiSlots", "medSlots", "lowSlots", "rigSlots")],
            "imp_names": [i.item.typeName for i in f.implants], "rah": [(m.item.typeName) for m in f.modules if not m.isEmpty and m.item.group.name == "Armor Resistance Shift Hardener"]}
    return inp, stats, info

def hp_sum(e): return e["shield"] + e["armor"] + e["hull"]

import service.port.eft as eft_mod
os.makedirs(args.out, exist_ok=True)
written = []
for (name, eftt, td) in FITS:
    if args.names and not any(name.startswith(n) for n in args.names): continue
    expected = check_lines(name, eftt)
    if td:
        probe = eft_mod.importEft(eftt.splitlines())
        modes = probe.ship.modes
        names = [re.sub(r"^.* (\w+) Mode$", r"\1", m.item.typeName).lower() for m in modes]
        runs = list(enumerate(names))
    else:
        runs = [(None, None)]
    for idx, suffix in runs:
        fname = name if suffix is None else f"{name}_{suffix}"
        res = compute(fname, eftt, idx)
        if res is None: continue
        inp, stats, info = res
        if info["nmods"] + info["nimp"] != expected:
            from collections import Counter
            want = Counter(l.split(",")[0].strip() for l in eftt.splitlines()[1:] if l.strip())
            got = Counter(n for n, _ in info["states"]) + Counter(info["imp_names"])
            problems.append(f"{fname}: dropped {dict(want - got)}; Pyfa kept {info['nmods']} modules + {info['nimp']} implants of {expected} lines (slots hi/mid/low/rig = {info['slots']})")
        if info["notes"]: problems.append(f"{fname}: " + "; ".join(info["notes"]))
        want_charge = [l for l in eftt.splitlines() if "," in l and not l.startswith("[")]
        loaded = [c for c in info["charges"] if c[1]]
        if len(loaded) != len(want_charge):
            problems.append(f"{fname}: wanted {len(want_charge)} charges, loaded {loaded}")
        hullname = eftt.splitlines()[0][1:-1].split(',')[0]
        words = [w for w in name.split("_") if w.lower() != hullname.lower()]
        title = ", ".join(p for p in [hullname, " ".join(words), f"{suffix} mode" if suffix else ""] if p)
        doc = {"name": title, "pyfa": PYFA, "pyfa_data": PYFA_DATA, "input": inp, "expected": stats}
        path = os.path.join(args.out, fname + ".json")
        # keep the known differences written by hand into the file this replaces
        if os.path.exists(path):
            with open(path) as fh:
                known = json.load(fh).get("known_differences")
            if known:
                doc["known_differences"] = known
        with open(path + ".new", "w") as fh:
            json.dump(doc, fh, indent=2); fh.write("\n")
        os.replace(path + ".new", path)
        eft_out = re.sub(r"^\[([^,]+), .*\]", lambda mm: f"[{mm.group(1)}, {fname}]", eftt, count=1)
        with open(os.path.join(args.out, fname + ".eft"), "w") as fh:
            fh.write(eft_out)
        written.append(fname)
        key = [s for s in info["states"] if any(k in s[0] for k in ("Bastion","Siege","Triage","Assault Damage","Reactive","Damage Control","Ancillary"))]
        print(f"{fname:38s} slots={info['slots']} ehp={stats['ehp']:.0f} rah={info['rah']} mode={info['mode']} key={key}")
print("\nPROBLEMS:" if problems else "\nno problems")
for p in problems: print(" -", p)
print(len(written), "fits written")
