package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetTopAllies(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	ownAlliance := &retrieval.Alliance{AllianceID: 99000001, Name: "Own Alliance"}
	friends := &retrieval.Alliance{AllianceID: 99000002, Name: "Friends"}
	pals := &retrieval.Alliance{AllianceID: 99000003, Name: "Pals"}
	ownCorp := &retrieval.Corporation{CorporationID: 98000001, Name: "Own Corp"}
	sisterCorp := &retrieval.Corporation{CorporationID: 98000002, Name: "Sister Corp"}
	friendCorp := &retrieval.Corporation{CorporationID: 98000003, Name: "Friend Corp"}
	palCorp := &retrieval.Corporation{CorporationID: 98000004, Name: "Pal Corp"}
	loneCorp := &retrieval.Corporation{CorporationID: 98000005, Name: "Lone Corp"}
	starterCorp := &retrieval.Corporation{CorporationID: 1000167, Name: "State War Academy"}

	pilot := func(id int) *retrieval.Pilot { return &retrieval.Pilot{CharacterID: id} }
	attacker := func(who *retrieval.Pilot, corp *retrieval.Corporation, alliance *retrieval.Alliance) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: who, CorpAtKill: corp, AllianceAtKill: alliance}
	}
	rat := &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: 23, NPC: true}, CorpAtKill: &retrieval.Corporation{CorporationID: 1000127}}
	target := pilot(1)
	kill := func(daysAgo int, victim *retrieval.Pilot, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailTime: now.AddDate(0, 0, -daysAgo), Victim: &retrieval.KillmailVictim{Pilot: victim}, Attackers: attackers}
	}

	killmails := []*retrieval.Killmail{
		kill(1, pilot(50),
			attacker(target, ownCorp, ownAlliance),
			attacker(pilot(2), sisterCorp, ownAlliance), // the target's alliance
			attacker(pilot(3), friendCorp, friends),
			attacker(pilot(4), friendCorp, friends), // the same ally twice on one kill counts once
			attacker(pilot(5), loneCorp, nil),
			attacker(pilot(6), starterCorp, nil), // an NPC corp
			rat),
		kill(2, pilot(51), attacker(target, ownCorp, ownAlliance), attacker(pilot(7), palCorp, pals), attacker(pilot(3), friendCorp, friends)),
		kill(3, pilot(52), attacker(target, ownCorp, ownAlliance), attacker(pilot(8), palCorp, pals)),
		// the target shot a Pals pilot, alongside a lone corp pilot
		kill(4, pilot(54), attacker(target, ownCorp, ownAlliance), attacker(pilot(5), loneCorp, nil)),
		kill(5, pilot(55), attacker(target, ownCorp, ownAlliance), attacker(pilot(5), loneCorp, nil)),
		// the target's loss: its attackers are not allies
		kill(0, target, attacker(pilot(9), friendCorp, friends), attacker(pilot(10), loneCorp, nil)),
		// a kill without the target
		kill(0, pilot(53), attacker(pilot(11), friendCorp, friends)),
	}
	killmails[3].Victim.AllianceAtKill = pals
	pilotTarget := &retrieval.IntelTarget{IDNumber: 1, TypeOfEntity: types.PILOT}

	ranking := aggregate.GetTopAllies(10, killmails, pilotTarget, nil, false)
	allies := ranking.Top
	assert.Equal(t, 1, ranking.KilledAllies, "Pals, counted though not left out")

	names := []string{}
	for _, ally := range allies {
		names = append(names, ally.Name())
	}
	assert.Equal(t, []string{"Friends", "Pals", "Lone Corp"}, names, "Friends and Pals each share 2 kills over 2 days with 2 pilots, so they go by name, and Lone Corp's 3 kills still come after every alliance")
	assert.Equal(t, 2, allies[0].Killmails)
	assert.Len(t, allies[0].PilotIDs, 2)
	assert.Equal(t, types.ALLIANCE, allies[0].Type())
	assert.Equal(t, types.CORPORATION, allies[2].Type(), "a corp in no alliance is its own ally")
	assert.Equal(t, 3, allies[2].Killmails, "the loss does not count")
	assert.Equal(t, 0, allies[0].Killed)
	assert.Equal(t, 1, allies[1].Killed, "the target killed a Pals pilot")

	t.Run("limit", func(t *testing.T) {
		assert.Len(t, aggregate.GetTopAllies(1, killmails, pilotTarget, nil, false).Top, 1)
	})

	t.Run("a corp target in no alliance leaves out only its own corp", func(t *testing.T) {
		corpKills := []*retrieval.Killmail{kill(1, pilot(50), attacker(target, loneCorp, nil), attacker(pilot(5), loneCorp, nil), attacker(pilot(3), friendCorp, friends))}

		allies := aggregate.GetTopAllies(10, corpKills, &retrieval.IntelTarget{IDNumber: loneCorp.CorporationID, TypeOfEntity: types.CORPORATION}, nil, false).Top

		assert.Len(t, allies, 1)
		assert.Equal(t, "Friends", allies[0].Name())
	})

	t.Run("an alliance target leaves out its own alliance", func(t *testing.T) {
		allies := aggregate.GetTopAllies(10, killmails, &retrieval.IntelTarget{IDNumber: ownAlliance.AllianceID, TypeOfEntity: types.ALLIANCE}, nil, false).Top

		for _, ally := range allies {
			assert.NotEqual(t, "Own Alliance", ally.Name())
		}
		assert.Len(t, allies, 3)
	})

	t.Run("ignored allies are left out, and the next take their places", func(t *testing.T) {
		ranking := aggregate.GetTopAllies(2, killmails, pilotTarget, map[aggregate.AllyKey]bool{{Type: types.ALLIANCE, ID: friends.AllianceID}: true}, false)
		allies := ranking.Top
		assert.Equal(t, map[aggregate.AllyKey]int{{Type: types.ALLIANCE, ID: friends.AllianceID}: 2}, ranking.IgnoredKillmails, "the kills Friends shared, though left out")

		assert.Equal(t, "Pals", allies[0].Name())
		assert.Equal(t, "Lone Corp", allies[1].Name())
	})

	t.Run("skipKilled leaves out allies the target also killed", func(t *testing.T) {
		ranking := aggregate.GetTopAllies(10, killmails, pilotTarget, nil, true)
		allies := ranking.Top
		assert.Equal(t, 1, ranking.KilledAllies, "Pals")

		names := []string{}
		for _, ally := range allies {
			names = append(names, ally.Name())
		}
		assert.Equal(t, []string{"Friends", "Lone Corp"}, names, "the target killed a Pals pilot")
	})

	t.Run("an alliance target and its holding are both its own, whichever is first on the kill", func(t *testing.T) {
		holding := &retrieval.Alliance{AllianceID: 99000004, Name: "Holding Alliance"}
		holdingCorp := &retrieval.Corporation{CorporationID: 98000006, Name: "Holding Corp"}
		alliance := &retrieval.IntelTarget{IDNumber: ownAlliance.AllianceID, TypeOfEntity: types.ALLIANCE,
			Holdings: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: holding.AllianceID}}}
		holdingFirst := kill(1, pilot(70), attacker(pilot(71), holdingCorp, holding), attacker(pilot(72), ownCorp, ownAlliance), attacker(pilot(3), friendCorp, friends))

		names := []string{}
		for _, ally := range aggregate.GetTopAllies(10, []*retrieval.Killmail{holdingFirst}, alliance, nil, false).Top {
			names = append(names, ally.Name())
		}
		assert.Equal(t, []string{"Friends"}, names)
	})

	t.Run("a loss of the target's is no kill, even with its own pilots among the attackers", func(t *testing.T) {
		// an awox: a pilot of the target corp killed by another of its pilots, with a Friends pilot helping
		awox := kill(1, pilot(60), attacker(pilot(61), loneCorp, nil), attacker(pilot(3), friendCorp, friends))
		awox.Victim.CorpAtKill = loneCorp
		corp := &retrieval.IntelTarget{IDNumber: loneCorp.CorporationID, TypeOfEntity: types.CORPORATION}

		ranking := aggregate.GetTopAllies(10, []*retrieval.Killmail{awox}, corp, nil, false)

		assert.Empty(t, ranking.Top)
		assert.Zero(t, ranking.KilledAllies)
	})

	t.Run("no kills is an empty list", func(t *testing.T) {
		allies := aggregate.GetTopAllies(10, killmails[5:], pilotTarget, nil, false).Top
		assert.Equal(t, []aggregate.AllyActivity{}, allies)
	})
}

