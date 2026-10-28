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
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
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

	t.Run("a structure victim has no pilot, its corp has its logo and links, and it has no alliance when it had none", func(t *testing.T) {
		assert.Equal(t, "Mobile Depot", card.VictimName)
		assert.Nil(t, card.Victim)
		assert.Equal(t, &EntityRef{ID: 98828090, Type: "CORPORATION", Name: "Wormhole Campus", IconURL: "https://images.evetech.net/corporations/98828090/logo?size=32",
			ZkillURL: "https://zkillboard.com/corporation/98828090/", EveWhoURL: "https://evewho.com/corporation/98828090"}, card.VictimCorp)
		assert.Nil(t, card.VictimAlliance)
	})

	t.Run("the final blow has its pilot, corp, and alliance, each with links for the menu", func(t *testing.T) {
		assert.Equal(t, "https://zkillboard.com/character/2121909368/", card.FinalBlow.ZkillURL)
		assert.Equal(t, []string{"URSA PRIMUS", "URSA."}, []string{card.FinalBlowCorp.Name, card.FinalBlowAlliance.Name})
		assert.Equal(t, "https://images.evetech.net/corporations/98744491/logo?size=32", card.FinalBlowCorp.IconURL)
		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", card.FinalBlowAlliance.IconURL)
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

		assert.Equal(t, "https://images.evetech.net/corporations/98744491/logo?size=32", fleetmate.Corporation.IconURL)
		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", fleetmate.Alliance.IconURL)
	})

	t.Run("corp rows carry their alliance logo, and none when the corp has no alliance", func(t *testing.T) {
		killmailTime := []time.Time{time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)}
		corporations := buildCorporationSummaries([]aggregate.CorporationActivity{
			{Corporation: primus, Alliance: ursa, Activity: killmailTime},
			{Corporation: &retrieval.Corporation{CorporationID: 98828090, Name: "Wormhole Campus"}, Activity: killmailTime},
		})

		assert.Equal(t, "https://images.evetech.net/alliances/99013330/logo?size=32", corporations[0].Alliance.IconURL)
		assert.Nil(t, corporations[1].Alliance)
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
				{ID: 2, Name: "Amy", Corporation: entityRef(types.CORPORATION, 98744491, "URSA PRIMUS"), Alliance: entityRef(types.ALLIANCE, 99013330, "URSA."), ZkillURL: "https://zkillboard.com/character/2/", EveWhoURL: "https://evewho.com/character/2"},
				{ID: 1, Name: "zed", Corporation: entityRef(types.CORPORATION, 98000009, "Elsewhere Inc."), Left: true, ZkillURL: "https://zkillboard.com/character/1/", EveWhoURL: "https://evewho.com/character/1"},
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

func TestBuildKillmailCardUnresolvedNames(t *testing.T) {
	t.Run("a victim, corp, or alliance not looked up yet gets no ref, so the card's menu has no empty sections", func(t *testing.T) {
		killmail := &retrieval.Killmail{
			KillmailID: 1,
			Victim: &retrieval.KillmailVictim{
				Pilot:          &retrieval.Pilot{CharacterID: 2121909368},
				CorpAtKill:     &retrieval.Corporation{},
				AllianceAtKill: &retrieval.Alliance{AllianceID: 99013330},
			},
		}

		card := buildKillmailCards([]*retrieval.Killmail{killmail}, &retrieval.IntelTarget{TypeOfEntity: types.SYSTEM})[0]

		assert.Nil(t, card.Victim)
		assert.Nil(t, card.VictimCorp)
		assert.Nil(t, card.VictimAlliance)
	})
}

// namesESIClient answers name lookups with its matches, or its error
type namesESIClient struct {
	api.ESIClient
	matches map[int]api.EntityIDAndName
	err     error
}

func (m namesESIClient) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	return m.matches, m.err
}

