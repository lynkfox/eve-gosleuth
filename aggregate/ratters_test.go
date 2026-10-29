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

func TestGetRatters(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const (
		smartbomb, blaster = 3561, 562
		npcCorp, gankCorp  = 1000127, 98000001
	)
	esi := ItemGroupESIClient{items: map[int]api.ESIItem{
		smartbomb: {ItemId: smartbomb, Name: "Large EMP Smartbomb II", GroupID: 72},
		blaster:   {ItemId: blaster, Name: "Small Electron Blaster I", GroupID: 74},
	}}

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	amy := &retrieval.Pilot{CharacterID: 1, Name: "Amy"}
	bob := &retrieval.Pilot{CharacterID: 2, Name: "Bob"}
	carol := &retrieval.Pilot{CharacterID: 3, Name: "Carol"}
	gunner := &retrieval.Pilot{CharacterID: 4, Name: "Gunner"}

	rat := func(shipID int, name string) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{
			Pilot:      &retrieval.Pilot{CharacterID: shipID, NPC: true},
			CorpAtKill: &retrieval.Corporation{CorporationID: npcCorp},
			Ship:       &retrieval.Item{ItemID: shipID, Name: name},
		}
	}
	player := func(weapon int) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: gunner, CorpAtKill: &retrieval.Corporation{CorporationID: gankCorp}, Weapon: &retrieval.Item{ItemID: weapon}}
	}
	structure := &retrieval.KillmailAttacker{
		Pilot:      &retrieval.Pilot{CharacterID: 35832, NPC: true},
		CorpAtKill: &retrieval.Corporation{CorporationID: gankCorp},
		Ship:       &retrieval.Item{ItemID: 35832, Name: "Astrahus"},
	}
	loss := func(id int, who *retrieval.Pilot, hoursAgo int, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: now.Add(-time.Duration(hoursAgo) * time.Hour),
			Victim:       &retrieval.KillmailVictim{Pilot: who, Ship: &retrieval.Item{ItemID: 24690, Name: "Vexor Navy Issue"}},
			Attackers:    attackers,
		}
	}

	killmails := []*retrieval.Killmail{
		loss(1, amy, 1, rat(23, "Sansha's Ravager"), player(blaster)),
		loss(2, bob, 5, rat(23, "Sansha's Ravager"), rat(24, "Centus Tyrant"), rat(23, "Sansha's Ravager")),
		loss(3, carol, 3, rat(23, "Sansha's Ravager"), player(smartbomb)),
		loss(4, amy, 2, player(blaster)),             // no NPCs
		loss(5, amy, 0, structure, player(blaster)),  // a structure is not a rat
		loss(6, nil, 0, rat(23, "Sansha's Ravager")), // no pilot
	}

	ratters := aggregate.GetRatters(killmails, esi)

	assert.Len(t, ratters, 3, "only losses with NPCs on them")
	names := []string{}
	for _, ratter := range ratters {
		names = append(names, ratter.Pilot.Name)
	}
	assert.Equal(t, []string{"Bob", "Carol", "Amy"}, names, "NPC only losses first, then smartbombs, then other players, even when newer")

	bobLoss := ratters[0].Losses[0]
	assert.Equal(t, aggregate.RatsOnly, bobLoss.Kind)
	assert.Equal(t, 0, bobLoss.Players)
	assert.Equal(t, []aggregate.RatCount{
		{Ship: &retrieval.Item{ItemID: 23, Name: "Sansha's Ravager"}, Count: 2},
		{Ship: &retrieval.Item{ItemID: 24, Name: "Centus Tyrant"}, Count: 1},
	}, bobLoss.Rats, "rats counted by type, the most numerous first")

	assert.Equal(t, aggregate.RatsSmartbombs, ratters[1].Losses[0].Kind)
	assert.Equal(t, 1, ratters[1].Losses[0].Players)
	assert.Equal(t, aggregate.RatsAndPlayers, ratters[2].Losses[0].Kind)
	assert.Len(t, ratters[2].Losses, 1)

	t.Run("a player with a smartbomb and another with a blaster is not a smartbomb only loss", func(t *testing.T) {
		mixed := []*retrieval.Killmail{loss(7, carol, 0, rat(23, "Sansha's Ravager"), player(smartbomb), player(blaster))}

		ratters := aggregate.GetRatters(mixed, esi)

		assert.Equal(t, aggregate.RatsAndPlayers, ratters[0].Losses[0].Kind)
		assert.Equal(t, 2, ratters[0].Losses[0].Players)
	})

	t.Run("a pilot's surest loss ranks them, their losses stay newest first", func(t *testing.T) {
		both := []*retrieval.Killmail{killmails[0], loss(8, amy, 9, rat(23, "Sansha's Ravager")), killmails[1]}

		ratters := aggregate.GetRatters(both, esi)

		assert.Equal(t, "Amy", ratters[0].Pilot.Name, "both have NPC only losses, and Amy lost a ship more recently")
		assert.Equal(t, aggregate.RatsOnly, ratters[0].BestKind)
		assert.Equal(t, []int{1, 8}, []int{ratters[0].Losses[0].Killmail.KillmailID, ratters[0].Losses[1].Killmail.KillmailID})
	})

	t.Run("gate sentries and CONCORD are not rats", func(t *testing.T) {
		const sentryGun, concord = 3740, 3880
		withPolice := ItemGroupESIClient{items: map[int]api.ESIItem{
			sentryGun: {ItemId: sentryGun, Name: "Minmatar Sentry Gun I", GroupID: 99},
			concord:   {ItemId: concord, Name: "CONCORD Police Battleship", GroupID: 301},
			blaster:   esi.items[blaster],
		}}
		ganker := loss(9, amy, 0, rat(concord, "CONCORD Police Battleship"), rat(sentryGun, "Minmatar Sentry Gun I"))
		ratted := loss(10, bob, 0, rat(concord, "CONCORD Police Battleship"), rat(23, "Sansha's Ravager"), player(blaster))

		ratters := aggregate.GetRatters([]*retrieval.Killmail{ganker, ratted}, withPolice)

		assert.Len(t, ratters, 1, "a loss to CONCORD and sentries alone is no ratter's")
		assert.Equal(t, []aggregate.RatCount{{Ship: &retrieval.Item{ItemID: 23, Name: "Sansha's Ravager"}, Count: 1}}, ratters[0].Losses[0].Rats)
		assert.Equal(t, 1, ratters[0].Losses[0].Players, "CONCORD is not a player either")
		assert.Equal(t, 0, aggregate.PvENPCs(ganker.Attackers, withPolice))
		assert.Equal(t, 1, aggregate.PvENPCs(ratted.Attackers, withPolice))
	})

	t.Run("no losses with NPCs is an empty list", func(t *testing.T) {
		assert.Equal(t, []aggregate.Ratter{}, aggregate.GetRatters(killmails[3:5], esi))
	})
}
