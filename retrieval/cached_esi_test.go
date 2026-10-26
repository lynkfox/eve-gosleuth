package retrieval_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// RawESIClient stands in for the uncached ESI, counting each call. Safe for concurrent use.
type RawESIClient struct {
	api.ESIClient
	mu      sync.Mutex
	calls   map[string]int
	names   []int           // the ids asked for by the last GetNamesFromEveIDs
	failing map[string]bool // calls that return an error
	deleted map[int]bool    // characters the ESI answers 404 for, as for a biomassed character
}

// fails reports whether a call should return an error, set with failing
func (m *RawESIClient) fails(call string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failing[call]
}

func (m *RawESIClient) count(call string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		m.calls = map[string]int{}
	}
	m.calls[call]++
}

func (m *RawESIClient) Calls(call string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[call]
}

func (m *RawESIClient) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	m.count("character")
	if m.deleted[idNumber] {
		return nil, fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "esi.evetech.net", Status: 404, Message: "Character has been deleted!"})
	}
	if m.fails("character") {
		return nil, api.ErrRequestFailedToComplete
	}
	return &api.ESICharacter{CharacterID: idNumber, Name: "Luxxianna Seraph"}, nil
}

func (m *RawESIClient) GetSystemInfo(idNumber int) (*api.ESISystem, error) {
	m.count("system")
	return &api.ESISystem{SystemID: idNumber, Name: "Jita", ConstellationID: 20000020}, nil
}

func (m *RawESIClient) GetStargateInfo(idNumber int) (*api.ESIStargate, error) {
	m.count("stargate")
	stargate := &api.ESIStargate{StargateID: idNumber}
	stargate.Destination.SystemID = 30000144
	return stargate, nil
}

func (m *RawESIClient) GetAllianceInfo(idNumber int) (*api.ESIAlliance, error) {
	m.count("alliance")
	return &api.ESIAlliance{AllianceID: idNumber, Name: "URSA."}, nil
}

func (m *RawESIClient) GetCorporationHistory(characterID int) (*api.ESICorporationHistory, error) {
	m.count("corporation history")
	return &api.ESICorporationHistory{CharacterID: characterID, Records: []api.ESICorporationHistoryItem{{CorporationID: 98744491}}}, nil
}

func (m *RawESIClient) GetAllianceCorporations(allianceID int) (*api.ESIAllianceCorporations, error) {
	m.count("alliance corporations")
	return &api.ESIAllianceCorporations{AllianceID: allianceID, CorporationIDs: []int{98744491}}, nil
}

func (m *RawESIClient) GetStarInfo(idNumber int) (*api.ESIStar, error) {
	m.count("star")
	return &api.ESIStar{SolarSystemID: 30000142, TypeID: 45041}, nil
}

func (m *RawESIClient) GetPlanetInfo(idNumber int) (*api.ESIPlanetInfo, error) {
	m.count("planet")
	return &api.ESIPlanetInfo{PlanetID: idNumber, Name: "Jita IV", SystemID: 30000142, TypeID: 11}, nil
}

func (m *RawESIClient) GetCategoryInfo(idNumber int) (*api.ESICategory, error) {
	m.count("category")
	return &api.ESICategory{CategoryID: idNumber, Name: "Ship"}, nil
}

func (m *RawESIClient) GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error) {
	m.count(typeOfEntity.String())
	if typeOfEntity == types.REGION {
		return &api.ESIRegion{RegionID: int64(idNumber), Name: "The Forge"}, nil
	}
	return &api.ESIConstellation{ConstellationID: idNumber, RegionID: 10000002, Name: "Kimotoro"}, nil
}

func (m *RawESIClient) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	m.count("killmail")
	if m.fails("killmail") {
		return nil, api.ErrRequestFailedToComplete
	}
	return &api.ESIKillmail{KillmailID: zkillmail.KillmailID, SolarSystemID: 31001861}, nil
}

func (m *RawESIClient) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	m.count("ids")
	if names[0] == "Nobody" {
		return map[int]api.EntityIDAndName{}, nil
	}
	return map[int]api.EntityIDAndName{2121139046: {ID: 2121139046, Name: names[0], Category: "characters"}}, nil
}

func (m *RawESIClient) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	m.count("names")
	m.mu.Lock()
	m.names = idsNumbers
	m.mu.Unlock()
	names := map[int]string{}
	for _, idNumber := range idsNumbers {
		names[idNumber] = "Corp"
	}
	return names, nil
}

