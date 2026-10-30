package main

import (
	"fmt"
	"sort"
	"strings"
)

// scrubbed is the one scrubber both demo files go through, so an entity in both is the same made-up one in each
var scrubbed = newScrubber(newRenamer())

// The tour's own alliance, with its executor, have names picked by hand: short, pleasant, and easy to type.
const (
	demoAllianceName   = "Bearded Sleuths"
	demoAllianceTicker = "BEARD"
	demoCorpName       = "Honeypot Holdings"
	demoCorpTicker     = "HNYPT"
	demoCEOName        = "Barnaby Honeycomb"
	demoSources        = "made-up players for the tour"
)

// pinAlliance gives the searched alliance, and its executor corp and CEO where the display shows them, the tour's own names
func pinAlliance(demo *allianceDemo) {
	fixedNames[demo.Target.ID] = fakeEntity{ID: fakeIDBase + 2*fakeIDSpan + 1, Name: demoAllianceName, Ticker: demoAllianceTicker}
	for _, related := range demo.Target.Display.Related {
		switch related.Title {
		case "Executor Corp":
			fixedNames[related.ID] = fakeEntity{ID: fakeIDBase + fakeIDSpan + 1, Name: demoCorpName, Ticker: demoCorpTicker}
		case "Executor CEO":
			fixedNames[related.ID] = fakeEntity{ID: fakeIDBase + 1, Name: demoCEOName}
		}
	}
}

// scrubAlliance renames every pilot, corp, and alliance in the demo to a made-up one and trims it, see .claude/docs/tour-plan.md, Demo data
func scrubAlliance(demo *allianceDemo) {
	pinAlliance(demo)
	// the tour's summary line says where its data came from, as a real search's does
	demo.Report.Sources = demoSources
	trimmed := trimReport(&demo.Report)
	trimmed.allies(demo.Allies.Allies)
	trimmed.allies(demo.AlliesWithKilled.Allies)
	if len(demo.Members.Rows) > maxMemberRows {
		demo.Members.Rows = demo.Members.Rows[:maxMemberRows]
	}
	if len(demo.Members.Corporations) > maxMemberRows {
		demo.Members.Corporations = demo.Members.Corporations[:maxMemberRows]
	}
	demo.SearchName = demo.Target.Display.Name
	rename(demo)
}

// scrubSystem does the same for the system demo, sharing the made-up names with the alliance demo's where the same entity is in both
func scrubSystem(demo *systemDemo) {
	demo.Report.Sources = demoSources
	trimReport(&demo.Report)
	rename(demo)
}

// rename runs a demo through the scrubber and back
func rename(demo any) {
	tree, err := toTree(demo)
	must(err)
	scrubbed.collect(tree)
	scrubbed.prepare()
	tree = scrubbed.rewrite(tree, "")
	must(fromTree(tree, demo))
}

// verifyClean refuses a demo set that still holds a real name, id, or link, listing where each is
func verifyClean(demos ...any) error {
	scrubbed.prepare()
	var problems []string
	for _, demo := range demos {
		tree, err := toTree(demo)
		if err != nil {
			return err
		}
		for _, leak := range scrubbed.leaks(tree) {
			problems = append(problems, fmt.Sprintf("%T%s", demo, leak))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("the demo still holds %d real names or ids:\n%s", len(problems), strings.Join(problems, "\n"))
	}
	return nil
}

// reportScrubbed prints what the renaming did, for a look at whether it caught anything it should not have
func reportScrubbed() {
	fmt.Printf("Renamed %d entities\n", len(scrubbed.names.fakes))
	var keys []string
	for key, count := range scrubbed.found {
		keys = append(keys, fmt.Sprintf("%s x%d", key, count))
	}
	sort.Strings(keys)
	fmt.Printf("Names replaced in free text, by field: %s\n", strings.Join(keys, ", "))
}
