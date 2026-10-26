package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// ItemGroupESIClient answers item lookups from a fixed list of types and their groups, failing for any other type
type ItemGroupESIClient struct {
	api.ESIClient
	items map[int]api.ESIItem
}

func (m ItemGroupESIClient) GetItemInfo(idNumber int) (*api.ESIItem, error) {
	item, found := m.items[idNumber]
	if !found {
		return nil, api.ErrRequestFailedToComplete
	}
	return &item, nil
}

func TestGetPIPilots(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const (
		epithal, squall, noctis, rifter = 655, 81008, 2998, 587
		crane, charon                   = 12729, 20185
		coolant, water, tritanium       = 9832, 3645, 34
		planet, moon                    = 40009077, 40009078
	)
	esi := ItemGroupESIClient{items: map[int]api.ESIItem{
		epithal:   {ItemId: epithal, Name: "Epithal", GroupID: 28},
		squall:    {ItemId: squall, Name: "Squall", GroupID: 28},
		noctis:    {ItemId: noctis, Name: "Noctis", GroupID: 28},
		rifter:    {ItemId: rifter, Name: "Rifter", GroupID: 25},
		crane:     {ItemId: crane, Name: "Crane", GroupID: 1202},
		charon:    {ItemId: charon, Name: "Charon", GroupID: 513},
		coolant:   {ItemId: coolant, Name: "Coolant", GroupID: 1034},
		water:     {ItemId: water, Name: "Water", GroupID: 1042},
		tritanium: {ItemId: tritanium, Name: "Tritanium", GroupID: 18},
	}}
	planets := map[int]bool{planet: true}

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	amy := &retrieval.Pilot{CharacterID: 1, Name: "Amy"}
	bob := &retrieval.Pilot{CharacterID: 2, Name: "Bob"}
	carol := &retrieval.Pilot{CharacterID: 3, Name: "Carol"}
	quantity := func(n int) *int { return &n }
	loss := func(id int, who *retrieval.Pilot, ship int, location int, hoursAgo int, items ...api.ESIKillmailItem) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: now.Add(-time.Duration(hoursAgo) * time.Hour),
			LocationID:   location,
			Victim:       &retrieval.KillmailVictim{Pilot: who, Ship: &retrieval.Item{ItemID: ship}, Items: items},
		}
	}

	killmails := []*retrieval.Killmail{
		loss(1, amy, epithal, planet, 30,
			api.ESIKillmailItem{ItemTypeID: coolant, QuantityDestroyed: quantity(1000)},
			api.ESIKillmailItem{ItemTypeID: coolant, QuantityDropped: quantity(500)},
			api.ESIKillmailItem{ItemTypeID: tritanium, QuantityDropped: quantity(9)}),
		loss(2, amy, squall, planet, 10),
		loss(3, bob, squall, planet, 20, api.ESIKillmailItem{ItemTypeID: water, QuantityDropped: quantity(40)}),
		loss(4, bob, epithal, moon, 1),   // near a moon, not a planet
		loss(5, bob, noctis, planet, 2),  // a salvager, not a hauler
		loss(6, bob, rifter, planet, 3),  // not a hauler
		loss(7, nil, epithal, planet, 4), // no pilot
	}

	pilots := aggregate.GetPIPilots(killmails, planets, esi)

	assert.Len(t, pilots, 2, "haulers near planets, and nothing else without PI aboard")
	assert.Equal(t, "Amy", pilots[0].Pilot.Name, "Amy lost a hauler most recently")
	assert.True(t, pilots[0].CarriedPI)
	ids := []int{}
	for _, loss := range pilots[0].Losses {
		ids = append(ids, loss.Killmail.KillmailID)
	}
	assert.Equal(t, []int{2, 1}, ids, "newest first")
	assert.Empty(t, pilots[0].Losses[0].Cargo, "the Squall carried nothing")
	assert.Equal(t, []aggregate.PICargo{{Item: &api.ESIItem{ItemId: coolant, Name: "Coolant", GroupID: 1034}, Quantity: 1500}}, pilots[0].Losses[1].Cargo,
		"coolant is added up across stacks, tritanium is not PI")
	assert.Equal(t, planet, pilots[0].Losses[1].PlanetID)

	assert.Equal(t, "Bob", pilots[1].Pilot.Name)
	assert.Len(t, pilots[1].Losses, 1, "only the Squall near the planet counts")
	assert.True(t, pilots[1].CarriedPI)

	t.Run("no hauler losses near planets and no PI aboard is an empty list", func(t *testing.T) {
		assert.Equal(t, []aggregate.PIPilot{}, aggregate.GetPIPilots(killmails[3:], planets, esi))
	})

	t.Run("a T2 transport near a planet counts, a freighter does not", func(t *testing.T) {
		transports := []*retrieval.Killmail{loss(9, carol, crane, planet, 0), loss(10, bob, charon, planet, 0)}

		pilots := aggregate.GetPIPilots(transports, planets, esi)

		assert.Len(t, pilots, 1)
		assert.Equal(t, "Carol", pilots[0].Pilot.Name)
	})

	t.Run("any ship with PI aboard counts, wherever it died", func(t *testing.T) {
		withCargo := append(killmails, loss(8, carol, rifter, moon, 0, api.ESIKillmailItem{ItemTypeID: water, QuantityDestroyed: quantity(5)}))

		pilots := aggregate.GetPIPilots(withCargo, planets, esi)

		assert.Len(t, pilots, 3)
		assert.Equal(t, "Carol", pilots[0].Pilot.Name)
		assert.Equal(t, 0, pilots[0].Losses[0].PlanetID, "it was not nearest a planet")
		assert.Equal(t, 5, pilots[0].Losses[0].Cargo[0].Quantity)
	})
}
