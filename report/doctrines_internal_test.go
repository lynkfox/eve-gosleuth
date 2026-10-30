package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func tileTexts(tiles []doctrineTile) []string {
	out := []string{}
	for _, t := range tiles {
		out = append(out, t.text)
	}
	return out
}

func TestDoctrineTilesRange(t *testing.T) {
	low := &FightStats{EHP: 84200, Tank: "Shield", Weakest: "Thermal", WeakestResist: 0.30, SensorType: "Radar", SensorStrength: 20.04, Neut: 12.5, RemoteRep: 300, LocalRep: 45}
	high := &FightStats{EHP: 91500, Tank: "Shield", Weakest: "Thermal", WeakestResist: 0.351, SensorType: "Radar", SensorStrength: 20.04, Neut: 12.5, RemoteRep: 300, LocalRep: 52}

	assert.Equal(t, []string{"84.2k – 91.5k EHP", "Shield tank", "Shoot: Thermal (30 – 35% resist)", "Sensor Radar 20", "Neut 12.5 GJ/s", "Remote rep 300 HP/s", "Local rep 45 – 52 HP/s"},
		tileTexts(doctrineRangeTiles(low, high)))
	assert.Equal(t, tileTexts(doctrineRangeTiles(low, nil)), tileTexts(doctrineRangeTiles(low, low)), "no max: as for a fit lost once")
	assert.Equal(t, "Shoot: Thermal (30% resist)", tileTexts(doctrineRangeTiles(low, nil))[2])
	assert.Equal(t, "Shoot: Thermal (30 – 35% resist)", tileTexts(doctrineRangeTiles(high, low))[2], "lower first even when handed the other way")
	assert.Nil(t, doctrineRangeTiles(nil, high))
}

func TestDoctrineModuleNote(t *testing.T) {
	assert.Equal(t, "Grappler", doctrineModule(ModuleCount{Name: "Grappler", Count: 1}))
	assert.Equal(t, "2× Grappler (1 fit had a different type)", doctrineModule(ModuleCount{Name: "Grappler", Count: 2, Different: 1}))
	assert.Equal(t, "Grappler (2 fits had a different type)", doctrineModule(ModuleCount{Name: "Grappler", Count: 1, Different: 2}))
}
