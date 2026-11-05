package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// a group with this many pilots or fewer on a battle report starts unticked, as it is more likely passing by than on the side, and listing it reads all its killmails (user, 2026-11-05)
const (
	battleReportMinAlliancePilots = 3
	battleReportMinCorpPilots     = 5
)

// BattleReportGroupCard is a corporation or alliance on a battle report's team
type BattleReportGroupCard struct {
	Type   string `json:"type"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Pilots int    `json:"pilots"`
	Ticked bool   `json:"ticked"` // whether it starts ticked
}

// BattleReportCard is a battle report's sides, for the user to choose from
type BattleReportCard struct {
	ID       string                    `json:"id"`
	Teams    [][]BattleReportGroupCard `json:"teams"`
	NPCCorps int                       `json:"npcCorps"` // how many NPC corporations were left out
}

// BattleReportChoice is what to save for one side: its groups, added to a saved coalition or a new one
type BattleReportChoice struct {
	CoalitionID int            `json:"coalitionId"` // 0 for a new coalition
	NewName     string         `json:"newName"`     // a new coalition's name
	Groups      []WarAddedCard `json:"groups"`
}

// BattleReportSides are the two sides as saved
type BattleReportSides struct {
	A WarSideCard `json:"a"`
	B WarSideCard `json:"b"`
}

// battleReportError says in plain words why a battle report could not be read
func battleReportError(err error) error {
	switch {
	case errors.Is(err, api.ErrNotBattleReportLink):
		return errors.New("That is not a link to an EVE Tools battle report, which looks like https://br.evetools.org/br/6aebcccc8b4155c7de8bb2d8")
	case errors.Is(err, api.ErrBattleReportNoTeams):
		return errors.New("That is an automatic battle report, which has no saved sides. Only a saved report, a br.evetools.org/br/ link, has them.")
	case errors.Is(err, api.ErrBattleReportProcessing):
		return errors.New("EVE Tools is still putting that battle report together. Try again in a moment.")
	case errors.Is(err, api.ErrBattleReportFailed):
		return errors.New("EVE Tools could not give that battle report. Check the link, or enter the sides by hand.")
	case api.IsNotFound(err):
		return errors.New("EVE Tools has no such battle report")
	}
	return fmt.Errorf("Could not read the battle report: %s", api.FailureReason(err))
}

// battleReportTicked is whether a group starts ticked
func battleReportTicked(group api.BattleReportGroup) bool {
	if group.Type == types.CORPORATION {
		return group.Pilots > battleReportMinCorpPilots
	}
	return group.Pilots > battleReportMinAlliancePilots
}

/*
	ReadBattleReport reads the sides of a saved EVE Tools battle report, with their names, and saves nothing. It does not wait for a running search.

Parameters:
  - link: a link to the report, ie https://br.evetools.org/br/6aebcccc8b4155c7de8bb2d8

Returns:
  - BattleReportCard: its teams, most pilots first, without NPC corporations
  - error: a user readable message if it is not such a link, can not be read, or has fewer than two sides
*/
func (a *App) ReadBattleReport(link string) (card BattleReportCard, err error) {
	defer recoverAsError(&err)

	id, err := api.ParseBattleReportLink(link)
	if err != nil {
		return card, battleReportError(err)
	}
	report, err := a.battleReports.GetBattleReport(id)
	if err != nil {
		return card, battleReportError(err)
	}
	sides, npcCorps := report.Sides()

	ids := []int{}
	for _, side := range sides {
		for _, group := range side {
			ids = append(ids, group.ID)
		}
	}
	names, err := a.clients.ESIClient.GetNamesFromEveIDs(ids)
	if err != nil {
		return card, fmt.Errorf("Could not read the names of that battle report's groups: %s", api.FailureReason(err))
	}

	card = BattleReportCard{ID: id, Teams: [][]BattleReportGroupCard{}, NPCCorps: npcCorps}
	for _, side := range sides {
		if len(side) == 0 {
			continue
		}
		team := []BattleReportGroupCard{}
		for _, group := range side {
			team = append(team, BattleReportGroupCard{Type: group.Type.String(), ID: group.ID, Pilots: group.Pilots, Ticked: battleReportTicked(group),
				Name: warGroupName(WarAddedCard{Type: group.Type.String(), ID: group.ID, Name: names[group.ID]})})
		}
		card.Teams = append(card.Teams, team)
	}
	if len(card.Teams) < 2 {
		return BattleReportCard{}, errors.New("That battle report does not have two sides to read")
	}
	return card, nil
}

// battleReportPlan is a choice's coalition with its groups added, not yet saved. Nothing in it is ever taken out: a group unticked stays in a coalition it was in.
func (a *App) battleReportPlan(choice BattleReportChoice, label string) (retrieval.Coalition, error) {
	coalition := retrieval.Coalition{}
	if choice.CoalitionID != 0 {
		var found bool
		if coalition, found = a.cache.CoalitionByID(choice.CoalitionID); !found {
			return coalition, errNoCoalition
		}
	} else {
		name, err := a.validCoalitionName(choice.NewName, 0)
		if err != nil {
			return coalition, fmt.Errorf("Side %s: %w", label, err)
		}
		coalition.Name = name
		if len(choice.Groups) == 0 {
			return coalition, fmt.Errorf("Side %s: tick at least one group for the new coalition %s", label, name)
		}
	}

	added := []retrieval.CoalitionMember{}
	for _, group := range choice.Groups {
		if group.Type != types.CORPORATION.String() && group.Type != types.ALLIANCE.String() || group.ID <= 0 {
			return coalition, errors.New("Only a corporation or an alliance can be in a coalition")
		}
		member := retrieval.CoalitionMember{Type: group.Type, ID: group.ID, Name: warGroupName(group)}
		sameGroup := func(other retrieval.CoalitionMember) bool { return other.Type == member.Type && other.ID == member.ID }
		if !slices.ContainsFunc(added, sameGroup) && !slices.ContainsFunc(a.cache.CoalitionGroups(coalition, true), func(listed retrieval.TargetGroup) bool {
			return listed.Type.String() == member.Type && listed.ID == member.ID
		}) {
			added = append(added, member)
		}
	}
	coalition.Members = append(coalition.Members, added...)

	// a group that is the affiliate of another now in the coalition comes in through it, whichever was added first
	for _, member := range added {
		same := func(other retrieval.CoalitionMember) bool { return other == member }
		without := coalition
		without.Members = slices.DeleteFunc(slices.Clone(coalition.Members), same)
		if slices.ContainsFunc(a.cache.CoalitionGroups(without, true), func(listed retrieval.TargetGroup) bool {
			return listed.Type.String() == member.Type && listed.ID == member.ID
		}) {
			coalition = without
		}
	}
	return coalition, nil
}

// saveBattleReportChoices saves both sides' coalitions, or neither if either is refused
func (a *App) saveBattleReportChoices(choiceA BattleReportChoice, choiceB BattleReportChoice) (retrieval.Coalition, retrieval.Coalition, error) {
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	none := retrieval.Coalition{}
	coalitionA, err := a.battleReportPlan(choiceA, "A")
	if err != nil {
		return none, none, err
	}
	coalitionB, err := a.battleReportPlan(choiceB, "B")
	if err != nil {
		return none, none, err
	}
	if coalitionA.ID != 0 && coalitionA.ID == coalitionB.ID || strings.EqualFold(coalitionA.Name, coalitionB.Name) {
		return none, none, fmt.Errorf("%s can not be on both sides", coalitionA.Name)
	}
	for _, member := range coalitionA.Members {
		if slices.ContainsFunc(coalitionB.Members, func(other retrieval.CoalitionMember) bool { return other.Type == member.Type && other.ID == member.ID }) {
			return none, none, fmt.Errorf("%s is listed on both sides", member.Name)
		}
	}
	return a.cache.SaveCoalition(coalitionA), a.cache.SaveCoalition(coalitionB), nil
}

/*
	SaveBattleReportSides saves a battle report's two sides as coalitions, for the war setup window to choose. Groups already in a coalition stay whether ticked or not. It does not wait for a running search.

Parameters:
  - choiceA, choiceB: what to save for each side

Returns:
  - BattleReportSides: the coalitions, as sides for the form
  - error: a user readable message: a name not allowed, a coalition gone, or a group on both sides. Nothing is saved.
*/
func (a *App) SaveBattleReportSides(choiceA BattleReportChoice, choiceB BattleReportChoice) (sides BattleReportSides, err error) {
	defer recoverAsError(&err)
	coalitionA, coalitionB, err := a.saveBattleReportChoices(choiceA, choiceB)
	if err != nil {
		return sides, err
	}
	a.saveCacheQuietly()
	card := func(coalition retrieval.Coalition) WarSideCard {
		side := WarSideCard{Type: coalitionType, ID: coalition.ID, Name: coalition.Name}
		if groups, err := a.cache.WarSideGroups(retrieval.WarSide{Type: coalitionType, ID: coalition.ID, Name: coalition.Name}); err == nil {
			side.Count = len(groups)
		}
		return side
	}
	return BattleReportSides{A: card(coalitionA), B: card(coalitionB)}, nil
}

/*
	SaveBattleReportToWar saves a battle report's two sides as coalitions and makes them the war's sides, then builds the report again from what the war holds.
	Its next run reads every list. It takes a.mu.

Parameters:
  - id: the war
  - choiceA, choiceB: what to save for each side. The frontend sends a side's corporation or alliance among its groups when the side was one, so it stays on it.

Returns:
  - WarReport: the report after the change
  - error: a user readable message: the war gone, a name not allowed, a coalition gone, or a group on both sides, none of which save anything. Should a coalition be removed in the moment after it is saved, the coalitions stay saved and the war does not change.
*/
func (a *App) SaveBattleReportToWar(id int, choiceA BattleReportChoice, choiceB BattleReportChoice) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	a.warMu.Lock()
	defer a.warMu.Unlock()

	war, found := a.cache.WarByID(id)
	if !found {
		return report, errNoWar
	}
	coalitionA, coalitionB, err := a.saveBattleReportChoices(choiceA, choiceB)
	if err != nil {
		return report, err
	}

	oldName := war.Name
	if war.Name == war.SideA.Name+" vs "+war.SideB.Name {
		name := coalitionA.Name + " vs " + coalitionB.Name
		if len([]rune(name)) > maxCoalitionName {
			name = string([]rune(name)[:maxCoalitionName])
		}
		if other, taken := a.cache.WarNamed(name); !taken || other.ID == war.ID {
			war.Name = name
		}
	}
	war.SideA = retrieval.WarSide{Type: coalitionType, ID: coalitionA.ID, Name: coalitionA.Name}
	war.SideB = retrieval.WarSide{Type: coalitionType, ID: coalitionB.ID, Name: coalitionB.Name}

	sides, err := a.warSides(war)
	if err != nil {
		return report, err
	}
	// a group now listed on a side is no longer added by hand, or ignored
	listed := func(group retrieval.WarAddedGroup) bool {
		return slices.ContainsFunc(slices.Concat(sides.A, sides.B), func(other retrieval.TargetGroup) bool {
			return other.Type.String() == group.Type && other.ID == group.ID
		})
	}
	war.AddedA = slices.DeleteFunc(slices.Clone(war.AddedA), listed)
	war.AddedB = slices.DeleteFunc(slices.Clone(war.AddedB), listed)
	war.Ignored = slices.DeleteFunc(slices.Clone(war.Ignored), listed)
	war = a.cache.SaveWar(war)
	sides.AddedA, sides.AddedB, sides.Ignored = war.AddedA, war.AddedB, war.Ignored
	if oldName != war.Name {
		a.renameRecentWar(war.ID, oldName, war.Name)
	}
	a.saveCacheQuietly()

	a.lastReport, a.lastTarget = nil, nil
	return a.buildWarReport(war, sides), nil
}
