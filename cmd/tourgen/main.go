// tourgen writes the new user tour's demo reports, frontend/dist/tour/alliance.json and system.json, from real searches made with the
// user's settings and cache, with every pilot, corp, and alliance renamed to a made-up one (see .claude/docs/tour-plan.md).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/gui"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

func main() {
	alliance := flag.String("alliance", "Dracarys.", "the alliance whose report the tour walks through")
	system := flag.String("system", "J163225", "the system the tour switches to")
	out := flag.String("out", filepath.Join("frontend", "dist", "tour"), "the folder the demo files are written to")
	maxKillmails := flag.Int("max", 100, "the most killmails each search collects")
	days := flag.Int("days", 30, "only killmails newer than this many days")
	raw := flag.String("raw", "", "a folder of unscrubbed alliance.json and system.json from an earlier run, scrubbed again with no search made")
	saveRaw := flag.String("save-raw", "", "also write the unscrubbed files to this folder (keep it out of the repository), to scrub again later with -raw")
	flag.Parse()

	var allianceDemo allianceDemo
	var systemDemo systemDemo
	if *raw != "" {
		read(filepath.Join(*raw, "alliance.json"), &allianceDemo)
		read(filepath.Join(*raw, "system.json"), &systemDemo)
	} else {
		app, done := gui.NewHeadlessApp("tourgen")
		atExit = append(atExit, done)
		limits := gui.SearchLimits{MaxKillmails: *maxKillmails, KillmailPages: 5, KillmailsNewerThanDays: *days, UseCachedKillmails: true, IncludeHoldings: true}
		must(app.SetSearchLimits(limits))
		allianceDemo = captureAlliance(app, *alliance)
		systemDemo = captureSystem(app, *system)
	}
	if *saveRaw != "" {
		must(os.MkdirAll(*saveRaw, 0o755))
		write(filepath.Join(*saveRaw, "alliance.json"), allianceDemo)
		write(filepath.Join(*saveRaw, "system.json"), systemDemo)
	}
	scrubAlliance(&allianceDemo)
	scrubSystem(&systemDemo)
	reportScrubbed()
	must(verifyClean(allianceDemo, systemDemo))

	must(os.MkdirAll(*out, 0o755))
	write(filepath.Join(*out, "alliance.json"), allianceDemo)
	write(filepath.Join(*out, "system.json"), systemDemo)
	runAtExit()
}

// read loads a demo file written by an earlier run
func read(path string, value any) {
	data, err := os.ReadFile(path)
	must(err)
	must(json.Unmarshal(data, value))
}

// captureAlliance runs the alliance search as the page does, then the calls its tabs make
func captureAlliance(app *gui.App, name string) allianceDemo {
	target, report := search(app, name, types.ALLIANCE.String())
	members, err := app.GetMembers(target.ID, target.Type, 0, 150)
	must(err)
	allies, err := app.RankAllies(target.ID, target.Type, false)
	must(err)
	withKilled, err := app.RankAllies(target.ID, target.Type, true)
	must(err)
	holdings, err := app.GetHoldings(target.ID, target.Type)
	must(err)
	members.HasMore = false
	return allianceDemo{SearchName: target.Display.Name, Target: target, Report: report, Members: members, Allies: allies, AlliesWithKilled: withKilled, Holdings: holdings}
}

// captureSystem runs the system search
func captureSystem(app *gui.App, name string) systemDemo {
	target, report := search(app, name, types.SYSTEM.String())
	return systemDemo{SearchName: target.Display.Name, Target: target, Report: report}
}

// search runs SearchTarget then CollectIntel, as the page's search does, and takes the search back out of the user's recent searches at exit unless it was there before
func search(app *gui.App, name string, searchType string) (gui.TargetResult, gui.IntelReport) {
	before, err := app.GetRecentSearches()
	must(err)
	target, err := app.SearchTarget(name, searchType, 0)
	must(err)
	if len(target.Choices) > 0 {
		must(fmt.Errorf("%q matches more than one %s", name, searchType))
	}
	if !slices.ContainsFunc(before, func(recent retrieval.RecentSearch) bool {
		return recent.Type == target.Type && strings.EqualFold(recent.Name, target.Display.Name)
	}) {
		// at exit, so a later step failing still takes it out
		atExit = append(atExit, func() {
			if _, err := app.RemoveRecentSearch(target.Display.Name, target.Type); err != nil {
				comm.Logger.WithError(err).Warn("Could not remove the search from the recent searches")
			}
		})
	}
	report, err := app.CollectIntel(target.ID, target.Type)
	must(err)
	return target, report
}

// write saves value as indented JSON at path, through a temporary file so a failure leaves the old file
func write(path string, value any) {
	data, err := json.Marshal(value)
	must(err)
	must(os.WriteFile(path+".tmp", append(data, '\n'), 0o644))
	must(os.Rename(path+".tmp", path))
	fmt.Printf("Wrote %s (%d KB)\n", path, len(data)>>10)
}

// atExit are the steps to undo or close on the way out, run last first by runAtExit, as os.Exit skips deferred calls
var atExit []func()

// runAtExit runs the atExit steps, last added first, so the cache closes after the recent searches are put back
func runAtExit() {
	for i := len(atExit) - 1; i >= 0; i-- {
		atExit[i]()
	}
	atExit = nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		runAtExit()
		os.Exit(1)
	}
}
