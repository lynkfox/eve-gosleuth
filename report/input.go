// Package report builds the PDF intel report: a briefing for fleet commanders from a finished search, in EVE time, with counts but no killmails.
// It reads only its Input, so it makes no API calls, and knows nothing of the gui package. See .claude/docs/pdf-report-plan.md.
package report

import "time"

// Section is one part of the report the user can choose to include. The Summary is always included.
type Section string

const (
	SectionSummary   Section = "summary"
	SectionActivity  Section = "activity"
	SectionDoctrines Section = "doctrines"
	SectionCapitals  Section = "capitals"
	SectionAppendix  Section = "appendix" // the EFT of each Fleet Doctrine, linked from its block
)

// Input is everything the report shows, filled by the gui from the search's report and target
type Input struct {
	Target      Target
	Scope       Scope
	Kills       Tally     // the target's kills
	Losses      Tally     // the target's losses, internal ones included (see aggregate.IsInternal)
	GeneratedAt time.Time // when the PDF was made, shown in the footer in EVE time
	AppVersion  string    // ie 0.9.3, shown in the footer as GoSleuth v0.9.3

	Activity  Activity
	Doctrines []Doctrine     // the fits tagged Fleet Doctrine, by hull class, largest first, then ship name
	Capitals  []CapitalClass // in report order: titans first, jump freighters last (ships.json capital_report_classes)
}

// Activity is when the target's killmails happened, in EVE time
type Activity struct {
	Grid     [7][24]int // killmails by weekday, Monday first, and hour
	Weeks    []Week     // each week from the oldest killmail's to the newest's, oldest first, empty weeks included
	TimeZone string     // from aggregate.PrimeTimeZone: EUTZ, USTZ, AUTZ, or Spread, empty with no killmails
}

// Week is one week's kills and losses, for the strip that shows whether activity is rising or falling
type Week struct {
	Start  time.Time // the Monday 00:00 EVE it starts on
	Kills  int
	Losses int // internal ones included
}

// Doctrine is one fit tagged Fleet Doctrine, every loss of it merged
type Doctrine struct {
	ShipName   string
	HullClass  string // ie Battleships, from ships.json hull_classes; the section shows a divider where it changes
	Render     []byte // the hull's render, a JPEG or PNG, nil for none (drawn as a plain box with the name)
	Role       string // the fit's naming tag besides Fleet Doctrine, ie Logi or Shield DPS, empty for none
	Losses     int
	Pilots     int // the different pilots who lost it
	LastSeen   time.Time
	LastSystem string
	Racks      []Rack      // High, Mid, Low, Rig, Subsystem, Drones, in that order, empty racks left out
	Fight      *FightStats // nil when the dogma engine had no numbers for it
	Reason     string      // why it is a Fleet Doctrine, in counts, ie "seen with 7 Ferox and 2 logi on one side", empty for none
	EFT        string      // the fit as EFT text, for the appendix
}

// Rack is one rack of a fit, its modules collapsed to counts
type Rack struct {
	Name    string // High, Mid, Low, Rig, Subsystem, Drones
	Modules []ModuleCount
}

// ModuleCount is a module and how many of it, ie 3 Heavy Missile Launcher II
type ModuleCount struct {
	Name  string
	Count int
}

// FightStats is what a fleet needs to know to shoot a fit, from the dogma engine with all skills V and every module active
type FightStats struct {
	Mode           string  // a tactical destroyer's mode, ie Defense, empty for other hulls
	EHP            float64 // omni, all three layers
	Tank           string  // Shield, Armor, or Hull: the layer with the most EHP
	Weakest        string  // EM, Thermal, Kinetic, or Explosive: the damage type the tanked layer resists least
	WeakestResist  float64 // that resist as a fraction, ie 0.35 for 35%
	SensorType     string  // Radar, Ladar, Magnetometric, Gravimetric, or Multispectral
	SensorStrength float64
	Neut           float64 // GJ per second its neutralizers and nosferatus take, 0 for none
	RemoteRep      float64 // HP per second it repairs others, all layers, 0 for none
	LocalRep       float64 // HP per second it repairs itself, all layers, 0 for none
}

// CapitalClass is one class of capital the target's pilots flew, ie Titans, with its hulls and pilots
type CapitalClass struct {
	Name   string
	Hulls  []CapitalHull  // by pilots, most first
	Pilots []CapitalPilot // one row per pilot and hull, newest sighting first
}

// CapitalHull is one capital hull of a class and how many of the target's pilots flew it
type CapitalHull struct {
	Name   string
	Icon   []byte // a 64 px icon, nil for none
	Pilots int
}

// CapitalPilot is a pilot seen flying a capital for the target
type CapitalPilot struct {
	Name       string
	Ship       string
	Corp       string // current, empty when not known
	Alliance   string // current, empty when in none or the same as the target
	LastSeen   time.Time
	LastSystem string
	Left       bool // no longer in the target's alliance, see aggregate.HasLeftTarget
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
