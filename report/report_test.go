package report_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func fixture() report.Input {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.Set(x, y, color.RGBA{59, 110, 165, 255})
		}
	}
	var logo bytes.Buffer
	_ = png.Encode(&logo, img)
	return report.Input{
		Target: report.Target{
			Name: "Ursa Major Holdings", Ticker: "URSA.", Type: "CORPORATION",
			Details: []report.Detail{{"Alliance", "Pandemic Horde"}, {"CEO", "Some Pilot"}, {"Members", "1,234"}, {"Founded", "Oct 3rd, 2012"}},
			Logo:    logo.Bytes(),
		},
		Scope: report.Scope{
			Killmails: 1523,
			Oldest:    time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC),
			Newest:    time.Date(2026, 10, 22, 21, 0, 0, 0, time.UTC),
			Limits:    []string{"At most 500 killmails", "Killmails newer than 30 days"},
			Sources: []report.Source{
				{Name: "Ursa Major Holdings", Main: true, Killmails: 1200, Oldest: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Newest: time.Date(2026, 10, 22, 21, 0, 0, 0, time.UTC)},
				{Name: "Child Corp One", Killmails: 323, Oldest: time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC), Newest: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
				{Name: "Child Corp Two"},
			},
			Stopped: true, RateLimited: true,
		},
		Kills:       report.Tally{Count: 1100, ISK: 123_456_789_012},
		Losses:      report.Tally{Count: 423, ISK: 45_600_000_000},
		GeneratedAt: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		AppVersion:  "0.9.3",
	}
}

func TestBuildFull(t *testing.T) {
	b, err := report.Build(fixture(), []report.Section{report.SectionSummary})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
	if p := os.Getenv("REPORT_SAMPLE"); p != "" {
		assert.NoError(t, os.WriteFile(p, b, 0o644))
	}
}

func TestBuildNoKillmails(t *testing.T) {
	in := fixture()
	in.Scope = report.Scope{}
	in.Kills, in.Losses = report.Tally{}, report.Tally{}
	in.Target.Logo = nil
	b, err := report.Build(in, nil)
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}

func TestBuildBadLogo(t *testing.T) {
	in := fixture()
	in.Target.Logo = []byte("definitely not an image at all")
	b, err := report.Build(in, nil)
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
	in.Target.Logo = append([]byte("\x89PNG\r\n\x1a\n"), []byte("garbage")...)
	b, err = report.Build(in, nil)
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}

func TestBuildUnknownSectionsIgnored(t *testing.T) {
	a, err := report.Build(fixture(), nil)
	assert.NoError(t, err)
	b, err := report.Build(fixture(), []report.Section{"nonsense", report.SectionSummary, report.SectionSummary})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
	assert.Equal(t, bytes.Count(a, []byte("/Type /Page\n")), bytes.Count(b, []byte("/Type /Page\n")))
}
