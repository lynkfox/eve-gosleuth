package report

import (
	"fmt"
	"slices"
	"strings"
)

const (
	doctrineImage  = 32.0 // the render's side
	doctrineGap    = 4.0
	doctrineLineH  = 4.6
	doctrineRackW  = 16.0
	doctrineTileH  = 6.0
	doctrineTileSp = 2.0
)

// doctrineTile is one cell of the fight stats strip
type doctrineTile struct {
	text string
	col  rgb
	bold bool
}

// doctrineLink is a doctrine's two internal links: to its block, and to its EFT in the appendix. Both 0 when the appendix isn't included.
type doctrineLink struct {
	block, eft int
}

// doctrines draws the Fleet Doctrines section: a block per doctrine fit, with a divider where the hull class changes, each linking to its EFT in the appendix when links has it
func (d *pageDoc) doctrines(doctrines []Doctrine, links []doctrineLink) {
	d.heading("Potential Fleet Doctrines")
	d.sans("", 9)
	d.text(colMuted)
	d.MultiCell(bodyWidth, 4.6, "Fits that may be doctrine - usually found multiple of in fights with logi.  This is a guestimate and should be taken as such.", "", "L", false)
	d.Ln(2)
	if len(doctrines) == 0 {
		d.sans("", 10)
		d.text(colMuted)
		d.CellFormat(bodyWidth, 6, "No fits were tagged Fleet Doctrine.", "", 1, "L", false, 0, "")
		d.Ln(2)
	}
	class := ""
	for i, dc := range doctrines {
		divider := ""
		if i == 0 || dc.HullClass != class {
			divider, class = dc.HullClass, dc.HullClass
		}
		link := doctrineLink{}
		if i < len(links) {
			link = links[i]
		}
		d.doctrineBlock(i, dc, divider, link)
	}
	d.text(colText)
}

// doctrineDividerH is the height of the hull class divider over a class's first block
const doctrineDividerH = 9.0

// doctrineBlock draws one doctrine, kept on one page with the hull class divider over it when it has one
func (d *pageDoc) doctrineBlock(index int, dc Doctrine, divider string, link doctrineLink) {
	textX := pageMargin + doctrineImage + doctrineGap
	textW := bodyWidth - doctrineImage - doctrineGap
	reason := d.doctrineReason(dc.Reason, textW)
	headH := doctrineImage
	textH := 9 + 5*3 + float64(len(reason))*doctrineLineH
	if link.eft != 0 {
		textH += 5
	}
	if h := textH; h > headH {
		headH = h
	}
	tiles := doctrineRangeTiles(dc.Fight, dc.FightMax)
	racks := dc.Racks
	if len(dc.Implants) > 0 {
		tiles = append([]doctrineTile{{text: fmt.Sprintf("Implants on %s of %s %s", count(dc.Implanted), count(dc.Losses), doctrinePlural(dc.Losses, "loss", "losses")), col: colWarn, bold: true}}, tiles...)
		implants := make([]ModuleCount, 0, len(dc.Implants))
		for _, name := range dc.Implants {
			implants = append(implants, ModuleCount{Name: name, Count: 1})
		}
		racks = append(slices.Clone(racks), Rack{Name: "Implants", Modules: implants})
	}
	height := headH + 2 + d.doctrineStrip(tiles, false) + d.doctrineRacks(racks, false) + 5
	if divider != "" {
		height += doctrineDividerH
	}
	d.keepTogether(height)
	if divider != "" {
		d.doctrineDivider(divider)
	}

	x, y := pageMargin, d.GetY()
	d.Bookmark(dc.ShipName, 1, y)
	if link.block != 0 {
		d.SetLink(link.block, y, d.PageNo())
	}
	if len(dc.Render) == 0 || !d.image(fmt.Sprintf("render-%d", index), dc.Render, x, y, doctrineImage, doctrineImage) {
		d.fill(colBoxBg)
		d.draw(colRule)
		d.SetLineWidth(0.2)
		d.Rect(x, y, doctrineImage, doctrineImage, "FD")
		d.sans("B", 8)
		d.text(colMuted)
		d.SetXY(x+1, y+doctrineImage/2-2)
		d.CellFormat(doctrineImage-2, 4, d.fit(dc.ShipName, doctrineImage-2), "", 0, "C", false, 0, "")
	}

	d.SetXY(textX, y)
	d.sans("B", 14)
	d.text(colText)
	d.CellFormat(textW, 8, d.fit(dc.ShipName, textW), "", 1, "L", false, 0, "")
	if dc.Role != "" {
		d.SetX(textX)
		d.sans("B", 8.5)
		d.text(colAccent)
		d.CellFormat(textW, 5, d.fit(dc.Role, textW), "", 1, "L", false, 0, "")
	}
	d.SetX(textX)
	d.sans("", 9.5)
	d.text(colText)
	d.CellFormat(textW, 5, fmt.Sprintf("%s %s · %s %s", count(dc.Losses), doctrinePlural(dc.Losses, "loss", "losses"), count(dc.Pilots), doctrinePlural(dc.Pilots, "pilot", "pilots")), "", 1, "L", false, 0, "")
	if !dc.LastSeen.IsZero() {
		line := "Last seen " + spokenDate(dc.LastSeen)
		if dc.LastSystem != "" {
			line += " in " + dc.LastSystem
		}
		d.SetX(textX)
		d.text(colMuted)
		d.CellFormat(textW, 5, d.fit(line, textW), "", 1, "L", false, 0, "")
	}
	d.sans("", 9)
	d.text(colMuted)
	for _, l := range reason {
		d.SetX(textX)
		d.CellFormat(textW, doctrineLineH, l, "", 1, "L", false, 0, "")
	}
	if link.eft != 0 {
		d.SetX(textX)
		d.sans("B", 9)
		d.text(colAccent)
		const label = "EFT fit in the appendix ›"
		d.CellFormat(d.GetStringWidth(label)+1, 5, label, "", 1, "L", false, link.eft, "")
	}

	d.SetXY(pageMargin, y+headH+2)
	d.doctrineStrip(tiles, true)
	d.doctrineRacks(racks, true)
	d.text(colText)
	d.SetX(pageMargin)
	d.Ln(1)
	d.draw(colRule)
	d.SetLineWidth(0.2)
	d.Line(pageMargin, d.GetY(), pageWidth-pageMargin, d.GetY())
	d.Ln(4)
}

