package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
)

// DogmaFile is static_data/dogma.json, made by cmd/sdegen from the same SDE as fit_types.json: what the fitting package's dogma engine reads
type DogmaFile struct {
	SDEBuild      int                     `json:"sde_build"`      // the SDE build the file was made from, which should match fit_types.json's
	CharacterType int                     `json:"character_type"` // the type holding a character's base attributes, which skills and implants modify
	Attributes    map[int]*DogmaAttribute `json:"attributes"`     // keyed by attribute id
	Effects       map[int]*DogmaEffect    `json:"effects"`        // keyed by effect id
	Types         map[int]*DogmaType      `json:"types"`          // keyed by type id: hulls, modules, charges, implants, subsystems, skills, tactical destroyer modes, and the character
	BonusSkills   map[int]int             `json:"bonus_skills"`   // a hull or subsystem bonus attribute, ie shipBonusMF, to the skill whose level multiplies it. A role bonus is not here.
	Modes         map[int][]DogmaMode     `json:"modes"`          // a tactical destroyer hull to its modes
	Skills        []int                   `json:"skills"`         // every skill, which a character with all skills trained has at V
	Buffs         map[int]*DogmaBuff      `json:"buffs"`          // a warfare buff (dbuffCollections in the SDE), which command bursts apply, keyed by its id
	Unpenalized   map[int][]int           `json:"unpenalized"`    // an effect to the attributes its modifiers are never stacking penalized on, though the attributes say they should be: ie Bastion's hull resists, as Pyfa has them
}

// DogmaAttribute is a dogma attribute's rules
type DogmaAttribute struct {
	AttributeID int     `json:"-"` // set from its key when loaded
	Name        string  `json:"name"`
	Default     float64 `json:"default"`             // its value on a type that doesn't list it
	Stackable   bool    `json:"stackable,omitempty"` // false: multiplying modifiers from modules are stacking penalized
	HighIsGood  bool    `json:"high_is_good,omitempty"`
	Max         int     `json:"max,omitempty"` // the attribute that caps it, ie 1 for a resonance, 0 for none
}

// DogmaEffect is a dogma effect: when it applies, and the modifiers it applies then. An effect with no modifiers either marks something (ie online, a slot) or is written in code by the fitting package.
type DogmaEffect struct {
	EffectID  int             `json:"-"` // set from its key when loaded
	Name      string          `json:"name"`
	Category  int             `json:"category"`            // 0 passive, 1 active, 2 target, 3 area, 4 online, 5 overload, 6 dungeon, 7 system
	Duration  int             `json:"duration,omitempty"`  // the attribute holding its cycle time in ms
	Discharge int             `json:"discharge,omitempty"` // the attribute holding the capacitor it uses each cycle
	Modifiers []DogmaModifier `json:"modifiers,omitempty"`
}

// DogmaModifier is one entry of an effect's modifierInfo in the SDE
type DogmaModifier struct {
	Domain    string `json:"domain"`          // shipID, charID, itemID, otherID, structureID, targetID
	Func      string `json:"func"`            // ItemModifier, LocationModifier, LocationGroupModifier, LocationRequiredSkillModifier, OwnerRequiredSkillModifier, EffectStopper
	Modified  int    `json:"modified"`        // the attribute changed
	Modifying int    `json:"modifying"`       // the source's attribute whose value changes it
	Operation int    `json:"operation"`       // -1 preAssign, 0 preMul, 1 preDiv, 2 modAdd, 3 modSub, 4 postMul, 5 postDiv, 6 postPercent, 7 postAssign
	Group     int    `json:"group,omitempty"` // LocationGroupModifier: the group of the items changed
	Skill     int    `json:"skill,omitempty"` // *RequiredSkillModifier: the skill the items changed require
}

