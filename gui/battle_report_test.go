package gui

import (
	"encoding/json"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// fakeBattleReports answers every report with the same JSON, or an error
type fakeBattleReports struct {
	body string
	err  error
}

func (f fakeBattleReports) GetBattleReport(id string) (*api.BattleReport, error) {
	if f.err != nil {
		return nil, f.err
	}
	report := &api.BattleReport{}
	return report, json.Unmarshal([]byte(f.body), report)
}

// pilots makes attackers of a group: its alliance or corp set, each a new character starting at first
func pilots(first int, count int, corp int, ally int) string {
	list := ""
	for at := range count {
		if at > 0 {
			list += ","
		}
		pilot, _ := json.Marshal(api.BattleReportPilot{Char: first + at, Corp: corp, Ally: ally})
		list += string(pilot)
	}
	return list
}

func battleCard(group int, name string) WarAddedCard {
	return WarAddedCard{Type: "ALLIANCE", ID: group, Name: name}
}

func TestReadBattleReport(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("groups start ticked over the cut offs, an alliance over 3 pilots and a corp over 5, and NPC corps are left out", func(t *testing.T) {
		app := newIntelApp(t)
		app.battleReports = fakeBattleReports{body: `{"status":"success",
			"teams":[["99013330","99013580","corp:98744491","corp:98000009","corp:1000170"],["99003581"]],
			"relateds":[{"kms":[{"attackers":[` + pilots(100, 4, 1, 99013330) + `,` + pilots(200, 3, 2, 99013580) + `,` + pilots(300, 6, 98744491, 0) + `,` + pilots(400, 5, 98000009, 0) + `]}]}]}`}

		card, err := app.ReadBattleReport("https://br.evetools.org/br/abc123")
		assert.NoError(t, err)
		assert.Equal(t, "abc123", card.ID)
		assert.Equal(t, 1, card.NPCCorps)
		assert.Equal(t, []BattleReportGroupCard{
			{Type: "CORPORATION", ID: 98744491, Name: "URSA PRIMUS", Pilots: 6, Ticked: true},
			{Type: "CORPORATION", ID: 98000009, Name: "corporation 98000009", Pilots: 5, Ticked: false},
			{Type: "ALLIANCE", ID: 99013330, Name: "URSA.", Pilots: 4, Ticked: true},
			{Type: "ALLIANCE", ID: 99013580, Name: "High Bear Nation", Pilots: 3, Ticked: false},
		}, card.Teams[0])
		assert.Equal(t, "Fraternity.", card.Teams[1][0].Name)
	})

	t.Run("what can not be read says why", func(t *testing.T) {
		app := newIntelApp(t)
		app.battleReports = fakeBattleReports{body: `{"status":"success","teams":[["99013330"]]}`}

		_, err := app.ReadBattleReport("https://zkillboard.com/alliance/1/")
		assert.ErrorContains(t, err, "not a link to an EVE Tools battle report")
		_, err = app.ReadBattleReport("https://br.evetools.org/related/31002067/202611050000")
		assert.ErrorContains(t, err, "automatic battle report")
		_, err = app.ReadBattleReport("https://br.evetools.org/br/abc123")
		assert.ErrorContains(t, err, "two sides")

		app.battleReports = fakeBattleReports{err: api.ErrBattleReportProcessing}
		_, err = app.ReadBattleReport("https://br.evetools.org/br/abc123")
		assert.ErrorContains(t, err, "still putting")
	})
}

func TestSaveBattleReportSides(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	ursa, bear, frat := battleCard(99013330, "URSA."), battleCard(99013580, "High Bear Nation"), battleCard(99003581, "Fraternity.")

	t.Run("makes new coalitions", func(t *testing.T) {
		app := newIntelApp(t)
		sides, err := app.SaveBattleReportSides(BattleReportChoice{NewName: "Side One", Groups: []WarAddedCard{ursa}}, BattleReportChoice{NewName: "Side Two", Groups: []WarAddedCard{frat, bear}})
		assert.NoError(t, err)
		assert.Equal(t, "Side One", sides.A.Name)
		assert.Equal(t, coalitionType, sides.A.Type)
		assert.Equal(t, 2, sides.B.Count)
	})

	t.Run("adds to a saved coalition and takes nothing out of it, unticked groups staying", func(t *testing.T) {
		app := newIntelApp(t)
		saved := app.cache.SaveCoalition(retrieval.Coalition{Name: "Old", Note: "kept", Members: []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: 99013580, Name: "High Bear Nation"}}})

		_, err := app.SaveBattleReportSides(BattleReportChoice{CoalitionID: saved.ID, Groups: []WarAddedCard{ursa}}, BattleReportChoice{NewName: "New", Groups: []WarAddedCard{frat}})
		assert.NoError(t, err)
		found, _ := app.cache.CoalitionByID(saved.ID)
		assert.Equal(t, "kept", found.Note)
		assert.Equal(t, []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: 99013580, Name: "High Bear Nation"}, {Type: "ALLIANCE", ID: 99013330, Name: "URSA."}}, found.Members)
	})

	t.Run("a group already in it is not added twice, nor one that comes in as an affiliate, whichever is first", func(t *testing.T) {
		app := newIntelApp(t)
		_, err := app.SaveHolding(99013330, "ALLIANCE", "URSA.", "High Bear Nation", "ALLIANCE", "")
		assert.NoError(t, err)

		sides, err := app.SaveBattleReportSides(BattleReportChoice{NewName: "One", Groups: []WarAddedCard{bear, ursa, ursa}}, BattleReportChoice{NewName: "Two", Groups: []WarAddedCard{frat}})
		assert.NoError(t, err)
		found, _ := app.cache.CoalitionByID(sides.A.ID)
		assert.Equal(t, []retrieval.CoalitionMember{{Type: "ALLIANCE", ID: 99013330, Name: "URSA."}}, found.Members)
		assert.Equal(t, 2, sides.A.Count)
	})

	t.Run("a refusal saves neither side", func(t *testing.T) {
		app := newIntelApp(t)
		first := BattleReportChoice{NewName: "One", Groups: []WarAddedCard{ursa}}
		for _, second := range []BattleReportChoice{
			{NewName: "", Groups: []WarAddedCard{frat}},
			{NewName: "Two"},
			{NewName: "one", Groups: []WarAddedCard{frat}},
			{NewName: "Two", Groups: []WarAddedCard{ursa}},
			{CoalitionID: -1},
			{NewName: "Two", Groups: []WarAddedCard{{Type: "PILOT", ID: 1, Name: "A pilot"}}},
		} {
			_, err := app.SaveBattleReportSides(first, second)
			assert.Error(t, err)
			coalitions, _ := app.GetCoalitions()
			assert.Empty(t, coalitions)
		}
	})
}

