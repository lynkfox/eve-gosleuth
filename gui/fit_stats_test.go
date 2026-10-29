package gui

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFitStats(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	hull := func(id int) *retrieval.Fit {
		return &retrieval.Fit{
			Hull: &retrieval.Item{ItemID: id},
			Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048}}, nil}, // Damage Control II, and an empty slot
		}
	}

	t.Run("a hull without modes has one entry, with the module counted", func(t *testing.T) {
		bare := fitStats(&retrieval.Fit{Hull: &retrieval.Item{ItemID: 587}}, false)
		stats := fitStats(hull(587), false)
		if assert.Len(t, stats, 1) && assert.Len(t, bare, 1) {
			assert.Empty(t, stats[0].Mode)
			assert.Less(t, stats[0].Hull.Resonance[0], bare[0].Hull.Resonance[0], "the Damage Control adds hull resist")
			assert.NotContains(t, stats[0].Assumptions, "Mutated modules at their base stats")
		}
	})

	t.Run("a tactical destroyer has one entry per mode, Defense first", func(t *testing.T) {
		stats := fitStats(hull(34317), false) // Confessor
		if assert.Len(t, stats, 3) {
			assert.Equal(t, "Defense", stats[0].Mode)
			assert.NotEqual(t, stats[0].Armor.Resonance, stats[1].Armor.Resonance, "the modes differ")
		}
	})

	t.Run("a mutated module is assumed at its base stats", func(t *testing.T) {
		stats := fitStats(hull(587), true)
		if assert.Len(t, stats, 1) {
			assert.Contains(t, stats[0].Assumptions, "Mutated modules at their base stats")
		}
	})

	t.Run("a hull dogma.json doesn't have has no stats", func(t *testing.T) {
		assert.Nil(t, fitStats(hull(999999999), false))
	})

	t.Run("the capsule's implants reach the engine, boosters don't", func(t *testing.T) {
		fit := hull(587)
		fit.Implants = []*retrieval.Item{{ItemID: 20499}}
		fit.Boosters = []*retrieval.Item{{ItemID: 10151}}
		in := fit.FittingInput()
		assert.Equal(t, []int{20499}, in.Implants)
		assert.Len(t, in.Modules, 1)
	})
}

func BenchmarkFitStats(b *testing.B) {
	comm.InitLogger(logrus.WarnLevel)
	_ = comm.InitStaticData(staticdata.Files)
	fit := &retrieval.Fit{Hull: &retrieval.Item{ItemID: 587}, Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048}}}}
	for b.Loop() {
		fitStats(fit, false)
	}
}
