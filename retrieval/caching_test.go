package retrieval_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const testCharacterID = 2121139046
const testCorporationID = 98739641

// CountingESIClient embeds the interface, implementing only the calls the cache makes and counting them
type CountingESIClient struct {
	api.ESIClient
	characterName  string
	characterCalls int
	itemCalls      int
}

func (m *CountingESIClient) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	m.characterCalls++
	return &api.ESICharacter{CharacterID: idNumber, Name: m.characterName, CorporationID: testCorporationID}, nil
}

func (m *CountingESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "Umbrian Dawn"}, nil
}

func (m *CountingESIClient) GetItemInfo(idNumber int) (*api.ESIItem, error) {
	m.itemCalls++
	return &api.ESIItem{ItemId: idNumber, Name: "Rifter"}, nil
}

func TestLoadCache(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("a missing database is created empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "eve-gosleuth", "cache.db")

		cache, err := retrieval.LoadCache(&CountingESIClient{}, path)

		assert.NoError(t, err)
		assert.Equal(t, 0, cache.Entries())
		assert.Equal(t, path, cache.Path)
		assert.FileExists(t, path)
	})

	t.Run("a corrupt database starts empty and is moved aside", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cache.db")
		assert.NoError(t, os.WriteFile(path, []byte(`{"version": 1, "known": {`+strings.Repeat("x", 200)), 0o600))

		cache, err := retrieval.LoadCache(&CountingESIClient{}, path)

		assert.Error(t, err)
		assert.FileExists(t, path+".corrupt")
		cache.Update(587, &api.ESIItem{ItemId: 587, Name: "Rifter"})
		assert.Equal(t, 1, cache.Entries(), "a fresh database is used in its place")
	})

	t.Run("a database from another schema version is emptied", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cache.db")
		cache := retrieval.NewCache(&CountingESIClient{}, path)
		cache.Update(587, &api.ESIItem{ItemId: 587, Name: "Rifter"})
		assert.NoError(t, cache.Close())

		db, err := sql.Open("sqlite", path)
		assert.NoError(t, err)
		_, err = db.Exec("PRAGMA user_version = 99")
		assert.NoError(t, err)
		db.Close()

		loaded, err := retrieval.LoadCache(&CountingESIClient{}, path)

		assert.NoError(t, err)
		assert.Equal(t, 0, loaded.Entries())
	})

	t.Run("killmails in the old json cache are imported once, then it is removed", func(t *testing.T) {
		dir := t.TempDir()
		legacy := `{"version": 2, "known": {"587": {"value": {"type_id": 587, "name": "Rifter"}, "cached_at": "2026-10-01T00:00:00Z"}},
			"killmails": {"139416795": {"value": {"killmail_id": 139416795, "solar_system_id": 31001861}, "cached_at": "2026-09-01T00:00:00Z", "permanent": true}}}`
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "cached_data.json"), []byte(legacy), 0o600))

		cache, err := retrieval.LoadCache(&CountingESIClient{}, filepath.Join(dir, "cache.db"))

		assert.NoError(t, err)
		assert.NoFileExists(t, filepath.Join(dir, "cached_data.json"))
		assert.Equal(t, 1, cache.Entries(), "only the killmail, the rest is cheap to fetch again")
		killmail := cache.CachedKillmail(139416795)
		assert.NotNil(t, killmail)
		assert.Equal(t, 31001861, killmail.SolarSystemID)
	})

	t.Run("an old json cache of another version is removed without importing", func(t *testing.T) {
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "cached_data.json"), []byte(`{"version": 1, "killmails": {}}`), 0o600))

		cache, err := retrieval.LoadCache(&CountingESIClient{}, filepath.Join(dir, "cache.db"))

		assert.NoError(t, err)
		assert.NoFileExists(t, filepath.Join(dir, "cached_data.json"))
		assert.Equal(t, 0, cache.Entries())
	})
}

