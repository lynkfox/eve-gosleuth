package main

import "github.com/lynkfox/go_eve_sleuth/gui"

// allianceDemo is frontend/dist/tour/alliance.json: what the tour's stand-in backend answers for the alliance search, each the App method's JSON exactly
type allianceDemo struct {
	SearchName       string           `json:"searchName"`       // typed into the search bar by the tour, the target's display name
	Target           gui.TargetResult `json:"target"`           // SearchTarget
	Report           gui.IntelReport  `json:"report"`           // CollectIntel
	Members          gui.MemberPage   `json:"members"`          // GetMembers from offset 0, every row the tour has; the stand-in slices it by offset and count
	Allies           gui.AlliesUpdate `json:"allies"`           // RankAllies with showKilled false
	AlliesWithKilled gui.AlliesUpdate `json:"alliesWithKilled"` // RankAllies with showKilled true
	Holdings         gui.Holdings     `json:"holdings"`         // GetHoldings
}

// systemDemo is frontend/dist/tour/system.json, for the short switch to a system report
type systemDemo struct {
	SearchName string           `json:"searchName"`
	Target     gui.TargetResult `json:"target"`
	Report     gui.IntelReport  `json:"report"`
}
