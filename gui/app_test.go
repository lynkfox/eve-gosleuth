// buildKillmailCards is unexported, so this tests from inside package gui
package gui

import (
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

func TestBuildKillmailCardGroups(t *testing.T) {
	ursa := &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."}
	killmail := &retrieval.Killmail{
		KillmailID:     139416795,
		KillmailTime:   time.Date(2026, 10, 25, 8, 58, 50, 0, time.UTC),
		DroppedValue:   1200000,
		DroppableValue: 2400000,
		Labels:         []string{"tz:au", "loc:w-space"},
		Victim: &retrieval.KillmailVictim{
			Ship:       &retrieval.Item{ItemID: 12200, Name: "Mobile Depot"},
			CorpAtKill: &retrieval.Corporation{CorporationID: 98828090, Name: "Wormhole Campus"},
		},
		Attackers: []*retrieval.KillmailAttacker{{
			Pilot:          &retrieval.Pilot{CharacterID: 2121909368, Name: "Astrid"},
			CorpAtKill:     &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS"},
			AllianceAtKill: ursa,
			FinalBlow:      true,
		}},
	}
	target := &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE}

	card := buildKillmailCards([]*retrieval.Killmail{killmail}, target)[0]

	t.Run("the victim corp has its logo, and no alliance logo when it had no alliance", func(t *testing.T) {
		assert.Equal(t, "Wormhole Campus", card.VictimCorp)
		assert.Equal(t, "https://images.evetech.net/corporations/98828090/logo?size=32", card.VictimCorpIcon)
		assert.Empty(t, card.VictimAlliance)
		assert.Empty(t, card.VictimAllianceIcon)
	})

	t.Run("the final blow has its corp and alliance, each with a logo", func(t *testing.T) {
		assert.Equal(t, []string{"URSA PRIMUS", "URSA."}, []string{card.FinalBlowCorp, card.FinalBlowAlliance})
		assert.Equal(t, "https://images.evetech.net/corporations/98744491/logo?size=32", card.FinalBlowCorpIcon)
		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", card.FinalBlowAllianceIcon)
	})

	t.Run("the card shows what actually dropped, not what could have", func(t *testing.T) {
		assert.Equal(t, 1200000.0, card.DroppedValue)
	})

	t.Run("labels are still collected for the card, the frontend just does not show them", func(t *testing.T) {
		assert.Equal(t, []string{"tz:au", "loc:w-space"}, card.Labels)
	})
}

func TestRowGroupIcons(t *testing.T) {
	ursa := &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."}
	primus := &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS", Alliance: ursa}

	t.Run("fleetmate rows carry their corp and alliance logos from EVE's image server", func(t *testing.T) {
		fleetmate := buildFleetmates([]aggregate.PilotSeen{{Pilot: &retrieval.Pilot{CharacterID: 2121909368, Name: "Astrid", Corporation: primus}}})[0]

		assert.Equal(t, "https://images.evetech.net/corporations/98744491/logo?size=32", fleetmate.CorporationIcon)
		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", fleetmate.AllianceIcon)
	})

	t.Run("corp rows carry their alliance logo, and none when the corp has no alliance", func(t *testing.T) {
		killmailTime := []time.Time{time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)}
		corporations := buildCorporationSummaries([]aggregate.CorporationActivity{
			{Corporation: primus, Alliance: ursa, Activity: killmailTime},
			{Corporation: &retrieval.Corporation{CorporationID: 98828090, Name: "Wormhole Campus"}, Activity: killmailTime},
		})

		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", corporations[0].AllianceIcon)
		assert.Empty(t, corporations[1].AllianceIcon)
	})
}

func TestBuildCapitalShips(t *testing.T) {
	t.Run("each capital has its image and pilots, and none is an empty list", func(t *testing.T) {
		ursa := &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."}
		target := &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE}
		ships := buildCapitalShips([]aggregate.CapitalShipPilots{{
			Ship: &retrieval.Item{ItemID: 19722, Name: "Naglfar"},
			Pilots: []*retrieval.Pilot{
				{CharacterID: 2, Name: "Amy", Corporation: &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS", Alliance: ursa}},
				{CharacterID: 1, Name: "zed", Corporation: &retrieval.Corporation{CorporationID: 98000009, Name: "Elsewhere Inc."}},
				{CharacterID: 3, Name: api.DELETED_CHARACTER_NAME},
			},
		}}, target)

		assert.Equal(t, []CapitalShip{{ID: 19722, Name: "Naglfar", ImageURL: "https://images.evetech.net/types/19722/icon?size=64",
			Pilots: []CapitalPilot{
				{ID: 2, Name: "Amy", Corporation: "URSA PRIMUS", Alliance: "URSA.", CorporationIcon: "https://images.evetech.net/corporations/98744491/logo?size=32", AllianceIcon: "https://images.evetech.net/alliances/99013330/logo?size=32", ZkillURL: "https://zkillboard.com/character/2/", EveWhoURL: "https://evewho.com/character/2"},
				{ID: 1, Name: "zed", Corporation: "Elsewhere Inc.", CorporationIcon: "https://images.evetech.net/corporations/98000009/logo?size=32", Left: true, ZkillURL: "https://zkillboard.com/character/1/", EveWhoURL: "https://evewho.com/character/1"},
				{ID: 3, Name: api.DELETED_CHARACTER_NAME, ZkillURL: "https://zkillboard.com/character/3/", EveWhoURL: "https://evewho.com/character/3"},
			}}}, ships)
		assert.Equal(t, []CapitalShip{}, buildCapitalShips(nil, target))
	})
}

// categoryESIClient knows the Ship and Structure categories, and fails for any other
type categoryESIClient struct {
	api.ESIClient
	lookups *int
}

func (m categoryESIClient) GetCategoryInfo(idNumber int) (*api.ESICategory, error) {
	*m.lookups++
	names := map[int]string{6: "Ship", 65: "Structure"}
	if name, found := names[idNumber]; found {
		return &api.ESICategory{CategoryID: idNumber, Name: name}, nil
	}
	return nil, api.ErrRequestFailedToComplete
}

func TestLabelNames(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	lookups := 0
	cards := []KillmailCard{
		{Labels: []string{"cat:6", "solo", "loc:highsec"}},
		{Labels: []string{"cat:6", "cat:65", "cat:999", "cat:x"}},
	}

	names := labelNames(cards, categoryESIClient{lookups: &lookups})

	assert.Equal(t, map[string]string{"cat:6": "Ship", "cat:65": "Structure"}, names, "a category that can not be looked up is left for the frontend to show by id")
	assert.Equal(t, 3, lookups, "each category is looked up once")
}