func (m *RawESIClient) GetCharacterAffiliations(characterIDs []int) (map[int]api.ESIAffiliation, error) {
	m.count("affiliations")
	if m.fails("affiliations") {
		return map[int]api.ESIAffiliation{}, api.ErrRequestFailedToComplete
	}
	affiliations := map[int]api.ESIAffiliation{}
	for _, characterID := range characterIDs {
		affiliations[characterID] = api.ESIAffiliation{CharacterID: characterID, CorporationID: 98744491, AllianceID: 99013330}
	}
	return affiliations, nil
}

func (m *RawESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	m.count("corporation")
	return &api.ESICorporation{CorporationID: idNumber, Name: "URSA PRIMUS", AllianceID: 99013330}, nil
}

func (m *RawESIClient) GetItemInfo(idNumber int) (*api.ESIItem, error) {
	m.count("item")
	if m.fails("item") {
		return nil, api.ErrRequestFailedToComplete
	}
	return &api.ESIItem{ItemId: idNumber, Name: "Loki"}, nil
}

// total is every call made so far
func (m *RawESIClient) total() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	sum := 0
	for _, count := range m.calls {
		sum += count
	}
	return sum
}

func TestCachedESIClient(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	newCache := func(t *testing.T) (*retrieval.CachedEveIDs, *RawESIClient) {
		raw := &RawESIClient{}
		return retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json")), raw
	}

	t.Run("entity, stargate, star, and constellation lookups reach the ESI once", func(t *testing.T) {
		cache, raw := newCache(t)
		var client api.ESIClient = cache

		for range 3 {
			character, err := client.GetCharacterInfo(2121139046)
			assert.NoError(t, err)
			assert.Equal(t, "Luxxianna Seraph", character.Name)

			stargate, err := client.GetStargateInfo(50001248)
			assert.NoError(t, err)
			assert.Equal(t, 30000144, stargate.Destination.SystemID)

			corps, err := client.GetAllianceCorporations(99013330)
			assert.NoError(t, err)
			assert.Equal(t, []int{98744491}, corps.CorporationIDs)

			history, err := client.GetCorporationHistory(2121139046)
			assert.NoError(t, err)
			assert.Equal(t, 98744491, history.Records[0].CorporationID)

			alliance, err := client.GetAllianceInfo(99013330)
			assert.NoError(t, err)
			assert.Equal(t, 99013330, alliance.AllianceID, "the corp list does not overwrite the alliance cached under the same id")

			star, err := client.GetStarInfo(40009076)
			assert.NoError(t, err)
			assert.Equal(t, 45041, star.TypeID)

			constellation, err := client.GetEsiResponse(20000020, types.CONSTELLATION)
			assert.NoError(t, err)
			assert.Equal(t, "Kimotoro", constellation.(*api.ESIConstellation).Name)
		}

		assert.Equal(t, 1, raw.Calls("character"))
		assert.Equal(t, 1, raw.Calls("stargate"))
		assert.Equal(t, 1, raw.Calls("star"))
		assert.Equal(t, 1, raw.Calls("alliance corporations"))
		assert.Equal(t, 1, raw.Calls("corporation history"))
		assert.Equal(t, 1, raw.Calls("alliance"))
		assert.Equal(t, 1, raw.Calls("CONSTELLATION"))
	})

	t.Run("killmails are cached and survive a new SDE release", func(t *testing.T) {
		cache, raw := newCache(t)
		zkillmail := api.ZkbResponse{KillmailID: 139416795}

		_, err := cache.GetKillmail(zkillmail)
		assert.NoError(t, err)
		cache.UpdateSDERelease(time.Now().UTC().Add(time.Hour))
		killmail, err := cache.GetKillmail(zkillmail)

		assert.NoError(t, err)
		assert.Equal(t, 139416795, killmail.KillmailID)
		assert.Equal(t, 1, raw.Calls("killmail"))
		assert.True(t, cache.Killmails[139416795].Permanent)
	})

	t.Run("name searches are cached, but searches that find nothing are not", func(t *testing.T) {
		cache, raw := newCache(t)

		for range 2 {
			found, err := cache.GetEveIDsFromNames([]string{"Luxxianna Seraph"})
			assert.NoError(t, err)
			assert.Contains(t, found, 2121139046)

			_, err = cache.GetEveIDsFromNames([]string{"Nobody"})
			assert.NoError(t, err)
		}

		assert.Equal(t, 3, raw.Calls("ids"))
	})

	t.Run("id to name lookups only ask the ESI for ids not already cached", func(t *testing.T) {
		cache, raw := newCache(t)

		_, err := cache.GetNamesFromEveIDs([]int{98744491, 1000045})
		assert.NoError(t, err)
		names, err := cache.GetNamesFromEveIDs([]int{98744491, 1000045, 98739641})

		assert.NoError(t, err)
		assert.Len(t, names, 3)
		assert.Equal(t, 2, raw.Calls("names"))
		assert.Equal(t, []int{98739641}, raw.names)
	})

	t.Run("killmails, name searches, and names are typed again after a save and load", func(t *testing.T) {
		cache, raw := newCache(t)
		_, _ = cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})
		_, _ = cache.GetEveIDsFromNames([]string{"Luxxianna Seraph"})
		_, _ = cache.GetNamesFromEveIDs([]int{98744491})
		assert.NoError(t, retrieval.SaveCache(cache))

		loaded, err := retrieval.LoadCache(raw, cache.Path)
		assert.NoError(t, err)

		killmail, err := loaded.GetKillmail(api.ZkbResponse{KillmailID: 139416795})
		assert.NoError(t, err)
		assert.Equal(t, 31001861, killmail.SolarSystemID)
		found, _ := loaded.GetEveIDsFromNames([]string{"Luxxianna Seraph"})
		assert.Equal(t, "characters", found[2121139046].Category)
		names, _ := loaded.GetNamesFromEveIDs([]int{98744491})
		assert.Equal(t, "Corp", names[98744491])

		assert.Equal(t, 1, raw.Calls("killmail"))
		assert.Equal(t, 1, raw.Calls("ids"))
		assert.Equal(t, 1, raw.Calls("names"))
	})

	t.Run("concurrent lookups are safe", func(t *testing.T) {
		cache, _ := newCache(t)

		var wait sync.WaitGroup
		for index := range 20 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				_, _ = cache.GetStargateInfo(50000000 + index%5)
				_, _ = cache.GetNamesFromEveIDs([]int{98744491 + index%3})
				_, _ = cache.GetKillmail(api.ZkbResponse{KillmailID: index % 4})
			}()
		}
		wait.Wait()

		assert.Len(t, cache.Known, 5)
		assert.Len(t, cache.Killmails, 4)
	})
}

