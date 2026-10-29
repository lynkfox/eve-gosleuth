package comm_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoadFitTagRules(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("the embedded rules load", func(t *testing.T) {
		assert.NoError(t, comm.InitStaticData(staticdata.Files))
		cyno := comm.FitTagRules().Tags["Cyno"]
		if assert.NotNil(t, cyno) {
			assert.True(t, cyno.Role)
			assert.Positive(t, cyno.Priority)
		}
		assert.Equal(t, []string{"Passive", "Active", "Buffer"}, comm.FitTagRules().Exclusive["tank style"])
	})

	t.Run("an edited copy's null tag removes it, and the rest merge over the built in rules", func(t *testing.T) {
		overrides := fstest.MapFS{"fit_tag_rules.json": {Data: []byte(`{"tags": {"Ratting": null, "Tackle": {"threshold": 3}}}`)}}
		assert.NoError(t, comm.InitStaticData(staticdata.Files, overrides))
		assert.NotContains(t, comm.FitTagRules().Tags, "Ratting")
		tackle := comm.FitTagRules().Tags["Tackle"]
		if assert.NotNil(t, tackle) {
			assert.Equal(t, 3.0, tackle.Threshold)
			assert.NotEmpty(t, tackle.Signals, "kept from the built in rule")
		}
	})

	t.Run("a file without tags is an error, and leaves the previous rules", func(t *testing.T) {
		assert.NoError(t, comm.InitStaticData(staticdata.Files))
		assert.Error(t, comm.LoadFitTagRules(fstest.MapFS{"rules.json": {Data: []byte(`{"tags": {}}`)}}, "rules.json"))
		assert.Contains(t, comm.FitTagRules().Tags, "Tackle")
	})
}
