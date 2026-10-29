# Golden fits

Reference numbers from Pyfa for `fitting.Compute`, checked by the golden test. Each fit has two files with the same name:

- `<name>.eft` is the fit as EFT text, for people and for re-running Pyfa.
- `<name>.json` is the test case:

```json
{
  "name": "Rifter, shield buffer",
  "pyfa": "v2.69.3 (d2722a4c)",
  "pyfa_data": "the SDE build or date of Pyfa's own eve.db",
  "input": { "hull": 587, "modules": [{ "type": 3841, "slot": "mid" }], "implants": [], "mode": 0 },
  "expected": { "shield": { "hp": 0, "resonance": [1, 1, 1, 1], "ehp": 0 }, "...": "every field of fitting.Stats" },
  "known_differences": [{ "field": "armor.resonance", "reason": "why our number differs from Pyfa's on purpose" }]
}
```

- `input` is a `fitting.Input` and `expected` a `fitting.Stats`, in their json form.
- Pyfa computes with an All 5 character, every module active (none overheated), no boosts, and uniform damage for EHP and the Reactive Armor Hardener. A tactical destroyer has one file per mode.
- `known_differences` names fields left out of the check, each with its reason, ie where Pyfa skips a dogma operation we implement.
