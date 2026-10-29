package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// dogmaCharacterType is the type that holds a character's base attributes, which skills and implants modify
const dogmaCharacterType = 1373

// dogma group and category ids that pick which types dogma.json holds
const (
	groupShipModifiers     = 1306 // tactical destroyer modes, all unpublished
	groupTacticalDestroyer = 1305
	categorySkill          = 16
	functionEffectStopper  = "EffectStopper"
	attributeSkillLevel    = 280
)

// outputAttributes are the attributes the fitting package reads as results. dogma.json keeps these and everything that modifies them, so adding to this list (ie for DPS, speed, or capacitor stability) is the way to grow the file.
var outputAttributes = []int{
	9,   // hp: a hull's structure
	263, // shieldCapacity
	265, // armorHP

	271, 272, 273, 274, // shield em, explosive, kinetic, thermal resonance
	267, 268, 269, 270, // armor em, explosive, kinetic, thermal resonance
	113, 111, 109, 110, // hull em, explosive, kinetic, thermal resonance

	479, // shieldRechargeRate

	482, // capacitorCapacity
	55,  // rechargeRate: the capacitor's
	6,   // capacitorNeed
	73,  // duration
	51,  // speed

	68,   // shieldBonus: shield boosted per cycle
	84,   // armorDamageAmount: armor repaired per cycle
	83,   // structureDamageAmount: hull repaired per cycle
	90,   // powerTransferAmount: capacitor transmitted per cycle, or drained by a nosferatu
	97,   // energyNeutralizerAmount: capacitor neutralized per cycle
	1886, // chargedArmorDamageMultiplier: an ancillary armor repairer loaded with paste

	208, 209, 210, 211, // radar, ladar, magnetometric, gravimetric sensor strength

	2253, // ECMResistance
	2112, // sensorDampenerResistance
	2113, // weaponDisruptionResistance
	2114, // targetPainterResistance
	2115, // stasisWebifierResistance
	2045, // energyWarfareResistance
	2116, // remoteRepairImpedance
	2135, // remoteAssistanceImpedance
	6463, // remoteCapacitorImpedance

	1849, // resistanceShiftAmount: the Reactive Armor Hardener's
	2746, // resistanceMultiplier: Assault Damage Control's
}

// structuralAttributes are the attributes the engine itself reads to find what a modifier applies to
var structuralAttributes = []int{
	182, 183, 184, 1285, 1289, 1290, // requiredSkill1 to requiredSkill6
	attributeSkillLevel,
}

// codeEffects are effects kept even with no modifiers, as the fitting package handles them in code or reads them as markers
var codeEffects = []int{
	4, 26, 27, 4936, 5275, // shield, hull, armor, ancillary shield, and ancillary armor repair
	4928,                                     // Reactive Armor Hardener
	6184, 6185, 6186, 6188, 6651, 6652, 7166, // remote capacitor, hull, shield, armor, ancillary armor, ancillary shield, and mutadaptive armor repair
	6187, 6197, // energy neutralizers and nosferatus, for neut pressure
	16,                           // online
	11, 12, 13, 2663, 3772, 6306, // low, high, mid, rig, subsystem, and service slots
}

// unpenalized are effects whose modifiers on these attributes are never stacking penalized, though the attributes say they should be: what Pyfa writes by hand, and in game
var unpenalized = map[int][]int{
	6658: {109, 110, 111, 113}, // Bastion's hull resists, which stack fully with a Damage Control's
}

// droppedDomains are modifier domains for effects projected on a target, which nothing in a fit applies
var droppedDomains = []string{"targetID", "target"}

// sdeAttribute is the part of a dogmaAttributes.jsonl line the extract reads
type sdeAttribute struct {
	Key        int     `json:"_key"`
	Name       string  `json:"name"`
	Default    float64 `json:"defaultValue"`
	Stackable  bool    `json:"stackable"`
	HighIsGood bool    `json:"highIsGood"`
	Max        int     `json:"maxAttributeID"`
}

// sdeModifier is one modifierInfo entry of a dogmaEffects.jsonl line
type sdeModifier struct {
	Domain    string `json:"domain"`
	Func      string `json:"func"`
	Modified  int    `json:"modifiedAttributeID"`
	Modifying int    `json:"modifyingAttributeID"`
	Operation int    `json:"operation"`
	Group     int    `json:"groupID"`
	Skill     int    `json:"skillTypeID"`
}

// sdeEffect is the part of a dogmaEffects.jsonl line the extract reads
type sdeEffect struct {
	Key       int           `json:"_key"`
	Name      string        `json:"name"`
	Category  int           `json:"effectCategoryID"`
	Duration  int           `json:"durationAttributeID"`
	Discharge int           `json:"dischargeAttributeID"`
	Modifiers []sdeModifier `json:"modifierInfo"`
}

