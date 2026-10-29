package retrieval

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// FitTag is a tag a fit has, read from its modules by the rules in fit_tag_rules.json
type FitTag struct {
	Name     string   `json:"name"`
	Meaning  string   `json:"meaning"`
	Priority int      `json:"priority"` // the order the fit shows its tags in, lowest first, 0 after the rest
	Role     bool     `json:"role"`     // it can name the fit's row
	Hidden   bool     `json:"hidden"`   // never shown, only found by the filter
	Weight   float64  `json:"weight"`   // its signals' weights added up
	Share    float64  `json:"share"`    // the same as a share of the racks its modules are in, ie 1 for a full rack
	Reasons  []string `json:"reasons"`  // what gave it weight, ie "Warp Scrambler II ×2"
}

// tagPart is one thing on a fit a signal can match: a module, a drone stack, or the hull
type tagPart struct {
	item     *Item
	fitType  *comm.FitType
	quantity int
	rack     float64 // the slots in its rack, which its weight is a share of
}

/*
	Tags reads the fit's tags by the rules in fit_tag_rules.json: those whose weight reaches their threshold, with each of the rule's requires matched and none of its forbids, only the heaviest of each exclusive set.
	The Fleet Doctrine tag is read from battles, not the fit, so it is never here.

Returns:
  - []FitTag: by Priority (those without one last), then name. Empty for a nil fit.
*/
func (f *Fit) Tags() []FitTag {
	tags := []FitTag{}
	if f == nil {
		return tags
	}
	rules := comm.FitTagRules()
	hull, parts := f.tagParts()
	for name, rule := range rules.Tags {
		if rule.FleetDoctrine != nil {
			continue
		}
		if tag, applies := scoreTag(name, rule, hull, parts); applies {
			tags = append(tags, tag)
		}
	}
	tags = keepHeaviestExclusive(tags, rules.Exclusive)

	slices.SortFunc(tags, func(a, b FitTag) int {
		if a.Priority != b.Priority {
			if a.Priority == 0 || b.Priority == 0 {
				return cmp.Compare(b.Priority, a.Priority) // 0 goes after the rest
			}
			return cmp.Compare(a.Priority, b.Priority)
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return tags
}

// tagParts lists the fit's hull, and its modules and drones with the size of the rack each is a share of. Drones, fighters, and the hull are a share of an average of the high, mid, and low racks.
func (f *Fit) tagParts() (*comm.FitType, []tagPart) {
	var hull *comm.FitType
	if f.Hull != nil {
		hull = comm.FitTypeOf(f.Hull.ItemID)
	}
	averageRack := max(float64(len(f.Highs)+len(f.Mids)+len(f.Lows))/3, 1)

	parts := []tagPart{}
	if hull != nil {
		parts = append(parts, tagPart{item: f.Hull, fitType: hull, quantity: 1, rack: averageRack})
	}
	for _, rack := range [][]*FittedModule{f.Highs, f.Mids, f.Lows, f.Rigs, f.Subsystems, f.Services} {
		for _, slot := range rack {
			if slot != nil && slot.Module != nil {
				parts = append(parts, tagPart{item: slot.Module, fitType: comm.FitTypeOf(slot.Module.ItemID), quantity: 1, rack: float64(len(rack))})
			}
		}
	}
	for _, bay := range [][]*FitCargo{f.Drones, f.Fighters} {
		for _, stack := range bay {
			if stack != nil && stack.Item != nil {
				parts = append(parts, tagPart{item: stack.Item, fitType: comm.FitTypeOf(stack.Item.ItemID), quantity: stack.Quantity, rack: averageRack})
			}
		}
	}
	return hull, parts
}

// signalMatches reports whether a signal counts a part: the hull for a hull signal, otherwise a module or drone, on a hull of the signal's sizes
func signalMatches(signal *comm.FitTagSignal, hull *comm.FitType, part tagPart) bool {
	isHull := part.fitType == hull
	return signal.Hull == isHull && signal.FitsHull(hull) && signal.Matches(part.fitType)
}

// scoreTag adds up a tag's signals over the fit's parts, and reports whether it applies
func scoreTag(name string, rule *comm.FitTagRule, hull *comm.FitType, parts []tagPart) (FitTag, bool) {
	tag := FitTag{Name: name, Meaning: rule.Meaning, Priority: rule.Priority, Role: rule.Role, Hidden: rule.Hidden, Reasons: []string{}}

	anyMatch := func(signal *comm.FitTagSignal) bool {
		return slices.ContainsFunc(parts, func(part tagPart) bool { return signalMatches(signal, hull, part) })
	}
	for index := range rule.Requires {
		if !anyMatch(&rule.Requires[index]) {
			return tag, false
		}
	}
	for index := range rule.Forbids {
		if anyMatch(&rule.Forbids[index]) {
			return tag, false
		}
	}

	counted := map[string]int{} // reasons by name, in the order first seen
	order := []string{}
	for index := range rule.Signals {
		signal := &rule.Signals[index]
		each := signal.Weight
		if len(signal.BonusedBy) > 0 && comm.HullBonused(hull, signal.BonusedBy) {
			each = signal.BonusedWeight
		}
		weight, share := 0.0, 0.0
		for _, part := range parts {
			if !signalMatches(signal, hull, part) {
				continue
			}
			weight += each * float64(part.quantity)
			share += each * float64(part.quantity) / part.rack
			if each > 0 {
				if _, seen := counted[part.item.Name]; !seen {
					order = append(order, part.item.Name)
				}
				counted[part.item.Name] += part.quantity
			}
		}
		if signal.Max > 0 && weight > signal.Max {
			share *= signal.Max / weight
			weight = signal.Max
		}
		tag.Weight += weight
		tag.Share += share
	}

	for _, reason := range order {
		if counted[reason] > 1 {
			reason = fmt.Sprintf("%s ×%d", reason, counted[reason])
		}
		tag.Reasons = append(tag.Reasons, reason)
	}
	if rule.Threshold <= 0 && rule.ShareThreshold <= 0 {
		return tag, false
	}
	byWeight := rule.Threshold <= 0 || tag.Weight >= rule.Threshold
	// a rack's share adds up in floats, so a full one may fall a hair short of 1
	byShare := rule.ShareThreshold <= 0 || tag.Share >= rule.ShareThreshold-1e-9
	return tag, byWeight && byShare
}

// keepHeaviestExclusive leaves only the heaviest of each exclusive set's tags, a tie going to the one listed first
func keepHeaviestExclusive(tags []FitTag, exclusive map[string][]string) []FitTag {
	for _, set := range exclusive {
		keep := ""
		best := 0.0
		for _, name := range set {
			for _, tag := range tags {
				if tag.Name == name && (keep == "" || tag.Weight > best) {
					keep, best = name, tag.Weight
				}
			}
		}
		tags = slices.DeleteFunc(tags, func(tag FitTag) bool { return slices.Contains(set, tag.Name) && tag.Name != keep })
	}
	return tags
}
