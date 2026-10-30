package report_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func capitalFixture() []report.CapitalClass {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.Set(x, y, color.RGBA{165, 110, 59, 255})
		}
	}
	var icon bytes.Buffer
	_ = png.Encode(&icon, img)
	seen := time.Date(2026, 10, 20, 19, 0, 0, 0, time.UTC)
	long := "The Extremely Long Corporation Name Of Doom Incorporated"
	many := make([]report.CapitalPilot, 80)
	for i := range many {
		many[i] = report.CapitalPilot{Name: fmt.Sprintf("Carrier Pilot %d", i), Ship: "Thanatos", Corp: long, Alliance: "Some Alliance", LastSeen: seen, LastSystem: "1DQ1-A"}
	}
	return []report.CapitalClass{
		{Name: "Titans", Hulls: []report.CapitalHull{{Name: "Avatar", Icon: icon.Bytes(), Pilots: 1}, {Name: "Erebus", Pilots: 1}},
			Pilots: []report.CapitalPilot{
				{Name: "Pilot One", Ship: "Avatar", Corp: long, Alliance: "Pandemic Horde", LastSeen: seen, LastSystem: "Jita"},
				{Name: "Pilot Two", Ship: "Erebus", Corp: "Small", LastSeen: seen, LastSystem: "T5ZI-S", Left: true},
			}},
		{Name: "Dreadnoughts and Lancers", Hulls: []report.CapitalHull{{Name: "Moros", Pilots: 2}, {Name: "Revelation", Pilots: 1}},
			Pilots: []report.CapitalPilot{
				{Name: "Pilot One", Ship: "Moros", Corp: "Corp", LastSeen: seen, LastSystem: "Jita"},
				{Name: "Pilot Three", Ship: "Revelation", LastSeen: seen},
			}},
		{Name: "Carriers", Hulls: []report.CapitalHull{{Name: "Thanatos", Pilots: 80}}, Pilots: many},
	}
}

func TestBuildCapitals(t *testing.T) {
	in := fixture()
	in.Capitals = capitalFixture()
	b, err := report.Build(in, []report.Section{report.SectionSummary, report.SectionCapitals})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}

func TestBuildNoCapitals(t *testing.T) {
	b, err := report.Build(fixture(), []report.Section{report.SectionCapitals})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}
