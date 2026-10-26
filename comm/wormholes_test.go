package comm_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestWormholeClass(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	cases := []struct {
		name   string
		system string
		region string
		class  string
	}{
		{"statics that agree give the class from their origin", "J055520", "B-R00008", "C2"},
		{"statics with conflicting origins fall back to the region", "J001025", "E-R00024", "C5"},
		{"statics with only unknown origins fall back to the region (shattered)", "J000186", "H-R00032", "C13"},
		{"k-space has no class", "Jita", "The Forge", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.class, comm.WormholeClass(c.system, c.region))
		})
	}

	t.Run("statics are loaded with their code and destination", func(t *testing.T) {
		statics := comm.JSpaceStatics["J001025"]

		assert.Equal(t, []string{"H296", "U210"}, []string{statics[0].Identification, statics[1].Identification})
		assert.Equal(t, []string{"C5", "Lowsec"}, []string{statics[0].Destination, statics[1].Destination})
	})
}