// doctrineDivider prints a hull class's name in small accent capitals with a thin rule after it, ie BATTLESHIPS ———
func (d *pageDoc) doctrineDivider(class string) {
	y := d.GetY() + 2
	d.SetXY(pageMargin, y)
	d.sans("B", 9)
	d.text(colAccent)
	label := strings.ToUpper(class)
	w := d.GetStringWidth(label) + 3
	d.CellFormat(w, 5, label, "", 0, "L", false, 0, "")
	d.draw(colRule)
	d.SetLineWidth(0.3)
	d.Line(pageMargin+w, y+2.5, pageWidth-pageMargin, y+2.5)
	d.SetXY(pageMargin, y+doctrineDividerH-2)
}

// doctrineReason splits the reason into at most three lines of the width
func (d *pageDoc) doctrineReason(reason string, width float64) []string {
	if reason == "" {
		return nil
	}
	d.sans("", 9)
	lines := d.SplitText(reason, width)
	if len(lines) > 3 {
		lines = append(lines[:2], d.fit(strings.Join(lines[2:], " "), width))
	}
	return lines
}

// doctrineRange formats a number as shown, or "low – high" when the two differ once formatted, lower first
func doctrineRange(low, high float64, format func(float64) string) string {
	l, h := format(low), format(high)
	if high < low {
		l, h = h, l
	}
	if l == h {
		return l
	}
	return l + " – " + h
}

// doctrineTiles lists the fight stats of a fit lost once
func doctrineTiles(f *FightStats) []doctrineTile {
	return doctrineRangeTiles(f, nil)
}

/*
doctrineRangeTiles lists the fight stats worth showing, the call a fleet commander makes in red.
Where max is given and a number differs from f's once rounded as shown, the tile shows "low – high".
*/
func doctrineRangeTiles(f, max *FightStats) []doctrineTile {
	if f == nil {
		return nil
	}
	if max == nil {
		max = f
	}
	num := func(dec int) func(float64) string {
		return func(v float64) string { return trimNum(v, dec) }
	}
	var t []doctrineTile
	if f.Mode != "" {
		t = append(t, doctrineTile{text: f.Mode + " mode", col: colMuted})
	}
	if f.EHP > 0 || max.EHP > 0 {
		t = append(t, doctrineTile{text: doctrineRange(f.EHP, max.EHP, doctrineEHP) + " EHP", col: colText, bold: true})
	}
	if f.Tank != "" {
		t = append(t, doctrineTile{text: f.Tank + " tank", col: colText})
	}
	if f.Weakest != "" {
		resist := doctrineRange(f.WeakestResist*100, max.WeakestResist*100, num(0))
		t = append(t, doctrineTile{text: fmt.Sprintf("Shoot: %s (%s%% resist)", f.Weakest, resist), col: colLoss, bold: true})
	}
	if f.SensorType != "" {
		t = append(t, doctrineTile{text: fmt.Sprintf("Sensor %s %s", f.SensorType, doctrineRange(f.SensorStrength, max.SensorStrength, num(1))), col: colText})
	}
	if f.Neut > 0 || max.Neut > 0 {
		t = append(t, doctrineTile{text: "Neut " + doctrineRange(f.Neut, max.Neut, num(1)) + " GJ/s", col: colWarn})
	}
	if f.RemoteRep > 0 || max.RemoteRep > 0 {
		t = append(t, doctrineTile{text: "Remote rep " + doctrineRange(f.RemoteRep, max.RemoteRep, num(0)) + " HP/s", col: colText})
	}
	if f.LocalRep > 0 || max.LocalRep > 0 {
		t = append(t, doctrineTile{text: "Local rep " + doctrineRange(f.LocalRep, max.LocalRep, num(0)) + " HP/s", col: colText})
	}
	return t
}

