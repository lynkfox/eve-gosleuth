package retrieval_test

import (
	"errors"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// MockZkillClient embeds the interface to satisfy its unexported methods, only GetZkillResults is implemented
type MockZkillClient struct {
	api.ZkillClient
	killmails []api.ZkbResponse
	err       error
}

func (m MockZkillClient) GetZkillResults(entityId int, entityType types.EntityType) (*[]api.ZkbResponse, error) {
	return &m.killmails, m.err
}

// MockESIClient embeds the interface, only GetKillmail is implemented
type MockESIClient struct {
	api.ESIClient
	failingIDs map[int]error
}

func (m MockESIClient) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	if err, found := m.failingIDs[zkillmail.KillmailID]; found {
		return nil, err
	}
	return &api.ESIKillmail{KillmailID: zkillmail.KillmailID}, nil
}

func CreateZkillmails(ids ...int) []api.ZkbResponse {
	killmails := []api.ZkbResponse{}
	for _, id := range ids {
		killmails = append(killmails, api.ZkbResponse{KillmailID: id})
	}
	return killmails
}

func CreateTarget(zkill MockZkillClient, esi MockESIClient) retrieval.IntelTarget {
	return retrieval.IntelTarget{
		IDNumber:     12345,
		TypeOfEntity: types.PILOT,
		Clients: retrieval.CollectionClientPool{
			ZkillClient: zkill,
			ESIClient:   esi,
		},
	}
}

func TestCollectKillmails(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("all killmails retrieved", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3)}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(*killmails))
		for index, km := range *killmails {
			assert.Equal(t, index+1, km.Zkillmail.KillmailID)
			assert.Equal(t, index+1, km.ESIKillmail.KillmailID)
		}
	})

	t.Run("partial zkill pull returns collected killmails and error", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2), err: api.ErrOnlyPartialZkillPull}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target)

		assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.Equal(t, 2, len(*killmails))
	})

	t.Run("failed esi killmail is skipped", func(t *testing.T) {
		esiErr := errors.New("esi failure")
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2, 3)}, MockESIClient{failingIDs: map[int]error{2: esiErr}})

		killmails, err := retrieval.CollectKillmails(target)

		assert.ErrorIs(t, err, retrieval.ErrIncompleteKillmails)
		assert.ErrorIs(t, err, esiErr)
		assert.Equal(t, 2, len(*killmails))
		assert.Equal(t, 1, (*killmails)[0].ESIKillmail.KillmailID)
		assert.Equal(t, 3, (*killmails)[1].ESIKillmail.KillmailID)
	})

	t.Run("partial zkill pull and failed esi killmail are both reported", func(t *testing.T) {
		esiErr := errors.New("esi failure")
		target := CreateTarget(MockZkillClient{killmails: CreateZkillmails(1, 2), err: api.ErrOnlyPartialZkillPull}, MockESIClient{failingIDs: map[int]error{1: esiErr}})

		killmails, err := retrieval.CollectKillmails(target)

		assert.ErrorIs(t, err, api.ErrOnlyPartialZkillPull)
		assert.ErrorIs(t, err, esiErr)
		assert.Equal(t, 1, len(*killmails))
		assert.Equal(t, 2, (*killmails)[0].ESIKillmail.KillmailID)
	})

	t.Run("no killmails found", func(t *testing.T) {
		target := CreateTarget(MockZkillClient{killmails: []api.ZkbResponse{}}, MockESIClient{})

		killmails, err := retrieval.CollectKillmails(target)

		assert.NoError(t, err)
		assert.Empty(t, *killmails)
	})
}
