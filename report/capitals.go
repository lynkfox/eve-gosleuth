package report

import (
	"fmt"
	"strings"
)

const (
	capitalIconSize = 8.0 // mm, a hull's icon in the strip
	capitalStripH   = 9.5 // mm, one line of the hull strip
)

// capitals draws the Capital Pilots section: a count line by class, then per class its hulls and a table of its pilots
func (d *pageDoc) capitals(classes []CapitalClass) {
	d.heading("Capital Pilots")
	if len(classes) == 0 {
		d.sans("", 9.5)
		d.text(colMuted)
		d.CellFormat(bodyWidth, 6, "No capital pilots were seen.", "", 1, "L", false, 0, "")
		d.text(colText)
		return
	}

	all := map[string]bool{}
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		names := capitalDistinct(c.Pilots)
		for _, p := range c.Pilots {
			all[p.Name] = true
		}
		parts = append(parts, fmt.Sprintf("%d %s", len(names), c.Name))
	}
	d.sans("B", 10.5)
	d.text(colText)
	d.MultiCell(bodyWidth, 5.5, fmt.Sprintf("%d capital pilots: %s", len(all), strings.Join(parts, ", ")), "", "L", false)
	d.sans("", 8.5)
	d.text(colMuted)
	d.MultiCell(bodyWidth, 4.5, "Pilots seen flying a capital for the target on a killmail. Left: no longer in the target's alliance.", "", "L", false)
	d.Ln(2)
	d.text(colText)

	for _, c := range classes {
		d.capitalClass(c)
	}
}

// capitalDistinct is the pilots' distinct names
func capitalDistinct(pilots []CapitalPilot) map[string]bool {
	m := map[string]bool{}
	for _, p := range pilots {
		m[p.Name] = true
	}
	return m
}

// capitalClass draws one class: its sub-heading, its hull strip, and its pilot table
func (d *pageDoc) capitalClass(c CapitalClass) {
	lines := d.capitalStripLines(c.Hulls)
	// the sub-heading, the strip, the table header, and a few rows stay together
	d.keepTogether(8 + float64(lines)*capitalStripH + 6 + 5.8*3)

	d.sans("B", 11)
	d.text(colAccent)
	d.CellFormat(bodyWidth, 6.5, fmt.Sprintf("%s (%d)", c.Name, len(capitalDistinct(c.Pilots))), "", 1, "L", false, 0, "")
	d.text(colText)
	d.capitalStrip(c.Hulls)
	d.Ln(1.5)

	cols := []column{
		{Title: "Pilot", Width: 34}, {Title: "Ship", Width: 28}, {Title: "Corp / Alliance", Width: 50},
		{Title: "Last seen", Width: 24}, {Title: "System", Width: 30}, {Title: "Left", Width: 14},
	}
	d.sans("", 9)
	rows := make([][]string, 0, len(c.Pilots))
	for _, p := range c.Pilots {
		corp := p.Corp
		if p.Alliance != "" {
			if corp != "" {
				corp += " / "
			}
			corp += p.Alliance
		}
		left := ""
		if p.Left {
			left = "Left"
		}
		seen := ""
		if !p.LastSeen.IsZero() {
			seen = spokenDate(p.LastSeen)
		}
		rows = append(rows, []string{
			d.fit(p.Name, cols[0].Width-1.5), d.fit(p.Ship, cols[1].Width-1.5), d.fit(corp, cols[2].Width-1.5),
			seen, d.fit(p.LastSystem, cols[4].Width-1.5), left,
		})
	}
	d.table(cols, rows)
	d.Ln(4)
}

// capitalStripItem is the label of a hull in the strip and its width, icon included
func (d *pageDoc) capitalStripItem(h CapitalHull) (string, float64) {
	d.sans("", 9)
	label := fmt.Sprintf("%s ×%d", h.Name, h.Pilots)
	return label, capitalIconSize + 1.5 + d.GetStringWidth(label) + 6
}

// capitalStripLines counts the lines the hulls wrap onto
func (d *pageDoc) capitalStripLines(hulls []CapitalHull) int {
	if len(hulls) == 0 {
		return 0
	}
	lines, x := 1, 0.0
	for _, h := range hulls {
		_, w := d.capitalStripItem(h)
		if x > 0 && x+w > bodyWidth {
			lines++
			x = 0
		}
		x += w
	}
	return lines
}

// capitalStrip draws the hulls' icons with their names and pilot counts, wrapping onto more lines
func (d *pageDoc) capitalStrip(hulls []CapitalHull) {
	x, y := 0.0, d.GetY()
	for _, h := range hulls {
		label, w := d.capitalStripItem(h)
		if x > 0 && x+w > bodyWidth {
			x = 0
			y += capitalStripH
		}
		px := pageMargin + x
		if !d.image("capicon-"+h.Name, h.Icon, px, y, capitalIconSize, capitalIconSize) {
			d.fill(colBoxBg)
			d.draw(colRule)
			d.SetLineWidth(0.2)
			d.Rect(px, y, capitalIconSize, capitalIconSize, "FD")
		}
		d.sans("", 9)
		d.text(colText)
		d.SetXY(px+capitalIconSize+1.5, y)
		d.CellFormat(w-capitalIconSize-1.5, capitalIconSize, label, "", 0, "L", false, 0, "")
		x += w
	}
	if len(hulls) > 0 {
		d.SetXY(pageMargin, y+capitalStripH)
	}
}
