package api_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetEveIdsOrNames(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("GetNamesFromEveIDs maps id to names", func(t *testing.T) {
		jsonData := `[
			{
			  "category": "alliance",
			  "id": 99005382,
			  "name": "Jita Holding Inc."
			},
			{
			  "category": "inventory_type",
			  "id": 33468,
			  "name": "Astero"
			},
			{
			  "category": "solar_system",
			  "id": 30000142,
			  "name": "Jita"
			},
			{
			  "category": "character",
			  "id": 2112625428,
			  "name": "CCP Zoetrope"
			},
			{
			  "category": "corporation",
			  "id": 98739641,
			  "name": "Umbrian Dawn"
			}
		  ]`

		mock := CreateMockResposne(jsonData, "GET", "https://esi.evetech.net/latest/universe/names/", 200)

		esiClient := api.ESIConnection{
			Client: mock,
		}

		result, err := esiClient.GetNamesFromEveIDs([]int{123, 234, 456, 789})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Umbrian Dawn", result[98739641])

	})

	t.Run("GetEveIDsfromNames returns map of id to names", func(t *testing.T) {
		jsonData := `{
			"alliances": [
			  {"id": 99013330, "name": "URSA."}
			],
			"characters": [
			  {"id": 2112625428, "name": "CCP Zoetrope"}
			],
			"corporations": [
			  {"id": 98739641, "name": "Umbrian Dawn"}
			],
			"inventory_types": [
			  {"id": 33468, "name": "Astero"}
			],
			"systems": [
			  {"id": 30000142, "name": "Jita"}
			]
		  }`

		mock := CreateMockResposne(jsonData, "GET", "https://esi.evetech.net/latest/universe/names/", 200)

		esiClient := api.ESIConnection{
			Client: mock,
		}

		result, err := esiClient.GetEveIDsFromNames([]string{"abc", "dbe", "efg"})

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Umbrian Dawn", result[98739641].Name)
	})

	t.Run("GetCharacterInfo returns the character and adds id number to response", func(t *testing.T) {
		jsonData := `{
		  "alliance_id": 99013330,
		  "birthday": "2023-04-18T04:16:00Z",
		  "bloodline_id": 11,
		  "corporation_id": 98744491,
		  "description": "<font size=\"13\" color=\"#bfffffff\"></font><font size=\"12\" color=\"#bfffffff\">Wormholes. Its whats for dinner.<br><br><br><br>Former member of </font><font size=\"12\" color=\"#ffd98d00\"><a href=\"showinfo:2//917701062\">EVE University</a></font><font size=\"12\" color=\"#bfffffff\"> - the premier learning community for EVE Online. If you need to learn, that is a place to go.<br><br></font><font size=\"14\" color=\"#ffffffff\"><u>EVE Wiki by EVE University</u><br></font><font size=\"12\" color=\"#ffffe400\"><loc><a href=\"http://wiki.eveuniversity.org/\">http://wiki.eveuniversity.org/</a></loc><br><br></font><font size=\"14\" color=\"#ffffffff\"><u>Join EVE University</u><br></font><font size=\"12\" color=\"#ffffe400\"><a href=\"https://wiki.eveuniversity.org/Applying_to_EVE_University\">https://wiki.eveuniversity.org/Applying_to_EVE_University</a><br><br><br></font><font size=\"12\" color=\"#bfffffff\">There is/was no war in C6 Space.<br><br>But if there ever was... here is a timeline of it:<br><br>https://lynkfox.github.io/eve_battle_report_timeline/war.html</font>",
		  "gender": "female",
		  "name": "Luxxianna Seraph",
		  "race_id": 1,
		  "security_status": -0.693360423
		}`

		mock := CreateMockResposne(jsonData, "GET", "https://esi.evetech.net/characters/2121139046", 200)

		esiClient := api.ESIConnection{
			Client: mock,
		}

		result, err := esiClient.GetCharacterInfo(2121139046)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2121139046, result.CharacterID)
		assert.Equal(t, "Luxxianna Seraph", result.Name)

	})
}
