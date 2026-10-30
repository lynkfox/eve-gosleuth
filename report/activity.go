package report

import (
	"fmt"
	"strconv"
)

var activityDays = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// activityTint blends from a light tint of the accent up to the accent itself, f in 0 to 1
func activityTint(f float64) rgb {
	light := rgb{222, 232, 244}
	mix := func(a, b int) int { return a + int(float64(b-a)*f+0.5) }
	return rgb{mix(light.r, colAccent.r), mix(light.g, colAccent.g), mix(light.b, colAccent.b)}
}

// activity draws the Activity section: the weekday by hour heatmap with hourly totals under it, the time zone label, and kills and losses per week
func (d *pageDoc) activity(a Activity, s Scope) {
	d.keepTogether(60)
	d.heading("Activity")

	total, maxCell := 0, 0
	for _, row := range a.Grid {
		for _, v := range row {
			total += v
			maxCell = max(maxCell, v)
		}
	}
	if total == 0 && len(a.Weeks) == 0 {
		d.sans("", 9.5)
		d.text(colMuted)
		d.CellFormat(bodyWidth, 6, "No killmails to chart.", "", 1, "L", false, 0, "")
		d.text(colText)
		return
	}

	d.sans("B", 10.5)
	d.text(colText)
	zone := "Prime time: " + a.TimeZone
	switch a.TimeZone {
	case "Spread":
		zone = "Spread across time zones"
	case "":
		zone = "Prime time: not enough killmails"
	}
	d.CellFormat(bodyWidth, 5.5, zone, "", 1, "L", false, 0, "")
	d.sans("", 9)
	d.text(colMuted)
	d.CellFormat(bodyWidth, 5, d.fit(scopeLine(s)+", all times EVE", bodyWidth), "", 1, "L", false, 0, "")
	d.Ln(2)

	d.activityHeatmap(a, maxCell)
	d.Ln(3)
	d.activityHours(a)
	d.Ln(3)
	d.activityWeeks(a.Weeks)
	d.text(colText)
}

// activityGeometry is the heatmap's column widths: a day label, 24 hours, and a total
const (
	activityLabelW = 12.0
	activityTotalW = 14.0
	activityCellW  = (bodyWidth - activityLabelW - activityTotalW) / 24
	activityCellH  = 5.5
)

// activityHeatmap draws the 7 by 24 grid, a cell tinted by how busy it is, with each day's total at the end
func (d *pageDoc) activityHeatmap(a Activity, maxCell int) {
	d.keepTogether(9*activityCellH + 6)
	y := d.GetY()
	d.sans("", 6.5)
	d.text(colMuted)
	for h := 0; h < 24; h++ {
		d.SetXY(pageMargin+activityLabelW+float64(h)*activityCellW, y)
		d.CellFormat(activityCellW, 4.5, fmt.Sprintf("%02d", h), "", 0, "C", false, 0, "")
	}
	d.SetXY(pageMargin+activityLabelW+24*activityCellW, y)
	d.CellFormat(activityTotalW, 4.5, "Total", "", 0, "R", false, 0, "")
	y += 4.5
	for day, row := range a.Grid {
		ry := y + float64(day)*activityCellH
		d.SetXY(pageMargin, ry)
		d.sans("", 8)
		d.text(colText)
		d.CellFormat(activityLabelW, activityCellH, activityDays[day], "", 0, "L", false, 0, "")
		sum := 0
		for h, v := range row {
			sum += v
			x := pageMargin + activityLabelW + float64(h)*activityCellW
			strong := false
			if v > 0 && maxCell > 0 {
				f := float64(v) / float64(maxCell)
				d.fill(activityTint(f))
				strong = f > 0.5
			} else {
				d.fill(colBoxBg)
			}
			d.Rect(x, ry, activityCellW, activityCellH, "F")
			if v > 0 {
				d.sans("", 5.5)
				if strong {
					d.text(rgb{255, 255, 255})
				} else {
					d.text(colText)
				}
				d.SetXY(x, ry)
				d.CellFormat(activityCellW, activityCellH, strconv.Itoa(v), "", 0, "C", false, 0, "")
			}
		}
		d.SetXY(pageMargin+activityLabelW+24*activityCellW, ry)
		d.sans("B", 8)
		d.text(colText)
		d.CellFormat(activityTotalW, activityCellH, count(sum), "", 0, "R", false, 0, "")
	}
	// thin white lines between cells keep them readable
	d.draw(rgb{255, 255, 255})
	d.SetLineWidth(0.15)
	for h := 0; h <= 24; h++ {
		x := pageMargin + activityLabelW + float64(h)*activityCellW
		d.Line(x, y, x, y+7*activityCellH)
	}
	for r := 0; r <= 7; r++ {
		d.Line(pageMargin+activityLabelW, y+float64(r)*activityCellH, pageMargin+activityLabelW+24*activityCellW, y+float64(r)*activityCellH)
	}
	d.SetXY(pageMargin, y+7*activityCellH)
}

