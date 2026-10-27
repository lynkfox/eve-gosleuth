package retrieval_test

import (
	"fmt"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// MockTargetESIClient embeds the interface, only the name lookup and info calls used by GetTargetInfo are implemented
type MockTargetESIClient struct {
	api.ESIClient
	matches  map[int]api.EntityIDAndName
	idsError error // returned by GetEveIDsFromNames when set, as if the ESI failed
}

func (m MockTargetESIClient) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	return m.matches, m.idsError
}

func (m MockTargetESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "Umbrian Dawn"}, nil
}

func (m MockTargetESIClient) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	return &api.ESICharacter{CharacterID: idNumber, Name: "CCP Zoetrope"}, nil
}

// RecordingZkillClient records the entity it was asked for
type RecordingZkillClient struct {
	api.ZkillClient
	requestedID   int
	requestedType types.EntityType
}

func (m *RecordingZkillClient) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	m.requestedID = entityId
	m.requestedType = entityType
	return api.ZkillList{Ended: true}, nil
}

func TestGetTargetInfo(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("an ESI failure says why rather than that nothing was found", func(t *testing.T) {
		failure := fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "esi.evetech.net", Status: 503, Message: "The datasource tranquility is temporarily unavailable"})
		target := retrieval.IntelTarget{
			Name:    "Luxxianna Seraph",
			Clients: retrieval.CollectionClientPool{ESIClient: MockTargetESIClient{idsError: failure}},
		}

		target.GetTargetInfo()

		assert.Error(t, target.Error)
		assert.Equal(t, "Could not search the ESI: esi.evetech.net returned 503 Service Unavailable: The datasource tranquility is temporarily unavailable", target.DisplayMessage)
	})

	t.Run("no match sets an error", func(t *testing.T) {
		target := retrieval.IntelTarget{
			Name:    "Not A Real Name",
			Clients: retrieval.CollectionClientPool{ESIClient: MockTargetESIClient{matches: map[int]api.EntityIDAndName{}}},
		}

		target.GetTargetInfo()

		assert.Error(t, target.Error)
		assert.Equal(t, "Nothing found by that name. Check spelling?", target.DisplayMessage)
		assert.Equal(t, 0, target.IDNumber)
		assert.Nil(t, target.Data)
	})

	t.Run("single match sets id, type, and data, and the id is used for zkill", func(t *testing.T) {
		zkill := &RecordingZkillClient{}
		target := retrieval.IntelTarget{
			Name: "Umbrian Dawn",
			Clients: retrieval.CollectionClientPool{
				ZkillClient: zkill,
				ESIClient: MockTargetESIClient{matches: map[int]api.EntityIDAndName{
					98739641: {ID: 98739641, Name: "Umbrian Dawn", Category: "corporations"},
				}},
			},
		}

		target.GetTargetInfo()

		assert.NoError(t, target.Error)
		assert.Equal(t, 98739641, target.IDNumber)
		assert.Equal(t, types.CORPORATION, target.TypeOfEntity)
		assert.Equal(t, "Umbrian Dawn", target.Data.(*api.ESICorporation).Name)

		_, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())

		assert.NoError(t, err)
		assert.Equal(t, 98739641, zkill.requestedID)
		assert.Equal(t, types.CORPORATION, zkill.requestedType)
	})
}

func TestGetTargetInfoOfType(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	sharedName := MockTargetESIClient{matches: map[int]api.EntityIDAndName{
		98739641:   {ID: 98739641, Name: "Umbrian Dawn", Category: "corporations"},
		2121139046: {ID: 2121139046, Name: "Umbrian Dawn", Category: "characters"},
	}}

	t.Run("picks the match of the searched type when the name is shared", func(t *testing.T) {
		target := retrieval.IntelTarget{Name: "Umbrian Dawn", Clients: retrieval.CollectionClientPool{ESIClient: sharedName}}

		target.GetTargetInfoOfType(types.CORPORATION)

		assert.NoError(t, target.Error)
		assert.Equal(t, 98739641, target.IDNumber)
		assert.Equal(t, types.CORPORATION, target.TypeOfEntity)
		assert.Equal(t, "Umbrian Dawn", target.Data.(*api.ESICorporation).Name)
	})

	t.Run("no match of the searched type sets an error", func(t *testing.T) {
		target := retrieval.IntelTarget{Name: "Umbrian Dawn", Clients: retrieval.CollectionClientPool{ESIClient: sharedName}}

		target.GetTargetInfoOfType(types.ALLIANCE)

		assert.Error(t, target.Error)
		assert.Equal(t, "No alliance found by that name. Check spelling?", target.DisplayMessage)
		assert.Equal(t, 0, target.IDNumber)
		assert.Nil(t, target.Data)
	})
}

func TestIntelTargetIsGroup(t *testing.T) {
	alliance := retrieval.IntelTarget{IDNumber: 99000001, TypeOfEntity: types.ALLIANCE, Holdings: []retrieval.TargetGroup{
		{Type: types.CORPORATION, ID: 98000002}, {Type: types.ALLIANCE, ID: 99000003},
	}}

	assert.True(t, alliance.IsGroup(98000009, 99000001), "the alliance itself")
	assert.True(t, alliance.IsGroup(98000002, 0), "a holding corp in no alliance")
	assert.True(t, alliance.IsGroup(98000002, 99000777), "a holding corp in another alliance")
	assert.True(t, alliance.IsGroup(98000009, 99000003), "a corp in a holding alliance")
	assert.False(t, alliance.IsGroup(98000009, 99000777))
	assert.False(t, alliance.IsGroup(0, 0))
	assert.False(t, alliance.IsGroup(99000001, 0), "an alliance id as a corp is not the alliance")

	corp := retrieval.IntelTarget{IDNumber: 98000001, TypeOfEntity: types.CORPORATION}
	assert.True(t, corp.IsGroup(98000001, 99000777))
	assert.False(t, corp.IsGroup(98000002, 0))

	pilot := retrieval.IntelTarget{IDNumber: 98000001, TypeOfEntity: types.PILOT, Holdings: alliance.Holdings}
	assert.False(t, pilot.IsGroup(98000002, 0), "pilots have no groups")
}

