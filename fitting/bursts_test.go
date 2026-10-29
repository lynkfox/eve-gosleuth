package fitting_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// real types, from the embedded dogma.json
const (
	damnation       = 22474
	armorBurstI     = 42526
	armorBurstII    = 43552
	armorEnergizing = 42832 // Armor Energizing Charge: armor resists
	emColumn        = 0     // a resonance's EM entry
)

// TestCommandBursts checks a fit's own bursts reach its ship, only the strongest of each buff counts, and the stats say so
func TestCommandBursts(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	compute := func(modules ...fitting.Module) *fitting.Stats {
		stats, err := fitting.Compute(fitting.Input{Hull: damnation, Modules: modules})
		assert.NoError(t, err)
		return stats
	}
	burst := func(burstType int) fitting.Module {
		return fitting.Module{Type: burstType, Charge: armorEnergizing, Slot: comm.SlotHigh}
	}

	bare := compute()
	one := compute(burst(armorBurstII))
	assert.False(t, bare.Boosted)
	assert.True(t, one.Boosted)
	assert.Less(t, one.Armor.Resonance[emColumn], bare.Armor.Resonance[emColumn], "Armor Energizing adds armor resist")
	assert.Equal(t, bare.Shield.Resonance, one.Shield.Resonance, "but no shield resist")

	// a second burst giving the same buff adds nothing, and a weaker one beside a stronger one doesn't weaken it
	assert.Equal(t, one.Armor.Resonance, compute(burst(armorBurstII), burst(armorBurstII)).Armor.Resonance)
	assert.Equal(t, one.Armor.Resonance, compute(burst(armorBurstI), burst(armorBurstII)).Armor.Resonance)
	assert.Less(t, one.Armor.Resonance[emColumn], compute(burst(armorBurstI)).Armor.Resonance[emColumn], "the T2 burst is the stronger")

	// an unloaded burst gives nothing
	unloaded := compute(fitting.Module{Type: armorBurstII, Slot: comm.SlotHigh})
	assert.False(t, unloaded.Boosted)
	assert.Equal(t, bare.Armor.Resonance, unloaded.Armor.Resonance)
}