// sdeDogmaType is a typeDogma.jsonl line, with which effect is the default
type sdeDogmaType struct {
	Key        int `json:"_key"`
	Attributes []struct {
		AttributeID int     `json:"attributeID"`
		Value       float64 `json:"value"`
	} `json:"dogmaAttributes"`
	Effects []struct {
		EffectID  int  `json:"effectID"`
		IsDefault bool `json:"isDefault"`
	} `json:"dogmaEffects"`
}

// dogmaSource is what ExtractDogma read from the SDE that the bonus checks need again
type dogmaSource struct {
	attributeNames map[int]string
	skillIDs       map[string]int        // published skill name to type id
	typeNames      map[int]string        // type id to name
	typeGroups     map[int]int           // type id to group id
	rawDogma       map[int]*sdeDogmaType // the kept types' dogma, before trimming
	traits         map[int]*sdeTraits    // hull or subsystem to its typeBonus line
	bonus          *bonusTable           // the bonus attribute to skill derivation
	modeReport     []string              // tactical destroyer hulls without exactly 3 modes, and modes without a hull
	attributeIDs   map[string]int        // attribute name to id
}

// candidate is a type the dogma extract may keep
type candidate struct {
	group, category int
	always          bool // kept even when the SDE has no dogma for it, so the engine can tell a type with nothing relevant from an unknown one
}