func TestSaveCache(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("round trip restores typed values without calling the ESI", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "eve-gosleuth", "cache.db")
		client := &CountingESIClient{characterName: "Luxxianna Seraph"}

		cache := retrieval.NewCache(client, path)
		cache.ExpandPilot(testCharacterID, true)
		assert.NoError(t, cache.Close())

		loaded, err := retrieval.LoadCache(client, path)
		assert.NoError(t, err)

		pilot := retrieval.FetchExpanded[retrieval.Pilot](loaded, testCharacterID)
		assert.NotNil(t, pilot)
		assert.Equal(t, "Luxxianna Seraph", pilot.Name)
		assert.Equal(t, "Umbrian Dawn", pilot.Corporation.Name)

		raw, err := loaded.GetIfNotCached(testCharacterID, types.PILOT)
		assert.NoError(t, err)
		assert.Equal(t, "Luxxianna Seraph", (*raw).(*api.ESICharacter).Name)
		assert.Equal(t, 1, client.characterCalls)
	})

	t.Run("values are only read from the database when first wanted", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cache.db")
		client := &CountingESIClient{}
		cache := retrieval.NewCache(client, path)
		cache.Update(587, &api.ESIItem{ItemId: 587, Name: "Rifter"})
		cache.Update(588, &api.ESIItem{ItemId: 588, Name: "Reaper"})
		assert.NoError(t, cache.Close())

		loaded, _ := retrieval.LoadCache(client, path)
		assert.Empty(t, loaded.Known)

		item, err := loaded.GetItemInfo(587)

		assert.NoError(t, err)
		assert.Equal(t, "Rifter", item.Name)
		assert.Len(t, loaded.Known, 1)
		assert.Equal(t, 0, client.itemCalls)
	})

	t.Run("the SDE release seen is kept", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "cache.db")
		release := time.Date(2026, 10, 20, 11, 0, 0, 0, time.UTC)
		cache := retrieval.NewCache(&CountingESIClient{}, path)
		cache.UpdateSDERelease(release)
		assert.NoError(t, cache.Close())

		loaded, _ := retrieval.LoadCache(&CountingESIClient{}, path)

		assert.Equal(t, release, loaded.SDERelease)
	})

	t.Run("moving copies the cache to the new file and removes the old one", func(t *testing.T) {
		oldPath := filepath.Join(t.TempDir(), "cache.db")
		newPath := filepath.Join(t.TempDir(), "moved here", "cache.db")
		cache := retrieval.NewCache(&CountingESIClient{}, oldPath)
		cache.Update(587, &api.ESIItem{ItemId: 587, Name: "Rifter"})

		assert.NoError(t, cache.MoveTo(newPath))

		assert.Equal(t, newPath, cache.CurrentPath())
		assert.NoFileExists(t, oldPath)
		cache.Update(588, &api.ESIItem{ItemId: 588, Name: "Reaper"})
		assert.NoError(t, cache.Close())
		loaded, _ := retrieval.LoadCache(&CountingESIClient{}, newPath)
		assert.Equal(t, 2, loaded.Entries(), "what was moved and what was added after")
	})
}

