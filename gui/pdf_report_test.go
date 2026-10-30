package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/lynkfox/go_eve_sleuth/report"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
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

func pdfTestInit(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
}

func TestPDFInputLogoFromImages(t *testing.T) {
	logo := []byte("logo")
	rep := IntelReport{Images: map[string][]byte{api.EntityImageURL(types.ALLIANCE, 77): logo}}

	assert.Equal(t, logo, pdfInput(TargetResult{ID: 77, Type: "ALLIANCE"}, rep, "", time.Now()).Target.Logo)
	assert.Nil(t, pdfInput(TargetResult{ID: 78, Type: "ALLIANCE"}, rep, "", time.Now()).Target.Logo, "no entry means nil")
}

func TestPDFActivity(t *testing.T) {
	at := func(day, hour int) time.Time { return time.Date(2026, time.October, day, hour, 0, 0, 0, time.UTC) }
	// 2026-10-05 is a Monday
	activity := pdfActivity([]KillmailCard{
		{Outcome: "kill", Time: at(5, 19)},
		{Outcome: "loss", Time: at(11, 2)}, // Sunday of the first week
		{Outcome: "internal", Time: at(26, 19)},
		{Outcome: "kill", Time: at(28, 20)},
		{Outcome: "kill"}, // no time
	})

	assert.Equal(t, 2, activity.Grid[0][19], "two Mondays")
	assert.Equal(t, 1, activity.Grid[6][2])
	assert.Equal(t, 1, activity.Grid[2][20])
	assert.Len(t, activity.Weeks, 4, "the gap weeks are there, empty")
	assert.Equal(t, at(5, 0), activity.Weeks[0].Start)
	assert.Equal(t, report.Week{Start: at(5, 0), Kills: 1, Losses: 1}, activity.Weeks[0])
	assert.Equal(t, report.Week{Start: at(12, 0)}, activity.Weeks[1])
	assert.Equal(t, report.Week{Start: at(26, 0), Kills: 1, Losses: 1}, activity.Weeks[3])
	assert.Equal(t, 1, activity.Weeks[0].Kills)

	empty := pdfActivity(nil)
	assert.Empty(t, empty.Weeks)
	assert.Empty(t, empty.TimeZone)
}