// DogmaType is a type's dogma: its attributes and effects
type DogmaType struct {
	TypeID        int             `json:"-"` // set from its key when loaded
	Group         int             `json:"group"`
	Category      int             `json:"category"`
	Attributes    map[int]float64 `json:"attributes"` // only the attributes the engine's outputs depend on; any other reads as the attribute's default
	Effects       []int           `json:"effects,omitempty"`
	DefaultEffect int             `json:"default_effect,omitempty"` // the effect activating the module runs, 0 for none
}

// DogmaBuff is what a warfare buff does, ie Armor Energizing's armor resists. A burst sets its value; of several bursts giving one buff, only the strongest counts.
type DogmaBuff struct {
	Aggregate     string            `json:"aggregate"`                // Maximum or Minimum: which of several values counts
	Operation     int               `json:"operation"`                // as DogmaModifier.Operation
	Item          []int             `json:"item,omitempty"`           // attributes of the ship
	Location      []int             `json:"location,omitempty"`       // attributes of every module fitted to it
	LocationGroup []DogmaBuffTarget `json:"location_group,omitempty"` // attributes of its modules of a group
	LocationSkill []DogmaBuffTarget `json:"location_skill,omitempty"` // attributes of its modules that require a skill
}

// DogmaBuffTarget is an attribute a buff changes on the modules of a group, or those requiring a skill
type DogmaBuffTarget struct {
	Attribute int `json:"attribute"`
	Group     int `json:"group,omitempty"`
	Skill     int `json:"skill,omitempty"`
}

// DogmaMode is one of a tactical destroyer's modes
type DogmaMode struct {
	TypeID int    `json:"id"`
	Name   string `json:"name"` // ie Defense, Sharpshooter, Propulsion
}

// dogma is the loaded dogma.json. Empty until InitStaticData is called.
var dogma = DogmaFile{Types: map[int]*DogmaType{}}

// the dogma attributes and effects by name, for code that reads a known attribute or effect
var (
	dogmaAttributeNames = map[string]*DogmaAttribute{}
	dogmaEffectNames    = map[string]*DogmaEffect{}
)

/*
	LoadDogma reads dogma.json for the Dogma lookups, leaving them as they were if the file can not be read or has no types

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name, ie dogma.json

Returns:
  - error: the file could not be opened or read, or has no types
*/
func LoadDogma(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	loaded := DogmaFile{}
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}
	if len(loaded.Types) == 0 {
		return fmt.Errorf("the json file %s has no types", filename)
	}

	// null entries are what an edited copy removed
	types := make(map[int]*DogmaType, len(loaded.Types))
	for typeID, dogmaType := range loaded.Types {
		if dogmaType != nil {
			dogmaType.TypeID = typeID
			types[typeID] = dogmaType
		}
	}
	attributeNames := map[string]*DogmaAttribute{}
	attributes := make(map[int]*DogmaAttribute, len(loaded.Attributes))
	for attributeID, attribute := range loaded.Attributes {
		if attribute != nil {
			attribute.AttributeID = attributeID
			attributes[attributeID] = attribute
			attributeNames[attribute.Name] = attribute
		}
	}
	effectNames := map[string]*DogmaEffect{}
	effects := make(map[int]*DogmaEffect, len(loaded.Effects))
	for effectID, effect := range loaded.Effects {
		if effect != nil {
			effect.EffectID = effectID
			effects[effectID] = effect
			effectNames[effect.Name] = effect
		}
	}
	loaded.Types, loaded.Attributes, loaded.Effects = types, attributes, effects

	if fitBuild := FitTypesSDEBuild(); fitBuild != 0 && loaded.SDEBuild != fitBuild {
		Logger.Warnf("dogma.json is from SDE build %d but fit_types.json from %d, so ship stats may not match the fits", loaded.SDEBuild, fitBuild)
	}
	dogma, dogmaAttributeNames, dogmaEffectNames = loaded, attributeNames, effectNames
	return nil
}

