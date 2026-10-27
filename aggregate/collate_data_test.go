package aggregate_test

import (
	"encoding/json"
	"path/filepath"
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

// deployableLossESI is killmail 139416795: a deployable (no victim character_id) killed by three pilots
const deployableLossESI = `{"attackers":[
	{"alliance_id":99013330,"character_id":2121911624,"corporation_id":98744491,"damage_done":65608,"final_blow":false,"security_status":0.9,"ship_type_id":29988,"weapon_type_id":2478},
	{"alliance_id":99013330,"character_id":2121877924,"corporation_id":98744491,"damage_done":64223,"final_blow":false,"security_status":0.9,"ship_type_id":29988,"weapon_type_id":2478},
	{"alliance_id":99013330,"character_id":2121909368,"corporation_id":98744491,"damage_done":59685,"final_blow":true,"security_status":0.4,"ship_type_id":29988,"weapon_type_id":2478}],
	"killmail_id":139416795,"killmail_time":"2026-10-25T08:58:50Z","solar_system_id":31001861,
	"victim":{"alliance_id":99014851,"corporation_id":98828090,"damage_taken":189516,"items":[],"position":{"x":1,"y":2,"z":3},"ship_type_id":12200}}`

const deployableLossZkill = `{"killmail_id":139416795,"zkb":{"locationID":40447143,"hash":"cacac7bfe37cd8e30e0433c2aea5114403ab595b","totalValue":16572571.24,"npc":false,"solo":false,"awox":false}}`

// StubESIClient embeds the interface, answering every lookup the cache makes with placeholder data
type StubESIClient struct {
	api.ESIClient
}

func (m StubESIClient) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	return &api.ESICharacter{CharacterID: idNumber, Name: "Pilot", CorporationID: 98744491}, nil
}

func (m StubESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "Corp", AllianceID: 99013330}, nil
}

func (m StubESIClient) GetAllianceInfo(idNumber int) (*api.ESIAlliance, error) {
	return &api.ESIAlliance{AllianceID: idNumber, Name: "Alliance"}, nil
}

func (m StubESIClient) GetSystemInfo(idNumber int) (*api.ESISystem, error) {
	return &api.ESISystem{SystemID: idNumber, Name: "J-Space"}, nil
}

func (m StubESIClient) GetItemInfo(idNumber int) (*api.ESIItem, error) {
	return &api.ESIItem{ItemId: idNumber, Name: "Item"}, nil
}

func (m StubESIClient) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	names := map[int]string{}
	for _, idNumber := range idsNumbers {
		names[idNumber] = "Pilot"
	}
	return names, nil
}

func (m StubESIClient) GetCharacterAffiliations(characterIDs []int) (map[int]api.ESIAffiliation, error) {
	affiliations := map[int]api.ESIAffiliation{}
	for _, characterID := range characterIDs {
		affiliations[characterID] = api.ESIAffiliation{CharacterID: characterID, CorporationID: 98744491}
	}
	return affiliations, nil
}

func (m StubESIClient) GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error) {
	return nil, nil
}

// the deployable on killmail 139416795 belonged to Wormhole Campus in 10 Minutes' Notice
const victimCorpID = 98828090
const victimAllianceID = 99014851
const deployableTypeID = 12200
const astrahusTypeID = 35832

// deployableLoss returns a copy of killmail 139416795 with its id changed, so several can be collated together
func deployableLoss(t *testing.T, killmailID int) *api.ESIKillmail {
	var esiKM api.ESIKillmail
	assert.NoError(t, json.Unmarshal([]byte(deployableLossESI), &esiKM))
	esiKM.KillmailID = killmailID
	return &esiKM
}

// collate runs the esi killmails through CollateAllKillmailData with a stub ESI client and an empty cache
func collate(t *testing.T, esiKMs ...*api.ESIKillmail) *aggregate.CollectedData {
	var zkillKM api.ZkbResponse
	assert.NoError(t, json.Unmarshal([]byte(deployableLossZkill), &zkillKM))

	killmails := []retrieval.RawKillmail{}
	for _, esiKM := range esiKMs {
		// each killmail keeps its own id, the expanded killmail takes it from zKillboard's entry
		zkill := zkillKM
		zkill.KillmailID = esiKM.KillmailID
		killmails = append(killmails, retrieval.RawKillmail{Zkillmail: &zkill, ESIKillmail: esiKM})
	}

	var esiClient api.ESIClient = StubESIClient{}
	cache := retrieval.NewCache(esiClient, filepath.Join(t.TempDir(), "cache.json"))

	return aggregate.CollateAllKillmailData(&killmails, &esiClient, cache)
}

