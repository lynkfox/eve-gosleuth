package gui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
)

/*
	buildSummary lays a merged fit's losses side by side for the fitting window's first page and the PDF

Parameters:
  - losses: the fit's losses, in their FitGroup order (most implants first, then newest), each with its Fit and Stats

Returns:
  - *FitSummary: the summary, nil for fewer than two losses
*/
func buildSummary(losses []FitLoss) *FitSummary {
	if len(losses) < 2 {
		return nil
	}
	common := commonLossIndex(losses)
	commonFit := losses[common].Fit

	pilots := map[string]bool{}
	for _, loss := range losses {
		pilots[loss.PilotName] = true
	}

	implants, implanted := summaryImplants(losses)
	summary := &FitSummary{
		Fit: FitLayout{
			Highs:      summaryRack(losses, common, func(l FitLayout) []*FitSlot { return l.Highs }),
			Mids:       summaryRack(losses, common, func(l FitLayout) []*FitSlot { return l.Mids }),
			Lows:       summaryRack(losses, common, func(l FitLayout) []*FitSlot { return l.Lows }),
			Rigs:       summaryRack(losses, common, func(l FitLayout) []*FitSlot { return l.Rigs }),
			Subsystems: summaryRack(losses, common, func(l FitLayout) []*FitSlot { return l.Subsystems }),
			Drones:     slices.Clone(commonFit.Drones),
			Fighters:   slices.Clone(commonFit.Fighters),
			Boosters:   []FitPart{},
			Implants:   implants,
		},
		Implanted:  implanted,
		Pilots:     len(pilots),
		CommonLoss: common,
		Stats:      summaryStats(losses, common),
	}
	if summary.Fit.Drones == nil {
		summary.Fit.Drones = []FitPart{}
	}
	if summary.Fit.Fighters == nil {
		summary.Fit.Fighters = []FitPart{}
	}
	return summary
}

/*
	summaryImplants lists every implant seen on any of the losses, once each, in the order of their slots (those fit_types.json does not have last), with how many losses had it as its Quantity

Parameters:
  - losses: the fit's losses, each with its implants (a capsule's, see podsWithShips)

Returns:
  - []FitPart: the implants, empty for none
  - int: how many losses had any implants
*/
func summaryImplants(losses []FitLoss) ([]FitPart, int) {
	implants := []FitPart{}
	at := map[int]int{}
	implanted := 0
	for _, loss := range losses {
		if len(loss.Fit.Implants) > 0 {
			implanted++
		}
		seen := map[int]bool{}
		for _, implant := range loss.Fit.Implants {
			if seen[implant.ID] {
				continue
			}
			seen[implant.ID] = true
			if index, found := at[implant.ID]; found {
				implants[index].Quantity++
				continue
			}
			at[implant.ID] = len(implants)
			part := implant
			part.Quantity = 1
			implants = append(implants, part)
		}
	}
	slot := func(part FitPart) int {
		if fitType := comm.FitTypeOf(part.ID); fitType != nil && fitType.HeadSlot > 0 {
			return fitType.HeadSlot
		}
		return math.MaxInt
	}
	slices.SortStableFunc(implants, func(a, b FitPart) int { return cmp.Compare(slot(a), slot(b)) })
	return implants, implanted
}

// commonLossIndex is the index of the loss whose exact fit (its own module types, merged groups still one module) is the most common, ties to the group whose first loss comes first
func commonLossIndex(losses []FitLoss) int {
	counts := map[string]int{}
	first := map[string]int{}
	order := []string{}
	for index, loss := range losses {
		key := layoutKey(loss.Fit)
		if _, seen := first[key]; !seen {
			first[key] = index
			order = append(order, key)
		}
		counts[key]++
	}
	best := order[0]
	for _, key := range order[1:] {
		if counts[key] > counts[best] {
			best = key
		}
	}
	return first[best]
}