func TestHoldingKillsTogether(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	main := &retrieval.Corporation{CorporationID: 98000001, Name: "Main Corp"}
	mainAlliance := &retrieval.Alliance{AllianceID: 99000001, Name: "Main Alliance"}
	holdingCorp := &retrieval.Corporation{CorporationID: 98000002, Name: "Holding Corp"}
	holdingAlliance := &retrieval.Alliance{AllianceID: 99000003, Name: "Holding Alliance"}
	inHoldingAlliance := &retrieval.Corporation{CorporationID: 98000003, Name: "Corp in Holding Alliance"}
	other := &retrieval.Corporation{CorporationID: 98000009, Name: "Other Corp"}

	attacker := func(id int, corp *retrieval.Corporation, alliance *retrieval.Alliance) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: id}, CorpAtKill: corp, AllianceAtKill: alliance}
	}
	kill := func(victimCorp *retrieval.Corporation, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: 50}, CorpAtKill: victimCorp}, Attackers: attackers}
	}
	holdings := []retrieval.TargetGroup{{Type: types.CORPORATION, ID: holdingCorp.CorporationID}, {Type: types.ALLIANCE, ID: holdingAlliance.AllianceID}}
	target := &retrieval.IntelTarget{IDNumber: main.CorporationID, TypeOfEntity: types.CORPORATION, Holdings: holdings}

	killmails := []*retrieval.Killmail{
		kill(other, attacker(1, main, mainAlliance), attacker(2, holdingCorp, nil)),
		kill(other, attacker(1, main, mainAlliance), attacker(2, holdingCorp, nil), attacker(3, holdingCorp, nil), attacker(4, inHoldingAlliance, holdingAlliance)),
		kill(other, attacker(2, holdingCorp, nil)),                                                 // the holding alone is not together
		kill(holdingCorp, attacker(1, main, mainAlliance), attacker(2, holdingCorp, nil)),          // an internal loss is no kill
		kill(main, attacker(1, other, nil), attacker(2, holdingCorp, nil), attacker(3, main, nil)), // nor is any loss
	}

	assert.Equal(t, map[aggregate.AllyKey]int{
		{Type: types.CORPORATION, ID: holdingCorp.CorporationID}: 2,
		{Type: types.ALLIANCE, ID: holdingAlliance.AllianceID}:   1,
	}, aggregate.HoldingKillsTogether(killmails, target, holdings))
}
