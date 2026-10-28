package comm_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoadStaticData(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("Load Static Data", func(t *testing.T) {
		err := comm.InitStaticData(staticdata.Files)

		assert.NoError(t, err)
		assert.True(t, comm.CapitalGroups.ContainsName("Titan"))
		assert.True(t, comm.CapitalTypes.ContainsName("Bowhead"))
		assert.True(t, comm.HaulerGroups.ContainsIDNumber(28))
		assert.True(t, comm.HaulerExclusions.ContainsName("Noctis"))
		assert.True(t, comm.FitIgnoredGroups.ContainsName("Capsule"))
		assert.True(t, comm.UpwellStructureGroups.ContainsIDNumber(1657))
		assert.True(t, comm.ControlTowerGroups.ContainsIDNumber(365))
		assert.True(t, comm.PosBatteryGroups.ContainsIDNumber(417))
		assert.True(t, comm.MinorStructureGroups.ContainsIDNumber(4744))
		assert.True(t, comm.PIMaterialGroups.ContainsIDNumber(1042))
		assert.True(t, comm.SmartbombGroups.ContainsIDNumber(72))
	})

	t.Run("a type is matched against groups by its group in fit_types.json", func(t *testing.T) {
		assert.NoError(t, comm.InitStaticData(staticdata.Files))

		assert.True(t, comm.CapitalGroups.ContainsGroupOf(19720), "Revelation, a dreadnought")
		assert.True(t, comm.CapitalGroups.ContainsGroupOf(78576), "Azariel, a titan")
		assert.False(t, comm.CapitalGroups.ContainsGroupOf(587), "Rifter, a frigate")
		assert.True(t, comm.UpwellStructureGroups.ContainsGroupOf(35825), "Raitaru")
		assert.True(t, comm.ControlTowerGroups.ContainsGroupOf(12235), "Amarr Control Tower")
		assert.True(t, comm.MinorStructureGroups.ContainsGroupOf(81826), "Metenox Moon Drill")
		assert.True(t, comm.MinorStructureGroups.ContainsGroupOf(2233), "Customs Office, unpublished in the SDE")
		assert.False(t, comm.CapitalGroups.ContainsGroupOf(999999999), "a type fit_types.json does not have")
	})

	t.Run("an edited copy in an override folder is merged over the built in file", func(t *testing.T) {
		overrides := fstest.MapFS{"items.json": {Data: []byte(`{"pi_material_groups": {"1042": "Tier 1", "99999": "New Group"}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Equal(t, "Tier 1", comm.PIMaterialGroups.NameOf(1042), "the edited copy's name wins")
		assert.True(t, comm.PIMaterialGroups.ContainsIDNumber(99999), "the edited copy's additions")
		assert.True(t, comm.PIMaterialGroups.ContainsIDNumber(1034), "the built in groups it does not list")
		assert.True(t, comm.SmartbombGroups.ContainsIDNumber(72), "the built in sections it does not have")
		assert.True(t, comm.CapitalGroups.ContainsName("Titan"), "files not in the override folder still load")
	})

	t.Run("an id edited to null is removed", func(t *testing.T) {
		overrides := fstest.MapFS{"ships.json": {Data: []byte(`{"capital_types": {"34328": null}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.False(t, comm.CapitalTypes.ContainsName("Bowhead"))
		assert.True(t, comm.CapitalGroups.ContainsName("Titan"))
	})

	t.Run("a large file needs only the entries edited, and an entry only the fields", func(t *testing.T) {
		overrides := fstest.MapFS{
			"fit_types.json":      {Data: []byte(`{"types": {"587": {"group": 26}}}`)},
			"jspace_statics.json": {Data: []byte(`{"wormholes": {"H296": {"destination": "C6"}}}`)},
		}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Equal(t, 26, comm.FitTypeOf(587).GroupID)
		assert.Equal(t, "Rifter", comm.FitTypeOf(587).Name, "the fields not edited")
		assert.NotNil(t, comm.FitTypeOf(588), "the types not edited")
		statics := comm.JSpaceStatics["J001025"]
		assert.Equal(t, []string{"H296", "U210"}, []string{statics[0].Identification, statics[1].Identification})
		assert.Equal(t, "C6", statics[0].Destination)
		assert.Equal(t, "C5", statics[0].Origin, "the fields not edited")
	})

	t.Run("of two edited copies the first wins", func(t *testing.T) {
		first := fstest.MapFS{"items.json": {Data: []byte(`{"smartbomb_groups": {"72": "First"}}`)}}
		second := fstest.MapFS{"items.json": {Data: []byte(`{"smartbomb_groups": {"72": "Second", "73": "Second Only"}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, first, second))
		assert.Equal(t, "First", comm.SmartbombGroups.NameOf(72))
		assert.True(t, comm.SmartbombGroups.ContainsIDNumber(73))
	})

	t.Run("a broken edited copy is skipped", func(t *testing.T) {
		overrides := fstest.MapFS{"ships.json": {Data: []byte(`{"hauler_groups": {"28": `)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.True(t, comm.HaulerGroups.ContainsIDNumber(28))
	})

	t.Run("an edited copy the loader rejects is skipped", func(t *testing.T) {
		overrides := fstest.MapFS{"jspace_statics.json": {Data: []byte(`{"systems": {"J001025": ["NOPE"]}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Len(t, comm.JSpaceStatics["J001025"], 2)
	})

	t.Run("the files are given back as loaded, merged with the edited copies, indented", func(t *testing.T) {
		overrides := fstest.MapFS{"ships.json": {Data: []byte(`{"capital_types": {"34328": null, "99999": "New Capital"}}`)}}
		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))

		files := comm.StaticDataFiles()
		assert.Contains(t, files, "fit_types.json")
		assert.Contains(t, files, "jspace_statics.json")
		ships := string(files["ships.json"])
		assert.Contains(t, ships, `"99999": "New Capital"`)
		assert.NotContains(t, ships, "Bowhead")
		assert.Contains(t, ships, `"Titan"`, "the built in data")
		assert.Contains(t, ships, "\n    \"capital_groups\"", "indented")
	})

	t.Run("a file no source has is an error, and the others still load", func(t *testing.T) {
		builtIn := fstest.MapFS{"items.json": {Data: []byte(`{"pi_material_groups": {"1042": "Basic Commodities - Tier 1"}, "smartbomb_groups": {}}`)}}

		assert.Error(t, comm.InitStaticData(builtIn))
		assert.True(t, comm.PIMaterialGroups.ContainsName("Basic Commodities - Tier 1"))
	})
}