// layoutKey is the exact fit of a layout: its module types in each rack, in any order
func layoutKey(layout FitLayout) string {
	parts := []string{}
	for _, rack := range [][]*FitSlot{layout.Highs, layout.Mids, layout.Lows, layout.Rigs, layout.Subsystems} {
		ids := []int{}
		for _, slot := range rack {
			if slot != nil {
				ids = append(ids, moduleKey(slot.Module.ID, true))
			}
		}
		slices.Sort(ids)
		parts = append(parts, fmt.Sprint(ids))
	}
	return strings.Join(parts, "|")
}

// summaryRack builds one rack of a summary: a slot for each module of the common loss by base type name, empty slots last, each with the variants every loss fitted
func summaryRack(losses []FitLoss, common int, rackOf func(FitLayout) []*FitSlot) []*FitSlot {
	commonRack := rackOf(losses[common].Fit)

	// how many losses fitted each variant of each slot key, and the common loss's variant of it
	counts := map[int]map[int]int{}
	for _, loss := range losses {
		seen := map[[2]int]bool{}
		for _, slot := range rackOf(loss.Fit) {
			if slot == nil {
				continue
			}
			key := moduleKey(slot.Module.ID, false)
			if seen[[2]int{key, slot.Module.ID}] {
				continue
			}
			seen[[2]int{key, slot.Module.ID}] = true
			if counts[key] == nil {
				counts[key] = map[int]int{}
			}
			counts[key][slot.Module.ID]++
		}
	}
	parts := map[int]FitPart{}
	for _, loss := range losses {
		for _, slot := range rackOf(loss.Fit) {
			if slot != nil {
				parts[slot.Module.ID] = slot.Module
			}
		}
	}
	commonOf := map[int]int{} // the slot key to the variant the common loss fitted
	for _, slot := range commonRack {
		if slot != nil {
			commonOf[moduleKey(slot.Module.ID, false)] = slot.Module.ID
		}
	}

	variantsOf := map[int][]FitVariant{}
	for key, variants := range counts {
		list := []FitVariant{}
		for id, count := range variants {
			list = append(list, FitVariant{FitPart: parts[id], Losses: count})
		}
		slices.SortFunc(list, func(a, b FitVariant) int {
			if byLosses := cmp.Compare(b.Losses, a.Losses); byLosses != 0 {
				return byLosses
			}
			if aCommon, bCommon := a.ID == commonOf[key], b.ID == commonOf[key]; aCommon != bCommon {
				if aCommon {
					return -1
				}
				return 1
			}
			if byName := strings.Compare(a.Name, b.Name); byName != 0 {
				return byName
			}
			return cmp.Compare(a.ID, b.ID)
		})
		variantsOf[key] = list
	}

	rack := make([]*FitSlot, len(commonRack))
	filled := []*FitSlot{}
	for _, slot := range commonRack {
		if slot == nil {
			continue
		}
		key := moduleKey(slot.Module.ID, false)
		filled = append(filled, &FitSlot{Module: basePart(slot.Module, key, variantsOf[key]), Variants: slices.Clone(variantsOf[key])})
	}
	slices.SortStableFunc(filled, func(a, b *FitSlot) int {
		if byName := strings.Compare(a.Module.Name, b.Module.Name); byName != 0 {
			return byName
		}
		return cmp.Compare(a.Module.ID, b.Module.ID)
	})
	copy(rack, filled)
	return rack
}

// basePart is the part a summary slot shows: the base type of the common loss's module, or its most common variant when the base is not in fit_types.json (or the key is a merged group)
func basePart(module FitPart, key int, variants []FitVariant) FitPart {
	base := key
	if key < 0 { // a merged group: the common loss's module's own base type
		base = comm.BaseType(module.ID)
	}
	if fitType := comm.FitTypeOf(base); fitType != nil && fitType.Name != "" {
		return FitPart{ID: base, Name: fitType.Name, ImageURL: api.TypeImageURL(base), EveRefURL: api.EveRefTypeURL(base)}
	}
	if len(variants) > 0 {
		top := variants[0].FitPart
		return FitPart{ID: top.ID, Name: top.Name, ImageURL: top.ImageURL, EveRefURL: top.EveRefURL}
	}
	return FitPart{ID: module.ID, Name: module.Name, ImageURL: module.ImageURL, EveRefURL: module.EveRefURL}
}

