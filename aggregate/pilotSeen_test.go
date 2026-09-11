package aggregate_test

import (
	"encoding/json"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/stretchr/testify/assert"
)

func TestPilotSeen(t *testing.T) {
	pilot := retrieval.Pilot{
		Name: "John Doe",
		Corporation: &retrieval.Corporation{
			Name:  "JD's Corp",
			CeoID: 12345678899,
			Alliance: &retrieval.Alliance{
				Name: "JD's Corp Alliance",
			},
		}}
	pilotSeen := aggregate.PilotSeen{
		Pilot:          &pilot,
		TotalKillmails: 10,
		TimesAttacker:  5,
		TimesVictim:    2,
		UniqueDays:     3,
	}

	pilots := map[int]aggregate.PilotSeen{123334: pilotSeen}

	aggregate.GetMostFrequentFleetmates(1, pilots, &retrieval.IntelTarget{})

	jsonData, err := json.Marshal(pilotSeen)

	assert.NoError(t, err)
	assert.NotNil(t, jsonData)
	assert.Contains(t, string(jsonData), "\"name\":\"John Doe\"")

}
