package fitting

import (
	"fmt"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// burstEffects are the SDE names of the effects that give warfare buffs: the command bursts, and a titan's effect generator. The buffs reach the fit's own ship, as they do in game.
var burstEffects = []string{
	"moduleBonusWarfareLinkArmor", "moduleBonusWarfareLinkShield", "moduleBonusWarfareLinkSkirmish",
	"moduleBonusWarfareLinkInfo", "moduleBonusWarfareLinkMining", "moduleTitanEffectGenerator",
}

// buffSlots are the four buffs a burst can give, each an id and a value on the module (a charge assigns them there)
const buffSlots = 4

// givenBuff is one warfare buff a fit gives itself: the strongest value any of its bursts gives, and that burst
type givenBuff struct {
	buff   *comm.DogmaBuff
	value  float64
	source *Item
}

/*
	burstModifiers returns the warfare buffs the fit's own active command bursts give its ship, as modifiers. Of several bursts giving one buff only the strongest counts, as the buff's aggregate says, and the buff is stacking penalized like any module's.

Parameters:
  - fit: the fit, loaded but with its hand modifiers not yet added

Returns:
  - []HandModifier: the buffs' modifiers, for Fit.AddHand
*/
func burstModifiers(fit *Fit) []HandModifier {
	buffs := map[int]*givenBuff{}
	order := []int{} // the buffs in the order first given, so the modifiers come out the same each time
	for _, module := range fit.Modules {
		if !givesBuffs(fit, module) {
			continue
		}
		for slot := 1; slot <= buffSlots; slot++ {
			buffID := int(fit.AttrNamed(module, fmt.Sprintf("warfareBuff%dID", slot)))
			buff := comm.DogmaBuffOf(buffID)
			if buff == nil {
				continue
			}
			value := fit.AttrNamed(module, fmt.Sprintf("warfareBuff%dValue", slot))
			given, seen := buffs[buffID]
			if !seen {
				buffs[buffID] = &givenBuff{buff, value, module}
				order = append(order, buffID)
				continue
			}
			if stronger(buff.Aggregate, value, given.value) {
				given.value, given.source = value, module
			}
		}
	}

	var modifiers []HandModifier
	for _, buffID := range order {
		given := buffs[buffID]
		add := func(target *Item, attribute int) {
			value := given.value
			modifiers = append(modifiers, HandModifier{Source: given.source, Target: target, Attribute: attribute, Operation: given.buff.Operation, Value: func(*Fit) float64 { return value }})
		}
		for _, attribute := range given.buff.Item {
			add(fit.Ship, attribute)
		}
		for _, attribute := range given.buff.Location {
			for _, module := range fit.fitted(false) {
				add(module, attribute)
			}
		}
		for _, target := range given.buff.LocationGroup {
			for _, module := range fit.fitted(false) {
				if module.Type.Group == target.Group {
					add(module, target.Attribute)
				}
			}
		}
		for _, target := range given.buff.LocationSkill {
			for _, module := range fit.requiring(fit.fitted(false), target.Skill) {
				add(module, target.Attribute)
			}
		}
	}
	return modifiers
}

// givesBuffs reports whether a module is a burst whose effect is in force
func givesBuffs(fit *Fit, module *Item) bool {
	for _, name := range burstEffects {
		if activeEffectNamed(fit, module, name) {
			return true
		}
	}
	return false
}

// stronger reports whether a buff's value beats the one it has, by the buff's aggregate: the highest for Maximum, the lowest for Minimum (a resist bonus is negative)
func stronger(aggregate string, value float64, than float64) bool {
	if aggregate == "Minimum" {
		return value < than
	}
	return value > than
}
