package comm

import "fmt"

const VERSION = "0.0.2"

var (
	CapitalShips     StaticData
	ControlTowers    StaticData
	UpwellStructures StaticData
)

func InitStaticData(staticDataPath string) (err error) {
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
	err = CapitalShips.LoadStaticType(fmt.Sprintf("%s/capital_ships.json", staticDataPath))
	if err != nil {
		return err

	}

	Logger.Debug("Loading Upwell Structures")
	err = UpwellStructures.LoadStaticType(fmt.Sprintf("%s/upwell_structures.json", staticDataPath))
	if err != nil {
		return err
	}

	Logger.Debug("Loading Control Towers")
	err = ControlTowers.LoadStaticType(fmt.Sprintf("%s/control_towers.json", staticDataPath))

	return err
}
