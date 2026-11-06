// Package warexport writes a war's report as a folder of HTML pages and a JSON file. It works from a plain Input, with no API calls.
package warexport

import "time"

// Input is everything the export shows. Every string is cleaned before it is written, see cleanStrings.
type Input struct {
	Name     string
	FromDate string // YYYY-MM-DD, empty for no start
	Until    string // YYYY-MM-DD, empty while it goes on
	SideA    string
	SideB    string
	Exported time.Time
	Version  string

	A, B, Others Measures
	Counts       Counts
	Classes      []ClassCount // most systems first
	Reach        []ReachLine  // the groups zKillboard's cap kept from their start of the war
	GroupsA      []Group      // most pilots first
	GroupsB      []Group

	Days    []Day    // oldest first
	Systems []System // every system of the war
	Battles []Battle // newest first, skirmishes among them and ignored ones not
	FlownA  []Hull
	FlownB  []Hull

	AppearancesA, AppearancesB, AppearancesOthers []GroupAppearance // most battles first
}

// Measures are one side's totals on the Summary
type Measures struct {
	ISKLost           float64         `json:"iskLost"`
	ISKFights         float64         `json:"iskFights"`
	ISKTrash          float64         `json:"iskTrash"`
	ISKStructures     float64         `json:"iskStructures"`
	ShipsLost         int             `json:"shipsLost"`
	ShipsFights       int             `json:"shipsFights"`
	ShipsTrash        int             `json:"shipsTrash"`
	Citadels          []StructureKind `json:"citadels"`
	CitadelCount      int             `json:"citadelCount"`
	POS               int             `json:"pos"`
	StationRefs       int             `json:"stationRefs"`
	RefAttempts       int             `json:"refAttempts"`
	RefAttemptsUnsure int             `json:"refAttemptsUnsure"`
	Efficiency        float64         `json:"efficiency"` // 0 to 100
	HasEfficiency     bool            `json:"hasEfficiency"`
}

// StructureKind is how many of one structure type were lost
type StructureKind struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Icon  string `json:"icon"` // on the image server
}

// Counts are the war's battles and what it holds
type Counts struct {
	Battles    int `json:"battles"`
	Skirmishes int `json:"skirmishes"`
	Ignored    int `json:"ignored"`
	Systems    int `json:"systems"`
	Killmails  int `json:"killmails"`
}

// ClassCount is how many systems of a wormhole class or security band the war touched
type ClassCount struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

// ReachLine is a group whose list stopped short of the start of the war
type ReachLine struct {
	Group  string `json:"group"`
	Oldest string `json:"oldest"` // YYYY-MM-DD
}

// Group is an alliance, or a corp with no alliance, on a side
type Group struct {
	Name   string `json:"name"`
	Pilots int    `json:"pilots"`
	Icon   string `json:"icon"`
}

// Day is what each side lost on a day in EVE time
type Day struct {
	Date string  `json:"date"` // YYYY-MM-DD
	A    DaySide `json:"a"`
	B    DaySide `json:"b"`
}

// DaySide is one side's losses on a day
type DaySide struct {
	ISK        float64 `json:"isk"`
	Ships      int     `json:"ships"`
	Structures int     `json:"structures"`
}

// System is one system of the war
type System struct {
	ID       int
	Name     string
	Nickname string
	Class    string   // wormhole class, or the security band
	Statics  []string // destinations, ie C5, Highsec
	Battles  int
	SeenA    bool // a citadel or POCO of side A on a killmail here, lost or firing
	SeenB    bool
	LostA    time.Time // when side A first lost a citadel or POCO here, zero for never
	LostB    time.Time
}

// Battle is a row of the Battles page and a bubble of the Timeline
type Battle struct {
	Start        time.Time  `json:"start"`
	End          time.Time  `json:"end"`
	SystemID     int        `json:"-"` // for its color
	System       string     `json:"system"`
	Nickname     string     `json:"nickname"`
	OtherSystems []string   `json:"otherSystems"`
	Class        string     `json:"class"`
	Pilots       int        `json:"pilots"`
	Killmails    int        `json:"killmails"`
	Skirmish     bool       `json:"skirmish"`
	Attacked     bool       `json:"attacked"` // a station was marked under attack in it, for the Timeline's ring
	Notes        []string   `json:"notes"`
	Marks        []Mark     `json:"marks"` // the station chits of the Battles tab
	A            BattleSide `json:"a"`
	B            BattleSide `json:"b"`
	Others       BattleSide `json:"others"`
	Links        []Link     `json:"links"`  // only those that pass safeLink
	Holder       string     `json:"holder"` // a or b when killmails show only that side's structures in its system, else empty. Set by Build.
}

// Mark is a chit on a battle saying what happened to a station in it, as the Battles tab shows it
type Mark struct {
	Side  string    `json:"side"`  // a or b
	Title string    `json:"title"` // Citadel, POS, or POCO
	Word  string    `json:"word"`  // the same in a sentence
	State string    `json:"state"` // destroyed, reffed, attacked, or unsure
	At    time.Time `json:"at"`    // for reffed, when the station died
	Notes []string  `json:"notes"` // what marked it, ie Fortizar fired in J155928 on 2 killmails
}

// BattleSide is one side's part in a battle. ISKLost leaves out the trash attached to it.
type BattleSide struct {
	Group      string  `json:"group"` // its largest group, empty if it had none there
	GroupIcon  string  `json:"groupIcon"`
	Pilots     int     `json:"pilots"`
	ISKLost    float64 `json:"iskLost"`
	ShipsLost  int     `json:"shipsLost"`
	Structures int     `json:"structures"`
}

// Link is a battle report link
type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Hull is a ship flown by a side: each pilot once per battle, and those destroyed
type Hull struct {
	Name string `json:"name"`
	Seen int    `json:"seen"`
	Lost int    `json:"lost"`
	Icon string `json:"icon"`
}

// GroupAppearance is the battles a group was seen in on a side: an alliance with its corps, or a corp with no alliance
type GroupAppearance struct {
	Name    string
	Battles int
	Corps   []CorpAppearance
}

// CorpAppearance is a corp under its alliance
type CorpAppearance struct {
	Name    string
	Battles int
}
