package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// firstMode is the tactical destroyer mode the fitting window opens on: the tank it has under fire
const firstMode = "Defense"

/*
	fitStats works out a lost fit's numbers with the dogma engine, once per mode for a tactical destroyer

Parameters:
  - fit: the fit, with the implants of the capsule lost with it if any
  - abyssal: whether it has a mutated module, which counts at its base type's stats

Returns:
  - []FitStats: one entry, or one per mode with Defense first. nil if dogma.json doesn't have the hull.
*/
func fitStats(fit *retrieval.Fit, abyssal bool) []FitStats {
	in := fit.FittingInput()
	modes := slices.Clone(fitting.Modes(in.Hull))
	slices.SortStableFunc(modes, func(a, b comm.DogmaMode) int {
		switch {
		case a.Name == b.Name:
			return 0
		case a.Name == firstMode:
			return -1
		case b.Name == firstMode:
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	if len(modes) == 0 {
		modes = []comm.DogmaMode{{}} // no mode
	}

	all := make([]FitStats, 0, len(modes))
	for _, mode := range modes {
		in.Mode = mode.TypeID
		stats, err := fitting.Compute(in)
		if err != nil {
			comm.Logger.WithError(err).Debugf("No stats for hull %d", in.Hull)
			return nil
		}
		if abyssal {
			stats.Assumptions = append(stats.Assumptions, "Mutated modules at their base stats")
		}
		if len(stats.Unknown) > 0 {
			noun := "items"
			if len(stats.Unknown) == 1 {
				noun = "item"
			}
			stats.Assumptions = append(stats.Assumptions, fmt.Sprintf("%d %s newer than the game data left out", len(stats.Unknown), noun))
		}
		all = append(all, FitStats{Mode: mode.Name, Stats: stats})
	}
	return all
}
