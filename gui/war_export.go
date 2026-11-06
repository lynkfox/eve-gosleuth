package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/warexport"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
	ExportWar saves a war's report as a web page and a JSON file in a folder of their own, under the parent folder the user picks once, and opens the folder. It reads only what the war holds, so it makes no API calls.

Parameters:
  - id: the war

Returns:
  - string: the page's path, empty if the user closed the folder picker
  - error: a user readable message if the war is gone or its files could not be written
*/
func (a *App) ExportWar(id int) (path string, err error) {
	defer recoverAsError(&err)

	parent := a.currentSettings().WarExportDir
	if info, statErr := os.Stat(parent); parent == "" || statErr != nil || !info.IsDir() {
		parent, err = a.pickWarExportFolder()
		if err != nil || parent == "" {
			return "", err
		}
	}
	path, err = a.exportWarTo(parent, id)
	if err != nil {
		return "", err
	}
	if err := openFolderSelecting(path); err != nil {
		comm.Logger.WithError(err).Warn("Could not open the war's export folder")
	}
	return path, nil
}

// exportWarTo writes a war's export under a folder. It holds a.mu only while it reads the war.
func (a *App) exportWarTo(parent string, id int) (string, error) {
	input, err := a.warExportInputOf(id)
	if err != nil {
		return "", err
	}
	files, err := warexport.Build(input)
	if err != nil {
		comm.Logger.WithError(err).Warn("Could not build the war's export")
		return "", fmt.Errorf("Could not make the war's pages: %w", err)
	}
	folder, err := warexport.Write(parent, input.Name, files)
	if err != nil {
		comm.Logger.Warnf("Could not write the war's export: %s", comm.RedactPaths(err.Error()))
		return "", errors.New("Could not write the war's files to that folder. Is one of them open, or the folder read only?")
	}
	comm.Logger.Infof("Exported a war to %s", comm.RedactPaths(folder))
	return filepath.Join(folder, files[0].Name), nil
}

// warExportInputOf reads a war for its export
func (a *App) warExportInputOf(id int) (warexport.Input, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing.Load() {
		return warexport.Input{}, errAppClosing
	}
	war, found := a.cache.WarByID(id)
	if !found {
		return warexport.Input{}, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return warexport.Input{}, err
	}
	report := a.buildWarReport(war, sides)
	return warExportInput(report, a.cache.SystemNicknamesOf(), a.version, time.Now()), nil
}

// pickWarExportFolder asks for the parent folder of war exports and remembers it. Empty if the user closed the picker.
func (a *App) pickWarExportFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a folder for war exports. Each war gets its own folder in it.", CanCreateDirectories: true})
	if err != nil {
		return "", fmt.Errorf("Could not open the folder picker: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	if err := a.updateSettings(func(settings *Settings) { settings.WarExportDir = dir }); err != nil {
		return "", errors.New("Could not save the folder in your settings")
	}
	return dir, nil
}

// GetWarExportFolder returns the parent folder of war exports, empty until one is chosen
func (a *App) GetWarExportFolder() string {
	return a.currentSettings().WarExportDir
}

// ChooseWarExportFolder asks for the parent folder of war exports. Cancelling leaves it as it was. It returns the folder now in use.
func (a *App) ChooseWarExportFolder() (folder string, err error) {
	defer recoverAsError(&err)
	a.waitReady()
	if _, err = a.pickWarExportFolder(); err != nil {
		return a.GetWarExportFolder(), err
	}
	return a.GetWarExportFolder(), nil
}

/*
	warExportInput turns a war's report into what the export shows. It only reads its arguments.

Parameters:
  - report: the war's report, with its export figures
  - everywhere: the system nicknames for the whole app, which the war's own win over
  - version: the app's version
  - now: when it is exported

Returns:
  - warexport.Input: for warexport.Build
*/
func warExportInput(report WarReport, everywhere map[int]string, version string, now time.Time) warexport.Input {
	nickname := func(id int) string {
		if name := report.SystemNicknames[id]; name != "" {
			return name
		}
		return everywhere[id]
	}
	input := warexport.Input{
		Name: report.War.Name, FromDate: report.War.FromDate, Until: report.War.UntilDate, SideA: report.War.SideA.Name, SideB: report.War.SideB.Name,
		Exported: now, Version: version,
		A: warExportMeasures(report.A, report.A.ISKLost+report.B.ISKLost > 0), B: warExportMeasures(report.B, report.A.ISKLost+report.B.ISKLost > 0), Others: warExportMeasures(report.Others, false),
		Counts: warexport.Counts{Battles: report.BattleCount, Skirmishes: report.Skirmishes, Ignored: report.IgnoredBattles, Systems: report.SystemCount, Killmails: report.KillmailCount},
	}
	for _, class := range report.Classes {
		input.Classes = append(input.Classes, warexport.ClassCount{Class: class.Class, Count: class.Count})
	}
	for _, reach := range report.Reach {
		input.Reach = append(input.Reach, warexport.ReachLine{Group: reach.Group, Oldest: reach.Oldest.UTC().Format(warDateLayout)})
	}
	for _, day := range report.Days {
		input.Days = append(input.Days, warexport.Day{Date: day.Date, A: warExportDay(day.A), B: warExportDay(day.B)})
	}
	for _, system := range report.Systems {
		row := warexport.System{ID: system.System.ID, Name: system.System.Name, Nickname: nickname(system.System.ID), Class: system.Class, Battles: system.Battles}
		for _, static := range system.System.Statics {
			row.Statics = append(row.Statics, static.Destination)
		}
		if presence, seen := report.Export.Presence[system.System.ID]; seen {
			row.SeenA, row.SeenB, row.LostA, row.LostB = presence.A.Seen, presence.B.Seen, presence.A.FirstLost, presence.B.FirstLost
		}
		input.Systems = append(input.Systems, row)
	}
	for _, battle := range report.Battles {
		if !battle.Ignored {
			input.Battles = append(input.Battles, warExportBattle(battle, nickname(battle.System.ID)))
		}
	}
	input.GroupsA, input.FlownA = warExportPeople(report.People["a"])
	input.GroupsB, input.FlownB = warExportPeople(report.People["b"])
	input.AppearancesA = warExportAppearances(report.Export.Appearances[aggregate.WarSideA])
	input.AppearancesB = warExportAppearances(report.Export.Appearances[aggregate.WarSideB])
	input.AppearancesOthers = warExportAppearances(report.Export.Appearances[aggregate.WarSideNone])
	return input
}