func TestPDFDoctrines(t *testing.T) {
	pdfTestInit(t)
	slot := func(name string) *FitSlot { return &FitSlot{Module: FitPart{Name: name}} }
	stats := &fitting.Stats{
		Shield: fitting.Layer{EHP: 100, Resonance: [4]float64{1, 1, 1, 1}},
		Armor:  fitting.Layer{EHP: 50000, Resonance: [4]float64{0.5, 0.35, 0.25, 0.1}},
		Hull:   fitting.Layer{EHP: 300},
		EHP:    50400, Neut: 12,
		LocalRep:  fitting.Repair{Armor: 40, Hull: 2},
		RemoteRep: fitting.Repair{Armor: 100, Shield: 10},
		Sensor:    fitting.Sensor{Type: "Radar", Strength: 21},
	}
	doctrineTag := retrieval.FitTag{Name: "Fleet Doctrine", Reasons: []retrieval.FitTagReason{
		{Name: "Ferox in its battle", Count: 7, Needed: 5}, {Name: "Logi in its battle", Count: 2, Needed: 1}}}
	ferox := FitGroup{
		ShipID: 16227, ShipName: "Ferox", LastLost: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC),
		Tags: []retrieval.FitTag{doctrineTag, {Name: "Hidden", Role: true, Hidden: true, Priority: 1}, {Name: "Late", Role: true, Priority: 0}, {Name: "Shield DPS", Role: true, Priority: 5}},
		Losses: []FitLoss{
			{PilotName: "A", Time: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), SystemName: "Jita", EFT: "[Ferox, x]",
				Fit: FitLayout{
					Highs:  []*FitSlot{slot("Heavy Missile Launcher II"), slot("Heavy Missile Launcher II"), nil, slot("Scan Probe Launcher")},
					Mids:   []*FitSlot{slot("Large Shield Extender II")},
					Drones: []FitPart{{Name: "Hobgoblin II", Quantity: 3}, {Name: "Hobgoblin II"}, {Name: "Warrior II", Quantity: 0}},
				},
				Stats: []FitStats{{Mode: "Defense", Stats: stats}}},
			{PilotName: "B", Time: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), SystemName: "Amarr"},
			{PilotName: "A", Time: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), SystemName: "Old"},
		},
	}
	rare := FitGroup{ShipName: "Rare", Tags: []retrieval.FitTag{{Name: "Fleet Doctrine"}}, Losses: []FitLoss{{PilotName: "Z"}}}
	plain := FitGroup{ShipName: "Plain", Losses: []FitLoss{{}, {}, {}, {}}}
	render := []byte("render")

	doctrines := pdfDoctrines([]FitGroup{rare, plain, ferox}, map[string][]byte{api.TypeRenderURLAt(16227, pdfRenderSize): render})

	assert.Len(t, doctrines, 2)
	assert.Equal(t, "Ferox", doctrines[0].ShipName, "a battlecruiser before a hull with no class")
	assert.Equal(t, "Battlecruisers", doctrines[0].HullClass)
	assert.Equal(t, otherHullClass, doctrines[1].HullClass)
	assert.Equal(t, "Rare", doctrines[1].ShipName)
	first := doctrines[0]
	assert.Equal(t, render, first.Render)
	assert.Nil(t, doctrines[1].Render)
	assert.Equal(t, "Shield DPS", first.Role, "priority 5 beats the one with none, and hidden ones are skipped")
	assert.Equal(t, 3, first.Losses)
	assert.Equal(t, 2, first.Pilots)
	assert.Equal(t, ferox.LastLost, first.LastSeen)
	assert.Equal(t, "Jita", first.LastSystem)
	assert.Equal(t, "[Ferox, x]", first.EFT)
	assert.Equal(t, "seen with 7 Ferox and 2 logi on one side", first.Reason)
	assert.Equal(t, []report.Rack{
		{Name: "High", Modules: []report.ModuleCount{{Name: "Heavy Missile Launcher II", Count: 2}, {Name: "Scan Probe Launcher", Count: 1}}},
		{Name: "Mid", Modules: []report.ModuleCount{{Name: "Large Shield Extender II", Count: 1}}},
		{Name: "Drones", Modules: []report.ModuleCount{{Name: "Hobgoblin II", Count: 4}, {Name: "Warrior II", Count: 1}}},
	}, first.Racks)

	if assert.NotNil(t, first.Fight) {
		assert.Equal(t, "Defense", first.Fight.Mode)
		assert.Equal(t, "Armor", first.Fight.Tank)
		assert.Equal(t, "EM", first.Fight.Weakest)
		assert.InDelta(t, 0.5, first.Fight.WeakestResist, 1e-9)
		assert.InDelta(t, 50400, first.Fight.EHP, 1e-9)
		assert.Equal(t, "Radar", first.Fight.SensorType)
		assert.InDelta(t, 110, first.Fight.RemoteRep, 1e-9)
		assert.InDelta(t, 42, first.Fight.LocalRep, 1e-9)
		assert.InDelta(t, 12, first.Fight.Neut, 1e-9)
	}
	assert.Nil(t, doctrines[1].Fight)
	assert.Equal(t, "", doctrines[1].Reason)
}

func TestDoctrineReason(t *testing.T) {
	tagged := func(reasons ...retrieval.FitTagReason) FitGroup {
		return FitGroup{Tags: []retrieval.FitTag{{Name: "Fleet Doctrine", Reasons: reasons}}}
	}
	assert.Equal(t, "seen with 7 Ferox on one side", doctrineReason(tagged(retrieval.FitTagReason{Name: "Ferox in its battle", Count: 7})))
	assert.Equal(t, "lost 5 times by 4 pilots in one alliance", doctrineReason(tagged(
		retrieval.FitTagReason{Name: "Losses of this fit in one alliance", Count: 5}, retrieval.FitTagReason{Name: "Pilots who lost it", Count: 4})))
	assert.Equal(t, "seen with 7 Ferox on one side; lost 5 times in one alliance", doctrineReason(tagged(
		retrieval.FitTagReason{Name: "Ferox in its battle", Count: 7}, retrieval.FitTagReason{Name: "Losses of this fit in one alliance", Count: 5})))
	assert.Equal(t, "", doctrineReason(tagged()))
}

