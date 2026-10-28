package retrieval

import (
	"slices"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// Fit is a killmail victim's ship as it was fitted, read from the killmail's items with BuildFit
type Fit struct {
	Hull *Item

	// each slot slice is as long as the hull has slots, with nil for an empty slot. A T3 cruiser's high, mid, and low counts include its subsystems' modifiers.
	Highs      []*FittedModule
	Mids       []*FittedModule
	Lows       []*FittedModule
	Rigs       []*FittedModule
	Subsystems []*FittedModule
	Services   []*FittedModule // an upwell structure's service modules

	Drones   []*FitCargo
	Fighters []*FitCargo // the fighter bay and launched fighters together
	Cargo    []*FitCargo // only what KeptInFitCargo shows: charges, scripts, boosters, and nanite paste
	Implants []*Item
	Boosters []*Item

	MissingTypes []int // item types fit_types.json does not have (newer than the SDE it was made from), placed by their flag alone. Includes the hull if it is missing.
}

// HasModules reports whether anything is fitted in any of the fit's slots, so a loss with nothing fitted can be left out
func (f *Fit) HasModules() bool {
	if f == nil {
		return false
	}
	for _, slots := range [][]*FittedModule{f.Highs, f.Mids, f.Lows, f.Rigs, f.Subsystems, f.Services} {
		for _, slot := range slots {
			if slot != nil {
				return true
			}
		}
	}
	return false
}

// FittedModule is a module in one of a fit's slots
type FittedModule struct {
	Module  *Item
	Charge  *Item // nil if unloaded
	Dropped bool  // it dropped from the wreck, false if it was destroyed
}

// FitCargo is a stack in a fit's drone bay, fighter bay, or cargo, with the destroyed and dropped quantities together
type FitCargo struct {
	Item     *Item
	Quantity int
}

// fitEntry is one item type at one flag on a killmail, its destroyed and dropped entries merged
type fitEntry struct {
	typeID    int
	flag      int
	fitType   *comm.FitType // nil if fit_types.json does not have it
	destroyed int
	dropped   int
}

// slotted is the module and charge entries at one slot flag, in killmail order
type slotted struct {
	slot     comm.FitSlot
	position int
	entries  []*fitEntry
}

/*
	BuildFit reads a killmail victim's fit from its items. Names come from fit_types.json, a type it does not have is looked up through the cache (ExpandItem) and placed by its flag alone.

	- Destroyed and dropped entries of the same type and flag are merged.
	- A charge shares its module's flag: the item whose type has a slot is the module, the other its charge (see moduleAt).
	- Items nested in a container or a packed ship are never placed: api.ESIKillmailItem does not decode a parent's items, as a ship in the maintenance bay keeps its own slot flags.
	- Cargo keeps only what KeptInFitCargo shows. Ignored holds and flags inventory_flags.json does not list are left out.

Parameters:
  - shipTypeID: the victim's hull
  - items: the victim's items, as the ESI lists them

Returns:
  - *Fit: the fit, never nil. A victim with no items has the hull and empty slots, one with no slots (ie a deployable) empty slices.
*/
func (cached *CachedEveIDs) BuildFit(shipTypeID int, items []api.ESIKillmailItem) *Fit {
	hullType := comm.FitTypeOf(shipTypeID)
	fit := &Fit{Hull: cached.fitItem(shipTypeID, hullType)}
	missing := map[int]struct{}{}
	if hullType == nil {
		missing[shipTypeID] = struct{}{}
	}

	// destroyed and dropped entries of the same type and flag become one
	entries := []*fitEntry{}
	merged := map[[2]int]*fitEntry{}
	for _, item := range items {
		key := [2]int{item.Flag, item.ItemTypeID}
		entry, found := merged[key]
		if !found {
			entry = &fitEntry{typeID: item.ItemTypeID, flag: item.Flag, fitType: comm.FitTypeOf(item.ItemTypeID)}
			merged[key] = entry
			entries = append(entries, entry)
		}
		if item.QuantityDestroyed != nil {
			entry.destroyed += *item.QuantityDestroyed
		}
		if item.QuantityDropped != nil {
			entry.dropped += *item.QuantityDropped
		}
	}

	// slot entries are gathered by flag first, since a charge shares its module's flag and a T3's slot counts depend on its subsystems
	slots := []*slotted{}
	byFlag := map[int]*slotted{}
	for _, entry := range entries {
		slot, position := comm.FitSlotOf(entry.flag)
		switch slot {
		case comm.SlotIgnored:
			continue
		case comm.SlotUnknown:
			comm.Logger.Debugf("Fit of %d: leaving out type %d at flag %d, inventory_flags.json does not list the flag", shipTypeID, entry.typeID, entry.flag)
			continue
		}
		// cargo is only shown when fit_types.json keeps it, so a type it does not have there (ore, loot, blueprints) is not missing from the fit
		if entry.fitType == nil && slot != comm.SlotCargo {
			missing[entry.typeID] = struct{}{}
		}

		switch slot {
		case comm.SlotHigh, comm.SlotMid, comm.SlotLow, comm.SlotRig, comm.SlotSubsystem, comm.SlotService:
			at, found := byFlag[entry.flag]
			if !found {
				at = &slotted{slot: slot, position: position}
				byFlag[entry.flag] = at
				slots = append(slots, at)
			}
			at.entries = append(at.entries, entry)
		case comm.SlotDroneBay:
			fit.Drones = addFitCargo(fit.Drones, cached.fitItem(entry.typeID, entry.fitType), entry.destroyed+entry.dropped)
		case comm.SlotFighterBay, comm.SlotFighterTube:
			fit.Fighters = addFitCargo(fit.Fighters, cached.fitItem(entry.typeID, entry.fitType), entry.destroyed+entry.dropped)
		case comm.SlotCargo:
			if entry.fitType != nil && comm.KeptInFitCargo(entry.fitType) {
				fit.Cargo = addFitCargo(fit.Cargo, cached.fitItem(entry.typeID, entry.fitType), entry.destroyed+entry.dropped)
			}
		case comm.SlotImplant:
			fit.Implants = addFitItem(fit.Implants, cached.fitItem(entry.typeID, entry.fitType))
		case comm.SlotBooster:
			fit.Boosters = addFitItem(fit.Boosters, cached.fitItem(entry.typeID, entry.fitType))
		}
	}

	modules := make([]*FittedModule, len(slots))
	for index, at := range slots {
		modules[index] = cached.moduleAt(shipTypeID, at)
	}

	high, mid, low := 0, 0, 0
	if hullType != nil {
		high, mid, low = hullType.HighSlots, hullType.MidSlots, hullType.LowSlots
		// a T3 cruiser's subsystems add its high, mid, and low slots
		for index, at := range slots {
			if at.slot != comm.SlotSubsystem || modules[index] == nil {
				continue
			}
			if subsystem := comm.FitTypeOf(modules[index].Module.ItemID); subsystem != nil {
				high += subsystem.HighSlotModifier
				mid += subsystem.MidSlotModifier
				low += subsystem.LowSlotModifier
			}
		}
		fit.Rigs = make([]*FittedModule, hullType.RigSlots)
		fit.Subsystems = make([]*FittedModule, hullType.SubsystemSlots)
		fit.Services = make([]*FittedModule, hullType.ServiceSlots)
	}
	fit.Highs, fit.Mids, fit.Lows = make([]*FittedModule, high), make([]*FittedModule, mid), make([]*FittedModule, low)

	for index, at := range slots {
		if modules[index] == nil {
			continue
		}
		switch at.slot {
		case comm.SlotHigh:
			fit.Highs = placeModule(fit.Highs, at.position, modules[index])
		case comm.SlotMid:
			fit.Mids = placeModule(fit.Mids, at.position, modules[index])
		case comm.SlotLow:
			fit.Lows = placeModule(fit.Lows, at.position, modules[index])
		case comm.SlotRig:
			fit.Rigs = placeModule(fit.Rigs, at.position, modules[index])
		case comm.SlotSubsystem:
			fit.Subsystems = placeModule(fit.Subsystems, at.position, modules[index])
		case comm.SlotService:
			fit.Services = placeModule(fit.Services, at.position, modules[index])
		}
	}

	for typeID := range missing {
		fit.MissingTypes = append(fit.MissingTypes, typeID)
	}
	slices.Sort(fit.MissingTypes)
	return fit
}

/*
	moduleAt picks the module and its charge from the entries at one slot flag, in killmail order.

	- A type with a slot is the module. Without one, the first entry that is not a charge is (a type fit_types.json does not have counts as one): some modules have no slot in fit_types.json (ie the Lux Kontos, quantum cores), and an item alone on a slot flag is fitted there.
	  Only when every entry is a charge is the first one the module.
	- The charge is a type fit_types.json has without a slot. A type it does not have is the charge only beside a module with a slot, as otherwise the two can not be told apart.
	- Anything else at the flag is left out.

Returns:
  - *FittedModule: the module, nil if the flag held nothing
*/
func (cached *CachedEveIDs) moduleAt(shipTypeID int, at *slotted) *FittedModule {
	var module, charge *fitEntry
	rest := []*fitEntry{}
	for _, entry := range at.entries {
		if entry.fitType != nil && entry.fitType.Slot != "" && module == nil {
			module = entry
			continue
		}
		rest = append(rest, entry)
	}
	hasSlot := module != nil
	if !hasSlot && len(rest) > 0 {
		// a charge, which KeptInFitCargo shows, never takes the slot from a module it can be told apart from
		pick := 0
		for index, entry := range rest {
			if !comm.KeptInFitCargo(entry.fitType) {
				pick = index
				break
			}
		}
		module = rest[pick]
		rest = slices.Delete(slices.Clone(rest), pick, pick+1)
	}
	if module == nil {
		return nil
	}

	// a known charge first, then a type fit_types.json does not have beside a module with a slot
	for _, entry := range rest {
		if charge == nil && entry.fitType != nil && entry.fitType.Slot == "" {
			charge = entry
		}
	}
	for _, entry := range rest {
		if charge == nil && entry.fitType == nil && hasSlot {
			charge = entry
		}
	}
	for _, entry := range rest {
		if entry != charge {
			comm.Logger.Debugf("Fit of %d: leaving out type %d at flag %d, it cannot be told apart from type %d in the same slot", shipTypeID, entry.typeID, entry.flag, module.typeID)
		}
	}

	fitted := &FittedModule{Module: cached.fitItem(module.typeID, module.fitType), Dropped: module.dropped > 0}
	if charge != nil {
		fitted.Charge = cached.fitItem(charge.typeID, charge.fitType)
	}
	return fitted
}

// fitItem names an item type from fit_types.json, or through the cache for a type it does not have
func (cached *CachedEveIDs) fitItem(typeID int, fitType *comm.FitType) *Item {
	if fitType == nil {
		return cached.ExpandItem(typeID)
	}
	return &Item{ItemID: typeID, TypeOfEntity: types.ITEM, Name: fitType.Name}
}

// placeModule puts a module in its slot, growing the slots for a position beyond the hull's count so the module is still shown
func placeModule(slots []*FittedModule, position int, module *FittedModule) []*FittedModule {
	if position >= len(slots) {
		slots = append(slots, make([]*FittedModule, position+1-len(slots))...)
	}
	slots[position] = module
	return slots
}

// addFitCargo adds a stack, merged with any of the same type already listed
func addFitCargo(stacks []*FitCargo, item *Item, quantity int) []*FitCargo {
	for _, stack := range stacks {
		if stack.Item.ItemID == item.ItemID {
			stack.Quantity += quantity
			return stacks
		}
	}
	return append(stacks, &FitCargo{Item: item, Quantity: quantity})
}

// addFitItem adds an implant or booster, once per type
func addFitItem(listed []*Item, item *Item) []*Item {
	if slices.ContainsFunc(listed, func(other *Item) bool { return other.ItemID == item.ItemID }) {
		return listed
	}
	return append(listed, item)
}