// summaryStats is each mode's range over the losses that have stats, in the order the losses list their modes
func summaryStats(losses []FitLoss, common int) []FitStatsRange {
	modes := []string{}
	for _, loss := range losses {
		for _, stats := range loss.Stats {
			if stats.Stats != nil && !slices.Contains(modes, stats.Mode) {
				modes = append(modes, stats.Mode)
			}
		}
	}
	ranges := []FitStatsRange{}
	for _, mode := range modes {
		all := []*fitting.Stats{}
		var reference *fitting.Stats
		for index, loss := range losses {
			for _, stats := range loss.Stats {
				if stats.Mode != mode || stats.Stats == nil {
					continue
				}
				all = append(all, stats.Stats)
				if reference == nil || index == common {
					reference = stats.Stats
				}
				break
			}
		}
		minimum, maximum := statsBounds(all, reference)
		ranges = append(ranges, FitStatsRange{Mode: mode, Min: minimum, Max: maximum})
	}
	return ranges
}

// statsBounds is the lowest and highest of every number in the stats, as new structs. The sensor type, assumptions, and unknown are the reference's; boosted is if any are.
func statsBounds(all []*fitting.Stats, reference *fitting.Stats) (*fitting.Stats, *fitting.Stats) {
	fresh := func() *fitting.Stats {
		out := &fitting.Stats{
			Sensor:      fitting.Sensor{Type: reference.Sensor.Type},
			EwarResist:  map[string]float64{},
			Assumptions: slices.Clone(reference.Assumptions),
			Unknown:     slices.Clone(reference.Unknown),
		}
		for _, stats := range all {
			out.Boosted = out.Boosted || stats.Boosted
		}
		return out
	}
	low, high := fresh(), fresh()
	first := true
	pick := func(lo, hi *float64, value float64) {
		if first || value < *lo {
			*lo = value
		}
		if first || value > *hi {
			*hi = value
		}
	}
	for _, stats := range all {
		for _, field := range statsFields(stats, low, high) {
			pick(field.lo, field.hi, field.value)
		}
		for key, value := range stats.EwarResist {
			lo, hi := low.EwarResist[key], high.EwarResist[key]
			if _, seen := low.EwarResist[key]; !seen {
				lo, hi = value, value
			} else {
				lo, hi = min(lo, value), max(hi, value)
			}
			low.EwarResist[key], high.EwarResist[key] = lo, hi
		}
		first = false
	}
	return low, high
}

type statsField struct {
	value  float64
	lo, hi *float64
}

// statsFields pairs each number of stats with the same one of the low and high structs
func statsFields(stats, low, high *fitting.Stats) []statsField {
	fields := []statsField{}
	add := func(value float64, lo, hi *float64) { fields = append(fields, statsField{value, lo, hi}) }
	layers := func(layer fitting.Layer, lo, hi *fitting.Layer) {
		add(layer.HP, &lo.HP, &hi.HP)
		add(layer.EHP, &lo.EHP, &hi.EHP)
		for index := range layer.Resonance {
			add(layer.Resonance[index], &lo.Resonance[index], &hi.Resonance[index])
		}
	}
	layers(stats.Shield, &low.Shield, &high.Shield)
	layers(stats.Armor, &low.Armor, &high.Armor)
	layers(stats.Hull, &low.Hull, &high.Hull)
	add(stats.EHP, &low.EHP, &high.EHP)
	repair := func(rep fitting.Repair, lo, hi *fitting.Repair) {
		add(rep.Shield, &lo.Shield, &hi.Shield)
		add(rep.Armor, &lo.Armor, &hi.Armor)
		add(rep.Hull, &lo.Hull, &hi.Hull)
		add(rep.Capacitor, &lo.Capacitor, &hi.Capacitor)
	}
	repair(stats.LocalRep, &low.LocalRep, &high.LocalRep)
	repair(stats.RemoteRep, &low.RemoteRep, &high.RemoteRep)
	add(stats.PassiveRegen, &low.PassiveRegen, &high.PassiveRegen)
	add(stats.Neut, &low.Neut, &high.Neut)
	add(stats.Sensor.Strength, &low.Sensor.Strength, &high.Sensor.Strength)
	return fields
}
