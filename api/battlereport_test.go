package api_test

import (
	"errors"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestParseBattleReportLink(t *testing.T) {
	for _, text := range []string{
		"https://br.evetools.org/br/6aebcccc8b4155c7de8bb2d8",
		"  br.evetools.org/br/6aebcccc8b4155c7de8bb2d8/  ",
		"https://www.br.evetools.org/br/6aebcccc8b4155c7de8bb2d8?x=1",
	} {
		id, err := api.ParseBattleReportLink(text)
		assert.NoError(t, err, text)
		assert.Equal(t, "6aebcccc8b4155c7de8bb2d8", id, text)
	}

	_, err := api.ParseBattleReportLink("https://br.evetools.org/related/31002067/202611050000")
	assert.ErrorIs(t, err, api.ErrBattleReportNoTeams)

	for _, text := range []string{"", "Jita", "https://br.evetools.org/", "https://br.evetools.org/br/", "https://br.evetools.org/br/a/b",
		"https://br.evetools.org/br/..%2Fx", "https://example.com/br/6aebcccc8b4155c7de8bb2d8", "https://kb.evetools.org/br/6aebcccc8b4155c7de8bb2d8"} {
		_, err := api.ParseBattleReportLink(text)
		assert.ErrorIs(t, err, api.ErrNotBattleReportLink, text)
	}
}

// the report's teams as the site sends them, with both spellings of a killmail, a structure with no pilot, and an NPC corporation
const battleReportJSON = `{"status":"success","teams":[["99000001","corp:98000001","corp:1000170"],["99000002"]],"relateds":[{"kms":[
	{"victim":{"char":1,"corp":98000002,"ally":99000002},"attackers":[{"char":2,"corp":98000003,"ally":99000001},{"char":3,"corp":98000001,"ally":0},{"char":2,"corp":98000003,"ally":99000001}]},
	{"vict":{"char":0,"corp":98000002,"ally":99000002},"atts":[{"char":4,"corp":98000001,"ally":0},{"char":5,"corp":1000170,"ally":0}]}
]}]}`

func TestBattleReportSides(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	mock := CreateMockResposne(battleReportJSON, "GET", "br/composition/abc", 200)
	report, err := api.BattleReportConnection{Client: mock}.GetBattleReport("abc")
	assert.NoError(t, err)
	assert.Equal(t, "br/composition/abc", mock.Endpoint)

	sides, npcCorps := report.Sides()
	assert.Equal(t, 1, npcCorps)
	assert.Equal(t, [][]api.BattleReportGroup{
		{{Type: types.CORPORATION, ID: 98000001, Pilots: 2}, {Type: types.ALLIANCE, ID: 99000001, Pilots: 1}},
		{{Type: types.ALLIANCE, ID: 99000002, Pilots: 1}},
	}, sides)
}

func TestGetBattleReportStatus(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("an error status is a failure", func(t *testing.T) {
		_, err := api.BattleReportConnection{Client: CreateMockResposne(`{"status":"error"}`, "GET", "", 200)}.GetBattleReport("abc")
		assert.ErrorIs(t, err, api.ErrBattleReportFailed)
	})

	t.Run("something that is not its json is a failure", func(t *testing.T) {
		_, err := api.BattleReportConnection{Client: CreateMockResposne(`<html>`, "GET", "", 200)}.GetBattleReport("abc")
		assert.ErrorIs(t, err, api.ErrBattleReportFailed)
	})

	t.Run("a request that fails says so", func(t *testing.T) {
		failure := errors.New("no route")
		_, err := api.BattleReportConnection{Client: &MockApiClient{err: failure}}.GetBattleReport("abc")
		assert.ErrorIs(t, err, failure)
	})
}
