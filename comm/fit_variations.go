package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// FitVariationsFile is the layout of fit_variations.json, written by cmd/sdegen from the same SDE as fit_types.json
type FitVariationsFile struct {
	SDEBuild int          `json:"sde_build"` // the SDE build the file was made from, which should match fit_types.json's
	Parents  map[int]*int `json:"parents"`   // each module variant to its base type, ie a Heavy Gunnar Compact Stasis Grappler to the Heavy Stasis Grappler I. A type that is its own base type is left out; null is what an edited copy removed.
}

// fitVariations are the loaded parents of fit_variations.json. Empty until InitStaticData is called.
var fitVariations = map[int]int{}

/*
	LoadFitVariations reads fit_variations.json for BaseType, leaving it as it was if the file can not be read or has no parents section

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name, ie fit_variations.json

Returns:
  - error: the file could not be opened or read, or has no parents section
*/
func LoadFitVariations(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	loaded := FitVariationsFile{}
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}
	if loaded.Parents == nil {
		return fmt.Errorf("the json file %s has no parents", filename) // ie an edited copy set parents to null
	}

	parents := make(map[int]int, len(loaded.Parents))
	for typeID, parent := range loaded.Parents {
		if parent != nil && *parent != typeID {
			parents[typeID] = *parent
		}
	}
	if fitBuild := FitTypesSDEBuild(); fitBuild != 0 && loaded.SDEBuild != fitBuild {
		Logger.Warnf("fit_variations.json is from SDE build %d but fit_types.json from %d, so some module variants may not merge", loaded.SDEBuild, fitBuild)
	}
	fitVariations = parents
	return nil
}

/*
	BaseType looks up a module's base type in fit_variations.json: the type its T2, meta, faction, deadspace, officer, and mutated variants descend from

Parameters:
  - typeID: the item type

Returns:
  - int: its base type, or typeID itself when it has none listed
*/
func BaseType(typeID int) int {
	if parent, found := fitVariations[typeID]; found {
		return parent
	}
	return typeID
}
