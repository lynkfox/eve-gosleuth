package fitting_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/stretchr/testify/assert"
)

// s4rahSpecs adds two Reactive Armor Hardeners and a hull with an uneven armor to the base test types
func s4rahSpecs() map[int]s4typeSpec {
	specs := s4baseSpecs()
	rah := map[string]float64{
		"armorEmDamageResonance": 0.85, "armorThermalDamageResonance": 0.85, "armorKineticDamageResonance": 0.85, "armorExplosiveDamageResonance": 0.85,
		"resistanceShiftAmount": 6, "duration": 10000,
	}
	specs[s4rahType] = s4typeSpec{attrs: rah, effects: []string{"adaptiveArmorHardener"}}
	specs[s4rahType2] = s4typeSpec{attrs: rah, effects: []string{"adaptiveArmorHardener"}}
	// armor with a hole to explosive and a wall at EM
	specs[s4oddHullType] = s4typeSpec{attrs: map[string]float64{
		"armorHP": 1000, "armorEmDamageResonance": 0.4, "armorThermalDamageResonance": 0.6, "armorKineticDamageResonance": 0.75, "armorExplosiveDamageResonance": 0.9,
	}}
	return specs
}

func s4rahSum(r [4]float64) float64 { return r[0] + r[1] + r[2] + r[3] }

// TestReactiveArmorHardener checks the hardener shifts its resistance toward the damage the hull takes most, and that this is recorded as an assumption
func TestReactiveArmorHardener(t *testing.T) {
	s4loadTestDogma(t, s4rahSpecs())
	bare := s4compute(t, fitting.Input{Hull: s4oddHullType})
	stats := s4compute(t, fitting.Input{Hull: s4oddHullType, Modules: []fitting.Module{s4module(s4rahType, 0)}})

	ratio := func(i int) float64 { return stats.Armor.Resonance[i] / bare.Armor.Resonance[i] }
	// explosive is the hole, so it gets the most resistance; EM the least
	assert.Less(t, ratio(3), ratio(2))
	assert.Less(t, ratio(2), ratio(1))
	assert.Less(t, ratio(1), ratio(0))
	assert.Less(t, ratio(3), 0.85)
	assert.Greater(t, ratio(0), 0.85)

	// the module only moves resistance around, so what it holds in total is what it started with (to the rounding of 3 decimals)
	assert.InDelta(t, 4*0.85, s4rahSum([4]float64{ratio(0), ratio(1), ratio(2), ratio(3)}), 0.005)
	assert.Contains(t, stats.Assumptions, "Reactive Armor Hardener adapted to uniform damage")
	assert.Greater(t, stats.Armor.EHP, bare.Armor.EHP)
}

// TestReactiveArmorHardenerEvenHull checks that on a hull with equal resists the loop averages back to even resonances
func TestReactiveArmorHardenerEvenHull(t *testing.T) {
	s4loadTestDogma(t, s4rahSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4rahType, 0)}})
	// the hull's armor is 0.5 across; the module settles on a loop whose average is close to even
	for i := range stats.Armor.Resonance {
		assert.InDelta(t, 0.5*0.85, stats.Armor.Resonance[i], 0.03)
	}
	assert.InDelta(t, 4*0.5*0.85, s4rahSum(stats.Armor.Resonance), 0.005)
}

// TestReactiveArmorHardenerDeterministic checks the same fit gives the same numbers, whichever order the modules are listed
func TestReactiveArmorHardenerDeterministic(t *testing.T) {
	s4loadTestDogma(t, s4rahSpecs())
	a := s4compute(t, fitting.Input{Hull: s4oddHullType, Modules: []fitting.Module{s4module(s4rahType, 0)}})
	b := s4compute(t, fitting.Input{Hull: s4oddHullType, Modules: []fitting.Module{s4module(s4prepType, 0), s4module(s4rahType, 0)}})
	assert.Equal(t, a.Armor.Resonance, b.Armor.Resonance)
}

// TestTwoReactiveArmorHardeners checks a second hardener adds its resistance (unadapted) rather than being ignored, and the assumption is listed once
func TestTwoReactiveArmorHardeners(t *testing.T) {
	s4loadTestDogma(t, s4rahSpecs())
	one := s4compute(t, fitting.Input{Hull: s4oddHullType, Modules: []fitting.Module{s4module(s4rahType, 0)}})
	two := s4compute(t, fitting.Input{Hull: s4oddHullType, Modules: []fitting.Module{s4module(s4rahType, 0), s4module(s4rahType2, 0)}})
	for i := range one.Armor.Resonance {
		assert.Less(t, two.Armor.Resonance[i], one.Armor.Resonance[i])
		// stacking penalized, so the second is worth no more than a whole extra 0.85: exactly that where the first gave all its resistance away (EM here), as the second then leads the chain
		assert.GreaterOrEqual(t, two.Armor.Resonance[i], one.Armor.Resonance[i]*0.85-1e-9)
	}
	count := 0
	for _, a := range two.Assumptions {
		if a == "Reactive Armor Hardener adapted to uniform damage" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}
