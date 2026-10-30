package report

import (
	"bytes"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// summary draws page 1: the header, scope, and kills and losses
func (d *pageDoc) summary(in Input) {
	t := in.Target
	textX := pageMargin
	top := d.GetY()
	logoSize := 0.0
	if len(t.Logo) > 0 {
		logoSize = 26
		if d.logo(t.Logo, pageMargin, top, logoSize) {
			textX = pageMargin + logoSize + 5
		} else {
			logoSize = 0
		}
	}

	d.SetXY(textX, top)
	d.sans("B", 22)
	d.text(colText)
	nameW := d.GetStringWidth(t.Name) + 2
	d.CellFormat(nameW, 10, t.Name, "", 0, "L", false, 0, "")
	if t.Ticker != "" {
		d.sans("", 14)
		d.text(colMuted)
		d.CellFormat(0, 10, "["+t.Ticker+"]", "", 0, "L", false, 0, "")
	}
	d.SetXY(textX, top+10)
	d.sans("", 10.5)
	d.text(colMuted)
	d.CellFormat(0, 6, typeWords(t.Type), "", 1, "L", false, 0, "")
	if end := top + logoSize + 3; d.GetY() < end && logoSize > 0 {
		d.SetY(end)
	} else {
		d.SetY(d.GetY() + 2)
	}

	d.SetLeftMargin(pageMargin)
	d.keyValues(t.Details, 38)

	d.Ln(4)

	d.scopeBox(in.Scope)
	d.Ln(5)
	d.killTiles(in.Kills, in.Losses)

	// Phase 2: the Key findings box goes here, below the kills and losses.
}

// typeWords turns the contract's type code into words
func typeWords(t string) string {
	switch strings.ToUpper(t) {
	case "CORPORATION":
		return "Corporation"
	case "ALLIANCE":
		return "Alliance"
	case "PILOT":
		return "Pilot"
	case "SYSTEM":
		return "System"
	}
	return t
}

// logo draws an image, returning false (and drawing nothing) if it can't be read
func (d *pageDoc) logo(data []byte, x, y, size float64) (ok bool) {
	kind := ""
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		kind = "PNG"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		kind = "JPG"
	default:
		return false
	}
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	info := d.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: kind}, bytes.NewReader(data))
	if d.Error() != nil || info == nil {
		d.ClearError()
		return false
	}
	d.ImageOptions("logo", x, y, size, size, false, fpdf.ImageOptions{ImageType: kind}, 0, "")
	if d.Error() != nil {
		d.ClearError()
		return false
	}
	return true
}

// scopeBox says what the search covered and any caveat on it
func (d *pageDoc) scopeBox(s Scope) {
	lines := []struct {
		text string
		col  rgb
	}{{scopeLine(s), colText}}
	for _, source := range s.Sources {
		lines = append(lines, struct {
			text string
			col  rgb
		}{sourceLine(source), colText})
	}
	for _, l := range s.Limits {
		lines = append(lines, struct {
			text string
			col  rgb
		}{l, colText})
	}
	if s.Stopped {
		lines = append(lines, struct {
			text string
			col  rgb
		}{"The search was stopped early, so this covers only what was collected by then.", colWarn})
	}
	if s.RateLimited {
		lines = append(lines, struct {
			text string
			col  rgb
		}{"The ESI rate limited the search, so killmails that needed it are missing.", colWarn})
	}
	const lh = 5.2
	h := float64(len(lines))*lh + 5.5 + 9
	d.fill(colBoxBg)
	d.draw(colRule)
	d.SetLineWidth(0.2)
	y := d.GetY()
	d.RoundedRect(pageMargin, y, bodyWidth, h, 1.5, "1234", "FD")
	d.SetXY(pageMargin+3, y+2)
	d.sans("B", 9)
	d.text(colAccent)
	d.CellFormat(0, 5, "SCOPE", "", 1, "L", false, 0, "")
	for i, l := range lines {
		d.SetX(pageMargin + 3)
		d.sans("", 9.5)
		if i == 0 {
			d.sans("B", 9.5)
		}
		d.text(l.col)
		d.CellFormat(0, lh, l.text, "", 1, "L", false, 0, "")
	}
	d.SetX(pageMargin + 3)
	d.sans("", 8.5)
	d.text(colMuted)
	d.SetY(d.GetY() + 1)
	d.SetX(pageMargin + 3)
	d.CellFormat(0, 5, "Killmails show only deaths: not who survived, objectives, or fights that left no killmail.", "", 1, "L", false, 0, "")
	d.SetY(y + h)
}

// scopeLine is "<N> killmails from <oldest> to <newest>"
func scopeLine(s Scope) string {
	if s.Killmails == 0 {
		return "No killmails"
	}
	return killmailSpan(s.Killmails, s.Oldest, s.Newest)
}

// sourceLine is one list's share, ie "   Ursa Major Holdings (main): 1,200 killmails from Aug 1st, 2026 to Oct 22nd, 2026"
func sourceLine(source Source) string {
	name := source.Name
	if source.Main {
		name += " (main)"
	}
	if source.Killmails == 0 {
		return "   " + name + ": no killmails"
	}
	return "   " + name + ": " + killmailSpan(source.Killmails, source.Oldest, source.Newest)
}

// killmailSpan is "<N> killmails from <oldest> to <newest>", or "on <date>" when both are the same day
func killmailSpan(n int, oldest, newest time.Time) string {
	word := " killmails"
	if n == 1 {
		word = " killmail"
	}
	text := count(n) + word
	if oldest.IsZero() || newest.IsZero() {
		return text
	}
	if spokenDate(oldest) == spokenDate(newest) {
		return text + " on " + spokenDate(newest)
	}
	return text + " from " + spokenDate(oldest) + " to " + spokenDate(newest)
}

// killTiles draws the kills and losses tiles side by side and the ISK efficiency under them
func (d *pageDoc) killTiles(kills, losses Tally) {
	d.heading("Kills and losses")
	d.Ln(1)
	gap := 6.0
	w := (bodyWidth - gap) / 2
	y := d.GetY()
	tile := func(x float64, label string, t Tally, fg, bg rgb) {
		d.fill(bg)
		d.draw(fg)
		d.SetLineWidth(0.3)
		d.RoundedRect(x, y, w, 30, 1.5, "1234", "FD")
		d.SetXY(x, y+3)
		d.sans("", 9)
		d.text(fg)
		d.CellFormat(w, 5, strings.ToUpper(label), "", 1, "C", false, 0, "")
		d.SetX(x)
		d.sans("B", 24)
		d.CellFormat(w, 12, count(t.Count), "", 1, "C", false, 0, "")
		d.SetX(x)
		d.sans("", 11)
		d.text(colText)
		d.CellFormat(w, 6, shortISK(t.ISK)+" ISK", "", 1, "C", false, 0, "")
	}
	tile(pageMargin, "Kills", kills, colKill, colKillBg)
	tile(pageMargin+w+gap, "Losses", losses, colLoss, colLossBg)
	d.SetXY(pageMargin, y+34)
	if total := kills.ISK + losses.ISK; total > 0 {
		d.sans("", 10)
		d.text(colText)
		d.CellFormat(bodyWidth, 6, "ISK efficiency: "+trimNum(kills.ISK/total*100, 0)+"%", "", 1, "L", false, 0, "")
	}
}
