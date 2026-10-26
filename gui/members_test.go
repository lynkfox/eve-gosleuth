// the roster is unexported, so this tests from inside package gui
package gui

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// pagedJoins serves a corp's join records split into pages, recording which pages were asked for
type pagedJoins struct {
	api.EveWhoClient
	pages [][]api.EveWhoJoin
	asked *[]int
	err   error
}

func (m pagedJoins) GetCorpJoined(corporationID int, page int) (*api.EveWhoJoinList, error) {
	*m.asked = append(*m.asked, page)
	if m.err != nil {
		return nil, m.err
	}
	return &api.EveWhoJoinList{Characters: m.pages[page-1], Pagination: api.EveWhoPagination{Page: page, Pages: len(m.pages), HasNext: page < len(m.pages)}}, nil
}

// joins makes a page of join records for these pilot ids, newest first. A negative id is a pilot who has since left.
func joins(ids ...int) []api.EveWhoJoin {
	list := []api.EveWhoJoin{}
	for index, id := range ids {
		join := api.EveWhoJoin{CharacterID: id, StartDate: api.EveWhoTime{Time: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -index)}}
		if id < 0 {
			join.CharacterID = -id
			join.EndDate = &api.EveWhoTime{Time: time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)}
		}
		list = append(list, join)
	}
	return list
}

// memberESI answers the member detail lookups: every pilot was born Apr 18th 2023, sat 30 days in an NPC corp, then joined URSA PRIMUS on Jul 9th 2026
type memberESI struct {
	api.ESIClient
}

func (m memberESI) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	if idNumber == 666 {
		return &api.ESICharacter{CharacterID: idNumber, Name: api.DELETED_CHARACTER_NAME, Deleted: true}, nil
	}
	return &api.ESICharacter{CharacterID: idNumber, Name: fmt.Sprintf("Pilot %d", idNumber), CorporationID: 98744491, Birthday: time.Date(2023, 4, 18, 4, 16, 0, 0, time.UTC)}, nil
}

func (m memberESI) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "URSA PRIMUS"}, nil
}

func (m memberESI) GetCorporationHistory(characterID int) (*api.ESICorporationHistory, error) {
	return &api.ESICorporationHistory{CharacterID: characterID, Records: []api.ESICorporationHistoryItem{
		{CorporationID: 98744491, RecordID: 3, StartDate: time.Date(2026, 7, 9, 1, 22, 0, 0, time.UTC)},
		{CorporationID: 1000045, RecordID: 2, StartDate: time.Date(2026, 6, 9, 1, 22, 0, 0, time.UTC)},
	}}, nil
}

// allianceMemberESI is memberESI with URSA PRIMUS in the URSA. alliance
type allianceMemberESI struct {
	memberESI
}

func (m allianceMemberESI) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "URSA PRIMUS", AllianceID: 99013330}, nil
}

// allianceCorpsESI answers an alliance's corporations: corp 1 joined last month, corp 2 two years ago, corp 3's history has no record of the alliance
type allianceCorpsESI struct {
	memberESI
}

func (m allianceCorpsESI) GetAllianceCorporations(allianceID int) (*api.ESIAllianceCorporations, error) {
	return &api.ESIAllianceCorporations{AllianceID: allianceID, CorporationIDs: []int{2, 3, 1}}, nil
}

func (m allianceCorpsESI) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: fmt.Sprintf("Corp %c", 'A'+idNumber-1), MemberCount: idNumber * 10}, nil
}

func (m allianceCorpsESI) GetAllianceHistory(corporationID int) (*api.ESIAllianceHistory, error) {
	joined := map[int]time.Time{1: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), 2: time.Date(2024, 10, 25, 0, 0, 0, 0, time.UTC)}
	history := &api.ESIAllianceHistory{CorporationID: corporationID}
	if start, ok := joined[corporationID]; ok {
		history.Records = []api.ESIAllianceHistoryItem{{AllianceID: 99013330, StartDate: start}, {StartDate: start.AddDate(-1, 0, 0)}}
	}
	return history, nil
}

func rowNames(rows []MemberRow) []string {
	names := []string{}
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}

