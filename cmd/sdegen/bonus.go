package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// bonusAttributePrefixes start the name of an attribute a hull or subsystem carries for a bonus, lower case
var bonusAttributePrefixes = []string{"shipbonus", "elitebonus", "subsystembonus", "rolebonus"}

// bonusHandFixes places the bonus attributes the naming rules can't, by attribute name. The value is the skill whose level multiplies the bonus, or "" for a role bonus, which isn't scaled. It also overrides the rules.
var bonusHandFixes = map[string]string{
	// retired OLD subsystems whose skills (ie Amarr Electronic Systems) are unpublished, so the engine can't scale them; they only fit killmails from before the rebalance
	"subsystemBonusAmarrElectronic":     "",
	"subsystemBonusAmarrElectronic2":    "",
	"subsystemBonusCaldariElectronic":   "",
	"subsystemBonusCaldariElectronic2":  "",
	"subsystemBonusGallenteElectronic":  "",
	"subsystemBonusGallenteElectronic2": "",
	"subsystemBonusMinmatarElectronic":  "",
	"subsystemBonusMinmatarElectronic2": "",

	// the Navy Issue destroyers' extra lines are role bonuses in their traits (the rules read a number after the race as a skill bonus)
	"shipBonusNavyDestroyerAmarr6":    "",
	"shipBonusNavyDestroyerAmarr7":    "",
	"shipBonusNavyDestroyerCaldari4":  "",
	"shipBonusNavyDestroyerCaldari5":  "",
	"shipBonusNavyDestroyerGallente4": "",
	"shipBonusNavyDestroyerGallente5": "",
	"shipBonusNavyDestroyerMinmatar4": "",
	"shipBonusNavyDestroyerMinmatar5": "",
	"eliteBonusHeavyInterdictors3":    "", // the Laelaps's line is among its role bonuses

	// role bonuses by their traits, with names that don't say so
	"eliteBonusBombers":          "",
	"eliteBonusEscorts":          "",
	"subsystemBonusMassAddition": "",
	"shipBonusHPExtender1":       "",
	"shipBonusGasHold":           "",

	// the ORE Mackinaw's Development Edition lines, which Pyfa doesn't scale either
	"eliteBonusBarge1":  "Exhumers",
	"eliteBonusBarge2":  "Exhumers",
	"shipBonusORE1":     "Mining Barge",
	"shipBonusORE2":     "Mining Barge",
	"shipBonusORE3":     "Mining Barge",
	"shipBonusOREfrig2": "Mining Frigate",

	// the Command Carriers' jump range line is under each hull's own race carrier skill in its traits, and all are at V
	"shipBonusCommandCarrierCMJDShipJumpCap": "Command Carriers",

	// an Alliance Tournament dreadnought's line is under both its race skills in its traits, and either is at V
	"shipBonusSiegeSpeedMultiplier": "Gallente Dreadnought",
}

// raceNames are the one letter races in shipBonus<race><class><n>
var raceNames = map[string]string{"A": "Amarr", "C": "Caldari", "G": "Gallente", "M": "Minmatar"}

// hullClassNames are the classes in shipBonus<race><class><n>
var hullClassNames = map[string]string{"F": "Frigate", "D": "Destroyer", "C": "Cruiser", "BC": "Battlecruiser", "B": "Battleship", "I": "Hauler"}

// eliteBonusSkills are the T2 class skills by the name an eliteBonus<class><n> attribute carries, without its number
var eliteBonusSkills = map[string]string{
	"AssaultShips":         "Assault Frigates",
	"BlackOps":             "Black Ops",
	"CommandDestroyer":     "Command Destroyers",
	"CommandShips":         "Command Ships",
	"CovertOps":            "Covert Ops",
	"ElectronicAttackShip": "Electronic Attack Ships",
	"Expedition":           "Expedition Frigates",
	"FlagCruisers":         "Flag Cruisers",
	"Gunship":              "Assault Frigates",
	"Gunships":             "Assault Frigates",
	"HeavyGunship":         "Heavy Assault Cruisers",
	"HeavyInterdictors":    "Heavy Interdiction Cruisers",
	"Industrial":           "Transport Ships",
	"Interceptor":          "Interceptors",
	"Interdictors":         "Interdictors",
	"JumpFreighter":        "Jump Freighters",
	"LogiFrig":             "Logistics Frigates",
	"Logistics":            "Logistics Cruisers",
	"ReconShip":            "Recon Ships",
	"Violators":            "Marauders",
}

