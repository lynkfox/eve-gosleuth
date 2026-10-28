package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// InventoryFlagRange is one entry of inventory_flags.json: a run of inventory flags that are the same slot or hold
type InventoryFlagRange struct {
	Slot      FitSlot `json:"slot"`
	FirstFlag int     `json:"first_flag"` // the flag of the first slot, position 0
	LastFlag  int     `json:"last_flag"`  // the flag of the last slot, the same as FirstFlag for a hold
	Name      string  `json:"name"`       // a readable name, ie Low slots or Fleet hangar
}

type inventoryFlagsFile struct {
	Slots []InventoryFlagRange `json:"slots"`
}

// inventoryFlags maps every flag listed in inventory_flags.json to its range. It is empty until InitStaticData is called.
var inventoryFlags = map[int]InventoryFlagRange{}

// knownFitSlots are the slot values inventory_flags.json may use; SlotUnknown is only for flags it does not list
var knownFitSlots = map[FitSlot]bool{
	SlotHigh: true, SlotMid: true, SlotLow: true, SlotRig: true, SlotSubsystem: true, SlotService: true,
	SlotDroneBay: true, SlotFighterBay: true, SlotFighterTube: true, SlotImplant: true, SlotBooster: true,
	SlotCargo: true, SlotIgnored: true,
}

/*
	LoadInventoryFlags reads inventory_flags.json for FitSlotOf and FitSlotName, leaving the flags as they were if the file can not be read or is not valid

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name

Returns:
  - error: the file could not be opened or decoded, a range has an unknown slot or ends before it starts, or two ranges overlap
*/
func LoadInventoryFlags(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	contents := inventoryFlagsFile{}
	if err := json.NewDecoder(file).Decode(&contents); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}

	flags := map[int]InventoryFlagRange{}
	for _, entry := range contents.Slots {
		if !knownFitSlots[entry.Slot] {
			return fmt.Errorf("%s: flags %d-%d have an unknown slot %q", filename, entry.FirstFlag, entry.LastFlag, entry.Slot)
		}
		if entry.LastFlag < entry.FirstFlag {
			return fmt.Errorf("%s: %s ends at flag %d before it starts at %d", filename, entry.Name, entry.LastFlag, entry.FirstFlag)
		}
		for flag := entry.FirstFlag; flag <= entry.LastFlag; flag++ {
			if other, taken := flags[flag]; taken {
				return fmt.Errorf("%s: flag %d is in both %s and %s", filename, flag, other.Name, entry.Name)
			}
			flags[flag] = entry
		}
	}

	inventoryFlags = flags
	return nil
}

/*
	FitSlotOf reads a killmail item's inventory flag

Parameters:
  - flag: the item's flag on the killmail

Returns:
  - FitSlot: the slot or hold, SlotUnknown for a flag inventory_flags.json does not list
  - int: the slot's position from 0 for a numbered slot (ie 2 for the third low slot), 0 for a hold
*/
func FitSlotOf(flag int) (FitSlot, int) {
	entry, found := inventoryFlags[flag]
	if !found {
		return SlotUnknown, 0
	}
	return entry.Slot, flag - entry.FirstFlag
}

/*
	FitSlotName gives the readable name of the slot or hold an inventory flag is, ie Fleet hangar

Parameters:
  - flag: the item's flag on the killmail

Returns:
  - string: the name from inventory_flags.json, or "Unknown (flag N)" for a flag it does not list
*/
func FitSlotName(flag int) string {
	entry, found := inventoryFlags[flag]
	if !found {
		return fmt.Sprintf("Unknown (flag %d)", flag)
	}
	return entry.Name
}
