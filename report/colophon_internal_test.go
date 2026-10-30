package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColophonStaysOnOnePage(t *testing.T) {
	in := Input{Target: Target{Name: "Ursa Major Holdings", Ticker: "URSA."}, AppVersion: "1.0.0", ReleasesURL: "https://example.com/releases"}
	d := newDoc(in)
	d.AddPage()
	d.colophon(in)
	assert.Equal(t, 1, d.PageNo(), "the note fits on an empty page")
	whole := d.GetY() - pageMargin

	_, pageH := d.GetPageSize()
	for _, room := range []float64{whole - 15, whole - 2} {
		d := newDoc(in)
		d.AddPage()
		d.SetY(pageH - pageBottom - room)
		d.colophon(in)
		assert.NoError(t, d.Error())
		assert.Equal(t, 2, d.PageNo(), "with %.0f mm left the note moves on", room)
		assert.InDelta(t, pageMargin+whole, d.GetY(), 0.01, "with %.0f mm left, the whole note is on the next page", room)
	}
}