// bonusStems are bonus attributes named for one skill with a number after, by name without the number
var bonusStems = map[string]string{
	"shipBonusICS":                 "Industrial Command Ships",
	"shipBonusSoEECS":              "Expedition Command Ships",
	"shipBonusOreDestroyer":        "Mining Destroyer",
	"shipBonusOreIndustrial":       "ORE Hauler",
	"shipBonusORECapital":          "Capital Industrial Ships",
	"shipBonusUH":                  "Upwell Hauler",
	"shipBonusUFreighter":          "Upwell Freighter",
	"shipBonusAdvancedDreadnought": "Lancer Dreadnoughts",
	"shipBonusPDread":              "Precursor Dreadnought",
}

// namedBonuses are bonus attributes named for their skill with other prefixes than the ship, elite and subsystem ones, ie exhumersBonusOreMiningYield
var namedBonuses = []struct {
	pattern *regexp.Regexp
	skill   string
}{
	{regexp.MustCompile(`^exhumersBonus`), "Exhumers"},
	{regexp.MustCompile(`^miningBargeBonus`), "Mining Barge"},
	{regexp.MustCompile(`^expeditionFrigateBonus`), "Expedition Frigates"},
	{regexp.MustCompile(`^(miningFrigateBonus|miningFrigatesBonus|oreMiningFrigate)`), "Mining Frigate"},
	{regexp.MustCompile(`^industrialCommandBonus`), "Industrial Command Ships"},
	{regexp.MustCompile(`^capitalIndustrial(Ship|Command)Bonus`), "Capital Industrial Ships"},
	{regexp.MustCompile(`^eliteIndustrial`), "Transport Ships"},
}

// the patterns of the attribute names that say which skill a bonus is for. Each has a function building the skill's name from the pattern's groups.
var (
	// anything with role in it, and the Alliance Tournament ships' fixed bonuses
	roleName         = regexp.MustCompile(`(?i)role`)
	allianceTourney  = regexp.MustCompile(`^shipBonus(AT|TrackingAT|AllShieldResistAT)`)
	hullClassBonus   = regexp.MustCompile(`^shipBonus\d?([ACGM])(BC|F|D|C|B|I)\d?$`)
	capitalBonus     = regexp.MustCompile(`^shipBonus(Carrier|ForceAuxiliary|Supercarrier|Dreadnought|Titan)(PC|[ACGM])\d$`)
	t2DestroyerBonus = regexp.MustCompile(`^shipBonus(NavyDestroyer|TacticalDestroyer|StrategicCruiser)(Amarr|Caldari|Gallente|Minmatar)\d$`)
	precursorBonus   = regexp.MustCompile(`^shipBonusP(BC|BS|F|D|C)\d$`)
	edencomBonus     = regexp.MustCompile(`^shipBonusU(F|C|B)\d$`)
	subsystemBonus   = regexp.MustCompile(`^subsystemBonus(Amarr|Caldari|Gallente|Minmatar)(Core|Defensive|Offensive|Propulsion)\d?$`)
	commandCarrier   = regexp.MustCompile(`^shipBonusCommandCarrier`)
	eliteBonus       = regexp.MustCompile(`^eliteBonus([A-Za-z]+?)\d?$`)
	trailingNumber   = regexp.MustCompile(`\d+$`)
	racialIndustrial = regexp.MustCompile(`^(amarr|caldari|gallente|minmatar)IndustrialBonus`)
	freighterBonus   = regexp.MustCompile(`^freighterBonus([ACGMO])\d$`)
)

// bonusKind is what the naming rules made of a bonus attribute
type bonusKind int

const (
	bonusUnplaced bonusKind = iota // the rules don't know it
	bonusSkill                     // a bonus per level of a skill
	bonusRole                      // a fixed bonus of the hull, not scaled
)

