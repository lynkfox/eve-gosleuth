package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

type StaticData interface {
	LoadStaticType(staticFiles fs.FS, filename string) error
	ContainsIDNumber(idNumber int) bool
	ContainsName(name string) bool
	Contains(identifier interface{}) bool
}

type StaticValues struct {
	IDToNames map[int]string
	NameToIDs map[string]int
}

func (s *StaticValues) LoadStaticType(staticFiles fs.FS, filename string) error {

	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}

	defer file.Close()

	idToName := make(map[int]string)
	err = json.NewDecoder(file).Decode(&idToName)

	if err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}

	s.IDToNames = idToName
	for id, name := range idToName {
		s.NameToIDs[name] = id

	}

	return nil

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
