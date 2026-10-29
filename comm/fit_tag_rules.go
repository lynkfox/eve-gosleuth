package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// FitTagSignal is one thing that gives a tag weight: modules, drones, and implants (or the hull, with Hull) matching any of its types, groups, categories, or market groups. A hull signal with none of those matches any hull of its HullSizes.
// An NPCs signal matches the rats on the killmail instead, each one an NPC.
type FitTagSignal struct {
	Note       string `json:"note"` // for people reading the file, not used
	Types      []int  `json:"types"`
	Groups     []int  `json:"groups"`
	Categories []int  `json:"categories"`
	Markets    []int  `json:"markets"` // a market group or any group above the type's (InMarketGroup)
	Hull       bool   `json:"hull"`    // matches the hull instead of what is fitted to it
	NPCs       bool   `json:"npcs"`    // matches the NPCs met in PvE on the loss's killmail instead, ie rats, not gate sentries or CONCORD
	HullSizes  []int  `json:"hull_sizes"`

	Weight        float64  `json:"weight"`         // added for each match
	Max           float64  `json:"max"`            // the most the signal adds in all, 0 for no limit
	Once          bool     `json:"once"`           // its matches add the weight of one, however many: ie drones, whose count on a killmail leaves out those in space
	BonusedBy     []string `json:"bonused_by"`     // words in a hull bonus line (HullBonused) that make each match add BonusedWeight instead
	BonusedWeight float64  `json:"bonused_weight"` // added for each match on a hull bonused for it
}

/*
	Matches reports whether an item type is one the signal counts, by its type, group, category, or market group

Parameters:
  - fitType: the item's type, nil for one fit_types.json does not have

Returns:
  - bool: true if it matches. false for nil.
*/
func (s *FitTagSignal) Matches(fitType *FitType) bool {
	if fitType == nil {
		return false
	}
	if slices.Contains(s.Types, fitType.TypeID) || slices.Contains(s.Groups, fitType.GroupID) || slices.Contains(s.Categories, fitType.CategoryID) {
		return true
	}
	return slices.ContainsFunc(s.Markets, func(market int) bool { return InMarketGroup(fitType, market) })
}

/*
	AnyType reports whether the signal names no types, groups, categories, or market groups, so a hull signal matches every hull of its sizes, ie a capital

Returns:
  - bool: true if it names none of them
*/
func (s *FitTagSignal) AnyType() bool {
	return len(s.Types) == 0 && len(s.Groups) == 0 && len(s.Categories) == 0 && len(s.Markets) == 0
}

/*
	FitsHull reports whether the signal counts on a hull, by its HullSizes

Parameters:
  - hull: the hull's type, nil for one fit_types.json does not have

Returns:
  - bool: true if the signal has no HullSizes or the hull is one of them
*/
func (s *FitTagSignal) FitsHull(hull *FitType) bool {
	if len(s.HullSizes) == 0 {
		return true
	}
	return hull != nil && slices.Contains(s.HullSizes, hull.RigSize)
}

// FitTagRule is how a tag is read from a fit: it applies once its signals' weights reach Threshold and their share of the racks ShareThreshold (each that is set), with each of Requires matched and none of Forbids
type FitTagRule struct {
	Meaning  string `json:"meaning"`  // what it means, for the key
	Priority int    `json:"priority"` // the order a fit shows its tags in, lowest first, 0 after the rest
	Role     bool   `json:"role"`     // it can name a fit's row: the first of the fit's roles by Priority does
	Hidden   bool   `json:"hidden"`   // left off the fit list and the key, shown last and muted in the fitting window, ie the weapons

	Threshold      float64        `json:"threshold"`
	ShareThreshold float64        `json:"share_threshold"` // ie 1 for a full rack of the signals' modules
	Requires       []FitTagSignal `json:"requires"`
	Forbids        []FitTagSignal `json:"forbids"`
	Signals        []FitTagSignal `json:"signals"`

	TagWeights map[string]float64 `json:"tag_weights"` // weight added for each of these other tags the fit has, ie Buffer counting against Ratting. Only tags without tag_weights of their own count.

	FleetDoctrine *FleetDoctrineRule `json:"fleet_doctrine"` // set on the one tag read from battles and other losses rather than the fit
}

// FleetDoctrineRule is when a fit is a fleet doctrine: flown with BattleHulls of its hull and BattleLogi logi in one battle, or lost RepeatLosses times by RepeatPilots pilots of one alliance
type FleetDoctrineRule struct {
	BattleHulls  int `json:"battle_hulls"`
	BattleLogi   int `json:"battle_logi"`
	RepeatLosses int `json:"repeat_losses"`
	RepeatPilots int `json:"repeat_pilots"`
}

// FitTagRulesFile is the layout of fit_tag_rules.json
type FitTagRulesFile struct {
	BonusIgnores []string               `json:"bonus_ignores"` // a hull bonus line containing one of these never makes a signal bonused, ie "resistance"
	Exclusive    map[string][]string    `json:"exclusive"`     // sets of tags of which only the first listed the fit has applies
	Tags         map[string]*FitTagRule `json:"tags"`          // by tag name
}

// fitTagRules is the loaded fit_tag_rules.json. Empty until InitStaticData is called.
var fitTagRules = FitTagRulesFile{Tags: map[string]*FitTagRule{}}

/*
	LoadFitTagRules reads fit_tag_rules.json for FitTagRules, leaving them as they were if the file can not be read or has no tags

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name, ie fit_tag_rules.json

Returns:
  - error: the file could not be opened or read, or has no tags
*/
func LoadFitTagRules(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	loaded := FitTagRulesFile{}
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}
	for name, rule := range loaded.Tags {
		if rule == nil {
			delete(loaded.Tags, name) // a null entry removes a built in tag
		}
	}
	if len(loaded.Tags) == 0 {
		return fmt.Errorf("the json file %s has no tags", filename)
	}
	fitTagRules = loaded
	return nil
}

/*
	FitTagRules returns the loaded fit_tag_rules.json

Returns:
  - *FitTagRulesFile: the rules. Shared, so do not change them.
*/
func FitTagRules() *FitTagRulesFile {
	return &fitTagRules
}

/*
	HullBonused reports whether a hull, or a subsystem fitted to it, has a bonus line naming any of the words, ignoring case and the lines with a BonusIgnores phrase

Parameters:
  - bonusers: the hull's type and its fitted subsystems' types, any nil for one fit_types.json does not have
  - words: ie "Energy Neutralizer"

Returns:
  - bool: true if a bonus line of any of them contains one of the words. false for no words.
*/
func HullBonused(bonusers []*FitType, words []string) bool {
	if len(words) == 0 {
		return false
	}
	return slices.ContainsFunc(bonusers, func(bonuser *FitType) bool { return bonuser != nil && bonusLinesName(bonuser.Bonuses, words) })
}

// bonusLinesName reports whether a bonus line names any of the words, ignoring case and the lines with a BonusIgnores phrase
func bonusLinesName(lines []string, words []string) bool {
	for _, line := range lines {
		line = strings.ToLower(line)
		ignored := slices.ContainsFunc(fitTagRules.BonusIgnores, func(phrase string) bool { return strings.Contains(line, strings.ToLower(phrase)) })
		if ignored {
			continue
		}
		if slices.ContainsFunc(words, func(word string) bool { return strings.Contains(line, strings.ToLower(word)) }) {
			return true
		}
	}
	return false
}
