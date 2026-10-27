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
		assert.Equal(t, "No alliance found by that name. Check spelling or search type?", target.DisplayMessage)
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
