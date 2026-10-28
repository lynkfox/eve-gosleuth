package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"slices"
	"strconv"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// keptCategories are the SDE categories whose published types go in fit_types.json
var keptCategories = []int{
	6,  // Ship
	7,  // Module
	8,  // Charge
	18, // Drone
	20, // Implant (and boosters)
	23, // Starbase: control towers, for comm.TypeGroup
	32, // Subsystem
	65, // Structure
	66, // Structure Module
	87, // Fighter
}

// keptGroups are groups outside keptCategories whose published types go in fit_types.json
var keptGroups = []int{
	916, // Nanite Repair Paste
}

// keptUnpublishedGroups are groups whose types go in fit_types.json even though the SDE marks every one of them unpublished
var keptUnpublishedGroups = []int{
	1025, // Orbital Infrastructure: customs offices, for comm.TypeGroup
}

// cargoKept is which cargo a fit shows, written into fit_types.json for comm.KeptInFitCargo
var cargoKept = comm.FitCargoRules{
	Categories: []int{8},        // Charge: ammo, scripts, crystals
	Groups:     []int{303, 916}, // Booster, Nanite Repair Paste
}

// abyssalMetaGroup is the meta group of mutated modules, marked Abyssal in fit_types.json
const abyssalMetaGroup = 15

// hullCategories are the categories whose slot and hardpoint counts are kept
var hullCategories = []int{6, 65} // Ship, Structure

// subsystemCategory is the category whose slot modifiers are kept
const subsystemCategory = 32

// slotEffects maps the dogma effect that puts a module in a slot to that slot
var slotEffects = map[int]comm.FitSlot{
	11:   comm.SlotLow,       // loPower
	12:   comm.SlotHigh,      // hiPower
	13:   comm.SlotMid,       // medPower
	2663: comm.SlotRig,       // rigSlot
	3772: comm.SlotSubsystem, // subSystem
	6306: comm.SlotService,   // serviceSlot
}

// dogma attribute ids read into a FitType
const (
	attrLowSlots          = 12
	attrMedSlots          = 13
	attrHiSlots           = 14
	attrLauncherSlotsLeft = 101
	attrTurretSlotsLeft   = 102
	attrRigSlots          = 1137
	attrMaxSubSystems     = 1367
	attrHiSlotModifier    = 1374
	attrMedSlotModifier   = 1375
	attrLowSlotModifier   = 1376
	attrServiceSlots      = 2056
	attrFitsToShipType    = 1380
	attrImplantness       = 331
	attrBoosterness       = 1087
)

// sdeType is the part of a types.jsonl line the extract reads
type sdeType struct {
	Key         int               `json:"_key"`
	GroupID     int               `json:"groupID"`
	MetaGroupID int               `json:"metaGroupID"`
	Name        map[string]string `json:"name"`
	Published   bool              `json:"published"`
}

// sdeGroup is the part of a groups.jsonl line the extract reads
type sdeGroup struct {
	Key        int `json:"_key"`
	CategoryID int `json:"categoryID"`
}

// sdeTypeDogma is a typeDogma.jsonl line
type sdeTypeDogma struct {
	Key        int `json:"_key"`
	Attributes []struct {
		AttributeID int     `json:"attributeID"`
		Value       float64 `json:"value"`
	} `json:"dogmaAttributes"`
	Effects []struct {
		EffectID int `json:"effectID"`
	} `json:"dogmaEffects"`
}

// sdeRelease is the _sde.jsonl line naming the build
type sdeRelease struct {
	Key         string `json:"_key"`
	BuildNumber int    `json:"buildNumber"`
}

