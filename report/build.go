package report

import (
	"bytes"
	"fmt"
)

// build makes the PDF: the Summary first, then the chosen sections that exist, in the report's order whatever order they were asked in
func build(in Input, sections []Section) ([]byte, error) {
	d := newDoc(in)
	chosen := map[Section]bool{}
	for _, s := range sections {
		chosen[s] = true
	}
	// each section starts a page, with an entry in the PDF's outline (the reader's bookmarks pane)
	page := func(title string) {
		d.AddPage()
		d.sans("", 10)
		d.Bookmark(title, 0, -1)
	}
	page("Summary")
	d.summary(in)
	if chosen[SectionActivity] {
		page("Activity")
		d.activity(in.Activity, in.Scope)
	}
	// with the appendix, each doctrine links to its EFT and back
	links := []doctrineLink{}
	appendix := chosen[SectionAppendix] && len(in.Doctrines) > 0
	if appendix {
		for range in.Doctrines {
			links = append(links, doctrineLink{block: d.AddLink(), eft: d.AddLink()})
		}
	}
	if chosen[SectionDoctrines] {
		page("Potential Fleet Doctrines")
		d.doctrines(in.Doctrines, links)
	}
	if chosen[SectionCapitals] {
		page("Capital Pilots")
		d.capitals(in.Capitals)
	}
	if appendix {
		if !chosen[SectionDoctrines] {
			// nothing to link back to
			for i := range links {
				links[i].block = 0
			}
		}
		page("Appendix: EFT fits")
		d.appendix(in.Doctrines, links)
	}
	if err := d.Error(); err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}
	return buf.Bytes(), nil
}