func TestCacheExpiry(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	lastWeek := time.Now().UTC().Add(-7 * 24 * time.Hour)

	t.Run("data cached before the latest SDE release is refetched and overwritten in both layers", func(t *testing.T) {
		client := &CountingESIClient{characterName: "New Name"}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = lastWeek

		beforeRelease := lastWeek.Add(-time.Hour)
		cache.Known[testCharacterID] = &retrieval.CacheEntry{Value: &api.ESICharacter{CharacterID: testCharacterID, Name: "Old Name"}, CachedAt: beforeRelease}
		cache.Expanded[testCharacterID] = &retrieval.CacheEntry{Value: &retrieval.Pilot{CharacterID: testCharacterID, Name: "Old Name"}, CachedAt: beforeRelease}

		pilot := cache.ExpandPilot(testCharacterID, true)

		assert.Equal(t, "New Name", pilot.Name)
		assert.Equal(t, 1, client.characterCalls)
		assert.True(t, cache.Expanded[testCharacterID].CachedAt.After(lastWeek))
		assert.True(t, cache.Known[testCharacterID].CachedAt.After(lastWeek))
	})

	t.Run("items and systems cached after the latest SDE release are served from the cache, however old", func(t *testing.T) {
		client := &CountingESIClient{}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = lastWeek.Add(-60 * 24 * time.Hour)
		monthOld := lastWeek.Add(-30 * 24 * time.Hour)

		cache.Known[587] = &retrieval.CacheEntry{Value: &api.ESIItem{ItemId: 587, Name: "Rifter"}, CachedAt: monthOld}
		cache.Expanded[587] = &retrieval.CacheEntry{Value: &retrieval.Item{ItemID: 587, Name: "Rifter"}, CachedAt: monthOld}

		item := cache.ExpandItem(587)
		raw, err := cache.GetIfNotCached(587, types.ITEM)

		assert.Equal(t, "Rifter", item.Name)
		assert.NoError(t, err)
		assert.Equal(t, "Rifter", (*raw).(*api.ESIItem).Name)
		assert.Equal(t, 0, client.itemCalls)
	})

	t.Run("pilots are refetched after 24 hours even with no new SDE", func(t *testing.T) {
		client := &CountingESIClient{characterName: "New Name"}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = lastWeek

		dayOld := time.Now().UTC().Add(-25 * time.Hour)
		cache.Known[testCharacterID] = &retrieval.CacheEntry{Value: &api.ESICharacter{CharacterID: testCharacterID, Name: "Old Name"}, CachedAt: dayOld}
		cache.Expanded[testCharacterID] = &retrieval.CacheEntry{Value: &retrieval.Pilot{CharacterID: testCharacterID, Name: "Old Name"}, CachedAt: dayOld}

		pilot := cache.ExpandPilot(testCharacterID, true)

		assert.Equal(t, "New Name", pilot.Name)
		assert.Equal(t, 1, client.characterCalls)
	})

	t.Run("pilots cached within 24 hours are served from the cache", func(t *testing.T) {
		client := &CountingESIClient{characterName: "New Name"}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = lastWeek

		cache.Expanded[testCharacterID] = &retrieval.CacheEntry{Value: &retrieval.Pilot{CharacterID: testCharacterID, Name: "Cached Name"}, CachedAt: time.Now().UTC().Add(-23 * time.Hour)}

		pilot := cache.ExpandPilot(testCharacterID, true)

		assert.Equal(t, "Cached Name", pilot.Name)
		assert.Equal(t, 0, client.characterCalls)
	})

	t.Run("corporations and alliances are refetched after 24 hours too, raw or expanded", func(t *testing.T) {
		client := AllianceESIClient{}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cached_data.json"))
		dayOld := time.Now().UTC().Add(-25 * time.Hour)

		cache.Known[testCorporationID] = &retrieval.CacheEntry{Value: &api.ESICorporation{CorporationID: testCorporationID, Name: "Old Corp"}, CachedAt: dayOld}
		cache.Expanded[testCorporationID] = &retrieval.CacheEntry{Value: &retrieval.Corporation{CorporationID: testCorporationID, Name: "Old Corp"}, CachedAt: dayOld}
		cache.Known[99014851] = &retrieval.CacheEntry{Value: &api.ESIAlliance{AllianceID: 99014851, Name: "Old Alliance"}, CachedAt: dayOld}

		corp := cache.ExpandCorporation(testCorporationID, true)

		assert.Equal(t, "Umbrian Dawn", corp.Name)
		assert.Equal(t, "URSA.", corp.Alliance.Name)
	})

	t.Run("killmails never expire, even a year later with a newer SDE", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = time.Now().UTC()
		cache.Killmails[139416795] = &retrieval.CacheEntry{Value: &api.ESIKillmail{KillmailID: 139416795}, CachedAt: time.Now().UTC().Add(-365 * 24 * time.Hour), Permanent: true}

		killmail, err := cache.GetKillmail(api.ZkbResponse{KillmailID: 139416795})

		assert.NoError(t, err)
		assert.Equal(t, 139416795, killmail.KillmailID)
		assert.Equal(t, 0, cache.Client.(*RawESIClient).Calls("killmail"))
	})

	t.Run("a newer SDE release drops everything cached before it except killmails", func(t *testing.T) {
		cache := retrieval.NewCache(&CountingESIClient{}, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.Update(587, &api.ESIItem{ItemId: 587, Name: "Rifter"})
		cache.Killmails[139416795] = &retrieval.CacheEntry{Value: &api.ESIKillmail{KillmailID: 139416795}, CachedAt: lastWeek, Permanent: true}
		cache.EntityNames[98744491] = &retrieval.CacheEntry{Value: "URSA PRIMUS", CachedAt: lastWeek}

		dropped := cache.UpdateSDERelease(time.Now().UTC().Add(time.Minute))

		assert.Equal(t, 2, dropped)
		assert.Empty(t, cache.Known)
		assert.Empty(t, cache.EntityNames)
		assert.Contains(t, cache.Killmails, 139416795)
	})

	t.Run("an SDE release that is not newer changes nothing", func(t *testing.T) {
		cache := retrieval.NewCache(&CountingESIClient{}, filepath.Join(t.TempDir(), "cached_data.json"))
		cache.SDERelease = lastWeek
		cache.Known[587] = &retrieval.CacheEntry{Value: &api.ESIItem{ItemId: 587, Name: "Rifter"}, CachedAt: lastWeek.Add(-time.Hour)}

		assert.Equal(t, 0, cache.UpdateSDERelease(lastWeek))
		assert.Contains(t, cache.Known, 587)
	})
}

// AllianceESIClient embeds the interface, returning a corp that is in an alliance
type AllianceESIClient struct {
	api.ESIClient
}

func (m AllianceESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "Umbrian Dawn", AllianceID: 99014851}, nil
}

func (m AllianceESIClient) GetAllianceInfo(idNumber int) (*api.ESIAlliance, error) {
	return &api.ESIAlliance{AllianceID: idNumber, Name: "URSA."}, nil
}

func TestExpandCorporation(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("a corp cached without its alliance gets the alliance when it is later wanted", func(t *testing.T) {
		cache := retrieval.NewCache(AllianceESIClient{}, filepath.Join(t.TempDir(), "cache.json"))

		withoutAlliance := cache.ExpandCorporation(testCorporationID, false)
		assert.Nil(t, withoutAlliance.Alliance)

		withAlliance := cache.ExpandCorporation(testCorporationID, true)
		if assert.NotNil(t, withAlliance.Alliance) {
			assert.Equal(t, "URSA.", withAlliance.Alliance.Name)
		}

		assert.NotNil(t, retrieval.FetchExpanded[retrieval.Corporation](cache, testCorporationID).Alliance)
	})
}
