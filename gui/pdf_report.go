package gui

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
	ExportPDF saves the last search's report as a PDF to a file the user picks. It takes a.mu only to copy the report, so the file picker never holds up a search.
	The frontend shows it only for a corporation or alliance.

Parameters:
  - sections: the sections to include beyond the Summary, which is always first, ie summary

Returns:
  - string: the path written, empty if the user closed the picker
  - error: a user readable message if there is no report to export or the file could not be made or written
*/
func (a *App) ExportPDF(sections []string) (path string, err error) {
	defer recoverAsError(&err)

	input, found := a.lastPDFInput()
	if !found {
		return "", errors.New("Search for a corporation or alliance first, then export its report")
	}

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

// lastPDFInput makes the PDF's input from the last search, false if there is none
func (a *App) lastPDFInput() (report.Input, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastTarget == nil || a.lastReport == nil {
		return report.Input{}, false
	}
	return pdfInput(*a.lastTarget, *a.lastReport, a.version, time.Now()), true
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
		ReleasesURL: api.PROJECT_URL + "/releases",
	}
	if entityType, ok := searchTypes[target.Type]; ok {
		in.Target.Logo = rep.Images[api.EntityImageURL(entityType, target.ID)]
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
	in.Activity = pdfActivity(rep.Killmails)
	in.Latest, in.Biggest = pdfBattles(rep.Battles)
	in.MinPilots = battleMinPilots
	in.Doctrines = pdfDoctrines(rep.Fits, rep.Images)
	in.Capitals = pdfCapitals(target, rep.Capitals, rep.Images)
	return in
}

// fleetDoctrineTag is the fit tag that makes a fit one of the PDF's Fleet Doctrines (static_data/fit_tag_rules.json)
const fleetDoctrineTag = "Fleet Doctrine"

// isFleetDoctrine reports whether a fit has the Fleet Doctrine tag
func isFleetDoctrine(fit FitGroup) bool {
	return slices.ContainsFunc(fit.Tags, func(tag retrieval.FitTag) bool { return tag.Name == fleetDoctrineTag })
}

/*
	pdfImageURLs lists the images the PDF report of a corp or alliance draws, so the search can fetch them: the target's logo, each Fleet Doctrine hull's render, and each capital hull's icon

Parameters:
  - targetType: the target's type
  - targetID: its id
  - rep: its report, with Fits and Capitals filled

Returns:
  - []string: each url once, the logo (api.EntityImageURL), renders (api.TypeRenderURLAt at pdfRenderSize), and icons (api.TypeImageURL)
*/
func pdfImageURLs(targetType types.EntityType, targetID int, rep IntelReport) []string {
	urls := []string{}
	add := func(url string) {
		if url != "" && !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	add(api.EntityImageURL(targetType, targetID))
	for _, fit := range rep.Fits {
		if isFleetDoctrine(fit) {
			add(api.TypeRenderURLAt(fit.ShipID, pdfRenderSize))
		}
	}
	for _, capital := range rep.Capitals {
		add(api.TypeImageURL(capital.ID))
	}
	return urls
}

// pdfRenderSize is the size of a Fleet Doctrine hull's render in pixels, enough for print at its block's size
const pdfRenderSize = 256

// weekStart is the Monday 00:00 EVE (UTC) of the week a time falls in
func weekStart(at time.Time) time.Time {
	at = at.UTC()
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -((int(at.Weekday()) + 6) % 7))
}

// pdfActivity counts the killmails by weekday and hour and by week, in EVE time. Killmails with no time are left out.
func pdfActivity(killmails []KillmailCard) report.Activity {
	activity := report.Activity{}
	times := []time.Time{}
	var oldest, newest time.Time
	for _, km := range killmails {
		if km.Time.IsZero() {
			continue
		}
		at := km.Time.UTC()
		times = append(times, at)
		activity.Grid[(int(at.Weekday())+6)%7][at.Hour()]++
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
		if at.After(newest) {
			newest = at
		}
	}
	if len(times) == 0 {
		return activity
	}
	first := weekStart(oldest)
	for start := first; !start.After(weekStart(newest)); start = start.AddDate(0, 0, 7) {
		activity.Weeks = append(activity.Weeks, report.Week{Start: start})
	}
	for _, km := range killmails {
		if km.Time.IsZero() {
			continue
		}
		week := &activity.Weeks[int(weekStart(km.Time).Sub(first).Hours()/(24*7)+0.5)]
		switch km.Outcome {
		case "kill":
			week.Kills++
		case "loss", "internal":
			week.Losses++
		}
	}
	activity.TimeZone = aggregate.PrimeTimeZone(times)
	return activity
}

// pdfBattleCount is how many battles the PDF lists
const pdfBattleCount = 10

// pdfBattles picks the PDF's battles, at most pdfBattleCount each: the latest, newest first, and the biggest, most pilots first, then most killmails, then newest
func pdfBattles(battles []Battle) (latest, biggest []report.Battle) {
	newest := slices.Clone(battles)
	slices.SortStableFunc(newest, func(a, b Battle) int { return b.Start.Compare(a.Start) })
	largest := slices.Clone(newest)
	slices.SortStableFunc(largest, func(a, b Battle) int {
		return cmp.Or(cmp.Compare(b.Pilots, a.Pilots), cmp.Compare(b.Killmails, a.Killmails))
	})
	return pdfBattleRows(newest), pdfBattleRows(largest)
}

// pdfBattleRows turns the first pdfBattleCount battles into the PDF's rows
func pdfBattleRows(battles []Battle) []report.Battle {
	result := []report.Battle{}
	for _, battle := range battles[:min(len(battles), pdfBattleCount)] {
		result = append(result, report.Battle{
			Start: battle.Start, End: battle.End, System: battle.System.Name, OtherSystems: len(battle.OtherSystems),
			Pilots: battle.Pilots, Ships: battle.Ships, Killmails: battle.Killmails,
			ZkillURL: battle.ZkillURL, EveToolsURL: battle.EveToolsURL, WarbeaconURL: battle.WarbeaconURL,
		})
	}
	return result
}

// fitRole is the fit's role tag besides Fleet Doctrine, the one with the lowest priority (0 last), empty for none
func fitRole(fit FitGroup) string {
	best := retrieval.FitTag{}
	for _, tag := range fit.Tags {
		if !tag.Role || tag.Hidden || tag.Name == fleetDoctrineTag {
			continue
		}
		if best.Name == "" || (tag.Priority != 0 && (best.Priority == 0 || tag.Priority < best.Priority)) {
			best = tag
		}
	}
	return best.Name
}

// hullClassOrder is the order the PDF's Fleet Doctrines list the built-in hull classes in (ships.json hull_classes), largest first, which StaticValues has no order for. Other classes follow alphabetically, then otherHullClass.
var hullClassOrder = []string{"Capitals", "Battleships", "Battlecruisers", "Cruisers", "Destroyers", "Frigates"}

// otherHullClass is the class of a hull whose group hull_classes doesn't list
const otherHullClass = "Other hulls"

// hullClassOf names the hull class a ship is grouped under in the Fleet Doctrines
func hullClassOf(shipID int) string {
	return cmp.Or(comm.HullClasses.NameOf(comm.TypeGroup(shipID)), otherHullClass)
}

// hullClassRank is a hull class's place in hullClassOrder, those it doesn't list after, otherHullClass last
func hullClassRank(class string) int {
	if at := slices.Index(hullClassOrder, class); at >= 0 {
		return at
	}
	if class == otherHullClass {
		return len(hullClassOrder) + 1
	}
	return len(hullClassOrder)
}

// pdfDoctrines makes the Fleet Doctrine fits' blocks by hull class, largest first, then by ship name, then most losses and newest
func pdfDoctrines(fits []FitGroup, images map[string][]byte) []report.Doctrine {
	doctrines := []report.Doctrine{}
	chosen := []FitGroup{}
	for _, fit := range fits {
		if isFleetDoctrine(fit) && len(fit.Losses) > 0 {
			chosen = append(chosen, fit)
		}
	}
	slices.SortStableFunc(chosen, func(a, b FitGroup) int {
		classA, classB := hullClassOf(a.ShipID), hullClassOf(b.ShipID)
		return cmp.Or(cmp.Compare(hullClassRank(classA), hullClassRank(classB)), cmp.Compare(classA, classB),
			cmp.Compare(strings.ToLower(a.ShipName), strings.ToLower(b.ShipName)),
			cmp.Compare(len(b.Losses), len(a.Losses)), b.LastLost.Compare(a.LastLost))
	})
	for _, fit := range chosen {
		doctrine := report.Doctrine{
			ShipName:  fit.ShipName,
			HullClass: hullClassOf(fit.ShipID),
			Render:    images[api.TypeRenderURLAt(fit.ShipID, pdfRenderSize)],
			Role:      fitRole(fit),
			Losses:    len(fit.Losses),
			Racks:     pdfRacks(fit.Losses[0].Fit),
			Reason:    doctrineReason(fit),
			EFT:       fit.Losses[0].EFT,
		}
		pilots := map[string]bool{}
		newest := fit.Losses[0]
		for _, loss := range fit.Losses {
			pilots[loss.PilotName] = true
			if len(loss.Fit.Implants) > 0 {
				doctrine.Implanted++
			}
			// losses come most implants first, so the fullest set leads, in slot order
			for _, implant := range loss.Fit.Implants {
				if !slices.Contains(doctrine.Implants, implant.Name) {
					doctrine.Implants = append(doctrine.Implants, implant.Name)
				}
			}
			if loss.Time.After(newest.Time) {
				newest = loss
			}
			if doctrine.Fight == nil {
				doctrine.Fight = pdfFightStats(loss.Stats)
			}
		}
		doctrine.Pilots = len(pilots)
		doctrine.LastSeen, doctrine.LastSystem = newest.Time, newest.SystemName
		doctrines = append(doctrines, doctrine)
	}
	return doctrines
}

// pdfRacks collapses a fit's slots to counts by module name, in the order first seen, leaving empty racks out
func pdfRacks(layout FitLayout) []report.Rack {
	racks := []report.Rack{}
	add := func(name string, modules []report.ModuleCount) {
		if len(modules) > 0 {
			racks = append(racks, report.Rack{Name: name, Modules: modules})
		}
	}
	count := func(modules []report.ModuleCount, name string, quantity int) []report.ModuleCount {
		if at := slices.IndexFunc(modules, func(each report.ModuleCount) bool { return each.Name == name }); at >= 0 {
			modules[at].Count += quantity
			return modules
		}
		return append(modules, report.ModuleCount{Name: name, Count: quantity})
	}
	slots := func(fitted []*FitSlot) []report.ModuleCount {
		modules := []report.ModuleCount{}
		for _, slot := range fitted {
			if slot != nil {
				modules = count(modules, slot.Module.Name, 1)
			}
		}
		return modules
	}
	add("High", slots(layout.Highs))
	add("Mid", slots(layout.Mids))
	add("Low", slots(layout.Lows))
	add("Rig", slots(layout.Rigs))
	add("Subsystem", slots(layout.Subsystems))
	drones := []report.ModuleCount{}
	for _, drone := range layout.Drones {
		drones = count(drones, drone.Name, max(drone.Quantity, 1))
	}
	add("Drones", drones)
	return racks
}

// damageTypes are the damage types in the order of a fitting.Layer's Resonance
var damageTypes = [4]string{"EM", "Thermal", "Kinetic", "Explosive"}

// pdfFightStats reads the fight numbers from a loss's stats, its first entry (a tactical destroyer's Defense mode). nil when it has none.
func pdfFightStats(stats []FitStats) *report.FightStats {
	if len(stats) == 0 || stats[0].Stats == nil {
		return nil
	}
	first := stats[0]
	fight := &report.FightStats{
		Mode:           first.Mode,
		EHP:            first.EHP,
		SensorType:     first.Sensor.Type,
		SensorStrength: first.Sensor.Strength,
		Neut:           first.Neut,
		RemoteRep:      first.RemoteRep.Shield + first.RemoteRep.Armor + first.RemoteRep.Hull,
		LocalRep:       first.LocalRep.Shield + first.LocalRep.Armor + first.LocalRep.Hull,
	}
	tank, layer := "Shield", first.Shield
	if first.Armor.EHP > layer.EHP {
		tank, layer = "Armor", first.Armor
	}
	if first.Hull.EHP > layer.EHP {
		tank, layer = "Hull", first.Hull
	}
	weakest := 0
	for index, resonance := range layer.Resonance {
		if resonance > layer.Resonance[weakest] {
			weakest = index
		}
	}
	fight.Tank, fight.Weakest, fight.WeakestResist = tank, damageTypes[weakest], 1-layer.Resonance[weakest]
	return fight
}

/*
doctrineReason words the counts that made a fit a Fleet Doctrine, ie "seen with 7 Ferox and 2 logi on one side", or "lost 5 times by 4 pilots in one alliance".
The reasons are those of fit_tag_rules.json's Fleet Doctrine tag (aggregate.FleetDoctrineInBattle and FleetDoctrineRepeated). Both are joined with "; ".
*/
func doctrineReason(fit FitGroup) string {
	var reasons []retrieval.FitTagReason
	for _, tag := range fit.Tags {
		if tag.Name == fleetDoctrineTag {
			reasons = tag.Reasons
		}
	}
	var battle, repeated []string
	hulls, logi, losses, pilots := 0, 0, 0, 0
	hullName := ""
	for _, reason := range reasons {
		switch {
		case strings.HasSuffix(reason.Name, " in its battle") && reason.Name != "Logi in its battle":
			hullName, hulls = strings.TrimSuffix(reason.Name, " in its battle"), reason.Count
		case reason.Name == "Logi in its battle":
			logi = reason.Count
		case reason.Name == "Losses of this fit in one alliance":
			losses = reason.Count
		case reason.Name == "Pilots who lost it":
			pilots = reason.Count
		}
	}
	if hulls > 0 {
		line := fmt.Sprintf("seen with %d %s", hulls, hullName)
		if logi > 0 {
			line += fmt.Sprintf(" and %d logi", logi)
		}
		battle = append(battle, line+" on one side")
	}
	if losses > 0 {
		line := fmt.Sprintf("lost %d times", losses)
		if pilots > 0 {
			line += fmt.Sprintf(" by %d pilots", pilots)
		}
		repeated = append(repeated, line+" in one alliance")
	}
	return strings.Join(append(battle, repeated...), "; ")
}

// capitalClassOrder is the order the PDF lists the built-in capital classes in (ships.json capital_report_classes), which StaticValues has no order for. Other classes follow alphabetically.
var capitalClassOrder = []string{"Titans", "Supercarriers", "Dreadnoughts and Lancers", "Force Auxiliaries", "Carriers", "Capital Industrials", "Freighters", "Jump Freighters"}

// capitalClassOf names the class a capital hull is listed under: its group's report class, else its group's name made plural, else the ship's own name
func capitalClassOf(ship CapitalShip) string {
	group := comm.TypeGroup(ship.ID)
	if class := comm.CapitalReportClasses.NameOf(group); class != "" {
		return class
	}
	if name := comm.CapitalGroups.NameOf(group); name != "" {
		return name + "s"
	}
	return ship.Name
}

// pdfCapitals groups the capital hulls by class, in report order, each with its hulls by pilots and one row per pilot and hull, newest sighting first
func pdfCapitals(target TargetResult, capitals []CapitalShip, images map[string][]byte) []report.CapitalClass {
	// the target's own alliance, which is not worth repeating on every row
	ownAlliance := 0
	if target.Type == types.ALLIANCE.String() {
		ownAlliance = target.ID
	}
	for _, related := range target.Display.Related {
		if related.Type == types.ALLIANCE && ownAlliance == 0 {
			ownAlliance = related.ID
		}
	}

	byClass := map[string]*report.CapitalClass{}
	classes := []string{}
	for _, capital := range capitals {
		if len(capital.Pilots) == 0 {
			continue
		}
		name := capitalClassOf(capital)
		class := byClass[name]
		if class == nil {
			class = &report.CapitalClass{Name: name}
			byClass[name] = class
			classes = append(classes, name)
		}
		class.Hulls = append(class.Hulls, report.CapitalHull{Name: capital.Name, Icon: images[api.TypeImageURL(capital.ID)], Pilots: len(capital.Pilots)})
		for _, pilot := range capital.Pilots {
			row := report.CapitalPilot{Name: pilot.Name, Ship: capital.Name, LastSeen: pilot.LastSeen, LastSystem: pilot.LastSystem, Left: pilot.Left}
			if pilot.Corporation != nil {
				row.Corp = pilot.Corporation.Name
			}
			if pilot.Alliance != nil && pilot.Alliance.ID != ownAlliance {
				row.Alliance = pilot.Alliance.Name
			}
			class.Pilots = append(class.Pilots, row)
		}
	}

	rank := func(name string) int {
		if at := slices.Index(capitalClassOrder, name); at >= 0 {
			return at
		}
		return len(capitalClassOrder)
	}
	slices.SortStableFunc(classes, func(a, b string) int {
		return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a, b))
	})
	result := []report.CapitalClass{}
	for _, name := range classes {
		class := byClass[name]
		slices.SortStableFunc(class.Hulls, func(a, b report.CapitalHull) int { return cmp.Compare(b.Pilots, a.Pilots) })
		slices.SortStableFunc(class.Pilots, func(a, b report.CapitalPilot) int { return b.LastSeen.Compare(a.LastSeen) })
		result = append(result, *class)
	}
	return result
}
