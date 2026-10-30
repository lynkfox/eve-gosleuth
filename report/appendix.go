package report

import "strings"

const appendixLineH = 3.8 // a line of EFT

// appendix draws each Fleet Doctrine's fit as EFT text in monospace, so readers can paste it into Pyfa, each reached from its block's link and linking back to it
func (d *pageDoc) appendix(doctrines []Doctrine, links []doctrineLink) {
	d.heading("Appendix: EFT fits")
	d.sans("", 9)
	d.text(colMuted)
	d.MultiCell(bodyWidth, 4.6, "Each Fleet Doctrine's fit as EFT text, to paste into Pyfa or the game's fitting window. The charges and drones are its first loss's.", "", "L", false)
	d.Ln(3)
	for i, dc := range doctrines {
		if strings.TrimSpace(dc.EFT) == "" {
			continue
		}
		link := doctrineLink{}
		if i < len(links) {
			link = links[i]
		}
		lines := strings.Split(strings.TrimRight(strings.ReplaceAll(dc.EFT, "\r\n", "\n"), "\n"), "\n")
		// a fit that fits on a page stays on one; a longer one flows on
		_, pageH := d.GetPageSize()
		d.keepTogether(min(9+float64(len(lines))*appendixLineH+5, pageH-pageBottom-pageMargin))

		y := d.GetY()
		if link.eft != 0 {
			d.SetLink(link.eft, y, d.PageNo())
		}
		d.Bookmark(dc.ShipName, 1, y)
		d.SetX(pageMargin)
		d.sans("B", 11)
		d.text(colText)
		title := dc.ShipName
		if dc.Role != "" {
			title += " · " + dc.Role
		}
		back := ""
		if link.block != 0 {
			back = "‹ Back to the doctrine"
		}
		d.sans("B", 9)
		backW := d.GetStringWidth(back) + 1
		d.sans("B", 11)
		d.CellFormat(bodyWidth-backW, 7, d.fit(title, bodyWidth-backW-2), "", 0, "L", false, 0, "")
		if back != "" {
			d.sans("B", 9)
			d.text(colAccent)
			d.CellFormat(backW, 7, back, "", 0, "R", false, link.block, "")
		}
		d.Ln(8)

		d.SetFont(fontMono, "", 8)
		d.text(colText)
		for _, line := range lines {
			d.SetX(pageMargin + 2)
			d.CellFormat(bodyWidth-2, appendixLineH, d.fit(line, bodyWidth-2), "", 1, "L", false, 0, "")
		}
		d.Ln(2)
		d.draw(colRule)
		d.SetLineWidth(0.2)
		d.Line(pageMargin, d.GetY(), pageWidth-pageMargin, d.GetY())
		d.Ln(4)
	}
	d.text(colText)
}