func TestCollateAllKillmailData(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("a deployable loss with no victim pilot is collated from its attackers", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 139416795))

		assert.Len(t, collected.PilotsSeen, 3)
		for _, pilot := range collected.PilotsSeen {
			assert.Equal(t, 1, pilot.TimesAttacker)
			assert.Equal(t, 0, pilot.TimesVictim)
		}
		assert.Contains(t, collected.SystemsVisted, 31001861)
	})

	t.Run("killmails in the same system are counted under that system", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 1), deployableLoss(t, 2))

		assert.Len(t, collected.SystemsVisted, 1)
		assert.Equal(t, 2, collected.SystemsVisted[31001861].TotalKillmails)
		assert.Equal(t, []int{1, 2}, collected.SystemsVisted[31001861].KillmailIDs)
	})
}

func TestUpdateOrAddStructures(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("a victim with no pilot is recorded as a destroyed structure owned by its corp and alliance", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 139416795))

		structures := collected.SystemsVisted[31001861].StructuresSeen
		assert.Len(t, structures, 1)

		deployable := structures["12200-98828090-99014851"]
		if assert.NotNil(t, deployable) {
			assert.Equal(t, deployableTypeID, deployable.Structure.ItemID)
			assert.Equal(t, victimCorpID, deployable.Corporation.CorporationID)
			assert.Equal(t, victimAllianceID, deployable.Alliance.AllianceID)
			assert.Equal(t, 1, deployable.Destroyed)
			assert.Equal(t, []int{139416795}, deployable.KillmailIDs)
		}
		assert.Len(t, collected.SystemsVisted[31001861].StructuresBashed, 1)
	})

	t.Run("the same corp is tracked separately for each alliance it was in", func(t *testing.T) {
		inAlliance := deployableLoss(t, 1)
		alsoInAlliance := deployableLoss(t, 2)
		alsoInAlliance.KillmailTime = alsoInAlliance.KillmailTime.Add(time.Hour)
		afterLeaving := deployableLoss(t, 3)
		afterLeaving.KillmailTime = afterLeaving.KillmailTime.Add(2 * time.Hour)
		afterLeaving.Victim.AllianceID = nil

		structures := collate(t, inAlliance, alsoInAlliance, afterLeaving).SystemsVisted[31001861].StructuresSeen

		assert.Len(t, structures, 2)
		assert.Equal(t, 2, structures["12200-98828090-99014851"].Destroyed)
		assert.Equal(t, 1, structures["12200-98828090-0"].Destroyed)
		assert.Nil(t, structures["12200-98828090-0"].Alliance)
	})

	t.Run("an upwell structure attacker is recorded as seen, not destroyed", func(t *testing.T) {
		killmail := deployableLoss(t, 139416795)
		corpID, allianceID := victimCorpID, victimAllianceID
		killmail.Attackers = append(killmail.Attackers, api.ESIAttacker{
			CorporationID: &corpID,
			AllianceID:    &allianceID,
			DamageDone:    1000,
			ShipTypeID:    astrahusTypeID,
		})

		structures := collate(t, killmail).SystemsVisted[31001861].StructuresSeen

		assert.Len(t, structures, 2)
		astrahus := structures["35832-98828090-99014851"]
		if assert.NotNil(t, astrahus) {
			assert.Equal(t, 0, astrahus.Destroyed)
			assert.Equal(t, 1, astrahus.TotalKillmails)
		}
	})
}

func TestIsTargetLoss(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	collected := collate(t, deployableLoss(t, 139416795))
	assert.Len(t, collected.Killmails, 1)
	killmail := collected.Killmails[0]

	cases := []struct {
		name   string
		target retrieval.IntelTarget
		loss   bool
	}{
		{"the victim's alliance lost it", retrieval.IntelTarget{IDNumber: victimAllianceID, TypeOfEntity: types.ALLIANCE}, true},
		{"the victim's corp lost it", retrieval.IntelTarget{IDNumber: victimCorpID, TypeOfEntity: types.CORPORATION}, true},
		{"an attacking pilot got a kill", retrieval.IntelTarget{IDNumber: 2121909368, TypeOfEntity: types.PILOT}, false},
		{"an attacking alliance got a kill", retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE}, false},
		{"the victim's corp is a holding of another alliance, which lost it", retrieval.IntelTarget{IDNumber: 99000777, TypeOfEntity: types.ALLIANCE,
			Holdings: []retrieval.TargetGroup{{Type: types.CORPORATION, ID: victimCorpID}}}, true},
		{"the victim's alliance is a holding of another corp, which lost it", retrieval.IntelTarget{IDNumber: 98000777, TypeOfEntity: types.CORPORATION,
			Holdings: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: victimAllianceID}}}, true},
		{"the attacking alliance as a holding got a kill", retrieval.IntelTarget{IDNumber: 99000777, TypeOfEntity: types.ALLIANCE,
			Holdings: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: 99013330}}}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.loss, aggregate.IsTargetLoss(&c.target, killmail))
		})
	}
}