func rowIDs(rows []MemberRow) []int {
	ids := []int{}
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestRoster(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	ursa := &retrieval.Alliance{AllianceID: 99013330}
	inUrsaPrimus := &retrieval.Corporation{CorporationID: 98744491, Alliance: ursa}
	elsewhere := &retrieval.Corporation{CorporationID: 98000009}
	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }
	seen := map[int]aggregate.PilotSeen{
		101: {Pilot: &retrieval.Pilot{CharacterID: 101, Name: "Bob", Corporation: inUrsaPrimus}, LastSeen: day(20)},
		102: {Pilot: &retrieval.Pilot{CharacterID: 102, Name: "Cat", Corporation: inUrsaPrimus}, LastSeen: day(24)},
		7:   {Pilot: &retrieval.Pilot{CharacterID: 7, Name: "Left", Corporation: elsewhere}, LastSeen: day(25)},
		587: {Pilot: &retrieval.Pilot{CharacterID: 587, NPC: true}, LastSeen: day(25)},
	}

	t.Run("a corp's current members seen on killmails come first, most recent first, leaving out pilots who left and NPCs", func(t *testing.T) {
		list := newRoster(98744491, types.CORPORATION, seen, 0)

		assert.Equal(t, []string{"Cat", "Bob"}, rowNames(list.seen))
		assert.Equal(t, day(24), *list.seen[0].LastSeen)
	})

	t.Run("an alliance's list ignores the killmails", func(t *testing.T) {
		assert.Empty(t, newRoster(99013330, types.ALLIANCE, seen, 0).seen)
	})

	t.Run("the rest of a corp come from EveWho's joins newest first, leaving out who has left, the seen members, and repeats, reading pages only as needed", func(t *testing.T) {
		asked := []int{}
		eveWho := pagedJoins{pages: [][]api.EveWhoJoin{joins(201, -202, 101, 203), joins(201, 204)}, asked: &asked}
		list := newRoster(98744491, types.CORPORATION, seen, 0)

		list.readUpTo(3, eveWho)
		assert.Equal(t, []int{1}, asked, "page 2 is not read until it is needed")
		assert.Equal(t, []int{102, 101, 201, 203}, rowIDs(list.rows(0, 10)))

		list.readUpTo(10, eveWho)
		assert.Equal(t, []int{1, 2}, asked)
		assert.Equal(t, []int{102, 101, 201, 203, 204}, rowIDs(list.rows(0, 10)), "201 rejoining is listed once")
		assert.False(t, list.moreOnEveWho())
	})

	t.Run("when EveWho is down the list stops where it got to", func(t *testing.T) {
		asked := []int{}
		list := newRoster(98744491, types.CORPORATION, seen, 0)

		list.readUpTo(50, pagedJoins{asked: &asked, err: errors.New("down")})
		list.readUpTo(50, pagedJoins{asked: &asked, err: errors.New("down")})

		assert.Equal(t, []string{"Cat", "Bob"}, rowNames(list.rows(0, 50)))
		assert.Equal(t, []int{1}, asked, "EveWho is not asked again after it fails")
	})
}

