package comm

import "io/fs"

const VERSION = "0.0.2"

var (
	CapitalShips     StaticData
	ControlTowers    StaticData
	UpwellStructures StaticData
)

/*
	InitStaticData loads the static data json files into the package level StaticData values

Parameters:
  - staticFiles: a filesystem containing the static data json files at its root, normally staticdata.Files
*/
func InitStaticData(staticFiles fs.FS) (err error) {
	Logger.Info("Loading Static Data")
	CapitalShips = &StaticValues{
		IDToNames: make(map[int]string),
		NameToIDs: make(map[string]int),
	}

	ControlTowers = &StaticValues{
		IDToNames: make(map[int]string),
		NameToIDs: make(map[string]int),
	}

	UpwellStructures = &StaticValues{
		IDToNames: make(map[int]string),
		NameToIDs: make(map[string]int),
	}

	Logger.Debug("Loading Capital Ships")
	err = CapitalShips.LoadStaticType(staticFiles, "capital_ships.json")
	if err != nil {
		return err

	}

	Logger.Debug("Loading Upwell Structures")
	err = UpwellStructures.LoadStaticType(staticFiles, "upwell_structures.json")
	if err != nil {
		return err
	}

	Logger.Debug("Loading Control Towers")
	err = ControlTowers.LoadStaticType(staticFiles, "control_towers.json")
	if err != nil {
		return err
	}

	Logger.Debug("Loading J-Space Statics")
	return LoadJSpaceStatics(staticFiles, "jspace_statics.json")
}
