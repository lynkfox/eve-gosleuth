package report_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func activityFixture() report.Input {
	in := fixture()
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			in.Activity.Grid[d][h] = (d*h*7)%130 + 1
		}
	}
	in.Activity.Grid[2][20] = 1234
	for i := 0; i < 12; i++ {
		in.Activity.Weeks = append(in.Activity.Weeks, report.Week{Start: time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7*i), Kills: 10 + i*3, Losses: i * 2})
	}
	in.Activity.TimeZone = "USTZ"
	return in
}

func TestBuildActivity(t *testing.T) {
	b, err := report.Build(activityFixture(), []report.Section{report.SectionSummary, report.SectionActivity})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}

func TestBuildActivityEmpty(t *testing.T) {
	in := fixture()
	in.Scope = report.Scope{}
	b, err := report.Build(in, []report.Section{report.SectionActivity})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}
