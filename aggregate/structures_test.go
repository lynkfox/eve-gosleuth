package aggregate_test

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

func TestStructures(t *testing.T) {
	comm.InitLogger(logrus.WarnLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const (
		astrahus, fortizar, tower, artillery = 35832, 35833, 12235, 17773
		mainCorp, holdingCorp, enemyCorp     = 98000001, 98000002, 98000003
	)
	esi := ItemGroupESIClient{items: map[int]api.ESIItem{
		astrahus:  {ItemId: astrahus, Name: "Astrahus", GroupID: 1657},
		fortizar:  {ItemId: fortizar, Name: "Fortizar", GroupID: 1657},
		artillery: {ItemId: artillery, Name: "Large Artillery Battery", GroupID: 426},
		587:       {ItemId: 587, Name: "Rifter", GroupID: 25},
	}}
	target := &retrieval.IntelTarget{IDNumber: mainCorp, TypeOfEntity: types.CORPORATION,
		Holdings: []retrieval.TargetGroup{{ID: holdingCorp, Type: types.CORPORATION, Name: "Holding"}}}

	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	home := &retrieval.System{SystemID: 1, Name: "J100001"}
	other := &retrieval.System{SystemID: 2, Name: "J100002"}
	corp := func(id int) *retrieval.Corporation { return &retrieval.Corporation{CorporationID: id, Name: "Corp"} }
	item := func(id int) *retrieval.Item { return &retrieval.Item{ItemID: id, Name: esi.items[id].Name} }
	onKill := func(typeID, corpID int) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: typeID, NPC: true}, CorpAtKill: corp(corpID), Ship: item(typeID)}
	}
	pilot := func(id, corpID int) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: id, Name: "Pilot"}, CorpAtKill: corp(corpID), Ship: item(587), FinalBlow: true}
	}
	kill := func(id, daysAgo int, system *retrieval.System, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailID: id, KillmailTime: now.AddDate(0, 0, -daysAgo), System: system,
			Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: 900}, Ship: item(587), CorpAtKill: corp(enemyCorp)}, Attackers: attackers}
	}
	lost := func(id, daysAgo int, system *retrieval.System, typeID, corpID int, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailID: id, KillmailTime: now.AddDate(0, 0, -daysAgo), System: system,
			Victim: &retrieval.KillmailVictim{Ship: &retrieval.Item{ItemID: typeID, Name: "Victim"}, CorpAtKill: corp(corpID)}, Attackers: attackers}
	}

	used := []*retrieval.Killmail{ // newest first
		kill(6, 1, home, onKill(astrahus, mainCorp)),
		kill(5, 3, home, onKill(artillery, mainCorp), onKill(artillery, mainCorp)),
		kill(4, 5, other, onKill(fortizar, holdingCorp)),
		kill(3, 8, home, onKill(astrahus, mainCorp), onKill(fortizar, enemyCorp)),
		kill(2, 400, other, onKill(artillery, mainCorp)),
	}
	losses := []*retrieval.Killmail{
		lost(11, 2, other, fortizar, holdingCorp, pilot(50, enemyCorp), pilot(51, enemyCorp)), // after its kill
		lost(12, 10, home, astrahus, mainCorp, pilot(50, enemyCorp)),                          // before its newest kill: an earlier Astrahus
		lost(13, 300, other, tower, mainCorp, pilot(50, enemyCorp)),                           // the tower its battery stood for
	}

	t.Run("structures are found on kills by type, system, and owner, and grayed by a later loss", func(t *testing.T) {
		uses := aggregate.StructuresUsed(used, losses, target)
		assert.Len(t, uses, 4, "the enemy's Fortizar is not the target's")

		assert.Equal(t, astrahus, uses[0].Structure.ItemID, "standing ones first, newest kill first")
		assert.Equal(t, 2, uses[0].Kills)
		assert.Nil(t, uses[0].Loss, "its loss was before its newest kill")
		assert.Equal(t, now.AddDate(0, 0, -8), uses[0].FirstSeen)

		assert.Nil(t, uses[1].Structure, "a tower seen only through its battery has no type")
		assert.Equal(t, artillery, uses[1].Battery.ItemID)
		assert.Equal(t, 1, uses[1].Kills, "two batteries on one kill count it once")
		assert.Equal(t, home, uses[1].System)

		assert.Equal(t, fortizar, uses[2].Structure.ItemID)
		assert.Equal(t, 11, uses[2].Loss.KillmailID)
		assert.Equal(t, holdingCorp, uses[2].Via.ID)

		assert.Equal(t, 13, uses[3].Loss.KillmailID, "a tower's loss names the tower its battery stood for")
		assert.Equal(t, tower, uses[3].Structure.ItemID)
	})

	t.Run("structures destroyed are the target's kills of a structure", func(t *testing.T) {
		killmails := []*retrieval.Killmail{
			lost(20, 1, home, astrahus, enemyCorp, pilot(60, mainCorp), pilot(61, holdingCorp), pilot(62, enemyCorp)),
			kill(21, 2, home, pilot(60, mainCorp)),
			lost(22, 3, home, astrahus, mainCorp, pilot(62, enemyCorp)),
		}
		destroyed := aggregate.StructuresDestroyed(killmails, target)
		assert.Len(t, destroyed, 1, "a ship kill and the target's own loss are not")
		assert.Equal(t, 20, destroyed[0].Killmail.KillmailID)
		assert.Equal(t, 2, destroyed[0].Pilots, "the main's and the holding's pilots")

		killer := aggregate.KillerOf(losses[0])
		assert.Equal(t, enemyCorp, killer.Corporation.CorporationID)
		assert.Equal(t, 2, killer.Attackers)
	})
}
