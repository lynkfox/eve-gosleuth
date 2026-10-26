package retrieval_test

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// killmailWith builds a killmail in J105443 with a victim and attackers. A zero attacker id is an NPC, with no character.
func killmailWith(killmailID, victimID int, attackerIDs ...int) retrieval.RawKillmail {
	ptr := func(id int) *int { return &id }
	esiKM := &api.ESIKillmail{KillmailID: killmailID, SolarSystemID: 31001861}
	esiKM.Victim = api.ESIVictim{CharacterID: ptr(victimID), CorporationID: ptr(98000001), AllianceID: ptr(99000001), ShipTypeID: 29990}
	for _, attackerID := range attackerIDs {
		attacker := api.ESIAttacker{CorporationID: ptr(98000002), ShipTypeID: 29988, WeaponTypeID: ptr(2478)}
		if attackerID != 0 {
			attacker.CharacterID = ptr(attackerID)
		}
		esiKM.Attackers = append(esiKM.Attackers, attacker)
	}
	return retrieval.RawKillmail{Zkillmail: &api.ZkbResponse{KillmailID: killmailID}, ESIKillmail: esiKM}
}

func TestPrefetchKillmailEntities(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	killmails := []retrieval.RawKillmail{killmailWith(1, 101, 102, 103, 0), killmailWith(2, 102, 101, 104)}

	newCache := func(t *testing.T) (*retrieval.CachedEveIDs, *RawESIClient) {
		raw := &RawESIClient{}
		return retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cache.json")), raw
	}

	t.Run("pilots are looked up in bulk, everything else once each", func(t *testing.T) {
		cache, raw := newCache(t)

		cache.PrefetchKillmailEntities(killmails)

		assert.Equal(t, 1, raw.Calls("names"))
		assert.Equal(t, 1, raw.Calls("affiliations"))
		assert.ElementsMatch(t, []int{101, 102, 103, 104}, raw.names)
		assert.Equal(t, 0, raw.Calls("character"), "no pilot is looked up on its own")
		assert.Equal(t, 3, raw.Calls("corporation"), "both corps on the killmails and the pilots' current corp")
		assert.Equal(t, 2, raw.Calls("alliance"), "the victims' alliance and the pilots' current alliance")
		assert.Equal(t, 3, raw.Calls("item"), "two ships and a weapon")
		assert.Equal(t, 1, raw.Calls("system"))
	})

	t.Run("expanding the killmails afterwards needs no more calls", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.PrefetchKillmailEntities(killmails)
		calls := raw.total()

		for _, km := range killmails {
			assert.NotNil(t, km.Expand(nil, cache))
		}
		pilot := cache.ExpandPilot(103, true)

		assert.Equal(t, calls, raw.total())
		assert.Equal(t, "Corp", pilot.Name, "named by universe/names")
		assert.Equal(t, 98744491, pilot.Corporation.CorporationID, "their current corp, from characters/affiliation")
		assert.Equal(t, 99013330, pilot.Corporation.Alliance.AllianceID)
	})

	t.Run("a second prefetch finds everything cached", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.PrefetchKillmailEntities(killmails)
		calls := raw.total()

		cache.PrefetchKillmailEntities(killmails)

		assert.Equal(t, calls, raw.total())
	})

	t.Run("if the bulk call fails, pilots are looked up one at a time when expanded", func(t *testing.T) {
		cache, raw := newCache(t)
		raw.failing = map[string]bool{"affiliations": true}

		cache.PrefetchKillmailEntities(killmails)
		assert.Equal(t, 0, raw.Calls("character"))

		pilot := cache.ExpandPilot(101, true)

		assert.Equal(t, 1, raw.Calls("character"))
		assert.Equal(t, "Luxxianna Seraph", pilot.Name)
	})
}

func TestExpandFailures(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("a ship type the ESI fails for is unknown, the killmail is kept, and it is tried again next time", func(t *testing.T) {
		raw := &RawESIClient{failing: map[string]bool{"item": true}}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cache.db"))

		raw1 := killmailWith(1, 101, 102)
		killmail := raw1.Expand(nil, cache)

		if assert.NotNil(t, killmail) {
			assert.Equal(t, "(Unknown type)", killmail.Attackers[0].Ship.Name)
		}
		raw.mu.Lock()
		raw.failing = map[string]bool{}
		raw.mu.Unlock()
		assert.Equal(t, "Loki", cache.ExpandItem(29988).Name)
	})

	t.Run("a corp in no alliance is built once, not again on every lookup", func(t *testing.T) {
		cache := retrieval.NewCache(&CountingESIClient{}, filepath.Join(t.TempDir(), "cache.db"))

		first := cache.ExpandCorporation(testCorporationID, true)
		second := cache.ExpandCorporation(testCorporationID, true)

		assert.Nil(t, first.Alliance)
		assert.True(t, first.AllianceChecked)
		assert.Same(t, first, second, "the second lookup is the cached model")
	})

	t.Run("an alliance is cached as its readable model too", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))

		cache.ExpandAlliance(99013330)

		assert.NotNil(t, retrieval.FetchExpanded[retrieval.Alliance](cache, 99013330))
	})
}

func TestExpandTypeZero(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	raw := &RawESIClient{}
	cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cache.db"))

	// a killmail gives type 0 for an attacker with no known ship or weapon, the ESI has no real type for it
	for range 3 {
		assert.Equal(t, "Unknown", cache.ExpandItem(0).Name)
	}

	assert.Equal(t, 0, raw.Calls("item"), "never asked for, rather than asked again on every killmail")
}
