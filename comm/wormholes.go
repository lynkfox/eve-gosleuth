package comm

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// WormholeStatic is one static wormhole of a j-space system, from jspace_statics.json
type WormholeStatic struct {
	Identification string `json:"identification"` // the wormhole type code, ie H296
	Origin         string `json:"origin"`         // the class of system the wormhole type spawns in, or Unknown
	Destination    string `json:"destination"`    // Highsec, Lowsec, Nullsec, or C1 - C6
}

// JSpaceStatics maps a j-space system name to its statics. It is empty until InitStaticData is called.
var JSpaceStatics = map[string][]WormholeStatic{}

// SpaceClasses are the kinds of space that a system's region decides, from space_classes.json. Empty until InitStaticData is called.
type SpaceClasses struct {
	WormholeRegionClasses map[string]string `json:"wormhole_region_classes"` // the first letter of a wormhole region name (ie E-R00024) to the class of every system in it
	RegionSpaceLabels     map[string]string `json:"region_space_labels"`     // whole k-space regions with their own kind of space, ie Pochven to Triglavian
}

// Spaces is the loaded space_classes.json
var Spaces = SpaceClasses{WormholeRegionClasses: map[string]string{}, RegionSpaceLabels: map[string]string{}}

// LoadSpaceClasses reads the space classes file into Spaces, leaving it as it was if the file can not be read
func LoadSpaceClasses(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}
	defer file.Close()

	classes := SpaceClasses{}
	if err := json.NewDecoder(file).Decode(&classes); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}
	if classes.WormholeRegionClasses == nil {
		classes.WormholeRegionClasses = map[string]string{}
	}
	if classes.RegionSpaceLabels == nil {
		classes.RegionSpaceLabels = map[string]string{}
	}
	Spaces = classes
	return nil
}

// LoadJSpaceStatics reads the j-space system statics file into JSpaceStatics
func LoadJSpaceStatics(staticFiles fs.FS, filename string) error {
	file, err := staticFiles.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to load file %s: %w", filename, err)
	}

	defer file.Close()

	statics := map[string][]WormholeStatic{}
	if err := json.NewDecoder(file).Decode(&statics); err != nil {
		return fmt.Errorf("failed to unmarshal the json file %s: %w", filename, err)
	}

	JSpaceStatics = statics
	return nil
}

/*
	WormholeClass returns the class of a j-space system, ie C5.

The class comes from the Origin of the system's statics. A few systems list statics whose origins disagree, or are all Unknown (shattered systems), so then the class comes from the wormhole region name instead.

Parameters:
  - systemName: the system name, ie J105443
  - regionName: the system's region name, ie E-R00025

Returns:
  - string: the class, or an empty string if the system is not in j-space or the class can not be found
*/
func WormholeClass(systemName string, regionName string) string {
	origins := map[string]struct{}{}
	for _, static := range JSpaceStatics[systemName] {
		if static.Origin != "Unknown" {
			origins[static.Origin] = struct{}{}
		}
	}

	if len(origins) == 1 {
		for origin := range origins {
			return origin
		}
	}

	if _, isJSpace := JSpaceStatics[systemName]; isJSpace && strings.Contains(regionName, "-R") {
		return Spaces.WormholeRegionClasses[regionName[:1]]
	}

	return ""
}