func TestSaveBattleReportToWar(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	ursa, bear, frat := battleCard(99013330, "URSA."), battleCard(99013580, "High Bear Nation"), battleCard(99003581, "Fraternity.")

	t.Run("the war's sides become the coalitions, its default name follows, and a group now listed is no longer added or ignored", func(t *testing.T) {
		app := newIntelApp(t)
		war := app.cache.SaveWar(retrieval.War{Name: "URSA. vs Fraternity.",
			SideA:   retrieval.WarSide{Type: "ALLIANCE", ID: 99013330, Name: "URSA."},
			SideB:   retrieval.WarSide{Type: "ALLIANCE", ID: 99003581, Name: "Fraternity."},
			AddedA:  []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: 99013580, Name: "High Bear Nation"}},
			Ignored: []retrieval.WarAddedGroup{{Type: "CORPORATION", ID: 98744491, Name: "URSA PRIMUS"}},
		})

		report, err := app.SaveBattleReportToWar(war.ID,
			BattleReportChoice{NewName: "One", Groups: []WarAddedCard{ursa, bear}},
			BattleReportChoice{NewName: "Two", Groups: []WarAddedCard{frat, {Type: "CORPORATION", ID: 98744491, Name: "URSA PRIMUS"}}})
		assert.NoError(t, err)
		assert.Equal(t, "One vs Two", report.War.Name)
		assert.Equal(t, coalitionType, report.War.SideA.Type)
		assert.Equal(t, 2, report.War.SideA.Count)
		assert.Empty(t, report.War.AddedA)
		assert.Empty(t, report.War.Ignored)
	})

	t.Run("a name the user gave stays, and a war gone is refused with nothing saved", func(t *testing.T) {
		app := newIntelApp(t)
		war := app.cache.SaveWar(retrieval.War{Name: "The Big One",
			SideA: retrieval.WarSide{Type: "ALLIANCE", ID: 99013330, Name: "URSA."},
			SideB: retrieval.WarSide{Type: "ALLIANCE", ID: 99003581, Name: "Fraternity."}})
		report, err := app.SaveBattleReportToWar(war.ID, BattleReportChoice{NewName: "One", Groups: []WarAddedCard{ursa}}, BattleReportChoice{NewName: "Two", Groups: []WarAddedCard{frat}})
		assert.NoError(t, err)
		assert.Equal(t, "The Big One", report.War.Name)

		_, err = app.SaveBattleReportToWar(-5, BattleReportChoice{NewName: "Three", Groups: []WarAddedCard{ursa}}, BattleReportChoice{NewName: "Four", Groups: []WarAddedCard{frat}})
		assert.ErrorIs(t, err, errNoWar)
		_, named := app.cache.CoalitionNamed("Three")
		assert.False(t, named)
	})
}