func TestGetMembers(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	newApp := func(esi api.ESIClient, eveWho api.EveWhoClient, targetID int, targetType types.EntityType, memberCount int) *App {
		app := &App{clients: retrieval.CollectionClientPool{ESIClient: esi, EveWhoClient: eveWho}}
		app.setRoster(newRoster(targetID, targetType, map[int]aggregate.PilotSeen{}, memberCount))
		return app
	}

	t.Run("a corp's rows come with their details, named from the ESI", func(t *testing.T) {
		app := newApp(memberESI{}, pagedJoins{pages: [][]api.EveWhoJoin{joins(201, 202, 203)}, asked: &[]int{}}, 98744491, types.CORPORATION, 3)

		page, err := app.GetMembers(98744491, "CORPORATION", 1, 2)

		assert.NoError(t, err)
		assert.Equal(t, []string{"Pilot 202", "Pilot 203"}, rowNames(page.Rows))
		assert.Equal(t, 3, page.Total)
		assert.False(t, page.HasMore)
		assert.Empty(t, page.Corporations)
		row := page.Rows[0]
		assert.Equal(t, "https://images.evetech.net/characters/202/portrait?size=32", row.ImageURL)
		assert.Regexp(t, `^Apr 18th, 2023 \(`, row.Born)
		assert.Equal(t, "1m", row.NPCTime)
		assert.False(t, row.Left)
		assert.Nil(t, row.LastSeen)
	})

	t.Run("while EveWho has more, the corp's member count numbers the pages", func(t *testing.T) {
		app := newApp(memberESI{}, pagedJoins{pages: [][]api.EveWhoJoin{joins(201, 202), joins(203)}, asked: &[]int{}}, 98744491, types.CORPORATION, 662)

		page, _ := app.GetMembers(98744491, "CORPORATION", 0, 2)

		assert.True(t, page.HasMore)
		assert.Equal(t, 662, page.Total)
	})

	t.Run("an alliance's rows are its corporations, newest to the alliance first", func(t *testing.T) {
		app := newApp(allianceCorpsESI{}, pagedJoins{asked: &[]int{}}, 99013330, types.ALLIANCE, 0)

		page, err := app.GetMembers(99013330, "ALLIANCE", 0, 50)

		assert.NoError(t, err)
		assert.Empty(t, page.Rows)
		names := []string{}
		for _, corp := range page.Corporations {
			names = append(names, corp.Name)
		}
		assert.Equal(t, []string{"Corp A", "Corp B", "Corp C"}, names, "corp C has no known join date, so it is last")
		assert.Equal(t, 10, page.Corporations[0].MemberCount)
		assert.Equal(t, "https://images.evetech.net/corporations/1/logo?size=32", page.Corporations[0].IconURL)
		assert.NotEqual(t, "Unavailable", page.Corporations[1].TimeInAlliance)
		assert.Equal(t, "Unavailable", page.Corporations[2].TimeInAlliance)
		assert.Equal(t, 3, page.Total)
		assert.False(t, page.HasMore)

		second, _ := app.GetMembers(99013330, "ALLIANCE", 2, 50)
		assert.Equal(t, "Corp C", second.Corporations[0].Name)
	})

	t.Run("a deleted member says so", func(t *testing.T) {
		app := newApp(memberESI{}, pagedJoins{pages: [][]api.EveWhoJoin{joins(666)}, asked: &[]int{}}, 98744491, types.CORPORATION, 1)

		page, _ := app.GetMembers(98744491, "CORPORATION", 0, 1)

		assert.Equal(t, "Deleted", page.Rows[0].Born)
	})

	t.Run("a request for another target's members is refused", func(t *testing.T) {
		app := newApp(memberESI{}, pagedJoins{asked: &[]int{}}, 98744491, types.CORPORATION, 0)

		_, err := app.GetMembers(99013330, "ALLIANCE", 0, 50)

		assert.Error(t, err)
	})

	t.Run("at most three pages are looked up at once", func(t *testing.T) {
		ids := []int{}
		for index := range 400 {
			ids = append(ids, 1000+index)
		}
		app := newApp(memberESI{}, pagedJoins{pages: [][]api.EveWhoJoin{joins(ids...)}, asked: &[]int{}}, 98744491, types.CORPORATION, 400)

		page, _ := app.GetMembers(98744491, "CORPORATION", 0, 1000)

		assert.Len(t, page.Rows, 150)
		assert.True(t, page.HasMore)
	})
}

func TestMemberDetails(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	now := time.Date(2026, 10, 9, 1, 22, 0, 0, time.UTC)

	t.Run("time in corp counts from joining the current corp", func(t *testing.T) {
		row := memberDetails(MemberRow{ID: 1}, 98744491, types.CORPORATION, memberESI{}, now)
		assert.Equal(t, "3m 2d", row.TimeInCorp)
	})

	t.Run("a member EveWho still lists but the ESI has in another corp has left", func(t *testing.T) {
		assert.True(t, memberDetails(MemberRow{ID: 1}, 98000009, types.CORPORATION, memberESI{}, now).Left)
		assert.False(t, memberDetails(MemberRow{ID: 1}, 98744491, types.CORPORATION, memberESI{}, now).Left)
	})

	t.Run("for an alliance, leaving means their corp is no longer in it", func(t *testing.T) {
		assert.True(t, memberDetails(MemberRow{ID: 1}, 99013330, types.ALLIANCE, memberESI{}, now).Left, "memberESI's corp has no alliance")
		assert.False(t, memberDetails(MemberRow{ID: 1}, 99013330, types.ALLIANCE, allianceMemberESI{}, now).Left)
	})
}
