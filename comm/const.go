package comm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"testing/fstest"
)

const VERSION = "0.0.2"

// The id to name lists of the sectioned static data files: ships.json, structures.json, and items.json. Groups and categories are ESI ones, matched against a type's through fit_types.json (StaticValues.ContainsGroupOf). Empty until InitStaticData is called.
var (
	CapitalGroups        = newStaticValues() // the groups of the ships the Capital Pilots tab counts: titans, dreadnoughts, carriers, force auxiliaries, ...
	CapitalTypes         = newStaticValues() // capital ship types outside CapitalGroups, none built in, for edited copies
	HaulerGroups         = newStaticValues() // the groups of the ships that count as PI haulers: T1 haulers and T2 transports, not freighters
	HaulerExclusions     = newStaticValues() // ship types in HaulerGroups that are not haulers, ie the Noctis salvager
	FitIgnoredCategories = newStaticValues() // FitIgnoredCategories, FitIgnoredGroups, and FitIgnoredTypes are the hulls whose losses are not shown as fits (FitHullIgnored)
	FitIgnoredGroups     = newStaticValues()
	FitIgnoredTypes      = newStaticValues()
	LogiGroups           = newStaticValues() // LogiGroups and LogiTypes are the logistics hulls a fleet doctrine flies with: T2 logi, logi frigates, force auxiliaries, and the T1 logi cruisers and frigates
	LogiTypes            = newStaticValues()
	NPCNotRatGroups      = newStaticValues() // NPCs that don't mean PvE: gate and station sentry guns, CONCORD, police, and faction navies. Matched by the ESI's group of the NPC's ship, as fit_types.json has no NPCs.

	UpwellStructureGroups = newStaticValues() // the upwell structures the Structures tab and a j-space system's last structure lost cover
	ControlTowerGroups    = newStaticValues() // control towers, which the same cover, but are never on a killmail's attackers, their batteries are instead
	PosBatteryGroups      = newStaticValues() // control tower batteries, which stand in for their tower on kills
	MinorStructureGroups  = newStaticValues() // structures with no guns, ie Metenox moon drills and customs offices: listed with the structures destroyed, but never as a j-space system's last structure lost

	PIMaterialGroups = newStaticValues() // Planetary Industry resources and commodities
	SmartbombGroups  = newStaticValues() // smartbomb modules, the weapon a player on a ratting loss may have only bombed it with
	FitMergedGroups  = newStaticValues() // module groups whose types count as one module when merging fits, ie every ECM flavor
)

// staticFile is a static data file and the function that loads it
type staticFile struct {
	name string
	load func(fs.FS, string) error
}

// sectionedFile is a static data file of id to name lists, each list in the section of its name
type sectionedFile struct {
	name     string
	sections map[string]**StaticValues
}

// sectionedFiles are ships.json, structures.json, and items.json
func sectionedFiles() []sectionedFile {
	return []sectionedFile{
		{"ships.json", map[string]**StaticValues{
			"capital_groups":         &CapitalGroups,
			"capital_types":          &CapitalTypes,
			"hauler_groups":          &HaulerGroups,
			"hauler_exclusions":      &HaulerExclusions,
			"fit_ignored_categories": &FitIgnoredCategories,
			"fit_ignored_groups":     &FitIgnoredGroups,
			"fit_ignored_types":      &FitIgnoredTypes,
			"logi_groups":            &LogiGroups,
			"logi_types":             &LogiTypes,
			"npc_not_rat_groups":     &NPCNotRatGroups,
		}},
		{"structures.json", map[string]**StaticValues{
			"upwell_structure_groups": &UpwellStructureGroups,
			"control_tower_groups":    &ControlTowerGroups,
			"pos_battery_groups":      &PosBatteryGroups,
			"minor_structure_groups":  &MinorStructureGroups,
		}},
		{"items.json", map[string]**StaticValues{
			"pi_material_groups": &PIMaterialGroups,
			"smartbomb_groups":   &SmartbombGroups,
			"fit_merged_groups":  &FitMergedGroups,
		}},
	}
}

func newStaticValues() *StaticValues {
	return &StaticValues{IDToNames: make(map[int]string), NameToIDs: make(map[string]int)}
}

