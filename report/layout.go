package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// rgb is a print colour
type rgb struct{ r, g, b int }

// Print colours: a white page, dark text, one accent. Kill and loss colours stay fixed, as in the app.
var (
	colText   = rgb{26, 26, 26}
	colMuted  = rgb{110, 110, 110}
	colAccent = rgb{59, 110, 165}
	colKill   = rgb{46, 139, 87}
	colLoss   = rgb{196, 48, 48}
	colWarn   = rgb{176, 112, 0}
	colRule   = rgb{200, 200, 200}
	colZebra  = rgb{244, 246, 249}
	colKillBg = rgb{232, 244, 236}
	colLossBg = rgb{250, 235, 235}
	colWarnBg = rgb{253, 245, 226}
	colBoxBg  = rgb{247, 248, 250}
)

const (
	pageMargin = 15.0
	pageBottom = 18.0 // space kept below the content for the footer
	pageWidth  = 210.0
	bodyWidth  = pageWidth - 2*pageMargin
)

// pageDoc wraps the fpdf document with the report's helpers
type pageDoc struct {
	*fpdf.Fpdf
}

// newDoc makes an A4 portrait document with fonts, margins, and the footer set up
func newDoc(in Input) *pageDoc {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(true, pageBottom)
	registerFonts(pdf)
	pdf.AliasNbPages("")
	d := &pageDoc{pdf}
	d.SetTitle("GoSleuth intel: "+in.Target.Name, true)
	d.SetCreator("GoSleuth v"+in.AppVersion, true)
	d.SetFooterFunc(func() { d.footer(in) })
	return d
}

func (d *pageDoc) text(c rgb) { d.SetTextColor(c.r, c.g, c.b) }
func (d *pageDoc) fill(c rgb) { d.SetFillColor(c.r, c.g, c.b) }
func (d *pageDoc) draw(c rgb) { d.SetDrawColor(c.r, c.g, c.b) }
func (d *pageDoc) sans(style string, size float64) {
	d.SetFont(fontSans, style, size)
}

// footer draws the target, app version, and date and page number on every page
func (d *pageDoc) footer(in Input) {
	d.SetY(-12)
	d.sans("", 8)
	d.text(colMuted)
	d.draw(colRule)
	d.SetLineWidth(0.2)
	d.Line(pageMargin, d.GetY()-1, pageWidth-pageMargin, d.GetY()-1)
	left := in.Target.Name
	if in.Target.Ticker != "" {
		left += " [" + in.Target.Ticker + "]"
	}
	d.CellFormat(bodyWidth*0.33, 5, d.fit(left, bodyWidth*0.33), "", 0, "L", false, 0, "")
	d.CellFormat(bodyWidth*0.27, 5, "GoSleuth v"+in.AppVersion, "", 0, "C", false, 0, "")
	d.CellFormat(bodyWidth*0.40, 5, fmt.Sprintf("Generated %s · Page %d of {nb}", spokenDate(in.GeneratedAt), d.PageNo()), "", 0, "R", false, 0, "")
}

// fit cuts text to the width in the current font, ending it with an ellipsis, so it never runs past its cell
func (d *pageDoc) fit(text string, width float64) string {
	// a hair of slack, as a width measured from this same text can come back a rounding error short
	if d.GetStringWidth(text) <= width+0.01 {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && d.GetStringWidth(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return strings.TrimRight(string(runes), " ") + "…"
}

// room is the height left on the page above the footer
func (d *pageDoc) room() float64 {
	_, pageH := d.GetPageSize()
	return pageH - pageBottom - d.GetY()
}

// keepTogether starts a new page unless a block of the height fits on this one
func (d *pageDoc) keepTogether(height float64) {
	if d.room() < height {
		d.AddPage()
	}
}

// heading prints a section title with a thin accent rule under it
func (d *pageDoc) heading(title string) {
	d.sans("B", 13)
	d.text(colAccent)
	d.CellFormat(bodyWidth, 7, title, "", 1, "L", false, 0, "")
	d.draw(colAccent)
	d.SetLineWidth(0.4)
	y := d.GetY()
	d.Line(pageMargin, y, pageWidth-pageMargin, y)
	d.SetY(y + 2.5)
	d.text(colText)
}

// keyValues prints label/value rows, labels muted in a fixed column
func (d *pageDoc) keyValues(rows []Detail, labelW float64) {
	for _, r := range rows {
		d.SetX(pageMargin)
		d.sans("", 9.5)
		d.text(colMuted)
		d.CellFormat(labelW, 5.5, d.fit(r.Label, labelW-1), "", 0, "L", false, 0, "")
		d.text(colText)
		d.MultiCell(bodyWidth-labelW, 5.5, r.Value, "", "L", false)
	}
}

// column is one table column
type column struct {
	Title string
	Width float64
	Right bool // right-aligned, for numbers
}

// table prints a table with zebra rows, repeating the header after a page break
func (d *pageDoc) table(cols []column, rows [][]string) {
	const rowH = 5.8
	header := func() {
		d.sans("B", 9)
		d.fill(colAccent)
		d.text(rgb{255, 255, 255})
		d.SetX(pageMargin)
		for _, c := range cols {
			d.CellFormat(c.Width, rowH, c.Title, "", 0, align(c.Right), true, 0, "")
		}
		d.Ln(rowH)
	}
	_, pageH := d.GetPageSize()
	header()
	d.sans("", 9)
	for i, r := range rows {
		if d.GetY()+rowH > pageH-pageBottom {
			d.AddPage()
			header()
		}
		d.sans("", 9)
		d.text(colText)
		zebra := i%2 == 1
		if zebra {
			d.fill(colZebra)
		}
		d.SetX(pageMargin)
		for j, c := range cols {
			v := ""
			if j < len(r) {
				v = r[j]
			}
			d.CellFormat(c.Width, rowH, v, "", 0, align(c.Right), zebra, 0, "")
		}
		d.Ln(rowH)
	}
}

func align(right bool) string {
	if right {
		return "R"
	}
	return "L"
}

// spokenDate formats a time as the app does, ie Oct 29th, 2026, in EVE time
func spokenDate(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%s %d%s, %d", t.Format("Jan"), t.Day(), ordinal(t.Day()), t.Year())
}

// ordinal is the suffix of a day of the month: st, nd, rd, th
func ordinal(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}

// shortISK formats ISK as 1.23b, 45.6m, 789k, or plain under 1000
func shortISK(v float64) string {
	neg := ""
	if v < 0 {
		neg, v = "-", -v
	}
	switch {
	case v >= 1e9:
		return neg + trimNum(v/1e9, 2) + "b"
	case v >= 1e6:
		return neg + trimNum(v/1e6, 1) + "m"
	case v >= 1e3:
		return neg + strconv.FormatFloat(v/1e3, 'f', 0, 64) + "k"
	}
	return neg + strconv.FormatFloat(v, 'f', 0, 64)
}

// trimNum formats with up to dec decimals; 1.23b and 45.6m keep theirs, a whole number loses the point
func trimNum(v float64, dec int) string {
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// count formats an integer with thousands separators
func count(n int) string {
	s := strconv.Itoa(n)
	neg := ""
	if n < 0 {
		neg, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return neg + s
}