func TestPDFCapitals(t *testing.T) {
	pdfTestInit(t)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	ref := func(id int, name string) *EntityRef { return &EntityRef{ID: id, Name: name} }
	capitals := []CapitalShip{
		{ID: 20185, Name: "Charon", Pilots: []CapitalPilot{{Name: "F", LastSeen: day(1)}}},
		{ID: 19722, Name: "Naglfar", Pilots: []CapitalPilot{{Name: "A", LastSeen: day(2), LastSystem: "Jita", Corporation: ref(1, "Corp"), Alliance: ref(99, "Own")},
			{Name: "B", LastSeen: day(9), Alliance: ref(100, "Other"), Left: true}}},
		{ID: 19720, Name: "Revelation", Pilots: []CapitalPilot{{Name: "C", LastSeen: day(5)}, {Name: "D", LastSeen: day(6)}, {Name: "E", LastSeen: day(7)}}},
		{ID: 78576, Name: "Azariel", Pilots: []CapitalPilot{{Name: "T", LastSeen: day(3)}}},
		{ID: 19720, Name: "Empty"},
	}
	icon := []byte("icon")

	classes := pdfCapitals(TargetResult{ID: 99, Type: "ALLIANCE"}, capitals, map[string][]byte{api.TypeImageURL(19722): icon})

	names := []string{}
	for _, class := range classes {
		names = append(names, class.Name)
	}
	assert.Equal(t, []string{"Titans", "Dreadnoughts and Lancers", "Freighters"}, names)
	dreads := classes[1]
	assert.Equal(t, []report.CapitalHull{{Name: "Revelation", Pilots: 3}, {Name: "Naglfar", Icon: icon, Pilots: 2}}, dreads.Hulls)
	assert.Len(t, dreads.Pilots, 5)
	assert.Equal(t, "B", dreads.Pilots[0].Name, "newest sighting first")
	assert.Equal(t, "Other", dreads.Pilots[0].Alliance)
	assert.True(t, dreads.Pilots[0].Left)
	assert.Equal(t, "Naglfar", dreads.Pilots[0].Ship)
	var a report.CapitalPilot
	for _, row := range dreads.Pilots {
		if row.Name == "A" {
			a = row
		}
	}
	assert.Equal(t, report.CapitalPilot{Name: "A", Ship: "Naglfar", Corp: "Corp", LastSeen: day(2), LastSystem: "Jita"}, a, "the target's own alliance is left empty")
}

func TestCapitalClassOf(t *testing.T) {
	pdfTestInit(t)
	assert.Equal(t, "Titans", capitalClassOf(CapitalShip{ID: 78576, Name: "Azariel"}))
	assert.Equal(t, "Dreadnoughts and Lancers", capitalClassOf(CapitalShip{ID: 19720, Name: "Revelation"}))
	assert.Equal(t, "Mystery Ship", capitalClassOf(CapitalShip{ID: 999999999, Name: "Mystery Ship"}), "not in fit_types.json: the ship's own name")
}

func TestPDFDoctrinesByHullClass(t *testing.T) {
	doctrine := []retrieval.FitTag{{Name: "Fleet Doctrine"}}
	fit := func(id int, name string, losses int) FitGroup {
		return FitGroup{ShipID: id, ShipName: name, Tags: doctrine, Losses: make([]FitLoss, losses)}
	}
	// a frigate, two battlecruisers out of name order, a battleship, and an unlisted hull
	fits := []FitGroup{fit(587, "Rifter", 9), fit(16227, "Ferox", 2), fit(16229, "Brutix", 1), fit(638, "Raven", 1), fit(0, "Mystery", 20)}
	names, classes := []string{}, []string{}
	for _, d := range pdfDoctrines(fits, nil) {
		names, classes = append(names, d.ShipName), append(classes, d.HullClass)
	}
	assert.Equal(t, []string{"Raven", "Brutix", "Ferox", "Rifter", "Mystery"}, names)
	assert.Equal(t, []string{"Battleships", "Battlecruisers", "Battlecruisers", "Frigates", otherHullClass}, classes)
}