func warExportMeasures(m WarMeasures, hasEfficiency bool) warexport.Measures {
	out := warexport.Measures{ISKLost: m.ISKLost, ISKFights: m.ISKFights, ISKTrash: m.ISKTrash, ISKStructures: m.ISKStructures, ShipsLost: m.ShipsLost, ShipsFights: m.ShipsFights, ShipsTrash: m.ShipsTrash,
		Citadels: []warexport.StructureKind{}, CitadelCount: m.CitadelCount, POS: m.POS, StationRefs: m.StationRefs, RefAttempts: m.RefAttempts, RefAttemptsUnsure: m.RefAttemptsUnsure, Efficiency: m.Efficiency, HasEfficiency: hasEfficiency}
	for _, citadel := range m.Citadels {
		out.Citadels = append(out.Citadels, warexport.StructureKind{Name: citadel.Name, Count: citadel.Count, Icon: citadel.ImageURL})
	}
	return out
}

func warExportDay(m WarMeasures) warexport.DaySide {
	return warexport.DaySide{ISK: m.ISKLost, Ships: m.ShipsLost, Structures: m.CitadelCount + m.POS}
}

func warExportBattleSide(side WarBattleSide) warexport.BattleSide {
	out := warexport.BattleSide{Pilots: side.Pilots, ISKLost: side.ISKLost, ShipsLost: side.ShipsLost, Structures: side.Structures}
	if side.Group != nil {
		out.Group, out.GroupIcon = side.Group.Name, side.Group.IconURL
	}
	return out
}

func warExportBattle(battle WarBattle, nickname string) warexport.Battle {
	out := warexport.Battle{Start: battle.Start, End: battle.End, SystemID: battle.System.ID, System: battle.System.Name, Nickname: nickname, OtherSystems: battle.OtherSystems,
		Class: battle.Class, Pilots: battle.Pilots, Killmails: battle.Killmails, Skirmish: battle.Skirmish, Attacked: len(battle.StationRefs) > 0, Notes: []string{},
		A: warExportBattleSide(battle.A), B: warExportBattleSide(battle.B), Others: warExportBattleSide(battle.Others),
		Links: []warexport.Link{{Label: "zKillboard", URL: battle.ZkillURL}, {Label: "EVE Tools", URL: battle.EveToolsURL}, {Label: "Warbeacon", URL: battle.WarbeaconURL}}}
	for _, note := range battle.Notes {
		out.Notes = append(out.Notes, note.Text)
	}
	for _, ref := range battle.StationRefs {
		out.Marks = append(out.Marks, warExportMark(ref))
	}
	return out
}

// warExportMark is a battle's station chit, in the words of the Battles tab
func warExportMark(ref WarStationRef) warexport.Mark {
	mark := warexport.Mark{Side: ref.Side, Title: "Citadel", Word: "citadel", State: "attacked", At: ref.StructureAt, Notes: []string{}}
	switch ref.Kind {
	case "pos":
		mark.Title, mark.Word = "POS", "POS"
	case "poco":
		mark.Title, mark.Word = "POCO", "POCO"
	}
	switch {
	case ref.Destroyed:
		mark.State = "destroyed"
	case !ref.Attempt:
		mark.State = "reffed"
	case ref.Unsure:
		mark.State = "unsure"
	}
	for _, note := range ref.Notes {
		line := note.Type
		if note.Owner != "" {
			line += " of " + note.Owner
		}
		if note.Lost {
			line += " lost"
		} else {
			line += " fired"
		}
		if note.System != "" {
			line += " in " + note.System
		}
		plural := "s"
		if note.Killmails == 1 {
			plural = ""
		}
		mark.Notes = append(mark.Notes, fmt.Sprintf("%s on %d killmail%s", line, note.Killmails, plural))
	}
	return mark
}

func warExportPeople(people WarSidePeople) ([]warexport.Group, []warexport.Hull) {
	groups := make([]warexport.Group, 0, len(people.Participants))
	for _, group := range people.Participants {
		groups = append(groups, warexport.Group{Name: group.Group.Name, Pilots: group.Count, Icon: group.Group.IconURL})
	}
	hulls := make([]warexport.Hull, 0, len(people.Flown))
	for _, hull := range people.Flown {
		hulls = append(hulls, warexport.Hull{Name: hull.Name, Seen: hull.Seen, Lost: hull.Lost, Icon: hull.ImageURL})
	}
	return groups, hulls
}

func warExportAppearances(groups []aggregate.WarGroupAppearance) []warexport.GroupAppearance {
	out := make([]warexport.GroupAppearance, 0, len(groups))
	for _, group := range groups {
		row := warexport.GroupAppearance{Name: group.Group.Name, Battles: group.Battles}
		for _, corp := range group.Corps {
			row.Corps = append(row.Corps, warexport.CorpAppearance{Name: corp.Corp.Name, Battles: corp.Battles})
		}
		out = append(out, row)
	}
	return out
}
