// Package fitting is a dogma engine: it works out a fit's stats (resists, EHP, repair, sensors, ewar resistance) from static_data/dogma.json, with every skill at V, every module active, no heat and no fleet boosts
package fitting

import (
	"errors"
	"fmt"
	"math"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// ErrNotFinite is returned when a stat works out to NaN or infinity, which only a broken dogma.json can cause, and which json can't carry to the window
var ErrNotFinite = errors.New("a stat is not a finite number")

// ErrUnknownHull is returned when dogma.json has no type for the fit's hull, ie a hull newer than the SDE it was made from
var ErrUnknownHull = errors.New("the hull is not in dogma.json")

// Input is a fit as the engine reads it: type ids only, so it doesn't depend on how a killmail was read
type Input struct {
	Hull     int      `json:"hull"`
	Modules  []Module `json:"modules"`            // highs, mids, lows, rigs, subsystems, and services, in any order
	Implants []int    `json:"implants,omitempty"` // only those known to be in the pilot's head, ie from the capsule lost with the ship
	Mode     int      `json:"mode,omitempty"`     // a tactical destroyer's mode (comm.DogmaModesOf), 0 for none
}

// Module is a fitted module and the charge loaded in it
type Module struct {
	Type   int          `json:"type"`
	Charge int          `json:"charge,omitempty"` // 0 if unloaded
	Slot   comm.FitSlot `json:"slot"`
}

// Stats are a fit's numbers. Resonances are fractions of damage taken (1 is no resist), in the order EM, thermal, kinetic, explosive.
type Stats struct {
	Shield Layer   `json:"shield"`
	Armor  Layer   `json:"armor"`
	Hull   Layer   `json:"hull"`
	EHP    float64 `json:"ehp"` // omni, the three layers together

	LocalRep     Repair  `json:"local_rep"`     // HP per second from its own repairers while all cycle, ancillary ones with paste if loaded
	PassiveRegen float64 `json:"passive_regen"` // peak shield HP per second from its recharge
	RemoteRep    Repair  `json:"remote_rep"`    // HP (and GJ of capacitor) per second it gives others
	Neut         float64 `json:"neut"`          // GJ of capacitor per second its neutralizers and nosferatus take from a target, before the target's resistance

	Sensor     Sensor             `json:"sensor"`
	EwarResist map[string]float64 `json:"ewar_resist"` // keyed by the hull's ewar resistance attribute name, ie ECMResistance: the fraction of the ewar that still lands, so 1 is no resistance

	Assumptions []string `json:"assumptions,omitempty"` // what was assumed that the killmail doesn't say, ie that the Reactive Armor Hardener settled against uniform damage
	Unknown     []int    `json:"unknown,omitempty"`     // module, charge, and implant types dogma.json doesn't have, counted as not fitted
}

// Layer is one layer of tank
type Layer struct {
	HP        float64    `json:"hp"`
	Resonance [4]float64 `json:"resonance"`
	EHP       float64    `json:"ehp"` // against uniform damage
}

// Repair is HP per second for each layer, and capacitor for remote repair
type Repair struct {
	Shield    float64 `json:"shield"`
	Armor     float64 `json:"armor"`
	Hull      float64 `json:"hull"`
	Capacitor float64 `json:"capacitor,omitempty"`
}

// Sensor is a fit's sensor strength
type Sensor struct {
	Type     string  `json:"type"` // Radar, Ladar, Magnetometric, Gravimetric, or Multispectral when two or more tie
	Strength float64 `json:"strength"`
}

/*
	Compute works out a fit's stats

Parameters:
  - in: the fit

Returns:
  - *Stats: its stats
  - error: ErrUnknownHull if dogma.json has no type for the hull, ErrNotFinite if a stat is NaN or infinite, or the panic the engine raised (recorded in crash.log)
*/
func Compute(in Input) (stats *Stats, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			comm.RecordPanic("computing fit stats", recovered)
			stats, err = nil, fmt.Errorf("computing the stats of hull %d: %v", in.Hull, recovered)
		}
	}()

	fit, err := NewFit(in)
	if err != nil {
		return nil, err
	}
	fit.AddHand(handCodedModifiers(fit)...)
	stats = computeStats(fit)
	if !stats.finite() {
		return nil, fmt.Errorf("%w for hull %d", ErrNotFinite, in.Hull)
	}
	return stats, nil
}

// finite reports whether every number in the stats is finite
func (s *Stats) finite() bool {
	numbers := []float64{s.EHP, s.PassiveRegen, s.Sensor.Strength,
		s.LocalRep.Shield, s.LocalRep.Armor, s.LocalRep.Hull, s.LocalRep.Capacitor,
		s.RemoteRep.Shield, s.RemoteRep.Armor, s.RemoteRep.Hull, s.RemoteRep.Capacitor}
	for _, layer := range []Layer{s.Shield, s.Armor, s.Hull} {
		numbers = append(numbers, layer.HP, layer.EHP)
		numbers = append(numbers, layer.Resonance[:]...)
	}
	for _, value := range s.EwarResist {
		numbers = append(numbers, value)
	}
	for _, value := range numbers {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

/*
	Modes lists a tactical destroyer's modes, for computing its stats in each

Parameters:
  - hull: the hull's type id

Returns:
  - []comm.DogmaMode: its modes, nil for a hull without modes
*/
func Modes(hull int) []comm.DogmaMode {
	return comm.DogmaModesOf(hull)
}
