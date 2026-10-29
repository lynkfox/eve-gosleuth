package retrieval

import (
	"cmp"
	"slices"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// FitTag is a tag a fit has, read from its modules by the rules in fit_tag_rules.json
type FitTag struct {
	Name     string `json:"name"`
	Meaning  string `json:"meaning"`
	Priority int    `json:"priority"` // the order the fit shows its tags in, lowest first, 0 after the rest
	Role     bool   `json:"role"`     // it can name the fit's row
	Hidden   bool   `json:"hidden"`   // never shown, only found by the filter

	Weight         float64        `json:"weight"`         // its signals' weights added up
	Threshold      float64        `json:"threshold"`      // the weight it needed, 0 if it has none
	Share          float64        `json:"share"`          // the same as a share of the racks its modules are in, ie 1 for a full rack
	ShareThreshold float64        `json:"shareThreshold"` // the share it needed, 0 if it has none
	Reasons        []FitTagReason `json:"reasons"`        // what gave it weight, which adds up to Weight
}

// FitTagReason is one module, drone, hull, or other tag that gave a tag weight
type FitTagReason struct {
	Name   string  `json:"name"`
	Count  int     `json:"count"`  // how many of it, 0 for another tag
	Weight float64 `json:"weight"` // all of them together, negative for one counting against the tag
	Bonus  float64 `json:"bonus"`  // the part of Weight the hull's bonus for it added, 0 for none
}

// tagPart is one thing on a fit a signal can match: a module, a drone stack, or the hull
type tagPart struct {
	item     *Item
	fitType  *comm.FitType
	quantity int
	rack     float64 // the slots in its rack, which its weight is a share of
}

/*
	Tags reads the fit's tags by the rules in fit_tag_rules.json: those whose weight reaches their threshold, with each of the rule's requires matched and none of its forbids, only the first listed of each exclusive set.
	A rule's tag_weights are added once the other tags are settled, so they read only tags without tag_weights of their own.
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

	weighedByTags := map[string]FitTag{} // scored by their signals, waiting on the other tags
	for name, rule := range rules.Tags {
		if rule.FleetDoctrine != nil {
			continue
		}
		tag, possible := scoreTag(name, rule, hull, parts)
		switch {
		case !possible:
		case len(rule.TagWeights) > 0:
			weighedByTags[name] = tag
		case tagPasses(tag):
			tags = append(tags, tag)
		}
	}
	tags = keepFirstExclusive(tags, rules.Exclusive)

	settled := len(tags)
	for name, tag := range weighedByTags {
		for _, other := range tags[:settled] {
			if weight, weighs := rules.Tags[name].TagWeights[other.Name]; weighs {
				tag.Weight += weight
				tag.Reasons = append(tag.Reasons, FitTagReason{Name: other.Name, Weight: weight})
			}
		}
		if tagPasses(tag) {
			tags = append(tags, tag)
		}
	}

	sortFitTags(tags)
	return tags
}

/*
	MergeFitTags combines the tags of several fits, ie a merged fit's losses, whose drones can differ: each tag once, the first list's copy where it has one

Parameters:
  - lists: each fit's tags

Returns:
  - []FitTag: every tag any of them has, by Priority (those without one last), then name
*/
func MergeFitTags(lists ...[]FitTag) []FitTag {
	merged := []FitTag{}
	for _, tags := range lists {
		for _, tag := range tags {
			if !slices.ContainsFunc(merged, func(each FitTag) bool { return each.Name == tag.Name }) {
				merged = append(merged, tag)
			}
		}
	}
	sortFitTags(merged)
	return merged
}

// sortFitTags orders tags by Priority, those without one last, then name
func sortFitTags(tags []FitTag) {
	slices.SortFunc(tags, func(a, b FitTag) int {
		if a.Priority != b.Priority {
			if a.Priority == 0 || b.Priority == 0 {
				return cmp.Compare(b.Priority, a.Priority) // 0 goes after the rest
			}
			return cmp.Compare(a.Priority, b.Priority)
		}
		return cmp.Compare(a.Name, b.Name)
	})
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

// scoreTag adds up a tag's signals over the fit's parts, and reports false if its requires or forbids rule it out
func scoreTag(name string, rule *comm.FitTagRule, hull *comm.FitType, parts []tagPart) (FitTag, bool) {
	tag := FitTag{
		Name: name, Meaning: rule.Meaning, Priority: rule.Priority, Role: rule.Role, Hidden: rule.Hidden,
		Threshold: rule.Threshold, ShareThreshold: rule.ShareThreshold, Reasons: []FitTagReason{},
	}

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

	for index := range rule.Signals {
		signal := &rule.Signals[index]
		each, bonus := signal.Weight, 0.0
		if len(signal.BonusedBy) > 0 && comm.HullBonused(hull, signal.BonusedBy) {
			each, bonus = signal.BonusedWeight, signal.BonusedWeight-signal.Weight
		}
		weight, share := 0.0, 0.0
		reasons := []FitTagReason{}
		for _, part := range parts {
			if !signalMatches(signal, hull, part) {
				continue
			}
			weight += each * float64(part.quantity)
			share += each * float64(part.quantity) / part.rack
			reasons = addReason(reasons, FitTagReason{Name: part.item.Name, Count: part.quantity, Weight: each * float64(part.quantity), Bonus: bonus * float64(part.quantity)})
		}
		if signal.Max > 0 && weight > signal.Max {
			// each match keeps its part of the capped weight, so the reasons still add up
			for index := range reasons {
				reasons[index].Weight *= signal.Max / weight
				reasons[index].Bonus *= signal.Max / weight
			}
			share *= signal.Max / weight
			weight = signal.Max
		}
		tag.Weight += weight
		tag.Share += share
		for _, reason := range reasons {
			// a module two signals count is one reason, its count once
			if at := slices.IndexFunc(tag.Reasons, func(each FitTagReason) bool { return each.Name == reason.Name }); at >= 0 {
				tag.Reasons[at].Weight += reason.Weight
				tag.Reasons[at].Bonus += reason.Bonus
				tag.Reasons[at].Count = max(tag.Reasons[at].Count, reason.Count)
			} else {
				tag.Reasons = append(tag.Reasons, reason)
			}
		}
	}
	return tag, true
}

// addReason adds a match to a signal's reasons, counting it with any of the same name
func addReason(reasons []FitTagReason, reason FitTagReason) []FitTagReason {
	if at := slices.IndexFunc(reasons, func(each FitTagReason) bool { return each.Name == reason.Name }); at >= 0 {
		reasons[at].Count += reason.Count
		reasons[at].Weight += reason.Weight
		reasons[at].Bonus += reason.Bonus
		return reasons
	}
	return append(reasons, reason)
}

// tagPasses reports whether a scored tag reaches its threshold and its share threshold, each that is set, and has at least one
func tagPasses(tag FitTag) bool {
	if tag.Threshold <= 0 && tag.ShareThreshold <= 0 {
		return false
	}
	byWeight := tag.Threshold <= 0 || tag.Weight >= tag.Threshold
	// a rack's share adds up in floats, so a full one may fall a hair short of 1
	byShare := tag.ShareThreshold <= 0 || tag.Share >= tag.ShareThreshold-1e-9
	return byWeight && byShare
}

// keepFirstExclusive leaves only the first listed of each exclusive set's tags the fit has, ie Active over Buffer however much buffer it has
func keepFirstExclusive(tags []FitTag, exclusive map[string][]string) []FitTag {
	for _, set := range exclusive {
		keep := ""
		for _, name := range set {
			if slices.ContainsFunc(tags, func(tag FitTag) bool { return tag.Name == name }) {
				keep = name
				break
			}
		}
		tags = slices.DeleteFunc(tags, func(tag FitTag) bool { return slices.Contains(set, tag.Name) && tag.Name != keep })
	}
	return tags
}