/*
	classifyBonus works out from an attribute's name which skill's level multiplies it

Parameters:
  - name: the attribute's name, ie shipBonusMF

Returns:
  - string: the skill's name, empty for a role bonus or an attribute the rules can't place
  - bonusKind: whether the attribute was placed
*/
func classifyBonus(name string) (string, bonusKind) {
	if name == "" {
		return "", bonusUnplaced
	}
	// a few of CCP's names start with a capital
	name = strings.ToLower(name[:1]) + name[1:]
	if skill, fixed := bonusHandFixes[name]; fixed {
		if skill == "" {
			return "", bonusRole
		}
		return skill, bonusSkill
	}
	if roleName.MatchString(name) || allianceTourney.MatchString(name) {
		return "", bonusRole
	}

	if match := hullClassBonus.FindStringSubmatch(name); match != nil {
		return raceNames[match[1]] + " " + hullClassNames[match[2]], bonusSkill
	}
	if match := capitalBonus.FindStringSubmatch(name); match != nil {
		race := "Precursor"
		if match[2] != "PC" {
			race = raceNames[match[2]]
		}
		class := match[1]
		if class == "ForceAuxiliary" || class == "Supercarrier" {
			class = "Carrier" // they fly on the race's carrier skill
		}
		return race + " " + class, bonusSkill
	}
	if match := t2DestroyerBonus.FindStringSubmatch(name); match != nil {
		class := map[string]string{"NavyDestroyer": "Destroyer", "TacticalDestroyer": "Tactical Destroyer", "StrategicCruiser": "Strategic Cruiser"}[match[1]]
		return match[2] + " " + class, bonusSkill
	}
	if match := precursorBonus.FindStringSubmatch(name); match != nil {
		class := map[string]string{"BC": "Battlecruiser", "BS": "Battleship", "F": "Frigate", "D": "Destroyer", "C": "Cruiser"}[match[1]]
		return "Precursor " + class, bonusSkill
	}
	if match := edencomBonus.FindStringSubmatch(name); match != nil {
		class := map[string]string{"F": "Frigate", "C": "Cruiser", "B": "Battleship"}[match[1]]
		return "EDENCOM " + class, bonusSkill
	}
	if match := subsystemBonus.FindStringSubmatch(name); match != nil {
		return match[1] + " " + match[2] + " Systems", bonusSkill
	}
	for _, named := range namedBonuses {
		if named.pattern.MatchString(name) {
			return named.skill, bonusSkill
		}
	}
	if match := racialIndustrial.FindStringSubmatch(name); match != nil {
		return strings.ToUpper(match[1][:1]) + match[1][1:] + " Hauler", bonusSkill
	}
	if match := freighterBonus.FindStringSubmatch(name); match != nil {
		race := raceNames[match[1]]
		if match[1] == "O" {
			race = "ORE"
		}
		return race + " Freighter", bonusSkill
	}
	if commandCarrier.MatchString(name) {
		return "Command Carriers", bonusSkill
	}
	if skill, found := bonusStems[trailingNumber.ReplaceAllString(name, "")]; found {
		return skill, bonusSkill
	}
	if match := eliteBonus.FindStringSubmatch(name); match != nil {
		if skill, found := eliteBonusSkills[trailingNumber.ReplaceAllString(match[1], "")]; found {
			return skill, bonusSkill
		}
	}
	return "", bonusUnplaced
}

// sdeTraits is a typeBonus.jsonl line: a hull's bonus values, per skill and as role bonuses
type sdeTraits struct {
	Key  int `json:"_key"`
	Role []struct {
		Bonus float64 `json:"bonus"`
	} `json:"roleBonuses"`
	Types []struct {
		Skill   int `json:"_key"`
		Bonuses []struct {
			Bonus float64 `json:"bonus"`
		} `json:"_value"`
	} `json:"types"`
}

// bonusTable is the bonus attributes sorted into scaled by a skill, and role bonuses
type bonusTable struct {
	skill     map[int]int      // bonus attribute to the skill type whose level scales it
	skillName map[int]string   // the same, by the skill's name
	role      map[int]struct{} // role bonus attributes, not scaled
	unplaced  []int            // attributes the rules can't place, or whose skill the SDE doesn't have
}

