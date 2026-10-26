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

// wormholeRegionClasses maps the first letter of a wormhole region name (ie E-R00024) to the class of every system in it
var wormholeRegionClasses = map[string]string{
	"A": "C1",
	"B": "C2",
	"C": "C3",
	"D": "C4",
	"E": "C5",
	"F": "C6",
	"G": "Thera",
	"H": "C13",
	"K": "Drifter",
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
		return wormholeRegionClasses[regionName[:1]]
	}

	return ""
}