/*
	ExtractDogma reads what the dogma engine needs from a JSONL SDE

Parameters:
  - sde: the SDE's jsonl files at its root
  - fit: the fit_types extract made from the same SDE, whose types all go in

Returns:
  - *comm.DogmaFile: the extract, ready to marshal
  - *dogmaSource: what was read, for the bonus checks
  - error: a file that could not be read
*/
func ExtractDogma(sde fs.FS, fit *comm.FitTypesFile) (*comm.DogmaFile, *dogmaSource, error) {
	out := &comm.DogmaFile{
		SDEBuild:      fit.SDEBuild,
		CharacterType: dogmaCharacterType,
		Attributes:    map[int]*comm.DogmaAttribute{},
		Effects:       map[int]*comm.DogmaEffect{},
		Types:         map[int]*comm.DogmaType{},
		BonusSkills:   map[int]int{},
		Modes:         map[int][]comm.DogmaMode{},
		Unpenalized:   unpenalized,
	}
	source := &dogmaSource{
		attributeNames: map[int]string{},
		skillIDs:       map[string]int{},
		typeNames:      map[int]string{},
		typeGroups:     map[int]int{},
		rawDogma:       map[int]*sdeDogmaType{},
		traits:         map[int]*sdeTraits{},
	}

	groupCategory := map[int]int{}
	if err := eachLine(sde, "groups.jsonl", func(group *sdeGroup) { groupCategory[group.Key] = group.CategoryID }); err != nil {
		return nil, nil, err
	}

	// which types go in: the fit types, unpublished ones of the same categories (old killmails carry them), the tactical destroyer modes, the skills, and the character
	candidates := map[int]candidate{}
	err := eachLine(sde, "types.jsonl", func(item *sdeType) {
		category := groupCategory[item.GroupID]
		source.typeNames[item.Key] = item.Name["en"]
		source.typeGroups[item.Key] = item.GroupID
		if item.Published && category == categorySkill {
			source.skillIDs[item.Name["en"]] = item.Key
		}
		_, inFit := fit.Types[item.Key]
		switch {
		case inFit, item.Published && category == categorySkill, item.Key == out.CharacterType, item.GroupID == groupShipModifiers:
			candidates[item.Key] = candidate{item.GroupID, category, true}
		case !item.Published && slices.Contains(keptCategories, category):
			candidates[item.Key] = candidate{item.GroupID, category, false}
		}
	})
	if err != nil {
		return nil, nil, err
	}

	sdeAttributes := map[int]*sdeAttribute{}
	err = eachLine(sde, "dogmaAttributes.jsonl", func(attribute *sdeAttribute) {
		sdeAttributes[attribute.Key] = attribute
		source.attributeNames[attribute.Key] = attribute.Name
	})
	if err != nil {
		return nil, nil, err
	}

	// the effects the kept types use
	err = eachLine(sde, "typeDogma.jsonl", func(dogma *sdeDogmaType) {
		if _, kept := candidates[dogma.Key]; kept {
			source.rawDogma[dogma.Key] = dogma
		}
	})
	if err != nil {
		return nil, nil, err
	}
	usedEffects := map[int]struct{}{}
	for _, dogma := range source.rawDogma {
		for _, effect := range dogma.Effects {
			usedEffects[effect.EffectID] = struct{}{}
		}
	}
	sdeEffects := map[int]*sdeEffect{}
	err = eachLine(sde, "dogmaEffects.jsonl", func(effect *sdeEffect) {
		if _, used := usedEffects[effect.Key]; !used {
			return
		}
		// what is projected onto a target is never applied to a fit
		effect.Modifiers = slices.DeleteFunc(effect.Modifiers, func(modifier sdeModifier) bool {
			return modifier.Func == functionEffectStopper || slices.Contains(droppedDomains, modifier.Domain)
		})
		sdeEffects[effect.Key] = effect
	})
	if err != nil {
		return nil, nil, err
	}

	// who each hull's bonuses belong to: the bonus attribute names say, and typeBonus.jsonl and the hulls' own skill requirements confirm
	err = eachLine(sde, "typeBonus.jsonl", func(traits *sdeTraits) { source.traits[traits.Key] = traits })
	if err != nil {
		return nil, nil, err
	}
	source.bonus = deriveBonusSkills(source)
	// most skills scale their own bonuses through an effect of their own, ie Battlecruisers multiplies shipBonusCBC2 by its level, so the engine must scale only the rest or it scales twice
	scaledBySkills := skillScaledAttributes(candidates, source.rawDogma, sdeEffects)
	for attribute, skill := range source.bonus.skill {
		if _, scaled := scaledBySkills[attribute]; !scaled {
			out.BonusSkills[attribute] = skill
		}
	}

	// the attributes to keep, as a backward closure from the outputs: whatever modifies a kept attribute through a kept effect is kept too
	keptAttributes := map[int]struct{}{}
	add := func(id int) bool {
		grew := false
		for id != 0 {
			if _, have := keptAttributes[id]; have {
				break
			}
			attribute, known := sdeAttributes[id]
			if !known {
				break
			}
			keptAttributes[id] = struct{}{}
			grew = true
			id = attribute.Max
		}
		return grew
	}
	for _, id := range slices.Concat(outputAttributes, structuralAttributes, slices.Collect(maps.Keys(out.BonusSkills))) {
		add(id)
	}
	keptEffects := map[int]struct{}{}
	for grew := true; grew; {
		grew = false
		for _, id := range sortedKeys(sdeEffects) {
			effect := sdeEffects[id]
			_, already := keptEffects[id]
			relevant := slices.Contains(codeEffects, id)
			for _, modifier := range effect.Modifiers {
				if _, modifies := keptAttributes[modifier.Modified]; modifies {
					relevant = true
					if add(modifier.Modifying) {
						grew = true
					}
				}
			}
			if relevant && !already {
				keptEffects[id] = struct{}{}
				grew = true
			}
			if _, kept := keptEffects[id]; kept {
				if add(effect.Duration) || add(effect.Discharge) {
					grew = true
				}
			}
		}
	}

	for id := range keptAttributes {
		attribute := sdeAttributes[id]
		out.Attributes[id] = &comm.DogmaAttribute{Name: attribute.Name, Default: attribute.Default, Stackable: attribute.Stackable, HighIsGood: attribute.HighIsGood, Max: attribute.Max}
	}
	for id := range keptEffects {
		effect := sdeEffects[id]
		kept := &comm.DogmaEffect{Name: effect.Name, Category: effect.Category, Duration: effect.Duration, Discharge: effect.Discharge}
		for _, modifier := range effect.Modifiers {
			if _, modifies := keptAttributes[modifier.Modified]; modifies {
				kept.Modifiers = append(kept.Modifiers, comm.DogmaModifier{Domain: modifier.Domain, Func: modifier.Func, Modified: modifier.Modified, Modifying: modifier.Modifying, Operation: modifier.Operation, Group: modifier.Group, Skill: modifier.Skill})
			}
		}
		out.Effects[id] = kept
	}

	for id, item := range candidates {
		dogma, hasDogma := source.rawDogma[id]
		if !hasDogma && !item.always {
			continue
		}
		kept := &comm.DogmaType{Group: item.group, Category: item.category, Attributes: map[int]float64{}}
		if hasDogma {
			for _, attribute := range dogma.Attributes {
				if _, isKept := keptAttributes[attribute.AttributeID]; isKept && attribute.Value != sdeAttributes[attribute.AttributeID].Default {
					kept.Attributes[attribute.AttributeID] = attribute.Value
				}
			}
			for _, effect := range dogma.Effects {
				if _, isKept := keptEffects[effect.EffectID]; isKept {
					kept.Effects = append(kept.Effects, effect.EffectID)
					if effect.IsDefault && kept.DefaultEffect == 0 {
						kept.DefaultEffect = effect.EffectID
					}
				}
			}
		}
		out.Types[id] = kept
		if item.category == categorySkill {
			out.Skills = append(out.Skills, id)
		}
	}
	slices.Sort(out.Skills)

	linkModes(out, source)
	return out, source, nil
}

