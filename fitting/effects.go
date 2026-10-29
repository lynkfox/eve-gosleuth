package fitting

import (
	"github.com/lynkfox/go_eve_sleuth/comm"
)

// reactiveArmorHardenerEffect is the SDE name of the effect the Reactive Armor Hardener's resonance shifting is written in code for, since the SDE gives it no modifiers
const reactiveArmorHardenerEffect = "adaptiveArmorHardener"

/*
	activeEffectNamed finds a module's effect by name, if it is in force

Parameters:
  - fit: the fit
  - module: the module
  - name: the effect's SDE name

Returns:
  - bool: true if the module's type has the effect and it applies in the module's state
*/
func activeEffectNamed(fit *Fit, module *Item, name string) bool {
	if module == nil || module.Type == nil {
		return false
	}
	for _, effectID := range module.Type.Effects {
		if effect := comm.DogmaEffectOf(effectID); effect != nil && effect.Name == name && fit.Applies(module, effect) {
			return true
		}
	}
	return false
}

/*
	reactiveArmorHardeners lists the fit's modules whose Reactive Armor Hardener effect is in force, in fit order

Parameters:
  - fit: the fit

Returns:
  - []*Item: the modules
*/
func reactiveArmorHardeners(fit *Fit) []*Item {
	var found []*Item
	for _, module := range fit.Modules {
		if activeEffectNamed(fit, module, reactiveArmorHardenerEffect) {
			found = append(found, module)
		}
	}
	return found
}

/*
	handCodedModifiers returns the modifiers dogma.json can't express for this fit, ie the Reactive Armor Hardener's adapted resonances

	The first Reactive Armor Hardener is simulated against even damage and multiplies the ship's armor resonances by what it settles on (preMul, so it shares a stacking chain with a Damage Control). Any others only apply their own unadapted resonances: the game adapts each to the damage the ones before it have left, which isn't worked out.

Parameters:
  - fit: the fit, loaded but with nothing worked out yet

Returns:
  - []HandModifier: the modifiers, for Fit.AddHand
*/
func handCodedModifiers(fit *Fit) []HandModifier {
	var modifiers []HandModifier
	for position, rah := range reactiveArmorHardeners(fit) {
		rah := rah
		var settled *[4]float64 // worked out when first read, as reading the ship's resonances before every modifier is in place would keep them
		for t, name := range armorResonanceNames {
			attribute := comm.DogmaAttributeNamed(name)
			if attribute == nil {
				continue
			}
			t, attributeID := t, attribute.AttributeID
			modifiers = append(modifiers, HandModifier{
				Source:    rah,
				Target:    fit.Ship,
				Attribute: attributeID,
				Operation: 0,
				Value: func(fit *Fit) float64 {
					if position != 0 {
						return fit.Attr(rah, attributeID)
					}
					if settled == nil {
						result := adaptRAH(fit, rah)
						settled = &result
					}
					return settled[t]
				},
			})
		}
	}
	return modifiers
}

/*
	adaptRAH runs the Reactive Armor Hardener simulation on a fit's own numbers

Parameters:
  - fit: the fit
  - rah: the module

Returns:
  - [4]float64: the resonances it settles on
*/
func adaptRAH(fit *Fit, rah *Item) [4]float64 {
	var ship, own [4]float64
	for t, name := range armorResonanceNames {
		attribute := comm.DogmaAttributeNamed(name)
		if attribute == nil {
			ship[t], own[t] = 1, 1
			continue
		}
		ship[t] = fit.AttrExcluding(fit.Ship, attribute.AttributeID, rah)
		own[t] = fit.Attr(rah, attribute.AttributeID)
	}
	shift := fit.AttrNamed(rah, "resistanceShiftAmount") / 100
	return simulateRAH(ship, own, shift)
}
