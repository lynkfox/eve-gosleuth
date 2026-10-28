package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
)

// StaticValues is one id to name list of a sectioned static data file, ie the capital_groups of ships.json
type StaticValues struct {
	IDToNames map[int]string
	NameToIDs map[string]int
}

// staticValuesOf makes the StaticValues of an id to name list
func staticValuesOf(idToName map[int]string) *StaticValues {
	values := newStaticValues()
	for id, name := range idToName {
		values.IDToNames[id] = name
		values.NameToIDs[name] = id
	}
	return values
}

/*
	loadSections makes the loader of a static data file of id to name lists, which fills each StaticValues from the section of its name

Every section must be in the file, but as an edited copy is merged over the built in one (see loadMerged), it need only hold the sections it changes. Sections the file has but nothing reads are ignored.
A file that can not be read leaves every StaticValues as it was.

Parameters:
  - sections: each section's name to the StaticValues it fills

Returns:
  - func(fs.FS, string) error: the loader, for InitStaticData
*/
func loadSections(sections map[string]**StaticValues) func(fs.FS, string) error {
	return func(staticFiles fs.FS, filename string) error {
		file, err := staticFiles.Open(filename)
		if err != nil {
			return fmt.Errorf("unable to load file %s: %w", filename, err)
		}
		defer file.Close()

		read := map[string]map[int]string{}
		if err := json.NewDecoder(file).Decode(&read); err != nil {
			return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
		}

		loaded := map[string]*StaticValues{}
		for name := range sections {
			idToName, found := read[name]
			if !found {
				return fmt.Errorf("the json file %s has no %s section", filename, name)
			}
			loaded[name] = staticValuesOf(idToName)
		}
		for name, values := range sections {
			*values = loaded[name]
		}
		return nil
	}
}

func (s *StaticValues) ContainsIDNumber(idNumber int) bool {
	_, found := s.IDToNames[idNumber]

	return found
}

func (s *StaticValues) ContainsName(name string) bool {

	_, found := s.NameToIDs[name]

	return found
}

func (s *StaticValues) Contains(identifier interface{}) bool {

	if value, ok := identifier.(string); ok {
		return s.ContainsName(value)
	}
	if value, ok := identifier.(int); ok {
		return s.ContainsIDNumber(value)
	}
	Logger.Debugf("value of %s is not a string or int, cannot check if it is part of this static data", identifier)
	return false
}

// IDNumbers returns every id, smallest first
func (s *StaticValues) IDNumbers() []int {
	ids := make([]int, 0, len(s.IDToNames))
	for id := range s.IDToNames {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// NameOf returns an id's name, empty if it is not one of these
func (s *StaticValues) NameOf(idNumber int) string {
	return s.IDToNames[idNumber]
}

// ContainsGroupOf reports whether an item type's group, from fit_types.json (TypeGroup), is one of these
func (s *StaticValues) ContainsGroupOf(typeID int) bool {
	group := TypeGroup(typeID)
	return group != 0 && s.ContainsIDNumber(group)
}