func TestCacheCounts(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("lookups are counted as cached or fetched", func(t *testing.T) {
		raw := &RawESIClient{}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		start := cache.Counts()
		_, _ = cache.GetCharacterInfo(2121139046)
		_, _ = cache.GetCharacterInfo(2121139046)
		_, _ = cache.GetNamesFromEveIDs([]int{1, 2})
		_, _ = cache.GetNamesFromEveIDs([]int{1, 2, 3})

		assert.Equal(t, retrieval.CacheCounts{Cached: 3, Fetched: 4}, cache.Counts().Since(start))
		assert.Equal(t, retrieval.CacheCounts{}, retrieval.CountsOf(raw))
	})

	t.Run("the source counts hits from cache and the ESI, and why any failed", func(t *testing.T) {
		assert.Equal(t, "", retrieval.CacheCounts{}.Source())
		assert.Equal(t, "4 hits from cache", retrieval.CacheCounts{Cached: 4}.Source())
		assert.Equal(t, "1 from the ESI", retrieval.CacheCounts{Fetched: 1}.Source())
		assert.Equal(t, "95 hits from cache, 25 from the ESI", retrieval.CacheCounts{Cached: 95, Fetched: 25}.Source())
		assert.Equal(t, "95 hits from cache, 25 from the ESI, 2 failed: esi.evetech.net returned 502 Bad Gateway",
			retrieval.CacheCounts{Cached: 95, Fetched: 25, Failed: 2, Reason: "esi.evetech.net returned 502 Bad Gateway"}.Source())
		assert.Equal(t, "Killmail details 3/20 (1 hit from cache)", retrieval.CacheCounts{Cached: 1}.Annotate("Killmail details 3/20"))
		assert.Equal(t, "Killmail details 3/20", retrieval.CacheCounts{}.Annotate("Killmail details 3/20"))
	})

	t.Run("failed lookups are counted with the latest reason", func(t *testing.T) {
		raw := &RawESIClient{failing: map[string]bool{"character": true}}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		before := cache.Counts()
		_, _ = cache.GetCharacterInfo(2121139046)
		_, _ = cache.GetSystemInfo(30000142)

		assert.Equal(t, retrieval.CacheCounts{Fetched: 2, Failed: 1, Reason: api.ErrRequestFailedToComplete.Error()}, cache.Counts().Since(before))
		assert.Equal(t, retrieval.CacheCounts{}, cache.Counts().Since(cache.Counts()), "no failures since now, so no reason")
	})

	t.Run("killmail progress says whether each killmail came from the cache", func(t *testing.T) {
		raw := &RawESIClient{}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))
		target := retrieval.IntelTarget{IDNumber: 1, Clients: retrieval.CollectionClientPool{
			ZkillClient: MockZkillClient{killmails: CreateZkillmails(1, 2)},
			ESIClient:   cache,
		}}

		messages := []string{}
		comm.SetProgressReporter(func(message string) { messages = append(messages, message) })
		defer comm.SetProgressReporter(nil)

		_, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())
		assert.NoError(t, err)
		_, err = retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())
		assert.NoError(t, err)

		assert.Contains(t, messages, "Killmail details 1/2 #1 (1 from the ESI)")
		assert.Contains(t, messages, "Killmail details 2/2 #2 (1 hit from cache)")
		assert.Contains(t, messages, "Fetching killmail list from zKillboard (live, zKillboard is not cached)")
	})
}

