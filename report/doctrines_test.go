package report_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func doctrineRender() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for x := 0; x < 32; x++ {
		for y := 0; y < 32; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 8), 90, uint8(y * 8), 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func doctrineFixture(n int) []report.Doctrine {
	seen := time.Date(2026, 10, 20, 19, 0, 0, 0, time.UTC)
	racks := []report.Rack{
		{Name: "High", Modules: []report.ModuleCount{{Name: "Heavy Missile Launcher II", Count: 3}, {Name: "Small Energy Neutralizer II", Count: 1}}},
		{Name: "Mid", Modules: []report.ModuleCount{{Name: "Large Shield Extender II", Count: 2}}},
		{Name: "Drones", Modules: nil},
	}
	fight := &report.FightStats{Mode: "Defense", EHP: 84200, Tank: "Shield", Weakest: "Thermal", WeakestResist: 0.35,
		SensorType: "Magnetometric", SensorStrength: 21.4, Neut: 12.5, RemoteRep: 300, LocalRep: 45}
	var out []report.Doctrine
	for i := 0; i < n; i++ {
		classes := []string{"Battleships", "Battlecruisers", "Cruisers"}
		eft := fmt.Sprintf("[Ferox, Ferox %d]\nDamage Control II\n\nLarge Shield Extender II\n\nHeavy Missile Launcher II, Scourge Heavy Missile\n\nMedium Core Defense Field Extender I\n\nHobgoblin II x5\n", i)
		out = append(out, report.Doctrine{ShipName: fmt.Sprintf("Ferox %d", i), HullClass: classes[i*len(classes)/max(n, 1)], EFT: eft, Render: doctrineRender(), Role: "Shield DPS",
			Losses: 40 - i, Pilots: 12, LastSeen: seen, LastSystem: "Delve", Racks: racks, Fight: fight,
			Reason: "seen with 7 Ferox and 2 logi on one side"})
	}
	return out
}

func doctrineBuild(t *testing.T, ds []report.Doctrine) {
	in := fixture()
	in.Doctrines = ds
	for _, sections := range [][]report.Section{
		{report.SectionSummary, report.SectionDoctrines},
		{report.SectionSummary, report.SectionDoctrines, report.SectionAppendix},
		{report.SectionSummary, report.SectionAppendix},
	} {
		b, err := report.Build(in, sections)
		assert.NoError(t, err, sections)
		assert.True(t, bytes.HasPrefix(b, []byte("%PDF")), sections)
		assert.True(t, bytes.Contains(b, []byte("/Outlines")), "each section is in the bookmarks pane")
		if len(ds) > 0 && len(sections) == 3 {
			assert.True(t, bytes.Contains(b, []byte("/Annots")), "the doctrines link to the appendix")
		}
	}
}

func TestDoctrines(t *testing.T) {
	var mods []report.ModuleCount
	for i := 0; i < 40; i++ {
		mods = append(mods, report.ModuleCount{Name: fmt.Sprintf("Very Long Module Name Number %d II", i), Count: i + 1})
	}
	ds := []report.Doctrine{
		doctrineFixture(1)[0],
		{ShipName: "Guardian", Role: "Logi", Losses: 1, Pilots: 1, Racks: doctrineFixture(1)[0].Racks, Reason: "x"},
		{ShipName: strings.Repeat("Extraordinarily Long Ship Name ", 5), Losses: 9, Pilots: 3, LastSeen: time.Now(),
			Racks: []report.Rack{{Name: "Rig", Modules: mods}}, Reason: strings.Repeat("seen with many ships ", 20)},
	}
	doctrineBuild(t, ds)
}

func TestDoctrinesNone(t *testing.T) {
	doctrineBuild(t, nil)
}

func TestDoctrinesManyPages(t *testing.T) {
	doctrineBuild(t, doctrineFixture(12))
}