func TestIsInternal(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	// the attackers on killmail 139416795 all flew for URSA PRIMUS in URSA.
	const attackerCorpID, attackerAllianceID = 98744491, 99013330
	killmail := collate(t, deployableLoss(t, 139416795)).Killmails[0]

	withNPC := deployableLoss(t, 139416796)
	npcCorp := 1000125
	withNPC.Attackers = append(withNPC.Attackers, api.ESIAttacker{CorporationID: &npcCorp, ShipTypeID: 2836})
	npcKillmail := collate(t, withNPC).Killmails[0]

	victimSide := func(holdings ...retrieval.TargetGroup) retrieval.IntelTarget {
		return retrieval.IntelTarget{IDNumber: victimAllianceID, TypeOfEntity: types.ALLIANCE, Holdings: holdings}
	}

	cases := []struct {
		name     string
		target   retrieval.IntelTarget
		killmail *retrieval.Killmail
		internal bool
	}{
		{"every attacker is a holding alliance of the victim's", victimSide(retrieval.TargetGroup{Type: types.ALLIANCE, ID: attackerAllianceID}), killmail, true},
		{"every attacker is a holding corp of the victim's", victimSide(retrieval.TargetGroup{Type: types.CORPORATION, ID: attackerCorpID}), killmail, true},
		{"the attackers are not the target's", victimSide(), killmail, false},
		{"an NPC attacker as well is from outside", victimSide(retrieval.TargetGroup{Type: types.ALLIANCE, ID: attackerAllianceID}), npcKillmail, false},
		{"the target got the kill, not the loss", retrieval.IntelTarget{IDNumber: attackerAllianceID, TypeOfEntity: types.ALLIANCE,
			Holdings: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: 99000777}}}, killmail, false},
		{"pilots are never internal", retrieval.IntelTarget{IDNumber: 2121909368, TypeOfEntity: types.PILOT}, killmail, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.internal, aggregate.IsInternal(&c.target, c.killmail))
		})
	}
}

func TestPilotLastSeen(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("first and last seen are the oldest and latest killmails even when collected out of time order", func(t *testing.T) {
		middle := deployableLoss(t, 1)
		oldest := deployableLoss(t, 2)
		oldest.KillmailTime = middle.KillmailTime.Add(-48 * time.Hour)
		newest := deployableLoss(t, 3)
		newest.KillmailTime = middle.KillmailTime.Add(24 * time.Hour)

		collected := collate(t, middle, oldest, newest)

		assert.Equal(t, oldest.KillmailTime, collected.PilotsSeen[2121909368].FirstSeen)
		assert.Equal(t, newest.KillmailTime, collected.PilotsSeen[2121909368].LastSeen)
	})
}