// linkModes links each tactical destroyer hull to its modes by name: a mode type is named "<Hull> <Mode> Mode", ie "Confessor Defense Mode". Hulls without exactly 3 modes and modes without a hull go in source.modeReport.
func linkModes(out *comm.DogmaFile, source *dogmaSource) {
	hulls := map[int]string{}
	for id, group := range source.typeGroups {
		if group == groupTacticalDestroyer {
			hulls[id] = source.typeNames[id]
		}
	}
	for _, modeID := range sortedKeys(source.typeGroups) {
		if source.typeGroups[modeID] != groupShipModifiers {
			continue
		}
		name := source.typeNames[modeID]
		bestHull, bestLength := 0, 0
		for hullID, hullName := range hulls {
			if hullName != "" && strings.HasPrefix(name, hullName+" ") && strings.HasSuffix(name, " Mode") && len(hullName) > bestLength {
				bestHull, bestLength = hullID, len(hullName)
			}
		}
		if bestHull == 0 {
			source.modeReport = append(source.modeReport, fmt.Sprintf("mode %d %q has no tactical destroyer hull of that name", modeID, name))
			continue
		}
		modeName := strings.TrimSuffix(strings.TrimPrefix(name, hulls[bestHull]+" "), " Mode")
		out.Modes[bestHull] = append(out.Modes[bestHull], comm.DogmaMode{TypeID: modeID, Name: modeName})
	}
	for _, hullID := range sortedKeys(hulls) {
		if len(out.Modes[hullID]) != 3 {
			source.modeReport = append(source.modeReport, fmt.Sprintf("hull %d %s has %d modes, not 3", hullID, hulls[hullID], len(out.Modes[hullID])))
		}
	}
}

// sortedKeys returns a map's keys in order
func sortedKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

/*
	marshalDogma writes the extract as json with one attribute, effect, or type per line, in id order, so a new SDE diffs cleanly

Parameters:
  - dogma: the extract

Returns:
  - []byte: the json
  - error: a value that could not be encoded
*/
func marshalDogma(dogma *comm.DogmaFile) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "{\"sde_build\":%d,\"character_type\":%d", dogma.SDEBuild, dogma.CharacterType)

	for _, section := range []struct {
		name  string
		value any
	}{{"skills", dogma.Skills}, {"modes", dogma.Modes}, {"bonus_skills", dogma.BonusSkills}, {"unpenalized", dogma.Unpenalized}} {
		body, err := encode(section.value)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, ",\n%q:%s", section.name, body)
	}

	if err := writeEntries(&out, "attributes", dogma.Attributes); err != nil {
		return nil, err
	}
	if err := writeEntries(&out, "effects", dogma.Effects); err != nil {
		return nil, err
	}
	if err := writeEntries(&out, "types", dogma.Types); err != nil {
		return nil, err
	}
	out.WriteString("}\n")

	// check the hand built text is valid json before it is written
	if !json.Valid(out.Bytes()) {
		return nil, errors.New("the dogma extract is not valid json")
	}
	return out.Bytes(), nil
}

// writeEntries writes a map as a json object member, one entry per line in key order
func writeEntries[V any](out *bytes.Buffer, name string, entries map[int]V) error {
	fmt.Fprintf(out, ",\n%q:{", name)
	for index, id := range sortedKeys(entries) {
		body, err := encode(entries[id])
		if err != nil {
			return err
		}
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n" + strconv.Quote(strconv.Itoa(id)) + ":")
		out.Write(body)
	}
	out.WriteString("\n}")
	return nil
}

/*
	skillScaledAttributes lists the attributes some skill's own effect modifies, which the SDE already scales by the skill's level

Parameters:
  - candidates: the types kept, to find the skills among them
  - rawDogma: the kept types' dogma
  - effects: every effect

Returns:
  - map[int]struct{}: the attributes
*/
func skillScaledAttributes(candidates map[int]candidate, rawDogma map[int]*sdeDogmaType, effects map[int]*sdeEffect) map[int]struct{} {
	scaled := map[int]struct{}{}
	for id, item := range candidates {
		dogma := rawDogma[id]
		if item.category != categorySkill || dogma == nil {
			continue
		}
		for _, used := range dogma.Effects {
			if effect := effects[used.EffectID]; effect != nil {
				for _, modifier := range effect.Modifiers {
					scaled[modifier.Modified] = struct{}{}
				}
			}
		}
	}
	return scaled
}
