package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
)

// FitCargoRules says which cargo a fit shows: every type in one of the categories or groups
type FitCargoRules struct {
	Categories []int `json:"categories"`
	Groups     []int `json:"groups"`
}

// FitTypesFile is the layout of fit_types.json, written by cmd/sdegen from CCP's static data export
type FitTypesFile struct {
	SDEBuild  int              `json:"sde_build"`  // the SDE build the file was made from
	CargoKept FitCargoRules    `json:"cargo_kept"` // the cargo a fit shows, for KeptInFitCargo
	Types     map[int]*FitType `json:"types"`      // keyed by type id
}

// fitTypes is the loaded fit_types.json. Empty until InitStaticData is called.
var fitTypes = FitTypesFile{Types: map[int]*FitType{}}

/*
	LoadFitTypes reads fit_types.json for FitTypeOf, KeptInFitCargo, and FitTypesSDEBuild, leaving them as they were if the file can not be read

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name, ie fit_types.json

Returns:
  - error: the file could not be opened or read
*/
func LoadFitTypes(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	loaded := FitTypesFile{}
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}

	types := make(map[int]*FitType, len(loaded.Types))
	for typeID, fitType := range loaded.Types {
		if fitType == nil {
			continue // a null entry
		}
		fitType.TypeID = typeID
		types[typeID] = fitType
	}
	loaded.Types = types
	fitTypes = loaded
	return nil
}

/*
	FitTypeOf looks up an item type in fit_types.json

Parameters:
  - typeID: the item type

Returns:
  - *FitType: the type, nil if fit_types.json does not have it (ie a type newer than the SDE it was made from). Shared, so do not change it.
*/
func FitTypeOf(typeID int) *FitType {
	return fitTypes.Types[typeID]
}

/*
	KeptInFitCargo reports whether cargo of this type is shown with a fit: charges and ammo, scripts, boosters, and nanite paste. The rest of the cargo is not.

Parameters:
  - fitType: the cargo's type, nil for a type fit_types.json does not have

Returns:
  - bool: true if a fit shows it, false for nil
*/
func KeptInFitCargo(fitType *FitType) bool {
	if fitType == nil {
		return false
	}
	return slices.Contains(fitTypes.CargoKept.Categories, fitType.CategoryID) || slices.Contains(fitTypes.CargoKept.Groups, fitType.GroupID)
}

/*
	FitTypesSDEBuild returns the build of CCP's static data export that fit_types.json was made from

Returns:
  - int: the build number, 0 if none is loaded
*/
func FitTypesSDEBuild() int {
	return fitTypes.SDEBuild
}

// FitIgnoredHulls is the layout of fit_ignored_hulls.json: the hulls whose losses are not shown as fits, ie capsules, haulers, mining ships, and structures, by category, group, or single type. Each maps an id to a readable name.
type FitIgnoredHulls struct {
	Categories map[int]string `json:"categories"`
	Groups     map[int]string `json:"groups"`
	Types      map[int]string `json:"types"`
}

// fitIgnoredHulls is the loaded fit_ignored_hulls.json. Empty until InitStaticData is called.
var fitIgnoredHulls = FitIgnoredHulls{}

/*
	LoadFitIgnoredHulls reads fit_ignored_hulls.json for FitHullIgnored, leaving it as it was if the file can not be read

Parameters:
  - staticFiles: the filesystem holding the file
  - filename: the file's name, ie fit_ignored_hulls.json

Returns:
  - error: the file could not be opened or read
*/
func LoadFitIgnoredHulls(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	loaded := FitIgnoredHulls{}
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}
	fitIgnoredHulls = loaded
	return nil
}

/*
	FitHullIgnored reports whether a hull's losses are left out of fits, by its type, group, or category in fit_ignored_hulls.json

Parameters:
  - hull: the hull's type, nil for a type fit_types.json does not have

Returns:
  - bool: true if its fits are not shown. false for a nil hull, whose type is not known.
*/
func FitHullIgnored(hull *FitType) bool {
	if hull == nil {
		return false
	}
	_, byType := fitIgnoredHulls.Types[hull.TypeID]
	_, byGroup := fitIgnoredHulls.Groups[hull.GroupID]
	_, byCategory := fitIgnoredHulls.Categories[hull.CategoryID]
	return byType || byGroup || byCategory
}
