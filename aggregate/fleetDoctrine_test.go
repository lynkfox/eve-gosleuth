package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFleetDoctrine(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const (
		ferox, osprey, scimitar, rifter = 16227, 620, 11978, 587
		targetAlliance, enemyAlliance   = 99000001, 99000002
	)
	target := &retrieval.IntelTarget{IDNumber: targetAlliance, TypeOfEntity: types.ALLIANCE}
	start := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	ship := func(id int) *retrieval.Item {
		return &retrieval.Item{ItemID: id, Name: map[int]string{ferox: "Ferox", osprey: "Osprey", scimitar: "Scimitar", rifter: "Rifter"}[id]}
	}
	ours := func(pilot int, shipID int) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: pilot}, Ship: ship(shipID), AllianceAtKill: &retrieval.Alliance{AllianceID: targetAlliance}}
	}
	theirs := func(pilot int, shipID int) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: pilot}, Ship: ship(shipID), AllianceAtKill: &retrieval.Alliance{AllianceID: enemyAlliance}}
	}
	// a loss of the target's, its victim one of ours
	loss := func(id int, pilot int, shipID int, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: start.Add(time.Duration(id) * time.Minute),
			Victim:       &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: pilot}, Ship: ship(shipID), AllianceAtKill: &retrieval.Alliance{AllianceID: targetAlliance}},
			Attackers:    attackers,
		}
	}
	// a kill of the target's, the victim one of theirs
	kill := func(id int, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: start.Add(time.Duration(id) * time.Minute),
			Victim:       &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: 900}, Ship: ship(rifter), AllianceAtKill: &retrieval.Alliance{AllianceID: enemyAlliance}},
			Attackers:    attackers,
		}
	}

	t.Run("five of its hull on its side with logi is a doctrine in its battle", func(t *testing.T) {
		killmails := []*retrieval.Killmail{
			kill(1, ours(2, ferox), ours(3, ferox), ours(4, ferox), ours(5, osprey)),
			loss(2, 1, ferox, theirs(800, ferox), theirs(801, ferox)),
			kill(3, ours(6, ferox), ours(2, ferox)),
		}
		battles := map[int]string{1: "b", 2: "b", 3: "b"}

		tags := aggregate.FleetDoctrineInBattle(killmails, target, battles)

		if assert.Contains(t, tags, 2) {
			assert.Equal(t, "Fleet Doctrine", tags[2].Name)
			assert.Equal(t, []retrieval.FitTagReason{
				{Name: "Ferox in its battle", Count: 5, Needed: 5},
				{Name: "Logi in its battle", Count: 1, Needed: 1},
			}, tags[2].Reasons, "the victim and four others, each once; the enemy's Feroxes don't count")
		}
	})

	t.Run("without logi, or short a hull, or alone, it is not", func(t *testing.T) {
		noLogi := []*retrieval.Killmail{kill(1, ours(2, ferox), ours(3, ferox), ours(4, ferox), ours(5, ferox)), loss(2, 1, ferox)}
		assert.Empty(t, aggregate.FleetDoctrineInBattle(noLogi, target, map[int]string{1: "b", 2: "b"}))

		short := []*retrieval.Killmail{kill(1, ours(2, ferox), ours(3, ferox), ours(4, scimitar), theirs(800, ferox), theirs(801, ferox)), loss(2, 1, ferox)}
		assert.Empty(t, aggregate.FleetDoctrineInBattle(short, target, map[int]string{1: "b", 2: "b"}))

		withLogi := []*retrieval.Killmail{kill(1, ours(2, ferox), ours(3, ferox), ours(4, ferox), ours(5, ferox), ours(6, scimitar)), loss(2, 1, ferox)}
		assert.Contains(t, aggregate.FleetDoctrineInBattle(withLogi, target, map[int]string{1: "b", 2: "b"}), 2, "a T2 logi hull counts")
		assert.Empty(t, aggregate.FleetDoctrineInBattle(withLogi, target, map[int]string{}), "a loss in no battle")
	})

	t.Run("a fit lost five times by five pilots is a doctrine", func(t *testing.T) {
		losses := []*retrieval.Killmail{loss(1, 1, ferox), loss(2, 2, ferox), loss(3, 3, ferox), loss(4, 4, ferox), loss(5, 5, ferox)}

		tag, found := aggregate.FleetDoctrineRepeated(losses, target)

		assert.True(t, found)
		assert.Equal(t, []retrieval.FitTagReason{
			{Name: "Losses of this fit in one alliance", Count: 5, Needed: 5},
			{Name: "Pilots who lost it", Count: 5, Needed: 5},
		}, tag.Reasons)

		_, found = aggregate.FleetDoctrineRepeated(append(losses[:4:4], loss(6, 1, ferox)), target)
		assert.False(t, found, "five losses by four pilots")
	})

	t.Run("losses count by alliance, so two alliances' don't add up", func(t *testing.T) {
		pilot := &retrieval.IntelTarget{IDNumber: 1, TypeOfEntity: types.PILOT}
		elsewhere := loss(5, 5, ferox)
		elsewhere.Victim.AllianceAtKill = &retrieval.Alliance{AllianceID: enemyAlliance}
		losses := []*retrieval.Killmail{loss(1, 1, ferox), loss(2, 2, ferox), loss(3, 3, ferox), loss(4, 4, ferox), elsewhere}

		_, found := aggregate.FleetDoctrineRepeated(losses, pilot)

		assert.False(t, found)
	})
}
