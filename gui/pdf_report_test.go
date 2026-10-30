package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func TestPDFInputCountsKillsAndLosses(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, time.October, d, 12, 0, 0, 0, time.UTC) }
	target := TargetResult{ID: 1, Type: "ALLIANCE", Display: api.TargetDisplay{
		Name: "Test Alliance", Ticker: "TEST.",
		Related:    []api.RelatedEntity{{Title: "Executor", Name: "Exec Corp", Ticker: "EXEC"}, {Title: "CEO", Name: "Boss"}},
		DetailRows: []api.DetailRow{{Label: "Members", Value: "1,200"}, {Label: "Empty", Value: ""}},
	}}
	rep := IntelReport{Scope: ReportScope{Limits: []string{"At most 500 killmails"}, Sources: []ScopeSource{{Name: "Test Alliance", Main: true, Killmails: 4}}}, Stopped: true, RateLimited: true, Killmails: []KillmailCard{
		{Outcome: "kill", TotalValue: 100, Time: day(5)},
		{Outcome: "kill", TotalValue: 50.5, Time: day(9)},
		{Outcome: "loss", TotalValue: 10, Time: day(2)},
		{Outcome: "internal", TotalValue: 5, Time: day(7)},
		{Outcome: "", TotalValue: 999, Time: day(1).Add(-48 * time.Hour)},
	}}
	now := day(20)

	in := pdfInput(target, rep, "1.2.3", now)

	assert.Equal(t, "Test Alliance", in.Target.Name)
	assert.Equal(t, "TEST.", in.Target.Ticker)
	assert.Equal(t, "ALLIANCE", in.Target.Type)
	assert.Equal(t, []string{"At most 500 killmails"}, in.Scope.Limits)
	assert.Equal(t, []report.Source{{Name: "Test Alliance", Main: true, Killmails: 4}}, in.Scope.Sources)
	assert.Equal(t, "Executor", in.Target.Details[0].Label)
	assert.Equal(t, "Exec Corp [EXEC]", in.Target.Details[0].Value)
	assert.Equal(t, "Boss", in.Target.Details[1].Value)
	assert.Len(t, in.Target.Details, 3)
	assert.Equal(t, 2, in.Kills.Count)
	assert.InDelta(t, 150.5, in.Kills.ISK, 0.001)
	assert.Equal(t, 2, in.Losses.Count)
	assert.InDelta(t, 15, in.Losses.ISK, 0.001)
	assert.Equal(t, 5, in.Scope.Killmails)
	assert.Equal(t, day(1).Add(-48*time.Hour), in.Scope.Oldest)
	assert.Equal(t, day(9), in.Scope.Newest)
	assert.True(t, in.Scope.Stopped)
	assert.True(t, in.Scope.RateLimited)
	assert.Equal(t, now, in.GeneratedAt)
	assert.Equal(t, "1.2.3", in.AppVersion)
	assert.Nil(t, in.Target.Logo)
}

func TestPDFInputWithNoKillmails(t *testing.T) {
	in := pdfInput(TargetResult{Type: "CORPORATION"}, IntelReport{}, "", time.Now())
	assert.Equal(t, 0, in.Scope.Killmails)
	assert.True(t, in.Scope.Oldest.IsZero())
	assert.True(t, in.Scope.Newest.IsZero())
	assert.Zero(t, in.Kills.Count)
	assert.Zero(t, in.Losses.Count)
}

func TestPDFFileName(t *testing.T) {
	now := time.Date(2026, time.October, 29, 3, 0, 0, 0, time.UTC)
	assert.Equal(t, "URSA. intel 2026-10-29.pdf", pdfFileName("URSA.", now))
	assert.Equal(t, "A_B_C_D intel 2026-10-29.pdf", pdfFileName(`A/B:C*D`, now))
	assert.Equal(t, "target intel 2026-10-29.pdf", pdfFileName("  ", now))
}