// doctrineModule is a module's text in a rack, ie "3× Heavy Gunnar Compact Stasis Grappler (2 fits had a different type)"
func doctrineModule(m ModuleCount) string {
	text := m.Name
	if m.Count > 1 {
		text = fmt.Sprintf("%d× %s", m.Count, m.Name)
	}
	switch {
	case m.Different == 1:
		text += " (1 fit had a different type)"
	case m.Different > 1:
		text += fmt.Sprintf(" (%d fits had a different type)", m.Different)
	}
	return text
}

// doctrineEHP formats effective hit points short, ie 84.2k
func doctrineEHP(v float64) string {
	switch {
	case v >= 1e6:
		return trimNum(v/1e6, 2) + "m"
	case v >= 1e3:
		return trimNum(v/1e3, 1) + "k"
	}
	return trimNum(v, 0)
}

// doctrineStrip lays the tiles in wrapping rows from the current Y, drawing them when asked, and returns the height used
func (d *pageDoc) doctrineStrip(tiles []doctrineTile, draw bool) float64 {
	if len(tiles) == 0 {
		return 0
	}
	x, y := pageMargin, d.GetY()
	rows := 1
	for _, t := range tiles {
		style := ""
		if t.bold {
			style = "B"
		}
		d.sans(style, 8.5)
		w := d.GetStringWidth(t.text) + 4
		if w > bodyWidth {
			w = bodyWidth
		}
		if x+w > pageMargin+bodyWidth+0.01 {
			x = pageMargin
			rows++
		}
		if draw {
			ty := y + float64(rows-1)*(doctrineTileH+doctrineTileSp)
			d.fill(colBoxBg)
			d.Rect(x, ty, w, doctrineTileH, "F")
			d.text(t.col)
			d.SetXY(x, ty)
			// sized to its text, so never cut short: only a tile wider than the page is
			text := t.text
			if w == bodyWidth {
				text = d.fit(text, w-4)
			}
			d.CellFormat(w, doctrineTileH, text, "", 0, "C", false, 0, "")
		}
		x += w + doctrineTileSp
	}
	h := float64(rows)*(doctrineTileH+doctrineTileSp) + 1
	if draw {
		d.SetXY(pageMargin, y+h)
	}
	return h
}

// doctrineRacks prints each rack as a muted name and its modules with counts, and returns the height used
func (d *pageDoc) doctrineRacks(racks []Rack, draw bool) float64 {
	total := 0.0
	for _, r := range racks {
		if len(r.Modules) == 0 {
			continue
		}
		parts := make([]string, 0, len(r.Modules))
		for _, m := range r.Modules {
			parts = append(parts, doctrineModule(m))
		}
		d.sans("", 9)
		w := bodyWidth - doctrineRackW
		lines := len(d.SplitText(strings.Join(parts, ", "), w))
		if lines < 1 {
			lines = 1
		}
		h := float64(lines) * doctrineLineH
		total += h + 0.8
		if !draw {
			continue
		}
		y := d.GetY()
		d.SetXY(pageMargin, y)
		d.text(colMuted)
		d.CellFormat(doctrineRackW, doctrineLineH, r.Name+":", "", 0, "L", false, 0, "")
		d.text(colText)
		d.SetXY(pageMargin+doctrineRackW, y)
		d.MultiCell(w, doctrineLineH, strings.Join(parts, ", "), "", "L", false)
		d.SetXY(pageMargin, y+h+0.8)
	}
	return total
}

func doctrinePlural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
