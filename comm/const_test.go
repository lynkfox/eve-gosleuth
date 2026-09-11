package comm_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoadStaticData(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("Load Static Data", func(t *testing.T) {
		err := comm.InitStaticData("../static_data")

		assert.NoError(t, err)
		assert.True(t, comm.CapitalShips.ContainsIDNumber(73787))
		assert.True(t, comm.UpwellStructures.ContainsName("Raitaru"))
	})
}