// deriveBonusSkills sorts every bonus attribute by its name
func deriveBonusSkills(source *dogmaSource) *bonusTable {
	table := &bonusTable{skill: map[int]int{}, skillName: map[int]string{}, role: map[int]struct{}{}}
	for _, id := range sortedKeys(source.attributeNames) {
		name := source.attributeNames[id]
		skill, kind := classifyBonus(name)
		if kind == bonusUnplaced && !hasBonusPrefix(name) {
			continue
		}
		switch kind {
		case bonusRole:
			table.role[id] = struct{}{}
		case bonusSkill:
			skillID, found := source.skillIDs[skill]
			if !found {
				fmt.Printf("  no skill named %q for %s\n", skill, name)
				table.unplaced = append(table.unplaced, id)
				continue
			}
			table.skill[id] = skillID
			table.skillName[id] = skill
		default:
			table.unplaced = append(table.unplaced, id)
		}
	}
	return table
}

// hasBonusPrefix is whether an attribute is named like a hull bonus
func hasBonusPrefix(name string) bool {
	lower := strings.ToLower(name)
	return slices.ContainsFunc(bonusAttributePrefixes, func(prefix string) bool { return strings.HasPrefix(lower, prefix) })
}

// sameBonus is whether a trait's bonus is the value of a bonus attribute, whatever its sign
func sameBonus(trait float64, value float64) bool {
	return math.Abs(math.Abs(trait)-math.Abs(value)) < 1e-6
}

// bonusEvidence is how the hulls carrying a bonus attribute back its placement
type bonusEvidence struct {
	agree, conflict, unknown int
	conflicts                []string
}

// checkBonuses compares every placement with the hulls that carry the attribute: their traits in typeBonus.jsonl (a trait value under a skill, or among the role bonuses) and, for a scaled bonus, the skills the hull requires
func checkBonuses(source *dogmaSource) map[int]*bonusEvidence {
	evidence := map[int]*bonusEvidence{}
	table := source.bonus
	for _, hullID := range sortedKeys(source.rawDogma) {
		dogma := source.rawDogma[hullID]
		required := map[int]struct{}{}
		for _, attribute := range dogma.Attributes {
			if slices.Contains(structuralAttributes[:6], attribute.AttributeID) {
				required[int(attribute.Value)] = struct{}{}
			}
		}
		traits := source.traits[hullID]
		for _, attribute := range dogma.Attributes {
			skill, scaled := table.skill[attribute.AttributeID]
			_, isRole := table.role[attribute.AttributeID]
			if !scaled && !isRole {
				continue
			}
			found := evidence[attribute.AttributeID]
			if found == nil {
				found = &bonusEvidence{}
				evidence[attribute.AttributeID] = found
			}

			var matchesSkills []int
			matchesRole := false
			if traits != nil {
				for _, role := range traits.Role {
					matchesRole = matchesRole || sameBonus(role.Bonus, attribute.Value)
				}
				for _, perSkill := range traits.Types {
					if slices.ContainsFunc(perSkill.Bonuses, func(bonus struct {
						Bonus float64 `json:"bonus"`
					}) bool {
						return sameBonus(bonus.Bonus, attribute.Value)
					}) {
						matchesSkills = append(matchesSkills, perSkill.Skill)
					}
				}
			}

			problem := ""
			switch {
			case scaled && len(required) > 0 && !hasKey(required, skill):
				problem = fmt.Sprintf("%s does not require %s", source.typeNames[hullID], table.skillName[attribute.AttributeID])
			case scaled && len(matchesSkills) > 0 && !slices.Contains(matchesSkills, skill):
				problem = fmt.Sprintf("%s has the value %g under %s", source.typeNames[hullID], attribute.Value, source.typeNames[matchesSkills[0]])
			case scaled && len(matchesSkills) == 0 && matchesRole:
				problem = fmt.Sprintf("%s has the value %g only as a role bonus", source.typeNames[hullID], attribute.Value)
			case isRole && len(matchesSkills) > 0 && !matchesRole:
				problem = fmt.Sprintf("%s has the value %g under %s", source.typeNames[hullID], attribute.Value, source.typeNames[matchesSkills[0]])
			case len(matchesSkills) == 0 && !matchesRole:
				found.unknown++
				continue
			}
			if problem == "" {
				found.agree++
			} else {
				found.conflict++
				found.conflicts = append(found.conflicts, problem)
			}
		}
	}
	return evidence
}

// hasKey is whether a set has the key
func hasKey(set map[int]struct{}, key int) bool {
	_, found := set[key]
	return found
}

