package comm

import (
	"errors"
	"fmt"
	"io/fs"
)

const VERSION = "0.0.2"

var (
	CapitalShips     StaticData
	ControlTowers    StaticData
	UpwellStructures StaticData
	HaulerGroups     StaticData // the ESI groups of the ships that count as PI haulers: T1 haulers and T2 transports, not freighters
	HaulerExclusions StaticData // ship types in HaulerGroups that are not haulers, ie the Noctis salvager
	PIMaterialGroups StaticData // the ESI groups of Planetary Industry resources and commodities
	SmartbombGroups  StaticData // the ESI groups of smartbomb modules, the weapon a player on a ratting loss may have only bombed it with
	StructureGroups  StaticData // the ESI groups of the structures the Structures tab and a j-space system's last structure lost cover: upwell structures and control towers
	PosBatteryGroups StaticData // the ESI groups of control tower batteries, which stand in for their tower on kills, as a tower is never on a killmail's attackers
	// MinorStructureGroups and MinorStructures are the groups and types of structures that have no guns, ie Metenox moon drills and customs offices.
	// They are listed with the structures destroyed, but never as a j-space system's last structure lost.
	MinorStructureGroups StaticData
	MinorStructures      StaticData
)

func newStaticValues() *StaticValues {
	return &StaticValues{IDToNames: make(map[int]string), NameToIDs: make(map[string]int)}
}

/*
	InitStaticData loads the static data json files into the package level StaticData values

Each file is read from the first source that has a copy it can read, so an edited copy in an override folder replaces the built in one,
and a missing or broken edited copy falls back to the next source. A file no source can read leaves its values empty, and the other files still load.

Parameters:
  - staticFiles: a filesystem containing the static data json files at its root, normally staticdata.Files
  - overrides: filesystems tried first, in order, ie staticdata.Overrides()

Returns:
  - error: every file that could not be loaded from any source, nil if all loaded
*/
func InitStaticData(staticFiles fs.FS, overrides ...fs.FS) error {
	Logger.Info("Loading Static Data")
	sources := append(overrides, staticFiles)

	CapitalShips = newStaticValues()
	ControlTowers = newStaticValues()
	UpwellStructures = newStaticValues()
	HaulerGroups = newStaticValues()
	HaulerExclusions = newStaticValues()
	PIMaterialGroups = newStaticValues()
	SmartbombGroups = newStaticValues()
	StructureGroups = newStaticValues()
	PosBatteryGroups = newStaticValues()
	MinorStructureGroups = newStaticValues()
	MinorStructures = newStaticValues()
	Spaces = SpaceClasses{WormholeRegionClasses: map[string]string{}, RegionSpaceLabels: map[string]string{}}

	var errs []error
	for _, file := range []struct {
		name string
		load func(fs.FS, string) error
	}{
		{"capital_ships.json", CapitalShips.LoadStaticType},
		{"upwell_structures.json", UpwellStructures.LoadStaticType},
		{"control_towers.json", ControlTowers.LoadStaticType},
		{"hauler_groups.json", HaulerGroups.LoadStaticType},
		{"hauler_exclusions.json", HaulerExclusions.LoadStaticType},
		{"pi_material_groups.json", PIMaterialGroups.LoadStaticType},
		{"smartbomb_groups.json", SmartbombGroups.LoadStaticType},
		{"structure_groups.json", StructureGroups.LoadStaticType},
		{"pos_battery_groups.json", PosBatteryGroups.LoadStaticType},
		{"minor_structure_groups.json", MinorStructureGroups.LoadStaticType},
		{"minor_structures.json", MinorStructures.LoadStaticType},
		{"fit_ignored_hulls.json", LoadFitIgnoredHulls},
		{"jspace_statics.json", LoadJSpaceStatics},
		{"space_classes.json", LoadSpaceClasses},
		{"inventory_flags.json", LoadInventoryFlags},
		{"fit_types.json", LoadFitTypes},
	} {
		if err := loadFirstReadable(file.name, sources, file.load); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// loadFirstReadable loads a static data file from the first source that has it and can read it, warning about any copy it skips
func loadFirstReadable(name string, sources []fs.FS, load func(fs.FS, string) error) error {
	var errs []error
	for index, source := range sources {
		if _, err := fs.Stat(source, name); err != nil {
			continue // not in this source
		}
		if err := load(source, name); err != nil {
			Logger.WithError(err).Warnf("Static data %s could not be read from source %d, trying the next", name, index+1)
			errs = append(errs, err)
			continue
		}
		if index < len(sources)-1 {
			Logger.Infof("Using the edited copy of static data %s", name)
		}
		return nil
	}
	if len(errs) == 0 {
		return fmt.Errorf("static data %s was not found", name)
	}
	return errors.Join(errs...)
}
