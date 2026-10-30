package report

import (
	"fmt"
	"strconv"
	"strings"
)

/*
keyFindings are the Summary's 4 to 6 generated lines, each from a section's data, ie "Flies 3 potential doctrines: Ferox, Guardian, Sabre". Empty when nothing is worth saying.
Each line ends in " → <Section>" naming where it comes from. A line is left out when its data would say little:

  - doctrines: at least 1 doctrine; names up to 4, then "and N more".
  - capitals: at least 3 distinct pilots in the combat classes (industrial, freighter, and jump freighter classes are not counted, they say nothing of a fleet); up to 4 classes named.
  - activity: at least 20 killmails in the grid. The peak hours are the hours (wrapping midnight) around the busiest that hold at least half its killmails, left out when they span 16 hours or more;
    the peak days are those with at least 60% of the busiest day's killmails, "Tue to Thu" when they run on, a list otherwise, "every day" when all seven.
  - trend: at least 8 weeks, at least 20 killmails in the last 4 weeks and the 4 before, and a change of 30% or more.
*/
func keyFindings(in Input) []string {
	var lines []string
	for _, l := range []string{findingDoctrines(in), findingCapitals(in), findingActivity(in), findingTrend(in)} {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func findingDoctrines(in Input) string {
	if len(in.Doctrines) == 0 {
		return ""
	}
	var names []string
	for i, dc := range in.Doctrines {
		if i == 4 {
			break
		}
		n := dc.ShipName
		if dc.Role != "" {
			n += " (" + dc.Role + ")"
		}
		names = append(names, n)
	}
	noun := "doctrines"
	if len(in.Doctrines) == 1 {
		noun = "doctrine"
	}
	text := fmt.Sprintf("Flies %d potential %s: %s", len(in.Doctrines), noun, strings.Join(names, ", "))
	if more := len(in.Doctrines) - len(names); more > 0 {
		text += fmt.Sprintf(" and %d more", more)
	}
	return text + " → Potential Fleet Doctrines"
}

func findingCapitals(in Input) string {
	all := map[string]bool{}
	var parts []string
	for _, c := range in.Capitals {
		lower := strings.ToLower(c.Name)
		if strings.Contains(lower, "freighter") || strings.Contains(lower, "industrial") {
			continue
		}
		own := map[string]bool{}
		for _, p := range c.Pilots {
			own[p.Name] = true
			all[p.Name] = true
		}
		if len(own) > 0 {
			parts = append(parts, strconv.Itoa(len(own))+" "+lower)
		}
	}
	if len(all) < 3 {
		return ""
	}
	if len(parts) > 4 {
		parts = append(parts[:4], fmt.Sprintf("%d more classes", len(parts)-4))
	}
	return fmt.Sprintf("%d capital pilots: %s → Capital Pilots", len(all), strings.Join(parts, ", "))
}

func findingActivity(in Input) string {
	a := in.Activity
	var days [7]int
	var hours [24]int
	total := 0
	for d, row := range a.Grid {
		for h, v := range row {
			days[d] += v
			hours[h] += v
			total += v
		}
	}
	if total < 20 {
		return ""
	}

	topDay := 0
	for _, v := range days {
		topDay = max(topDay, v)
	}
	var on []int
	for d, v := range days {
		if float64(v) >= 0.6*float64(topDay) {
			on = append(on, d)
		}
	}
	dayText := ""
	switch {
	case len(on) == 7:
		dayText = "every day"
	case on[len(on)-1]-on[0] == len(on)-1 && len(on) > 1:
		dayText = activityDays[on[0]] + " to " + activityDays[on[len(on)-1]]
	default:
		names := make([]string, len(on))
		for i, d := range on {
			names[i] = activityDays[d]
		}
		dayText = strings.Join(names, ", ")
		if i := strings.LastIndex(dayText, ", "); i >= 0 {
			dayText = dayText[:i] + " and " + dayText[i+2:]
		}
	}

	peak := 0
	for h, v := range hours {
		if v > hours[peak] {
			peak = h
		}
	}
	busy := func(h int) bool { return float64(hours[(h%24+24)%24]) >= 0.5*float64(hours[peak]) }
	from, to := peak, peak
	for n := 1; n < 24 && busy(from-1); n++ {
		from--
	}
	for n := 1; n < 24 && busy(to+1); n++ {
		to++
	}
	span := to - from + 1
	text := "Peak " + dayText
	if span < 16 {
		text += fmt.Sprintf(", %02d:00 to %02d:00", (from%24+24)%24, (to+1)%24)
	}
	text += " EVE"
	if a.TimeZone != "" && a.TimeZone != "Spread" {
		text += " (" + a.TimeZone + ")"
	}
	return text + " → Activity"
}

func findingTrend(in Input) string {
	w := in.Activity.Weeks
	if len(w) < 8 {
		return ""
	}
	sum := func(ws []Week) int {
		n := 0
		for _, x := range ws {
			n += x.Kills + x.Losses
		}
		return n
	}
	last, before := sum(w[len(w)-4:]), sum(w[len(w)-8:len(w)-4])
	if last+before < 20 || last == before {
		return ""
	}
	switch {
	case float64(last) >= 1.3*float64(before):
		return fmt.Sprintf("Activity rising: %d killmails in the last 4 weeks against %d in the 4 before → Activity", last, before)
	case float64(before) >= 1.3*float64(last):
		return fmt.Sprintf("Activity falling: %d killmails in the last 4 weeks against %d in the 4 before → Activity", last, before)
	}
	return ""
}

// findings draws the Key findings box under the kills and losses, nothing when there are no lines
func (d *pageDoc) findings(lines []string) {
	if len(lines) == 0 {
		return
	}
	const pad, lineH = 2.5, 6.0
	height := 5 + float64(len(lines))*lineH + 2*pad
	d.keepTogether(height + 2)
	y := d.GetY()
	d.fill(colBoxBg)
	d.draw(colRule)
	d.SetLineWidth(0.2)
	d.RoundedRect(pageMargin, y, bodyWidth, height, 1.5, "1234", "FD")
	d.SetXY(pageMargin+3, y+pad)
	d.sans("B", 9)
	d.text(colAccent)
	d.CellFormat(bodyWidth-6, 5, "KEY FINDINGS", "", 1, "L", false, 0, "")
	for _, l := range lines {
		main, ref := l, ""
		if i := strings.LastIndex(l, " → "); i >= 0 {
			main, ref = l[:i], l[i+1:]
		}
		ly := d.GetY()
		d.fill(colAccent)
		d.Circle(pageMargin+4.5, ly+lineH/2, 0.7, "F")
		d.sans("", 9.5)
		refW := 0.0
		if ref != "" {
			d.sans("", 8)
			refW = d.GetStringWidth(ref) + 3
		}
		avail := bodyWidth - 10 - refW
		d.sans("", 9.5)
		d.text(colText)
		main = d.fit(main, avail)
		d.SetX(pageMargin + 7)
		d.CellFormat(d.GetStringWidth(main)+2, lineH, main, "", 0, "LM", false, 0, "")
		if ref != "" {
			d.sans("", 8)
			d.text(colMuted)
			d.CellFormat(refW, lineH, ref, "", 0, "LM", false, 0, "")
		}
		d.SetXY(pageMargin+3, ly+lineH)
	}
	d.SetY(y + height)
	d.text(colText)
}
