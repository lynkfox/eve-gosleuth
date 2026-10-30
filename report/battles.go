package report

import (
	"fmt"
	"strconv"
	"time"
)

// battleLinks are a battle's report sites in the order the Links column shows them
func battleLinks(b Battle) [][2]string {
	return [][2]string{{"zKill", b.ZkillURL}, {"EVE Tools", b.EveToolsURL}, {"Warbeacon", b.WarbeaconURL}}
}

// battles draws the Battles page: a note on how battles are estimated, then the Latest Battles and the Biggest Battles, each a table with links to its battle report sites
func (d *pageDoc) battles(latest, biggest []Battle) {
	d.heading("Latest Battles")
	if len(latest) == 0 && len(biggest) == 0 {
		d.sans("", 9.5)
		d.text(colMuted)
		d.CellFormat(bodyWidth, 6, "No battles of 10 or more pilots were found.", "", 1, "L", false, 0, "")
		d.text(colText)
		return
	}
	d.sans("", 8.5)
	d.text(colMuted)
	d.MultiCell(bodyWidth, 4.5, "Battles of 10 or more pilots on the target's killmails, all times EVE. They are estimates: killmails within 15 minutes of each other "+
		"that share a system or pilots are taken as one battle, and the counts are from the search's killmails only. The links open the whole fight on each site, "+
		"for the main system at the hour of the battle's middle.", "", "L", false)
	d.Ln(2)
	d.battleTable(latest)

	d.Ln(6)
	// the heading, the table header, and a few rows stay together
	d.keepTogether(10 + 5.8*4)
	d.heading("Biggest Battles")
	d.sans("", 8.5)
	d.text(colMuted)
	d.CellFormat(bodyWidth, 4.5, "Most pilots first.", "", 1, "L", false, 0, "")
	d.Ln(2)
	d.battleTable(biggest)
}

// battleTable draws a table of battles, one row each, its last column the battle's site links
func (d *pageDoc) battleTable(battles []Battle) {
	const rowH = 5.8
	cols := []column{
		{Title: "Date", Width: 25}, {Title: "Time", Width: 22}, {Title: "Length", Width: 15}, {Title: "System", Width: 36},
		{Title: "Pilots", Width: 13, Right: true}, {Title: "Ships", Width: 13, Right: true}, {Title: "Kills", Width: 12, Right: true},
		{Title: "Battle reports", Width: 44},
	}
	d.sans("B", 9)
	d.fill(colAccent)
	d.text(rgb{255, 255, 255})
	d.SetX(pageMargin)
	for _, c := range cols {
		d.CellFormat(c.Width, rowH, c.Title, "", 0, align(c.Right), true, 0, "")
	}
	d.Ln(rowH)

	for i, b := range battles {
		d.sans("", 9)
		d.text(colText)
		zebra := i%2 == 1
		if zebra {
			d.fill(colZebra)
		}
		system := b.System
		if b.OtherSystems > 0 {
			system += fmt.Sprintf(" +%d", b.OtherSystems)
		}
		values := []string{
			spokenDate(b.Start),
			b.Start.UTC().Format("15:04") + "–" + b.End.UTC().Format("15:04"),
			battleLength(b.End.Sub(b.Start)),
			d.fit(system, cols[3].Width-1.5),
			count(b.Pilots), count(b.Ships), count(b.Killmails),
		}
		d.SetX(pageMargin)
		for j, v := range values {
			d.CellFormat(cols[j].Width, rowH, v, "", 0, align(cols[j].Right), zebra, 0, "")
		}
		d.battleLinkCell(b, cols[len(cols)-1].Width, rowH, zebra)
		d.Ln(rowH)
	}
	d.text(colText)
}

// battleLinkCell draws a battle's site links side by side in one cell, each clickable, or a dash when its system is not known
func (d *pageDoc) battleLinkCell(b Battle, width, height float64, zebra bool) {
	x, y := d.GetX(), d.GetY()
	d.CellFormat(width, height, "", "", 0, "L", zebra, 0, "")
	d.SetXY(x+1, y)
	d.sans("", 8.5)
	d.text(colAccent)
	drawn := false
	for _, link := range battleLinks(b) {
		if link[1] == "" {
			continue
		}
		w := d.GetStringWidth(link[0]) + 2
		d.CellFormat(w, height, link[0], "", 0, "L", false, 0, link[1])
		drawn = true
	}
	if !drawn {
		d.text(colMuted)
		d.CellFormat(width-1, height, "–", "", 0, "L", false, 0, "")
	}
	d.text(colText)
	d.SetXY(x+width, y)
}

// battleLength words how long a battle ran, ie "<1 min", "46 min", or "2h 05m"
func battleLength(length time.Duration) string {
	minutes := int(length.Minutes())
	switch {
	case minutes < 1:
		return "<1 min"
	case minutes < 60:
		return strconv.Itoa(minutes) + " min"
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}