func TestCachedEntityIDsSurviveReload(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("pilot, corporation, and alliance ids are set when served from a reloaded cache", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cached_data.json")
		cache := retrieval.NewCache(&RawESIClient{}, path)
		// the ESI responses do not include their own id, so it is never saved
		cache.Update(2121139046, &api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph"})
		cache.Update(98744491, &api.ESICorporation{CorporationID: 98744491, Name: "URSA PRIMUS"})
		cache.Update(99013330, &api.ESIAlliance{AllianceID: 99013330, Name: "URSA."})
		assert.NoError(t, retrieval.SaveCache(cache))

		loaded, err := retrieval.LoadCache(&RawESIClient{}, path)
		assert.NoError(t, err)

		character, err := loaded.GetCharacterInfo(2121139046)
		assert.NoError(t, err)
		corporation, err := loaded.GetCorporationInfo(98744491)
		assert.NoError(t, err)
		alliance, err := loaded.GetAllianceInfo(99013330)
		assert.NoError(t, err)

		assert.Equal(t, 2121139046, character.CharacterID)
		assert.Equal(t, 98744491, corporation.CorporationID)
		assert.Equal(t, 99013330, alliance.AllianceID)
		assert.Equal(t, 0, loaded.Client.(*RawESIClient).Calls("character"))
	})
}

func TestCachedMissingIDsAreRefetched(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	newCache := func(t *testing.T) (*retrieval.CachedEveIDs, *RawESIClient) {
		raw := &RawESIClient{failing: map[string]bool{}}
		return retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json")), raw
	}
	missingID := func(value any) *retrieval.CacheEntry {
		return &retrieval.CacheEntry{Value: value, CachedAt: time.Now().UTC()}
	}

	t.Run("a cached pilot with no id is fetched again once, and the fixed copy is cached", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.Known[2121139046] = missingID(&api.ESICharacter{Name: "Luxxianna Seraph"})

		first, err := cache.GetCharacterInfo(2121139046)
		assert.NoError(t, err)
		second, err := cache.GetCharacterInfo(2121139046)
		assert.NoError(t, err)

		assert.Equal(t, 2121139046, first.CharacterID)
		assert.Equal(t, 2121139046, second.CharacterID)
		assert.Equal(t, 1, raw.Calls("character"))
	})

	t.Run("if the refetch fails the cached copy is used, and the next lookup tries again", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.Known[2121139046] = missingID(&api.ESICharacter{Name: "Luxxianna Seraph"})
		raw.failing["character"] = true

		character, err := cache.GetCharacterInfo(2121139046)

		assert.NoError(t, err)
		assert.Equal(t, "Luxxianna Seraph", character.Name)
		assert.Equal(t, 0, character.CharacterID)
		assert.Equal(t, 1, raw.Calls("character"))

		raw.failing["character"] = false
		character, _ = cache.GetCharacterInfo(2121139046)

		assert.Equal(t, 2121139046, character.CharacterID)
		assert.Equal(t, 2, raw.Calls("character"))
	})

	t.Run("pilot ids are now saved, so a reload does not refetch them", func(t *testing.T) {
		cache, raw := newCache(t)
		_, _ = cache.GetCharacterInfo(2121139046)
		assert.NoError(t, retrieval.SaveCache(cache))

		loaded, err := retrieval.LoadCache(raw, cache.Path)
		assert.NoError(t, err)
		character, _ := loaded.GetCharacterInfo(2121139046)

		assert.Equal(t, 2121139046, character.CharacterID)
		assert.Equal(t, 1, raw.Calls("character"))
	})

	t.Run("systems, killmails, and name searches with no id are fetched again", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.Known[30000142] = missingID(&api.ESISystem{Name: "Jita"})
		cache.Killmails[139416795] = missingID(&api.ESIKillmail{SolarSystemID: 31001861})
		cache.NameLookups["Luxxianna Seraph"] = missingID(&map[int]api.EntityIDAndName{0: {Name: "Luxxianna Seraph"}})

		system, _ := cache.GetSystemInfo(30000142)
		killmail, _ := cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})
		found, _ := cache.GetEveIDsFromNames([]string{"Luxxianna Seraph"})

		assert.Equal(t, 30000142, system.SystemID)
		assert.Equal(t, 139416795, killmail.KillmailID)
		assert.Contains(t, found, 2121139046)
		assert.Equal(t, []int{1, 1, 1}, []int{raw.Calls("system"), raw.Calls("killmail"), raw.Calls("ids")})
	})

	t.Run("a killmail whose refetch fails keeps the cached copy", func(t *testing.T) {
		cache, raw := newCache(t)
		cache.Killmails[139416795] = missingID(&api.ESIKillmail{SolarSystemID: 31001861})
		raw.failing["killmail"] = true

		killmail, err := cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})

		assert.NoError(t, err)
		assert.Equal(t, 31001861, killmail.SolarSystemID)
	})

	t.Run("a readable pilot with no id is rebuilt with its id", func(t *testing.T) {
		cache, _ := newCache(t)
		cache.Expanded[2121139046] = missingID(&retrieval.Pilot{Name: "Luxxianna Seraph"})

		assert.Nil(t, retrieval.FetchExpanded[retrieval.Pilot](cache, 2121139046))

		pilot := cache.ExpandPilot(2121139046, false)

		assert.Equal(t, 2121139046, pilot.CharacterID)
		assert.Equal(t, 2121139046, retrieval.FetchExpanded[retrieval.Pilot](cache, 2121139046).CharacterID)
	})
}

