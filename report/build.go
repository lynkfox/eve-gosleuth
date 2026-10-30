package report

import (
	"bytes"
	"fmt"
)

// build makes the PDF: the Summary first, then the chosen sections that exist
func build(in Input, sections []Section) ([]byte, error) {
	d := newDoc(in)
	d.AddPage()
	d.summary(in)
	// Sections beyond the Summary are added in phase 2; unknown ones are ignored and the Summary is never repeated.
	for _, s := range sections {
		switch s {
		case SectionSummary:
		}
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