// reportBonuses prints how many bonus attributes were placed, the ones that are not, and the placements the hulls disagree with
func reportBonuses(source *dogmaSource) {
	table := source.bonus
	evidence := checkBonuses(source)
	carried := map[int]struct{}{}
	for _, dogma := range source.rawDogma {
		for _, attribute := range dogma.Attributes {
			carried[attribute.AttributeID] = struct{}{}
		}
	}
	unplacedCarried := 0
	for _, id := range table.unplaced {
		if hasKey(carried, id) {
			unplacedCarried++
		}
	}
	fmt.Printf("bonus attributes: %d scaled by a skill, %d role bonuses, %d unplaced (%d of them on no type)\n", len(table.skill), len(table.role), len(table.unplaced), len(table.unplaced)-unplacedCarried)
	for _, id := range table.unplaced {
		if hasKey(carried, id) {
			fmt.Printf("  unplaced: %d %s, add it to bonusHandFixes\n", id, source.attributeNames[id])
		}
	}
	disagreements := 0
	for _, id := range sortedKeys(evidence) {
		found := evidence[id]
		// a hull or two with a value that happens to match another trait isn't worth a line
		if found.conflict*2 <= found.conflict+found.agree {
			continue
		}
		disagreements++
		placed := "role"
		if skill, scaled := table.skillName[id]; scaled {
			placed = skill
		}
		fmt.Printf("  hulls disagree with %s -> %s (%d of %d): %s\n", source.attributeNames[id], placed, found.conflict, found.conflict+found.agree, found.conflicts[0])
	}
	fmt.Printf("  %d placements the hulls disagree with\n", disagreements)
}

// pyfaSkill finds the getModifiedItemAttr('<name>') calls with skill='<skill>' as one of their arguments in Pyfa's effects.py
var (
	pyfaAttribute = regexp.MustCompile(`getModifiedItemAttr\(\s*['"](\w+)['"]\s*\)`)
	pyfaSkill     = regexp.MustCompile(`skill\s*=\s*['"]([^'"]+)['"]`)
)

// readPyfaBonuses reads which skill Pyfa names for each attribute, from its eos/effects.py (only read here, as a check for our table)
func readPyfaBonuses(pyfaDir string) (map[string]map[string]int, error) {
	text, err := os.ReadFile(filepath.Join(pyfaDir, "eos", "effects.py"))
	if err != nil {
		return nil, err
	}
	source := string(text)
	skills := map[string]map[string]int{}
	found := pyfaAttribute.FindAllStringSubmatchIndex(source, -1)
	for index, match := range found {
		end := min(match[1]+200, len(source))
		if index+1 < len(found) {
			end = min(end, found[index+1][0])
		}
		if skill := pyfaSkill.FindStringSubmatch(source[match[1]:end]); skill != nil {
			name := source[match[2]:match[3]]
			if skills[name] == nil {
				skills[name] = map[string]int{}
			}
			skills[name][skill[1]]++
		}
	}
	return skills, nil
}

// reportPyfa prints where our placements differ from what Pyfa's effects name
func reportPyfa(source *dogmaSource, pyfaDir string) error {
	pyfa, err := readPyfaBonuses(pyfaDir)
	if err != nil {
		return err
	}
	table := source.bonus
	disagreements := 0
	say := func(format string, args ...any) {
		disagreements++
		fmt.Printf("  pyfa: "+format+"\n", args...)
	}
	for _, id := range sortedKeys(source.attributeNames) {
		name := source.attributeNames[id]
		theirs := pyfa[name]
		skill, scaled := table.skillName[id]
		_, isRole := table.role[id]
		isBonus := hasBonusPrefix(name)
		switch {
		case scaled && len(theirs) > 0 && theirs[skill] == 0:
			say("%s is %s here but %v in Pyfa", name, skill, theirs)
		case isRole && len(theirs) > 0:
			say("%s is a role bonus here but %v in Pyfa", name, theirs)
		case isBonus && !scaled && !isRole && len(theirs) > 0:
			say("%s is unplaced here but %v in Pyfa", name, theirs)
		}
	}
	fmt.Printf("  %d disagreements with Pyfa's skill= arguments\n", disagreements)
	return nil
}