// activityHours draws the hourly totals as a bar row scaled to the busiest hour
func (d *pageDoc) activityHours(a Activity) {
	const barH = 18.0
	d.keepTogether(barH + 14)
	var hours [24]int
	top := 0
	for _, row := range a.Grid {
		for h, v := range row {
			hours[h] += v
			top = max(top, hours[h])
		}
	}
	d.sans("B", 9)
	d.text(colText)
	d.CellFormat(bodyWidth, 5, "Killmails by hour (EVE)", "", 1, "L", false, 0, "")
	y := d.GetY()
	base := y + 4 + barH
	d.fill(colAccent)
	for h, v := range hours {
		x := pageMargin + activityLabelW + float64(h)*activityCellW
		if v > 0 && top > 0 {
			bh := barH * float64(v) / float64(top)
			d.Rect(x+0.5, base-bh, activityCellW-1, bh, "F")
		}
		if v > 0 {
			d.sans("", 5.5)
			d.text(colMuted)
			d.SetXY(x-1, base-barH*float64(v)/float64(max(top, 1))-3.2)
			d.CellFormat(activityCellW+2, 3, strconv.Itoa(v), "", 0, "C", false, 0, "")
		}
		d.sans("", 6.5)
		d.text(colMuted)
		d.SetXY(x, base+0.5)
		d.CellFormat(activityCellW, 4, fmt.Sprintf("%02d", h), "", 0, "C", false, 0, "")
	}
	d.draw(colRule)
	d.SetLineWidth(0.2)
	d.Line(pageMargin+activityLabelW, base, pageMargin+activityLabelW+24*activityCellW, base)
	d.SetXY(pageMargin, base+5)
}

// activityWeeks draws kills and losses per week as paired bars with a few date labels
func (d *pageDoc) activityWeeks(weeks []Week) {
	if len(weeks) == 0 {
		return
	}
	const barH = 22.0
	d.keepTogether(barH + 22)
	ly := d.GetY()
	d.sans("B", 9)
	d.text(colText)
	d.CellFormat(bodyWidth, 5, "Kills and losses per week", "", 1, "L", false, 0, "")
	d.sans("", 8)
	d.text(colMuted)
	d.fill(colKill)
	d.Rect(pageMargin+bodyWidth-34, ly+1, 3, 3, "F")
	d.SetXY(pageMargin+bodyWidth-30, ly+0.2)
	d.CellFormat(12, 4.5, "Kills", "", 0, "L", false, 0, "")
	d.fill(colLoss)
	d.Rect(pageMargin+bodyWidth-17, ly+1, 3, 3, "F")
	d.SetXY(pageMargin+bodyWidth-13, ly+0.2)
	d.CellFormat(13, 4.5, "Losses", "", 0, "L", false, 0, "")
	d.SetXY(pageMargin, ly+5)

	top := 1
	for _, w := range weeks {
		top = max(top, w.Kills, w.Losses)
	}
	y := d.GetY() + 2
	base := y + barH
	slot := bodyWidth / float64(len(weeks))
	bw := slot * 0.42
	for i, w := range weeks {
		x := pageMargin + float64(i)*slot + slot*0.05
		if w.Kills > 0 {
			bh := barH * float64(w.Kills) / float64(top)
			d.fill(colKill)
			d.Rect(x, base-bh, bw, bh, "F")
		}
		if w.Losses > 0 {
			bh := barH * float64(w.Losses) / float64(top)
			d.fill(colLoss)
			d.Rect(x+bw, base-bh, bw, bh, "F")
		}
	}
	d.draw(colRule)
	d.SetLineWidth(0.2)
	d.Line(pageMargin, base, pageMargin+bodyWidth, base)

	// date labels, spread so they do not touch, the year added when the weeks cross one
	labels := min(len(weeks), 7)
	crossYear := weeks[0].Start.UTC().Year() != weeks[len(weeks)-1].Start.UTC().Year()
	d.sans("", 7)
	d.text(colMuted)
	for k := 0; k < labels; k++ {
		i := 0
		if labels > 1 {
			i = k * (len(weeks) - 1) / (labels - 1)
		}
		st := weeks[i].Start.UTC()
		label := st.Format("Jan ") + strconv.Itoa(st.Day())
		if crossYear {
			label += " '" + st.Format("06")
		}
		cx := pageMargin + float64(i)*slot + slot/2
		x := min(max(cx-10, pageMargin), pageMargin+bodyWidth-20)
		d.SetXY(x, base+1)
		al := "C"
		if x != cx-10 {
			al = map[bool]string{true: "L", false: "R"}[x == pageMargin]
		}
		d.CellFormat(20, 4, label, "", 0, al, false, 0, "")
	}
	d.SetXY(pageMargin, base+6)
}
