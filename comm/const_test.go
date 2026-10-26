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
		assert.True(t, comm.CapitalShips.ContainsIDNumber(73787))
		assert.True(t, comm.UpwellStructures.ContainsName("Raitaru"))
		assert.True(t, comm.HaulerGroups.ContainsIDNumber(28))
		assert.True(t, comm.HaulerExclusions.ContainsName("Noctis"))
		assert.True(t, comm.PIMaterialGroups.ContainsIDNumber(1042))
	})

	t.Run("an edited copy in an override folder replaces the built in file", func(t *testing.T) {
		overrides := fstest.MapFS{"pi_material_groups.json": {Data: []byte(`{"1042": "Basic Commodities - Tier 1"}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.True(t, comm.PIMaterialGroups.ContainsIDNumber(1042))
		assert.False(t, comm.PIMaterialGroups.ContainsIDNumber(1034), "only the edited copy's groups")
		assert.True(t, comm.CapitalShips.ContainsIDNumber(73787), "files not in the override folder still load")
	})

	t.Run("a broken edited copy falls back to the built in file", func(t *testing.T) {
		overrides := fstest.MapFS{"hauler_groups.json": {Data: []byte(`{"28": `)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.True(t, comm.HaulerGroups.ContainsIDNumber(28))
	})

	t.Run("a file no source has is an error, and the others still load", func(t *testing.T) {
		builtIn := fstest.MapFS{"capital_ships.json": {Data: []byte(`{"19720": "Revelation"}`)}}

		assert.Error(t, comm.InitStaticData(builtIn))
		assert.True(t, comm.CapitalShips.ContainsName("Revelation"))
	})
}
