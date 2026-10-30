package comm_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoadFitVariations(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("a listed type gives its base type and an unlisted one itself", func(t *testing.T) {
		files := fstest.MapFS{"v.json": {Data: []byte(`{"sde_build":1,"parents":{"41058":41040,"5":5}}`)}}

		assert.NoError(t, comm.LoadFitVariations(files, "v.json"))
		assert.Equal(t, 41040, comm.BaseType(41058))
		assert.Equal(t, 41040, comm.BaseType(41040))
		assert.Equal(t, 5, comm.BaseType(5))
		assert.Equal(t, 999, comm.BaseType(999))
	})

	t.Run("a file without parents is rejected and the old ones stay", func(t *testing.T) {
		good := fstest.MapFS{"v.json": {Data: []byte(`{"sde_build":1,"parents":{"2":1}}`)}}
		bad := fstest.MapFS{"v.json": {Data: []byte(`{"sde_build":1}`)}}

		assert.NoError(t, comm.LoadFitVariations(good, "v.json"))
		assert.Error(t, comm.LoadFitVariations(bad, "v.json"))
		assert.Error(t, comm.LoadFitVariations(fstest.MapFS{}, "v.json"))
		assert.Equal(t, 1, comm.BaseType(2))
	})

	t.Run("the built in file loads, and an override adds, changes, and removes", func(t *testing.T) {
		assert.NoError(t, comm.InitStaticData(staticdata.Files))
		assert.Equal(t, 41040, comm.BaseType(41058))
		assert.Equal(t, 25890, comm.BaseType(26290))

		overrides := fstest.MapFS{"fit_variations.json": {Data: []byte(`{"parents": {"41058": null, "26290": 1, "99999": 98765}}`)}}
		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.Equal(t, 41058, comm.BaseType(41058), "null removed it")
		assert.Equal(t, 1, comm.BaseType(26290), "changed")
		assert.Equal(t, 98765, comm.BaseType(99999), "added")
		assert.Equal(t, 41040, comm.BaseType(41054), "the rest stay")
	})
}