/*
	Extract reads the types a fit needs from a JSONL SDE

Parameters:
  - sde: the SDE's jsonl files at its root, ie an extracted folder or the zip

Returns:
  - *comm.FitTypesFile: the extract, ready to Marshal
  - error: a file that could not be read
*/
func Extract(sde fs.FS) (*comm.FitTypesFile, error) {
	extract := &comm.FitTypesFile{CargoKept: cargoKept, Types: map[int]*comm.FitType{}}

	err := eachLine(sde, "_sde.jsonl", func(release *sdeRelease) {
		if release.Key == "sde" {
			extract.SDEBuild = release.BuildNumber
		}
	})
	if err != nil {
		return nil, err
	}

	groupCategory := map[int]int{}
	err = eachLine(sde, "groups.jsonl", func(group *sdeGroup) {
		groupCategory[group.Key] = group.CategoryID
	})
	if err != nil {
		return nil, err
	}

	err = eachLine(sde, "types.jsonl", func(item *sdeType) {
		category := groupCategory[item.GroupID]
		kept := item.Published && (slices.Contains(keptCategories, category) || slices.Contains(keptGroups, item.GroupID))
		if !kept && !slices.Contains(keptUnpublishedGroups, item.GroupID) {
			return
		}
		extract.Types[item.Key] = &comm.FitType{TypeID: item.Key, Name: item.Name["en"], GroupID: item.GroupID, CategoryID: category, Abyssal: item.MetaGroupID == abyssalMetaGroup}
	})
	if err != nil {
		return nil, err
	}

	// maxSubSystems is 5 on T3 cruisers, which fit one subsystem of each of 4 groups, so a hull's subsystem slots are counted from the groups made for it
	subsystemGroups := map[int]map[int]struct{}{}
	err = eachLine(sde, "typeDogma.jsonl", func(dogma *sdeTypeDogma) {
		fitType, kept := extract.Types[dogma.Key]
		if !kept {
			return
		}
		for _, effect := range dogma.Effects {
			if slot, isSlot := slotEffects[effect.EffectID]; isSlot {
				if fitType.Slot != "" && fitType.Slot != slot {
					fmt.Printf("warning: %d %s has the slot effects of both %s and %s, keeping %s\n", fitType.TypeID, fitType.Name, fitType.Slot, slot, fitType.Slot)
					continue
				}
				fitType.Slot = slot
			}
		}

		isHull := slices.Contains(hullCategories, fitType.CategoryID)
		for _, attribute := range dogma.Attributes {
			value := int(math.Round(attribute.Value))
			if attribute.AttributeID == attrImplantness || attribute.AttributeID == attrBoosterness {
				fitType.HeadSlot = value
			}
			if isHull {
				switch attribute.AttributeID {
				case attrHiSlots:
					fitType.HighSlots = value
				case attrMedSlots:
					fitType.MidSlots = value
				case attrLowSlots:
					fitType.LowSlots = value
				case attrRigSlots:
					fitType.RigSlots = value
				case attrMaxSubSystems:
					fitType.SubsystemSlots = value
				case attrServiceSlots:
					fitType.ServiceSlots = value
				case attrTurretSlotsLeft:
					fitType.TurretSlots = value
				case attrLauncherSlotsLeft:
					fitType.LauncherSlots = value
				}
			}
			if fitType.CategoryID == subsystemCategory {
				switch attribute.AttributeID {
				case attrHiSlotModifier:
					fitType.HighSlotModifier = value
				case attrMedSlotModifier:
					fitType.MidSlotModifier = value
				case attrLowSlotModifier:
					fitType.LowSlotModifier = value
				case attrFitsToShipType:
					if subsystemGroups[value] == nil {
						subsystemGroups[value] = map[int]struct{}{}
					}
					subsystemGroups[value][fitType.GroupID] = struct{}{}
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}

	for hullID, groups := range subsystemGroups {
		if hull, kept := extract.Types[hullID]; kept && hull.SubsystemSlots > 0 {
			hull.SubsystemSlots = len(groups)
		}
	}

	if extract.SDEBuild == 0 {
		return nil, errors.New("_sde.jsonl has no build number")
	}
	reportOddities(extract)
	return extract, nil
}

// eachLine streams a jsonl file, decoding each line into a new T and handing it to use
func eachLine[T any](sde fs.FS, name string, use func(*T)) error {
	file, err := sde.Open(name)
	if err != nil {
		return fmt.Errorf("opening %s: %w", name, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for {
		line := new(T)
		if err := decoder.Decode(line); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		use(line)
	}
}

// reportOddities prints modules without a slot and hulls without slots, so a change in the SDE gets noticed
func reportOddities(extract *comm.FitTypesFile) {
	var noSlot, noHullSlots []string
	for _, id := range sortedIDs(extract.Types) {
		fitType := extract.Types[id]
		if (fitType.CategoryID == 7 || fitType.CategoryID == 66 || fitType.CategoryID == subsystemCategory) && fitType.Slot == "" {
			noSlot = append(noSlot, fmt.Sprintf("%d %s", id, fitType.Name))
		}
		if slices.Contains(hullCategories, fitType.CategoryID) && fitType.HighSlots+fitType.MidSlots+fitType.LowSlots+fitType.SubsystemSlots+fitType.ServiceSlots == 0 {
			noHullSlots = append(noHullSlots, fmt.Sprintf("%d %s", id, fitType.Name))
		}
	}
	for _, list := range []struct {
		what  string
		names []string
	}{{"modules without a slot effect", noSlot}, {"hulls without any slots", noHullSlots}} {
		if len(list.names) > 0 {
			fmt.Printf("%d %s:\n", len(list.names), list.what)
			for _, name := range list.names {
				fmt.Println("  ", name)
			}
		}
	}
}

// sortedIDs returns the type ids in numeric order
func sortedIDs(types map[int]*comm.FitType) []int {
	ids := make([]int, 0, len(types))
	for id := range types {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

/*
	marshalExtract writes the extract as json with one type per line, in type id order, so a new SDE diffs cleanly

Parameters:
  - extract: the extract

Returns:
  - []byte: the json
  - error: a value that could not be encoded
*/
func marshalExtract(extract *comm.FitTypesFile) ([]byte, error) {
	cargo, err := encode(extract.CargoKept)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "{\"sde_build\":%d,\n\"cargo_kept\":%s,\n\"types\":{", extract.SDEBuild, cargo)
	for index, id := range sortedIDs(extract.Types) {
		body, err := encode(extract.Types[id])
		if err != nil {
			return nil, err
		}
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n" + strconv.Quote(strconv.Itoa(id)) + ":")
		out.Write(body)
	}
	out.WriteString("\n}}\n")

	// check the hand built text is valid json before it is written
	if !json.Valid(out.Bytes()) {
		return nil, errors.New("the extract is not valid json")
	}
	return out.Bytes(), nil
}

// encode marshals a value without escaping <, >, and & (names such as "Shield & Armor" stay readable)
func encode(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}
