package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/stretchr/testify/assert"
)

func tzTimes(hours ...int) []time.Time {
	out := make([]time.Time, len(hours))
	for i, h := range hours {
		out[i] = time.Date(2026, 10, 1, h, 30, 0, 0, time.UTC)
	}
	return out
}

func TestPrimeTimeZone(t *testing.T) {
	cases := []struct {
		name  string
		hours []int
		want  string
	}{
		{"none", nil, ""},
		{"eutz", []int{18, 19, 20, 21, 3}, "EUTZ"},
		{"ustz", []int{0, 1, 2, 3, 17}, "USTZ"},
		{"autz", []int{8, 9, 10, 11, 20}, "AUTZ"},
		{"spread even", []int{1, 9, 18, 19, 2, 10}, "Spread"},
		{"close second", []int{18, 19, 20, 1, 2, 3}, "Spread"},
		{"below 40 percent", []int{18, 19, 6, 7, 14, 15, 23}, "Spread"},
		{"outside every band", []int{6, 7, 14, 15}, "Spread"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, aggregate.PrimeTimeZone(tzTimes(c.hours...)))
		})
	}
}

func TestPrimeTimeZoneUsesUTC(t *testing.T) {
	loc := time.FixedZone("x", 5*3600)
	tm := time.Date(2026, 10, 1, 23, 0, 0, 0, loc) // 18:00 UTC
	assert.Equal(t, "EUTZ", aggregate.PrimeTimeZone([]time.Time{tm}))
}
