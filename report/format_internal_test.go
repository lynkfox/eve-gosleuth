package report

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpokenDate(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 10, day, 23, 0, 0, 0, time.UTC) }
	for day, want := range map[int]string{1: "Oct 1st, 2026", 2: "Oct 2nd, 2026", 3: "Oct 3rd, 2026", 4: "Oct 4th, 2026",
		11: "Oct 11th, 2026", 12: "Oct 12th, 2026", 13: "Oct 13th, 2026", 21: "Oct 21st, 2026", 22: "Oct 22nd, 2026", 29: "Oct 29th, 2026"} {
		assert.Equal(t, want, spokenDate(d(day)))
	}
	// converted to EVE time
	est := time.FixedZone("x", -5*3600)
	assert.Equal(t, "Oct 30th, 2026", spokenDate(time.Date(2026, 10, 29, 20, 0, 0, 0, est)))
}

func TestShortISK(t *testing.T) {
	assert.Equal(t, "0", shortISK(0))
	assert.Equal(t, "999", shortISK(999))
	assert.Equal(t, "789k", shortISK(789_000))
	assert.Equal(t, "45.6m", shortISK(45_600_000))
	assert.Equal(t, "1.23b", shortISK(1_234_000_000))
	assert.Equal(t, "2b", shortISK(2_000_000_000))
}

func TestCount(t *testing.T) {
	assert.Equal(t, "7", count(7))
	assert.Equal(t, "1,234", count(1234))
	assert.Equal(t, "1,234,567", count(1234567))
}

func TestFit(t *testing.T) {
	d := newDoc(Input{})
	d.AddPage()
	d.sans("", 10)
	assert.Equal(t, "Short", d.fit("Short", 50))
	cut := d.fit("A name far too long to fit in a narrow cell", 30)
	assert.True(t, strings.HasSuffix(cut, "…"), cut)
	assert.LessOrEqual(t, d.GetStringWidth(cut), 30.0)
}
