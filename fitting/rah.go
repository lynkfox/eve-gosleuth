package fitting

import (
	"math"
	"sort"
)

// the order the Reactive Armor Hardener's resonances are kept in, and the ship's armor resonance attribute for each
var armorResonanceNames = [4]string{"armorEmDamageResonance", "armorThermalDamageResonance", "armorKineticDamageResonance", "armorExplosiveDamageResonance"}

const (
	rahMaxCycles     = 50   // the simulation stops here if it never repeats
	rahLoopTail      = 20   // with no loop found, the last this many cycles are averaged
	rahLoopTolerance = 1e-6 // how close two cycles' resonances must be to count as the same
)

// rahTieOrder is the order damage types are listed in before sorting by damage taken, so that types taking the same damage are ranked the way the game ranks them (EM, explosive, kinetic, thermal): the sort keeps this order for ties
var rahTieOrder = [4]int{0, 3, 2, 1}

/*
	simulateRAH works out the resonances a Reactive Armor Hardener settles on against even damage of all four types

	Each cycle the module takes damage per type as the ship's resonance times its own, ranks the types, and shifts resistance from the two taking least to the two taking most. When only one or two types take any damage, everything the others hold is moved at once to the ones taking it. Cycles repeat until a state recurs; the states in that loop are averaged. The ship's resonances are the ones without the module, and other modules' stacking penalties are not considered.

Parameters:
  - ship: the ship's armor resonances without this module, in the order EM, thermal, kinetic, explosive
  - own: the module's own resonances to start from
  - shift: the most resonance a cycle moves, as a fraction (resistanceShiftAmount / 100)

Returns:
  - [4]float64: the resonances the module settles on, rounded to 3 decimals
*/
func simulateRAH(ship, own [4]float64, shift float64) [4]float64 {
	// uniform damage: each type is a quarter of it, which only scales every figure alike, so the ranking is what the ship's resonance alone decides
	const share = 0.25
	var taken [4]float64
	for i := range taken {
		taken[i] = share * ship[i]
	}

	current := own
	var cycles [][4]float64
	loopStart := -rahLoopTail
	for len(cycles) < rahMaxCycles {
		order := rahTieOrder
		damage := func(typeIndex int) float64 { return taken[typeIndex] * current[typeIndex] }
		sort.SliceStable(order[:], func(a, b int) bool { return damage(order[a]) < damage(order[b]) })

		var change [4]float64
		switch {
		case damage(order[2]) == 0:
			// one type takes damage: it takes the three others' resistance
			for rank := 0; rank < 3; rank++ {
				change[rank] = 1 - current[order[rank]]
			}
			change[3] = -(change[0] + change[1] + change[2])
		case damage(order[1]) == 0:
			// two types take damage: they share the other two's resistance
			change[0] = 1 - current[order[0]]
			change[1] = 1 - current[order[1]]
			change[2] = -(change[0] + change[1]) / 2
			change[3] = change[2]
		default:
			change[0] = math.Min(shift, 1-current[order[0]])
			change[1] = math.Min(shift, 1-current[order[1]])
			change[2] = -(change[0] + change[1]) / 2
			change[3] = change[2]
		}
		next := current
		for rank := 0; rank < 4; rank++ {
			next[order[rank]] = current[order[rank]] + change[rank]
		}
		current = next

		for i, seen := range cycles {
			same := true
			for t := range current {
				if math.Abs(current[t]-seen[t]) > rahLoopTolerance {
					same = false
				}
			}
			if same {
				loopStart = i
				break
			}
		}
		if loopStart >= 0 {
			break
		}
		cycles = append(cycles, current)
	}

	tail := cycles
	if loopStart >= 0 {
		tail = cycles[loopStart:]
	} else if len(tail) > rahLoopTail {
		tail = tail[len(tail)-rahLoopTail:]
	}
	var average [4]float64
	if len(tail) == 0 {
		return own
	}
	for _, state := range tail {
		for t := range average {
			average[t] += state[t]
		}
	}
	for t := range average {
		average[t] = math.Round(average[t]/float64(len(tail))*1000) / 1000
	}
	return average
}
