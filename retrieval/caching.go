package retrieval

import (
	"encoding/json"
	"os"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type CachedEveIDs struct {
	Known    map[int]interface{} `json:"known"`
	Expanded map[int]interface{} `json:"expanded"`
	Client   api.ESIClient       `json:"-"`
}

func LoadCache(client api.ESIClient) (*CachedEveIDs, error) {
	file, _ := os.Open("cached_data.json")
	defer file.Close()

	file_info, _ := os.Stat("cached_data.json")
	if file == nil || file_info.Size() == 0 {
		empty_cache := CachedEveIDs{
			Known:    make(map[int]interface{}),
			Expanded: make(map[int]interface{}),
			Client:   client,
		}
		return &empty_cache, nil
	}

	decoder := json.NewDecoder(file)

	var cache CachedEveIDs

	if err := decoder.Decode(&cache); err != nil {
		comm.Logger.Fatalf("Failed to decode JSON: %s", err)
		return nil, err
	}

	cache.Client = client

	return &cache, nil
}

func SaveCache(c CachedEveIDs) {

	file, _ := os.OpenFile("cached_data.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.Encode(c)
}

/*
	Fetch checks the cached IDNumber: Esi Response interfaces and returns its pointer if found, else nil

if the data was last pulled more than 15 days ago for Characters, Corporations, and Alliances, it returns nil
*/
func (c *CachedEveIDs) Fetch(idNumber int) *interface{} {

	response, found := c.Known[idNumber]

	if !found {
		return nil
	}

	if date, ok := comm.HasProperties(response, "DateCached"); ok {
		date_cached := (date).(time.Time)
		threshold := time.Now().UTC().AddDate(0, 0, -15)
		if date_cached.Before(threshold) {
			return nil
		}
	}

	comm.Logger.Debugf("Found %d in esi cache", idNumber)
	return &response
}

/* FetchExpanded checks for an already expanded/human readable version of the data attached to an eve ID*/
func (c *CachedEveIDs) FetchExpanded(idNumber int) *interface{} {
	response, found := c.Expanded[idNumber]

	if !found {
		return nil
	}
	comm.Logger.Debugf("Found %d in expanded cache", idNumber)
	return &response
}

/* CastExpanded for an epxandable readable version of the data that is also a specific struct*/
func CastExpanded[T any](value any) (*T, error) {
	if value == nil {
		return nil, nil
	}

	if casted, ok := value.(*T); ok {
		return casted, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var data_casted T
	if err := json.Unmarshal(data, &data_casted); err != nil {
		return nil, err
	}

	return &data_casted, nil

}

/* Update adds the given interface of an Esi Response to the known id numbers */
func (c *CachedEveIDs) Update(idNumber int, item *interface{}) {
	_, found := c.Known[idNumber]
	if !found {
		c.Known[idNumber] = item
	} else {
		comm.Logger.Debugf("%d was already found in the cache, not updated", idNumber)
	}
}

/* UpdateExpanded adds the given interface of an expanded/human readable to the known id numbers */
func (c *CachedEveIDs) UpdateExpanded(idNumber int, item interface{}) {
	_, found := c.Expanded[idNumber]
	if !found {
		c.Expanded[idNumber] = item
	} else {
		comm.Logger.Debugf("%d was already found in the cache, not updated", idNumber)
	}
}

// GetIfNotCached takes an id number, esiClient and a type and returns the details of the IDnumber if known already, else calls the ESI and caches the resposne
func (c *CachedEveIDs) GetIfNotCached(idNumber int, typeOfEntity types.EntityType) (*interface{}, error) {

	found := c.Fetch(idNumber)

	if found != nil {
		wrapped, ok := (*found).(*interface{})
		if ok {
			return wrapped, nil
		}

		var err error
		var output any

		switch typeOfEntity {
		case types.PILOT:
			output, err = CastExpanded[api.ESICharacter](found)
		case types.CORPORATION:
			output, err = CastExpanded[api.ESICorporation](found)
		case types.ALLIANCE:
			output, err = CastExpanded[api.ESIAlliance](found)
		case types.SYSTEM:
			output, err = CastExpanded[api.ESISystem](found)
		case types.ITEM:
			output, err = CastExpanded[api.ESIItem](found)
		}

		if err != nil {
			comm.Logger.Errorf("Could not expand %s %d: %v", typeOfEntity, idNumber, err)
		}
		return &output, nil
	}

	var err error
	var output interface{}

	switch typeOfEntity {
	case types.PILOT:
		output, err = c.Client.GetCharacterInfo(idNumber)
	case types.CORPORATION:
		output, err = c.Client.GetCorporationInfo(idNumber)
	case types.ALLIANCE:
		output, err = c.Client.GetAllianceInfo(idNumber)
	case types.SYSTEM:
		output, err = c.Client.GetSystemInfo(idNumber)
	case types.ITEM:
		output, err = c.Client.GetItemInfo(idNumber)
	}

	if err == nil {
		c.Update(idNumber, &output)
	}

	return &output, err
}
