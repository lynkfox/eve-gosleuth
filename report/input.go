// Package report builds the PDF intel report: a briefing for fleet commanders from a finished search, in EVE time, with counts but no killmails.
// It reads only its Input, so it makes no API calls, and knows nothing of the gui package. See .claude/docs/pdf-report-plan.md.
package report

import "time"

// Section is one part of the report the user can choose to include. The Summary is always included.
type Section string

const (
	SectionSummary Section = "summary"
)

// Input is everything the report shows, filled by the gui from the search's report and target
type Input struct {
	Target      Target
	Scope       Scope
	Kills       Tally     // the target's kills
	Losses      Tally     // the target's losses, internal ones included (see aggregate.IsInternal)
	GeneratedAt time.Time // when the PDF was made, shown in the footer in EVE time
	AppVersion  string    // ie 0.9.3, shown in the footer as GoSleuth v0.9.3
}

// Target is the corp, alliance, pilot, or system the report is about
type Target struct {
	Name    string
	Ticker  string   // a corp's or alliance's, ie URSA., empty for others
	Type    string   // PILOT, CORPORATION, ALLIANCE, or SYSTEM
	Details []Detail // shown under the name, ie Alliance, CEO, Members, Founded
	Logo    []byte   // a PNG or JPEG of its logo or portrait, nil for none (filled in phase 2)
}

// Detail is a label and value shown under the target's name
type Detail struct {
	Label string
	Value string
}

// Scope is what the search covered, so a reader knows what the report can and can't say
type Scope struct {
	Killmails   int
	Oldest      time.Time // the oldest and newest killmail, zero when there are none
	Newest      time.Time
	Limits      []string // the search limits applied, each a plain phrase, ie "At most 500 killmails", "Killmails newer than 30 days"
	Sources     []Source // the target then each affiliate the search included, empty when it had none
	Stopped     bool     // the user stopped the search, so it holds what was collected by then
	RateLimited bool     // the ESI rate limited the search, so killmails that needed it are missing
}

// Source is the killmails one list of the search gave: the target's own, or an affiliate's
type Source struct {
	Name      string
	Main      bool // the target itself
	Killmails int
	Oldest    time.Time // zero with no killmails
	Newest    time.Time
}

// Tally is a count of killmails and the ISK they were worth
type Tally struct {
	Count int
	ISK   float64
}

/*
	Build makes the PDF report

Parameters:
  - in: what the report shows
  - sections: the sections to include beyond the Summary, which is always first. Unknown ones are ignored.

Returns:
  - []byte: the PDF file
  - error: if it could not be made
*/
func Build(in Input, sections []Section) ([]byte, error) {
	return build(in, sections)
}
