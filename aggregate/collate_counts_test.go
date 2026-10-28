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

// lossAt is deployableLoss with its id changed and moved later by after
func lossAt(t *testing.T, killmailID int, after time.Duration) *api.ESIKillmail {
	killmail := deployableLoss(t, killmailID)
	killmail.KillmailTime = killmail.KillmailTime.Add(after)
	return killmail
}

func TestCollatedCounts(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	const proteus = 29988
	const pilotID = 2121911624

	t.Run("a ship flown on several killmails is counted on each, as an attacker's ship", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, time.Hour))

		ship := collected.ShipsFlown[proteus]
		assert.Equal(t, proteus, ship.Ship.ItemID, "the attacker's own ship, not the victim's")
		assert.Equal(t, 6, ship.TotalKillmails, "three Proteus on each of two killmails")
		assert.Equal(t, 0, ship.Destroyed)
		assert.Len(t, ship.AttackerKillmailIDs, 6)
		assert.Empty(t, ship.VictimKillmailIDs)

		pilotShip := collected.PilotsSeen[pilotID].ShipsFlown[proteus]
		assert.Equal(t, 2, pilotShip.TotalKillmails)
		assert.Equal(t, []int{1, 2}, pilotShip.AttackerKillmailIDs)
	})

	t.Run("a victim's ship is counted as destroyed", func(t *testing.T) {
		killmail := lossAt(t, 1, 0)
		victimID := 2112000001
		killmail.Victim.CharacterID = &victimID

		ship := collate(t, killmail).ShipsFlown[deployableTypeID]

		assert.Equal(t, 1, ship.Destroyed)
		assert.Equal(t, []int{1}, ship.VictimKillmailIDs)
	})

	t.Run("a pilot's system is counted on every killmail in it", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, time.Hour), lossAt(t, 3, 2*time.Hour))

		system := collected.PilotsSeen[pilotID].SystemsVisited[31001861]
		assert.Equal(t, 3, system.TotalKillmails)
		assert.Equal(t, []int{1, 2, 3}, system.KillmailIDs)
	})

	t.Run("an alliance and corp are counted once per killmail, however many of their pilots were on it", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, time.Hour))

		alliance := collected.AlliancesSeen[99013330]
		assert.Equal(t, 2, alliance.TotalKillmails)
		assert.Len(t, alliance.Activity, 2)
		assert.Len(t, alliance.PilotIDs, 3)
		assert.Contains(t, alliance.SystemIDs, 31001861)

		corporation := collected.CorporationsSeen[98744491]
		assert.Equal(t, 2, corporation.TotalKillmails)
		assert.Len(t, corporation.KillmailIDs, 2)
	})
}