func TestMatchTargets(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	appWith := func(client api.ESIClient) *App {
		return &App{clients: retrieval.CollectionClientPool{ESIClient: client}}
	}

	t.Run("a name only an item has says so, rather than that nothing was found", func(t *testing.T) {
		app := appWith(namesESIClient{matches: map[int]api.EntityIDAndName{33468: {Category: "inventory_types", ID: 33468, Name: "Astero"}}})
		_, err := app.matchTargets("Astero")
		assert.EqualError(t, err, "Astero is an item, not a pilot, corporation, alliance, or system")
	})

	t.Run("no match at all asks to check the spelling", func(t *testing.T) {
		_, err := appWith(namesESIClient{}).matchTargets("Nobody")
		assert.EqualError(t, err, "Nothing found by that name. Check spelling?")
	})

	t.Run("choices have portraits and logos, but a system none", func(t *testing.T) {
		choices := targetChoices([]retrieval.TargetMatch{{Type: types.PILOT, ID: 90000001, Name: "Jita"}, {Type: types.SYSTEM, ID: 30000142, Name: "Jita"}})
		assert.Equal(t, []TargetChoice{
			{Type: "PILOT", ID: 90000001, Name: "Jita", ImageURL: api.EntityImageURL(types.PILOT, 90000001)},
			{Type: "SYSTEM", ID: 30000142, Name: "Jita"},
		}, choices)
	})
}

// moonESIClient knows one moon, in system 31000001 beside planet 40000010, and fails for any other
type moonESIClient struct {
	api.ESIClient
}

func (m moonESIClient) GetSystemInfo(idNumber int) (*api.ESISystem, error) {
	if idNumber == 31000001 {
		return &api.ESISystem{SystemID: idNumber, Name: "J100001", Planets: []api.ESIPlanet{{PlanetID: 40000010, Moons: []int{40000001}}}}, nil
	}
	return nil, api.ErrRequestFailedToComplete
}

func (m moonESIClient) GetMoonInfo(idNumber int) (*api.ESIMoon, error) {
	if idNumber == 40000001 {
		return &api.ESIMoon{MoonID: idNumber, Name: "J100001 III - Moon 2", SystemID: 31000001}, nil
	}
	return nil, api.ErrRequestFailedToComplete
}

func TestMoonName(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	system := &retrieval.System{SystemID: 31000001, Name: "J100001"}
	assert.Equal(t, "III - Moon 2", moonName(moonESIClient{}, 40000001, system), "the system's name is left off")
	assert.Equal(t, "J100001 III - Moon 2", moonName(moonESIClient{}, 40000001, nil))
	assert.Equal(t, "", moonName(moonESIClient{}, 0, system), "no moon is not looked up")
	assert.Equal(t, "", moonName(moonESIClient{}, 40000002, system), "a failed lookup names none")
}

func TestBuildStructuresLost(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	target := &retrieval.IntelTarget{IDNumber: 98000001, TypeOfEntity: types.CORPORATION}
	system := &retrieval.System{SystemID: 31000001, Name: "J100001"}
	holding := &retrieval.TargetGroup{ID: 98000002, Type: types.CORPORATION, Name: "Holding"}
	lost := func(id, typeID, moonID int, via *retrieval.TargetGroup) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailID: id, System: system, LocationID: moonID, Via: via,
			Victim: &retrieval.KillmailVictim{Ship: &retrieval.Item{ItemID: typeID, Name: "Structure"}, CorpAtKill: &retrieval.Corporation{CorporationID: 98000001, Name: "Corp"}}}
	}
	found := structureKillmails{lost: []*retrieval.Killmail{lost(1, 12235, 40000001, nil), lost(2, 35832, 40000001, holding), lost(3, 12235, 40000010, nil)}}

	report := buildStructures(moonESIClient{}, found, nil, target)
	assert.Empty(t, report.Used, "a structure never on a kill is not used")
	if assert.Len(t, report.Lost, 3, "every structure lost is listed, seen on a kill or not") {
		assert.Equal(t, "III - Moon 2", report.Lost[0].Moon, "a tower's moon is named")
		assert.Equal(t, "", report.Lost[0].Via)
		assert.Equal(t, "Corp", report.Lost[0].Owner.Name)
		assert.Equal(t, "", report.Lost[1].Moon, "an upwell structure has no moon")
		assert.Equal(t, "Holding", report.Lost[1].Via)
		assert.Equal(t, "", report.Lost[2].Moon, "a tower nearest a planet is not named after it")
	}
}
