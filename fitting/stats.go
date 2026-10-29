package fitting

import (
	"math"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// the SDE names of each layer's HP attribute and its four resonances (EM, thermal, kinetic, explosive)
var (
	shieldAttributes = layerAttributes{"shieldCapacity", [4]string{"shieldEmDamageResonance", "shieldThermalDamageResonance", "shieldKineticDamageResonance", "shieldExplosiveDamageResonance"}}
	armorAttributes  = layerAttributes{"armorHP", armorResonanceNames}
	hullAttributes   = layerAttributes{"hp", [4]string{"emDamageResonance", "thermalDamageResonance", "kineticDamageResonance", "explosiveDamageResonance"}}
)

type layerAttributes struct {
	hp        string
	resonance [4]string
}

// the ship's ewar resistance and remote assistance impedance attributes, by SDE name: ECM, sensor dampeners, weapon disruptors, target painters, webs, energy warfare, remote repair, remote electronic assistance, remote capacitor
var ewarResistanceNames = []string{
	"ECMResistance", "sensorDampenerResistance", "weaponDisruptionResistance", "targetPainterResistance", "stasisWebifierResistance",
	"energyWarfareResistance", "remoteRepairImpedance", "remoteAssistanceImpedance", "remoteCapacitorImpedance",
}

// the four sensor strengths by SDE name, with the name each is reported as
var sensorStrengths = []struct{ attribute, name string }{
	{"scanRadarStrength", "Radar"}, {"scanLadarStrength", "Ladar"}, {"scanMagnetometricStrength", "Magnetometric"}, {"scanGravimetricStrength", "Gravimetric"},
}

// which layer a repair effect repairs, and the module attribute holding what it repairs each cycle
type repairKind struct {
	layer  string // shield, armor, hull, or capacitor
	amount string
	paste  bool // multiplied by chargedArmorDamageMultiplier when a charge is loaded
}

// These tables name dogma's own effects and attributes, which say how to read the data rather than being game data, so they stay in code (and an edited dogma.json can't rename them).

// the local repair effects by SDE name. The ancillary shield booster is the same as a plain one: its charge only feeds it.
var localRepairEffects = map[string]repairKind{
	"shieldBoosting":       {"shield", "shieldBonus", false},
	"fueledShieldBoosting": {"shield", "shieldBonus", false},
	"armorRepair":          {"armor", "armorDamageAmount", false},
	"fueledArmorRepair":    {"armor", "armorDamageAmount", true},
	"structureRepair":      {"hull", "structureDamageAmount", false},
}

// the remote repair effects by SDE name. The mutadaptive repairer is counted at its base amount.
var remoteRepairEffects = map[string]repairKind{
	"shipModuleRemoteShieldBooster":            {"shield", "shieldBonus", false},
	"shipModuleAncillaryRemoteShieldBooster":   {"shield", "shieldBonus", false},
	"shipModuleRemoteArmorRepairer":            {"armor", "armorDamageAmount", false},
	"shipModuleAncillaryRemoteArmorRepairer":   {"armor", "armorDamageAmount", true},
	"ShipModuleRemoteArmorMutadaptiveRepairer": {"armor", "armorDamageAmount", false},
	"shipModuleRemoteHullRepairer":             {"hull", "structureDamageAmount", false},
	"shipModuleRemoteCapacitorTransmitter":     {"capacitor", "powerTransferAmount", false},
}

const mutadaptiveEffect = "ShipModuleRemoteArmorMutadaptiveRepairer"

// the neutralizer and nosferatu effects by SDE name, with the module attribute holding the capacitor each takes per cycle
var neutEffects = map[string]string{
	"energyNeutralizerFalloff": "energyNeutralizerAmount",
	"energyNosferatuFalloff":   "powerTransferAmount",
}

/*
	computeStats reads a fit's stats from the engine: resists and EHP per layer, local and remote repair, passive regen, sensors, and ewar resistance

Parameters:
  - fit: the fit, its hand-coded modifiers added

Returns:
  - *Stats: its stats, with what was assumed and the types left out
*/
func computeStats(fit *Fit) *Stats {
	stats := &Stats{Unknown: fit.Unknown}
	stats.Shield = readLayer(fit, shieldAttributes)
	stats.Armor = readLayer(fit, armorAttributes)
	stats.Hull = readLayer(fit, hullAttributes)
	stats.EHP = stats.Shield.EHP + stats.Armor.EHP + stats.Hull.EHP

	mutadaptive := false
	for _, module := range fit.Modules {
		if module == nil || module.Type == nil {
			continue
		}
		for _, effectID := range module.Type.Effects {
			effect := comm.DogmaEffectOf(effectID)
			if effect == nil || !fit.Applies(module, effect) {
				continue
			}
			if kind, ok := localRepairEffects[effect.Name]; ok {
				addRepair(fit, &stats.LocalRep, module, effect, kind)
			}
			if kind, ok := remoteRepairEffects[effect.Name]; ok {
				addRepair(fit, &stats.RemoteRep, module, effect, kind)
				mutadaptive = mutadaptive || effect.Name == mutadaptiveEffect
			}
			if amount, ok := neutEffects[effect.Name]; ok {
				if cycle := fit.Attr(module, effect.Duration) / 1000; effect.Duration != 0 && cycle > 0 {
					stats.Neut += fit.AttrNamed(module, amount) / cycle
				}
			}
		}
	}

	if recharge := fit.AttrNamed(fit.Ship, "shieldRechargeRate") / 1000; recharge > 0 {
		// the shield regenerates fastest at a quarter full, where it gives 2.5 times its capacity over the recharge time
		stats.PassiveRegen = 2.5 * stats.Shield.HP / recharge
	}

	stats.Sensor = readSensor(fit)
	stats.EwarResist = map[string]float64{}
	for _, name := range ewarResistanceNames {
		if comm.DogmaAttributeNamed(name) != nil {
			stats.EwarResist[name] = fit.AttrNamed(fit.Ship, name)
		}
	}

	if len(reactiveArmorHardeners(fit)) > 0 {
		stats.Assumptions = append(stats.Assumptions, "Reactive Armor Hardener adapted to uniform damage")
	}
	if mutadaptive {
		stats.Assumptions = append(stats.Assumptions, "Mutadaptive remote armor repairer at its base amount, without spool-up")
	}
	return stats
}

// readLayer reads one layer of the ship's tank. Its EHP is against even damage, so its HP over the mean of its resonances.
func readLayer(fit *Fit, attributes layerAttributes) Layer {
	layer := Layer{HP: fit.AttrNamed(fit.Ship, attributes.hp)}
	total := 0.0
	for t, name := range attributes.resonance {
		layer.Resonance[t] = fit.AttrNamed(fit.Ship, name)
		total += layer.Resonance[t]
	}
	if mean := total / 4; mean > 0 {
		layer.EHP = layer.HP / mean
	}
	return layer
}

// addRepair adds what one module's repair effect gives each second to the layer it repairs, its amount over its cycle time
func addRepair(fit *Fit, repair *Repair, module *Item, effect *comm.DogmaEffect, kind repairKind) {
	cycle := fit.Attr(module, effect.Duration) / 1000
	if effect.Duration == 0 || cycle <= 0 {
		return
	}
	amount := fit.AttrNamed(module, kind.amount)
	if kind.paste && module.Charge != nil {
		if multiplier := fit.AttrNamed(module, "chargedArmorDamageMultiplier"); multiplier > 0 {
			amount *= multiplier
		}
	}
	perSecond := amount / cycle
	switch kind.layer {
	case "shield":
		repair.Shield += perSecond
	case "armor":
		repair.Armor += perSecond
	case "hull":
		repair.Hull += perSecond
	case "capacitor":
		repair.Capacitor += perSecond
	}
}

// readSensor reads the ship's strongest sensor strength and which kind it is, Multispectral if two or more kinds tie
func readSensor(fit *Fit) Sensor {
	var sensor Sensor
	ties := 0
	for _, strength := range sensorStrengths {
		value := fit.AttrNamed(fit.Ship, strength.attribute)
		switch {
		case value > sensor.Strength+1e-9:
			sensor = Sensor{Type: strength.name, Strength: value}
			ties = 1
		case value > 0 && math.Abs(value-sensor.Strength) <= 1e-9:
			ties++
		}
	}
	if ties > 1 {
		sensor.Type = "Multispectral"
	}
	return sensor
}