func TestBattles(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("killmails within 15 minutes in one system are a battle, which starts with the first of them", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, 5*time.Minute), lossAt(t, 3, 2*time.Hour))

		assert.Len(t, collected.BattlesFought, 1)
		battle := collected.BattlesFought["31001861/202610250858"]
		assert.Equal(t, []int{1, 2}, battle.KillmailIDs)
		assert.Equal(t, 2, battle.TotalKillmails)
	})

	t.Run("a battle still going at the last killmail is kept", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, 2*time.Hour), lossAt(t, 3, 2*time.Hour+10*time.Minute))

		assert.Len(t, collected.BattlesFought, 1)
		assert.Equal(t, []int{2, 3}, collected.BattlesFought["31001861/202610251058"].KillmailIDs)
	})

	// elsewhere moves a killmail to another system, and with strangers gives it other attackers, so it shares no pilots
	elsewhere := func(killmail *api.ESIKillmail, system int, strangers bool) *api.ESIKillmail {
		killmail.SolarSystemID = system
		if strangers {
			for index := range killmail.Attackers {
				id := 2113000000 + index
				killmail.Attackers[index].CharacterID = &id
			}
		}
		return killmail
	}

	t.Run("a fight that moves to another system with the same pilots stays one battle", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), elsewhere(lossAt(t, 2, 5*time.Minute), 31000005, false), elsewhere(lossAt(t, 3, 10*time.Minute), 31000005, false))

		assert.Len(t, collected.BattlesFought, 1)
		assert.Equal(t, []int{1, 2, 3}, collected.BattlesFought["31001861/202610250858"].KillmailIDs)
		assert.Equal(t, collected.BattleOfKillmail()[1], collected.BattleOfKillmail()[3])
	})

	t.Run("an unrelated fight elsewhere at the same time is its own battle and does not split this one", func(t *testing.T) {
		collected := collate(t,
			lossAt(t, 1, 0),
			elsewhere(lossAt(t, 2, 2*time.Minute), 31000005, true),
			lossAt(t, 3, 8*time.Minute),
			elsewhere(lossAt(t, 4, 9*time.Minute), 31000005, true),
		)

		assert.Len(t, collected.BattlesFought, 2)
		assert.Equal(t, []int{1, 3}, collected.BattlesFought["31001861/202610250858"].KillmailIDs)
		assert.Equal(t, []int{2, 4}, collected.BattlesFought["31000005/202610250900"].KillmailIDs)
	})

	t.Run("a battle lasts as long as each killmail is within 15 minutes of the one before", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, 14*time.Minute), lossAt(t, 3, 28*time.Minute), lossAt(t, 4, 42*time.Minute))

		assert.Len(t, collected.BattlesFought, 1)
		assert.Equal(t, 4, collected.BattlesFought["31001861/202610250858"].TotalKillmails)
	})

	t.Run("every killmail is in a battle with an id, even on its own", func(t *testing.T) {
		collected := collate(t, lossAt(t, 1, 0), lossAt(t, 2, 2*time.Hour))

		assert.Empty(t, collected.BattlesFought, "a single killmail is not saved as a battle")
		assert.NotContains(t, collected.SystemsVisted[31001861].BattlesFoughtIDs, "")
		assert.Len(t, collected.SystemsVisted[31001861].BattlesFoughtIDs, 2)
	})
}

func TestFleetmates(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	ursa := &retrieval.Alliance{AllianceID: 99013330}
	inCorp := &retrieval.Corporation{CorporationID: 98744491, Alliance: ursa}
	seen := func(pilot *retrieval.Pilot) aggregate.PilotSeen {
		return aggregate.PilotSeen{Pilot: pilot, TotalKillmails: 1}
	}

	t.Run("a pilot's fleetmates leave out NPC and structure attackers", func(t *testing.T) {
		target := retrieval.IntelTarget{IDNumber: 1, TypeOfEntity: types.PILOT, Expanded: &retrieval.Pilot{CharacterID: 1, Corporation: inCorp}}
		pilots := map[int]aggregate.PilotSeen{
			1:     seen(&retrieval.Pilot{CharacterID: 1, Name: "Target", Corporation: inCorp}),
			2:     seen(&retrieval.Pilot{CharacterID: 2, Name: "Corpmate", Corporation: inCorp}),
			3:     seen(&retrieval.Pilot{CharacterID: 3, Name: "Alliance mate", Corporation: &retrieval.Corporation{CorporationID: 98000001, Alliance: ursa}}),
			4:     seen(&retrieval.Pilot{CharacterID: 4, Name: "Stranger", Corporation: &retrieval.Corporation{CorporationID: 98000002}}),
			29988: seen(&retrieval.Pilot{CharacterID: 29988, NPC: true, Corporation: inCorp}),
		}

		names := []string{}
		for _, pilot := range aggregate.GetMostFrequentFleetmates(10, pilots, &target) {
			names = append(names, pilot.Pilot.Name)
		}

		assert.ElementsMatch(t, []string{"Corpmate", "Alliance mate"}, names)
	})

	t.Run("a corp target's sister corps in its alliance are in its group", func(t *testing.T) {
		target := retrieval.IntelTarget{IDNumber: 98744491, TypeOfEntity: types.CORPORATION, Expanded: inCorp}

		assert.True(t, aggregate.IsSameGroupAsTarget(&target, retrieval.Corporation{CorporationID: 98000001, Alliance: ursa}))
		assert.False(t, aggregate.IsSameGroupAsTarget(&target, retrieval.Corporation{CorporationID: 98000002, Alliance: &retrieval.Alliance{AllianceID: 99000001}}))
	})
}