func TestCachedKillmail(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("says whether a killmail is cached without fetching it", func(t *testing.T) {
		raw := &RawESIClient{}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		assert.Nil(t, cache.CachedKillmail(139416795))
		assert.Equal(t, 0, raw.Calls("killmail"), "looking does not fetch")

		_, err := cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})
		assert.NoError(t, err)

		assert.Equal(t, 139416795, cache.CachedKillmail(139416795).KillmailID)
		assert.Equal(t, 1, raw.Calls("killmail"))
	})
}

func TestDeletedCharacters(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("a deleted character is cached for good, so the ESI is only asked once", func(t *testing.T) {
		raw := &RawESIClient{deleted: map[int]bool{90000001: true}}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		before := cache.Counts()
		character, err := cache.GetCharacterInfo(90000001)
		assert.NoError(t, err)
		assert.Equal(t, api.ESICharacter{CharacterID: 90000001, Name: api.DELETED_CHARACTER_NAME, Deleted: true}, *character)
		assert.Equal(t, "1 from the ESI, 1 failed: esi.evetech.net returned 404 Not Found: Character has been deleted!", cache.Counts().Since(before).Source())

		cache.UpdateSDERelease(time.Now().UTC().Add(time.Hour))
		assert.NoError(t, retrieval.SaveCache(cache))
		loaded, err := retrieval.LoadCache(raw, cache.Path)
		assert.NoError(t, err)

		again, err := loaded.GetCharacterInfo(90000001)
		assert.NoError(t, err)
		assert.True(t, again.Deleted)
		assert.Equal(t, 1, raw.Calls("character"), "not asked again after a reload or a new SDE")
	})

	t.Run("a deleted pilot keeps their killmail, named as deleted with no corp", func(t *testing.T) {
		raw := &RawESIClient{deleted: map[int]bool{90000001: true}}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		pilot := cache.ExpandPilot(90000001, true)

		assert.Equal(t, &retrieval.Pilot{CharacterID: 90000001, Name: api.DELETED_CHARACTER_NAME}, pilot)
		assert.Equal(t, 0, raw.Calls("CORPORATION"), "no corp lookup for an id of 0")
	})

	t.Run("a pilot that fails to load for another reason is unknown, and tried again next time", func(t *testing.T) {
		raw := &RawESIClient{failing: map[string]bool{"character": true}}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cached_data.json"))

		assert.Equal(t, &retrieval.Pilot{CharacterID: 2121139046, Name: "(Unknown pilot)"}, cache.ExpandPilot(2121139046, true))
		cache.ExpandPilot(2121139046, true)

		assert.Equal(t, 2, raw.Calls("character"))
	})
}