func TestGetTopMembers(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	// the three attackers on killmail 139416795 flew for this corp and alliance (URSA.)
	const attackerCorpID, attackerAllianceID = 98744491, 99013330

	memberIDs := func(members []aggregate.PilotSeen) []int {
		ids := []int{}
		for _, member := range members {
			ids = append(ids, member.Pilot.CharacterID)
		}
		return ids
	}

	t.Run("members are the pilots who flew for the target, most killmails first", func(t *testing.T) {
		// 2121909368 is on a second killmail, so ranks first
		second := deployableLoss(t, 2)
		second.KillmailTime = second.KillmailTime.Add(time.Hour)
		second.Attackers = second.Attackers[2:]

		collected := collate(t, deployableLoss(t, 1), second)
		alliance := retrieval.IntelTarget{IDNumber: attackerAllianceID, TypeOfEntity: types.ALLIANCE}

		members := aggregate.GetTopMembers(10, collected.PilotsSeen, &alliance)

		assert.Len(t, members, 3)
		assert.Equal(t, 2121909368, members[0].Pilot.CharacterID)
		assert.Equal(t, 2, members[0].TotalKillmails)
		assert.Len(t, aggregate.GetTopMembers(2, collected.PilotsSeen, &alliance), 2)
	})

	t.Run("pilots who flew for one of the target's holdings are members too", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 1))
		withHolding := retrieval.IntelTarget{IDNumber: 99000777, TypeOfEntity: types.ALLIANCE, Holdings: []retrieval.TargetGroup{{Type: types.CORPORATION, ID: attackerCorpID}}}
		without := retrieval.IntelTarget{IDNumber: 99000777, TypeOfEntity: types.ALLIANCE}

		assert.ElementsMatch(t, []int{2121911624, 2121877924, 2121909368}, memberIDs(aggregate.GetTopMembers(10, collected.PilotsSeen, &withHolding)))
		assert.Empty(t, aggregate.GetTopMembers(10, collected.PilotsSeen, &without))
	})

	t.Run("membership uses the corp and alliance on the killmail, so pilots who have since left still count", func(t *testing.T) {
		laterElsewhere := deployableLoss(t, 2)
		laterElsewhere.KillmailTime = laterElsewhere.KillmailTime.Add(24 * time.Hour)
		otherCorp, otherAlliance := 1000045, 99000001
		laterElsewhere.Attackers[0].CorporationID = &otherCorp
		laterElsewhere.Attackers[0].AllianceID = &otherAlliance

		collected := collate(t, deployableLoss(t, 1), laterElsewhere)
		corp := retrieval.IntelTarget{IDNumber: attackerCorpID, TypeOfEntity: types.CORPORATION}

		assert.ElementsMatch(t, []int{2121911624, 2121877924, 2121909368}, memberIDs(aggregate.GetTopMembers(10, collected.PilotsSeen, &corp)))
	})

	t.Run("the victim side has no members when the loss was a structure", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 1))
		victimCorp := retrieval.IntelTarget{IDNumber: victimCorpID, TypeOfEntity: types.CORPORATION}

		assert.Empty(t, aggregate.GetTopMembers(10, collected.PilotsSeen, &victimCorp))
	})

	t.Run("pilot targets have no members", func(t *testing.T) {
		collected := collate(t, deployableLoss(t, 1))
		pilot := retrieval.IntelTarget{IDNumber: 2121909368, TypeOfEntity: types.PILOT}

		assert.Empty(t, aggregate.GetTopMembers(10, collected.PilotsSeen, &pilot))
	})
}

func TestSystemTargetRankings(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("top pilots are every pilot on the killmails, without npc or structure attackers", func(t *testing.T) {
		killmail := deployableLoss(t, 1)
		killmail.Attackers = append(killmail.Attackers, api.ESIAttacker{DamageDone: 10, ShipTypeID: 23707}) // a rat with no character

		collected := collate(t, killmail)
		pilots := aggregate.GetTopPilots(10, collected.PilotsSeen)

		assert.Len(t, pilots, 3)
		for _, pilot := range pilots {
			assert.False(t, pilot.Pilot.NPC)
		}
	})

	t.Run("top corps count each killmail once per corp, as kills for attackers and losses for victims", func(t *testing.T) {
		first := deployableLoss(t, 1)
		second := deployableLoss(t, 2)
		second.KillmailTime = second.KillmailTime.Add(24 * time.Hour)

		corporations := aggregate.GetTopCorporations(10, collate(t, first, second).Killmails)

		assert.Len(t, corporations, 2)
		attackers, victims := corporations[0], corporations[1]
		if attackers.Corporation.CorporationID != 98744491 {
			attackers, victims = victims, attackers
		}
		assert.Equal(t, []int{2, 2, 0, 3, 2}, []int{attackers.Killmails, attackers.Kills, attackers.Losses, len(attackers.PilotIDs), attackers.UniqueDays()})
		assert.Equal(t, []int{2, 0, 2, 0}, []int{victims.Killmails, victims.Kills, victims.Losses, len(victims.PilotIDs)})
		assert.Equal(t, victimAllianceID, victims.Alliance.AllianceID)
	})

	t.Run("a corp on both sides of a killmail counts one killmail with a kill and a loss", func(t *testing.T) {
		awox := deployableLoss(t, 1)
		victimCorp := victimCorpID
		awox.Attackers[0].CorporationID = &victimCorp

		var victimSide aggregate.CorporationActivity
		for _, corporation := range aggregate.GetTopCorporations(10, collate(t, awox).Killmails) {
			if corporation.Corporation.CorporationID == victimCorpID {
				victimSide = corporation
			}
		}

		assert.Equal(t, []int{1, 1, 1}, []int{victimSide.Killmails, victimSide.Kills, victimSide.Losses})
	})

	t.Run("the alliance shown is from the corp's most recent killmail", func(t *testing.T) {
		older := deployableLoss(t, 1)
		older.KillmailTime = older.KillmailTime.Add(-48 * time.Hour)
		otherAlliance := 99000001
		older.Victim.AllianceID = &otherAlliance

		for _, corporation := range aggregate.GetTopCorporations(10, collate(t, deployableLoss(t, 2), older).Killmails) {
			if corporation.Corporation.CorporationID == victimCorpID {
				assert.Equal(t, victimAllianceID, corporation.Alliance.AllianceID)
			}
		}
	})
}
