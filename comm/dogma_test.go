package comm_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoadDogma(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)

	t.Run("the built in file loads, with its lookups by id and by name", func(t *testing.T) {
		assert.NoError(t, comm.InitStaticData(staticdata.Files))
		assert.NotNil(t, comm.DogmaTypeOf(587), "the Rifter")
		assert.Equal(t, 84, comm.DogmaAttributeNamed("armorDamageAmount").AttributeID)
		assert.Equal(t, "armorRepair", comm.DogmaEffectOf(27).Name)
		assert.Equal(t, 27, comm.DogmaEffectNamed("armorRepair").EffectID)
		assert.Equal(t, comm.FitTypesSDEBuild(), comm.DogmaSDEBuild())
		assert.NotEmpty(t, comm.DogmaSkills())
		assert.NotNil(t, comm.DogmaCharacterType())
		assert.Len(t, comm.DogmaModesOf(34317), 3, "the Confessor's modes")
		assert.True(t, comm.DogmaUnpenalized(6658, 109), "Bastion's hull resists")
		assert.False(t, comm.DogmaUnpenalized(6658, 267), "but not its armor's")
	})

	t.Run("an edited copy needs only the value it changes", func(t *testing.T) {
		// the user guide's example: a Medium Armor Repairer II's repair amount
		overrides := fstest.MapFS{"dogma.json": {Data: []byte(`{"types": {"3530": {"attributes": {"84": 400}}}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		repairer := comm.DogmaTypeOf(3530)
		assert.Equal(t, 400.0, repairer.Attributes[84])
		assert.NotEmpty(t, repairer.Effects, "the fields not edited")
		assert.Equal(t, 62, repairer.Group, "the fields not edited")
		assert.NotNil(t, comm.DogmaTypeOf(587), "the types not edited")
	})

	t.Run("entries edited to null are removed", func(t *testing.T) {
		overrides := fstest.MapFS{"dogma.json": {Data: []byte(`{"types": {"3530": null}, "attributes": {"84": null}, "effects": {"27": null}}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Nil(t, comm.DogmaTypeOf(3530))
		assert.Nil(t, comm.DogmaAttributeOf(84))
		assert.Nil(t, comm.DogmaAttributeNamed("armorDamageAmount"))
		assert.Nil(t, comm.DogmaEffectOf(27))
		assert.Nil(t, comm.DogmaEffectNamed("armorRepair"))
		assert.NotNil(t, comm.DogmaTypeOf(587))
	})

	t.Run("an edited copy that empties the types is skipped", func(t *testing.T) {
		overrides := fstest.MapFS{"dogma.json": {Data: []byte(`{"types": null}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.NotNil(t, comm.DogmaTypeOf(587))
	})

	t.Run("a dogma.json from another SDE build than fit_types.json is warned about", func(t *testing.T) {
		var logged bytes.Buffer
		comm.Logger.SetOutput(&logged)
		defer comm.InitLogger(logrus.WarnLevel)
		overrides := fstest.MapFS{"dogma.json": {Data: []byte(`{"sde_build": 1}`)}}

		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Equal(t, 1, comm.DogmaSDEBuild())
		assert.Contains(t, logged.String(), "dogma.json is from SDE build 1")
	})
}
