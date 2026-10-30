package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
	ExportPDF saves the last search's report as a PDF to a file the user picks. It does not wait for a running search.

Parameters:
  - sections: the sections to include beyond the Summary, which is always first, ie summary

Returns:
  - string: the path written, empty if the user closed the picker
  - error: a user readable message if there is no report to export or the file could not be made or written
*/
func (a *App) ExportPDF(sections []string) (path string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.lastTarget == nil || a.lastReport == nil {
		return "", errors.New("Search for a corporation or alliance first, then export its report")
	}
	input := pdfInput(*a.lastTarget, *a.lastReport, a.version, time.Now())

	name := input.Target.Ticker
	if name == "" {
		name = input.Target.Name
	}
	path, err = runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export PDF report",
		DefaultFilename: pdfFileName(name, input.GeneratedAt),
		Filters:         []runtime.FileFilter{{DisplayName: "PDF report (*.pdf)", Pattern: "*.pdf"}},
	})
	if err != nil {
		return "", fmt.Errorf("Could not open the file picker: %w", err)
	}
	if path == "" {
		return "", nil
	}

	chosen := make([]report.Section, 0, len(sections))
	for _, section := range sections {
		chosen = append(chosen, report.Section(section))
	}
	data, err := report.Build(input, chosen)
	if err != nil {
		comm.Logger.WithError(err).Warn("Could not build the PDF report")
		return "", fmt.Errorf("Could not make the PDF report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		comm.Logger.WithError(err).Warnf("Could not write %s", comm.RedactPaths(path))
		return "", fmt.Errorf("Could not write %s", filepath.Base(path))
	}
	comm.Logger.Infof("Saved the PDF report %s", comm.RedactPaths(path))
	return path, nil
}

// windowsInvalidFileChars are the characters a Windows file name can not hold
const windowsInvalidFileChars = `<>:"/\|?*`

// pdfFileName is the report's default file name, ie URSA. intel 2026-10-29.pdf, with characters a Windows file name can not hold replaced
func pdfFileName(name string, now time.Time) string {
	clean := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(windowsInvalidFileChars, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if clean == "" {
		clean = "target"
	}
	return fmt.Sprintf("%s intel %s.pdf", clean, now.UTC().Format("2006-01-02"))
}

/*
	pdfInput turns a finished search into what the PDF report shows. It only reads its arguments, so it needs no PDF to test.

Parameters:
  - target: the searched corp or alliance
  - rep: its report, with the scope it covered
  - version: the app's version
  - now: when the PDF is made
*/
func pdfInput(target TargetResult, rep IntelReport, version string, now time.Time) report.Input {
	display := target.Display
	in := report.Input{
		Target: report.Target{
			Name:   display.Name,
			Ticker: display.Ticker,
			Type:   target.Type,
		},
		Scope: report.Scope{
			Killmails:   len(rep.Killmails),
			Limits:      rep.Scope.Limits,
			Stopped:     rep.Stopped,
			RateLimited: rep.RateLimited,
		},
		GeneratedAt: now,
		AppVersion:  version,
	}
	for _, related := range display.Related {
		value := related.Name
		if related.Ticker != "" {
			value += " [" + related.Ticker + "]"
		}
		in.Target.Details = append(in.Target.Details, report.Detail{Label: related.Title, Value: value})
	}
	for _, row := range display.DetailRows {
		if row.Label != "" && row.Value != "" {
			in.Target.Details = append(in.Target.Details, report.Detail{Label: row.Label, Value: row.Value})
		}
	}

	for _, source := range rep.Scope.Sources {
		in.Scope.Sources = append(in.Scope.Sources, report.Source{Name: source.Name, Main: source.Main, Killmails: source.Killmails, Oldest: source.Oldest, Newest: source.Newest})
	}

	for _, km := range rep.Killmails {
		if !km.Time.IsZero() {
			if in.Scope.Oldest.IsZero() || km.Time.Before(in.Scope.Oldest) {
				in.Scope.Oldest = km.Time
			}
			if km.Time.After(in.Scope.Newest) {
				in.Scope.Newest = km.Time
			}
		}
		switch km.Outcome {
		case "kill":
			in.Kills.Count++
			in.Kills.ISK += km.TotalValue
		case "loss", "internal":
			in.Losses.Count++
			in.Losses.ISK += km.TotalValue
		}
	}
	return in
}
