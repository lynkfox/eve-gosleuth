package report_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/stretchr/testify/assert"
)

func TestBuildBattles(t *testing.T) {
	in := fixture()
	start := time.Date(2026, 10, 20, 19, 4, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		in.Biggest = append(in.Biggest, report.Battle{
			Start: start, End: start.Add(time.Duration(i*17) * time.Minute), System: fmt.Sprintf("System %d %s", i, strings.Repeat("W", i*3)),
			OtherSystems: i % 3, Pilots: 300 - i, Ships: 320 - i, Killmails: 90 - i,
			ZkillURL: "https://zkillboard.com/related/30000142/202610201900/", EveToolsURL: "https://br.evetools.org/related/30000142/202610201900",
			WarbeaconURL: "https://warbeacon.net/br/related/30000142/202610201900",
		})
	}
	in.Biggest[3].ZkillURL, in.Biggest[3].EveToolsURL, in.Biggest[3].WarbeaconURL = "", "", ""
	in.Latest = in.Biggest
	in.MinPilots = 10
	b, err := report.Build(in, []report.Section{report.SectionBattles})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
	assert.Equal(t, 54, bytes.Count(b, []byte("/Subtype /Link")), "each battle with a system links its three sites, in both tables")
	assert.Equal(t, 2, bytes.Count(b, []byte("/Type /Page\n")), "the Summary, then both tables on one page")
	if p := os.Getenv("REPORT_SAMPLE_BATTLES"); p != "" {
		assert.NoError(t, os.WriteFile(p, b, 0o644))
	}
}

func TestBuildNoBattles(t *testing.T) {
	b, err := report.Build(fixture(), []report.Section{report.SectionBattles})
	assert.NoError(t, err)
	assert.True(t, bytes.HasPrefix(b, []byte("%PDF")))
}