/*
	InitStaticData loads the static data json files into the package level values

Each file is merged across the sources that have it (see loadMerged), so an edited copy in an override folder need only hold what it adds, changes, or removes,
and a broken edited copy is skipped. A file no source can read leaves its values empty, and the other files still load.

Parameters:
  - staticFiles: a filesystem containing the static data json files at its root, normally staticdata.Files
  - overrides: filesystems of edited copies laid over staticFiles, the first winning, ie staticdata.Overrides()

Returns:
  - error: every file that could not be loaded from any source, nil if all loaded
*/
func InitStaticData(staticFiles fs.FS, overrides ...fs.FS) error {
	Logger.Info("Loading Static Data")
	sources := append(overrides, staticFiles)

	Spaces = SpaceClasses{WormholeRegionClasses: map[string]string{}, RegionSpaceLabels: map[string]string{}}
	loadedFiles = map[string][]byte{}

	files := []staticFile{}
	for _, file := range sectionedFiles() {
		for _, values := range file.sections {
			*values = newStaticValues()
		}
		files = append(files, staticFile{file.name, loadSections(file.sections)})
	}
	files = append(files,
		staticFile{"jspace_statics.json", LoadJSpaceStatics},
		staticFile{"space_classes.json", LoadSpaceClasses},
		staticFile{"inventory_flags.json", LoadInventoryFlags},
		staticFile{"fit_types.json", LoadFitTypes},
		staticFile{"dogma.json", LoadDogma}, // after fit_types.json, to compare their builds
		staticFile{"fit_tag_rules.json", LoadFitTagRules},
	)

	var errs []error
	for _, file := range files {
		if err := loadMerged(file.name, sources, file.load); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// loadedFiles are the static data files as InitStaticData last loaded them, merged with any edited copies, by name
var loadedFiles = map[string][]byte{}

/*
	StaticDataFiles returns the static data files as InitStaticData loaded them, each merged with any edited copies, as indented json

Returns:
  - map[string][]byte: each file's json by name. A file no source could load is left out.
*/
func StaticDataFiles() map[string][]byte {
	files := make(map[string][]byte, len(loadedFiles))
	for name, data := range loadedFiles {
		var indented bytes.Buffer
		if err := json.Indent(&indented, data, "", "    "); err != nil {
			files[name] = data // a loaded file is json, so this is not expected
			continue
		}
		files[name] = indented.Bytes()
	}
	return files
}

/*
	loadMerged loads a static data file merged across the sources that have it: the last source's copy, then each copy before it laid over that (mergeJSON), so the first source's wins

Each step is loaded as it is merged, so a copy that can not be read, or whose merge the loader rejects, is skipped with a warning, leaving the merge before it loaded.

Parameters:
  - name: the file's name
  - sources: the filesystems that may have it, the first winning, ie the override folder then the embedded files
  - load: the file's loader

Returns:
  - error: no source has the file, or none of their copies could be loaded
*/
func loadMerged(name string, sources []fs.FS, load func(fs.FS, string) error) error {
	var errs []error
	var loaded []byte // the merge the loader last took
	for index := len(sources) - 1; index >= 0; index-- {
		data, err := fs.ReadFile(sources[index], name)
		if errors.Is(err, fs.ErrNotExist) {
			continue // not in this source
		}
		if err == nil && loaded != nil {
			data, err = mergeJSON(loaded, data)
		}
		if err == nil {
			err = load(fstest.MapFS{name: {Data: data}}, name) // the loaders read a filesystem, and the merge is in memory
		}
		if err != nil {
			Logger.WithError(err).Warnf("Static data %s from source %d could not be used, it is skipped", name, index+1)
			errs = append(errs, err)
			continue
		}
		if index < len(sources)-1 {
			Logger.Infof("Using the edited copy of static data %s", name)
		}
		loaded = data
	}
	if loaded != nil {
		loadedFiles[name] = loaded
		return nil
	}
	if len(errs) == 0 {
		return fmt.Errorf("static data %s was not found", name)
	}
	return errors.Join(errs...)
}

/*
	mergeJSON lays an edited static data file over another

Objects are merged key by key, an edited key's value laid over the other's, and a key edited to null is removed. Anything else, lists included, is replaced whole.
Both must be a single json object, as every static data file is.

Parameters:
  - base: the file being edited
  - edited: the edited copy

Returns:
  - []byte: the merged file
  - error: either is not a single valid json object
*/
func mergeJSON(base []byte, edited []byte) ([]byte, error) {
	decode := func(data []byte) (map[string]any, error) {
		var value any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber() // ids and masses stay exact
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if _, err := decoder.Token(); err != io.EOF {
			return nil, errors.New("there is more after the json object")
		}
		object, isObject := value.(map[string]any)
		if !isObject {
			return nil, errors.New("it is not a json object")
		}
		return object, nil
	}
	under, err := decode(base)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the json being edited: %w", err)
	}
	over, err := decode(edited)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the edited json: %w", err)
	}
	var merged bytes.Buffer
	encoder := json.NewEncoder(&merged)
	encoder.SetEscapeHTML(false) // names keep their & as written, for StaticDataFiles
	if err := encoder.Encode(mergeValues(under, over)); err != nil {
		return nil, err
	}
	return merged.Bytes(), nil
}

// mergeValues lays one decoded json value over another, see mergeJSON
func mergeValues(under any, over any) any {
	underObject, underIsObject := under.(map[string]any)
	overObject, overIsObject := over.(map[string]any)
	if !underIsObject || !overIsObject {
		return over
	}
	merged := maps.Clone(underObject)
	for key, value := range overObject {
		if value == nil {
			delete(merged, key)
			continue
		}
		merged[key] = mergeValues(merged[key], value)
	}
	return merged
}