func TestTargetGroupIncludes(t *testing.T) {
	corp := retrieval.TargetGroup{Type: types.CORPORATION, ID: 98000002}
	alliance := retrieval.TargetGroup{Type: types.ALLIANCE, ID: 99000003}

	assert.True(t, corp.Includes(98000002, 99000777))
	assert.False(t, corp.Includes(99000003, 98000002), "a corp id as an alliance is not the corp")
	assert.True(t, alliance.Includes(98000009, 99000003), "any corp in the alliance")
	assert.False(t, alliance.Includes(99000003, 0))
	assert.False(t, retrieval.TargetGroup{Type: types.CORPORATION}.Includes(0, 0), "0 is never a group")
}

func TestMatchTargets(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("a pilot sharing its name with a ship type is the only match, the ship noted", func(t *testing.T) {
		client := MockTargetESIClient{matches: map[int]api.EntityIDAndName{
			95202785: {Category: "characters", ID: 95202785, Name: "Astero"},
			33468:    {Category: "inventory_types", ID: 33468, Name: "Astero"},
		}}

		found, err := retrieval.MatchTargets(client, "astero")

		assert.NoError(t, err)
		assert.Equal(t, retrieval.TargetMatches{Matches: []retrieval.TargetMatch{{Type: types.PILOT, ID: 95202785, Name: "Astero"}}, Item: true}, found)
	})

	t.Run("corporations and alliances with the name as their ticker come after those with the name", func(t *testing.T) {
		// as the ESI answers Jita
		client := MockTargetESIClient{matches: map[int]api.EntityIDAndName{
			99005382:  {Category: "alliances", ID: 99005382, Name: "Jita Holding Inc."},
			383768304: {Category: "corporations", ID: 383768304, Name: "jion ss Corp"},
			30000142:  {Category: "systems", ID: 30000142, Name: "Jita"},
		}}

		found, err := retrieval.MatchTargets(client, "jita")

		assert.NoError(t, err)
		assert.Equal(t, []retrieval.TargetMatch{
			{Type: types.SYSTEM, ID: 30000142, Name: "Jita"},
			{Type: types.CORPORATION, ID: 383768304, Name: "jion ss Corp", ByTicker: true},
			{Type: types.ALLIANCE, ID: 99005382, Name: "Jita Holding Inc.", ByTicker: true},
		}, found.Matches)
	})

	t.Run("matches are listed pilots, corporations, alliances, then systems, each by name", func(t *testing.T) {
		client := MockTargetESIClient{matches: map[int]api.EntityIDAndName{
			30000001: {Category: "systems", ID: 30000001, Name: "Aster"},
			99000001: {Category: "alliances", ID: 99000001, Name: "aster"},
			95202785: {Category: "characters", ID: 95202785, Name: "Aster"},
			98000002: {Category: "corporations", ID: 98000002, Name: "ASTER"},
		}}

		found, err := retrieval.MatchTargets(client, "Aster")

		assert.NoError(t, err)
		kinds := []types.EntityType{}
		for _, match := range found.Matches {
			kinds = append(kinds, match.Type)
		}
		assert.Equal(t, []types.EntityType{types.PILOT, types.CORPORATION, types.ALLIANCE, types.SYSTEM}, kinds)
	})

	t.Run("an item without the exact name is not noted", func(t *testing.T) {
		found, _ := retrieval.MatchTargets(MockTargetESIClient{matches: map[int]api.EntityIDAndName{33468: {Category: "inventory_types", ID: 33468, Name: "Astero"}}}, "Aster")
		assert.False(t, found.Item)
	})

	t.Run("no match is an empty list, and an ESI failure an error", func(t *testing.T) {
		found, err := retrieval.MatchTargets(MockTargetESIClient{}, "Nobody")
		assert.NoError(t, err)
		assert.Empty(t, found.Matches)

		_, err = retrieval.MatchTargets(MockTargetESIClient{idsError: api.ErrRequestFailedToComplete}, "Nobody")
		assert.Error(t, err)
	})
}

func TestGetTargetInfoOfTypePrefersExactName(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("of two corporations the ESI returns, the one with the exact name is the target", func(t *testing.T) {
		client := MockTargetESIClient{matches: map[int]api.EntityIDAndName{
			148134386: {Category: "corporations", ID: 148134386, Name: "Amarr Trading and Manufactoring"},
			98000001:  {Category: "corporations", ID: 98000001, Name: "Amarr"},
			30002187:  {Category: "systems", ID: 30002187, Name: "Amarr"},
		}}
		// map order is random, so ask enough times to see either corporation first
		for range 20 {
			target := retrieval.IntelTarget{Name: "amarr", Clients: retrieval.CollectionClientPool{ESIClient: client}}
			target.GetTargetInfoOfType(types.CORPORATION)
			assert.Equal(t, 98000001, target.IDNumber)
		}
	})
}