/*
	DogmaTypeOf looks up a type's dogma in dogma.json

Parameters:
  - typeID: the item type

Returns:
  - *DogmaType: the type, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaTypeOf(typeID int) *DogmaType {
	return dogma.Types[typeID]
}

/*
	DogmaAttributeOf looks up a dogma attribute in dogma.json

Parameters:
  - attributeID: the attribute

Returns:
  - *DogmaAttribute: the attribute, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaAttributeOf(attributeID int) *DogmaAttribute {
	return dogma.Attributes[attributeID]
}

/*
	DogmaAttributeNamed looks up a dogma attribute in dogma.json by its SDE name, ie shieldCapacity

Parameters:
  - name: the attribute's name

Returns:
  - *DogmaAttribute: the attribute, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaAttributeNamed(name string) *DogmaAttribute {
	return dogmaAttributeNames[name]
}

/*
	DogmaEffectOf looks up a dogma effect in dogma.json

Parameters:
  - effectID: the effect

Returns:
  - *DogmaEffect: the effect, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaEffectOf(effectID int) *DogmaEffect {
	return dogma.Effects[effectID]
}

/*
	DogmaEffectNamed looks up a dogma effect in dogma.json by its SDE name, ie armorRepair

Parameters:
  - name: the effect's name

Returns:
  - *DogmaEffect: the effect, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaEffectNamed(name string) *DogmaEffect {
	return dogmaEffectNames[name]
}

/*
	DogmaBonusSkill looks up the skill whose level multiplies a hull or subsystem bonus attribute

Parameters:
  - attributeID: the bonus attribute, ie shipBonusMF

Returns:
  - int: the skill's type id, 0 if the attribute is not scaled by a skill (ie a role bonus, or not a bonus)
*/
func DogmaBonusSkill(attributeID int) int {
	return dogma.BonusSkills[attributeID]
}

/*
	DogmaUnpenalized reports whether an effect's modifiers on an attribute are exempt from stacking penalties

Parameters:
  - effectID: the effect
  - attributeID: the attribute it modifies

Returns:
  - bool: true if the effect never penalizes that attribute
*/
func DogmaUnpenalized(effectID int, attributeID int) bool {
	return slices.Contains(dogma.Unpenalized[effectID], attributeID)
}

/*
	DogmaBuffOf looks up a warfare buff in dogma.json

Parameters:
  - buffID: the buff, ie the warfareBuff1ID of a command burst charge

Returns:
  - *DogmaBuff: the buff, nil if dogma.json does not have it. Shared, so do not change it.
*/
func DogmaBuffOf(buffID int) *DogmaBuff {
	return dogma.Buffs[buffID]
}

/*
	DogmaModesOf lists a tactical destroyer's modes

Parameters:
  - hullTypeID: the hull

Returns:
  - []DogmaMode: its modes, nil for a hull without modes. Shared, so do not change it.
*/
func DogmaModesOf(hullTypeID int) []DogmaMode {
	return dogma.Modes[hullTypeID]
}

/*
	DogmaSkills lists every skill in dogma.json, for a character with all skills trained

Returns:
  - []*DogmaType: the skills, leaving out any dogma.json has no type for. Shared, so do not change them.
*/
func DogmaSkills() []*DogmaType {
	skills := make([]*DogmaType, 0, len(dogma.Skills))
	for _, skillID := range dogma.Skills {
		if skill := dogma.Types[skillID]; skill != nil {
			skills = append(skills, skill)
		}
	}
	return skills
}

/*
	DogmaCharacterType returns the type holding a character's base attributes

Returns:
  - *DogmaType: the character type, nil if dogma.json is not loaded. Shared, so do not change it.
*/
func DogmaCharacterType() *DogmaType {
	return dogma.Types[dogma.CharacterType]
}

/*
	DogmaSDEBuild returns the build of CCP's static data export that dogma.json was made from

Returns:
  - int: the build number, 0 if none is loaded
*/
func DogmaSDEBuild() int {
	return dogma.SDEBuild
}
