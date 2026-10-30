package aggregate

import "time"

// tzBands are the reading rule's bands by UTC hour, the first hour in and the last hour in
var tzBands = []struct {
	name     string
	from, to int
}{
	{"EUTZ", 16, 22},
	{"USTZ", 0, 5},
	{"AUTZ", 8, 13},
}

/*
	PrimeTimeZone names the time zone the killmails' peak hours fall in, by the EVE hour each happened

The bands are a reading rule, not game data: EUTZ 16:00 to 22:59 EVE, USTZ 00:00 to 05:59, AUTZ 08:00 to 13:59. One band is the time zone when it holds at least 40% of the killmails
and at least 1.5 times the next band's, otherwise the killmails are Spread.

Parameters:
  - times: the killmails' times

Returns:
  - string: EUTZ, USTZ, AUTZ, or Spread, empty for no times
*/
func PrimeTimeZone(times []time.Time) string {
	if len(times) == 0 {
		return ""
	}
	counts := make([]int, len(tzBands))
	for _, t := range times {
		h := t.UTC().Hour()
		for i, b := range tzBands {
			if h >= b.from && h <= b.to {
				counts[i]++
			}
		}
	}
	best := 0
	for i := range counts {
		if counts[i] > counts[best] {
			best = i
		}
	}
	next := 0
	for i := range counts {
		if i != best && counts[i] > next {
			next = counts[i]
		}
	}
	top := float64(counts[best])
	if top >= 0.4*float64(len(times)) && top >= 1.5*float64(next) && counts[best] > 0 {
		return tzBands[best].name
	}
	return "Spread"
}
