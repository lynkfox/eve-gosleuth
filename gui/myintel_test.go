package gui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// intelESIClient knows a few pilots, corps, and alliances by name and id, as EVE spells them
type intelESIClient struct {
	api.ESIClient
}

var intelEntities = []api.EntityIDAndName{
	{Category: "alliances", ID: 99013330, Name: "URSA."},
	{Category: "alliances", ID: 99013580, Name: "High Bear Nation"},
	{Category: "alliances", ID: 99003581, Name: "Fraternity."},
	{Category: "corporations", ID: 98744491, Name: "URSA PRIMUS"},
	{Category: "characters", ID: 2121139046, Name: "Luxxianna Seraph"},
}

func (m intelESIClient) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	found := map[int]api.EntityIDAndName{}
	for _, entity := range intelEntities {
		if strings.EqualFold(entity.Name, names[0]) {
			found[entity.ID] = entity
		}
	}
	return found, nil
}

func (m intelESIClient) GetNamesFromEveIDs(ids []int) (map[int]string, error) {
	names := map[int]string{}
	for _, entity := range intelEntities {
		names[entity.ID] = entity.Name
	}
	return names, nil
}

func (m intelESIClient) name(id int) string {
	for _, entity := range intelEntities {
		if entity.ID == id {
			return entity.Name
		}
	}
	return ""
}

func (m intelESIClient) GetAllianceInfo(id int) (*api.ESIAlliance, error) {
	return &api.ESIAlliance{AllianceID: id, Name: m.name(id)}, nil
}

func (m intelESIClient) GetCorporationInfo(id int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: id, Name: m.name(id)}, nil
}

func (m intelESIClient) GetCharacterInfo(id int) (*api.ESICharacter, error) {
	return &api.ESICharacter{CharacterID: id, Name: m.name(id)}, nil
}

func TestMyIntel(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	intelApp := func(t *testing.T) *App {
		client := intelESIClient{}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cache.db"))
		t.Cleanup(func() { cache.Close() })
		return &App{cache: cache, clients: retrieval.CollectionClientPool{ESIClient: client}, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings()}
	}

	t.Run("nothing saved is no cards", func(t *testing.T) {
		groups, err := intelApp(t).GetMyIntel()
		assert.NoError(t, err)
		assert.Equal(t, []IntelGroup{}, groups)
	})

	t.Run("a card per main and per target with ignored allies, by name, a holding with its own card listing the mains it holds for", func(t *testing.T) {
		app := intelApp(t)
		_, err := app.SaveHolding(99013330, "ALLIANCE", "URSA.", "high bear nation", "ALLIANCE", "supers parked here")
		assert.NoError(t, err)
		assert.NoError(t, app.AddIgnoredAlly("PILOT", 2121139046, "fraternity.", "ALLIANCE"))
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013580, "URSA PRIMUS", "CORPORATION"))

		groups, err := app.GetMyIntel()

		assert.NoError(t, err)
		names := []string{}
		for _, group := range groups {
			names = append(names, group.Entity.Name)
		}
		assert.Equal(t, []string{"High Bear Nation", "Luxxianna Seraph", "URSA."}, names)

		ursa := groups[2]
		assert.Equal(t, []IntelLink{{EntityRef: *entityRef(searchTypes["ALLIANCE"], 99013580, "High Bear Nation"), Note: "supers parked here"}}, ursa.Holdings)
		assert.Equal(t, "URSA.", groups[0].HeldFor[0].Name, "High Bear Nation has a card for its ignored ally, so it lists its main")
		assert.Equal(t, "Fraternity.", groups[1].IgnoredAllies[0].Name, "the ally as EVE spells it")
		assert.Equal(t, "PILOT", groups[1].Entity.Type)
	})

	t.Run("an ally is ignored once, and can not be the target itself", func(t *testing.T) {
		app := intelApp(t)
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "Fraternity.", "ALLIANCE"))
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "FRATERNITY.", "ALLIANCE"))
		assert.Len(t, app.cache.IgnoredAlliesFor("ALLIANCE", 99013330), 1)

		assert.EqualError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "URSA.", "ALLIANCE"), "URSA. can not be its own ally")
		assert.Error(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "Nobody", "ALLIANCE"))
		assert.Error(t, app.AddIgnoredAlly("SYSTEM", 30000142, "Fraternity.", "ALLIANCE"))
	})

	t.Run("a removed ally is gone, and a target left with none has no card", func(t *testing.T) {
		app := intelApp(t)
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "Fraternity.", "ALLIANCE"))

		assert.NoError(t, app.RemoveIgnoredAlly("ALLIANCE", 99013330, "ALLIANCE", 99003581))

		groups, _ := app.GetMyIntel()
		assert.Empty(t, groups)
	})

	t.Run("removing a card removes its holdings and ignored allies, but keeps it as another main's holding", func(t *testing.T) {
		app := intelApp(t)
		_, err := app.SaveHolding(99013580, "ALLIANCE", "High Bear Nation", "URSA PRIMUS", "CORPORATION", "")
		assert.NoError(t, err)
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013580, "Fraternity.", "ALLIANCE"))
		_, err = app.SaveHolding(99013330, "ALLIANCE", "URSA.", "High Bear Nation", "ALLIANCE", "")
		assert.NoError(t, err)

		assert.NoError(t, app.RemoveMyIntelGroup("ALLIANCE", 99013580))

		groups, _ := app.GetMyIntel()
		assert.Len(t, groups, 1)
		assert.Equal(t, "URSA.", groups[0].Entity.Name)
		assert.Equal(t, "High Bear Nation", groups[0].Holdings[0].Name, "still URSA.'s holding")
		assert.Empty(t, app.cache.IgnoredAlliesFor("ALLIANCE", 99013580))
		assert.Error(t, app.RemoveMyIntelGroup("SYSTEM", 30000142))
	})

	t.Run("ignored allies saved in an earlier session are all read", func(t *testing.T) {
		app := intelApp(t)
		assert.NoError(t, app.AddIgnoredAlly("CORPORATION", 98744491, "Fraternity.", "ALLIANCE"))
		path := app.cache.Path
		assert.NoError(t, app.cache.Close())

		loaded, err := retrieval.LoadCache(intelESIClient{}, path)
		assert.NoError(t, err)
		app.cache = loaded
		groups, _ := app.GetMyIntel()

		assert.Len(t, groups, 1)
		assert.Equal(t, "URSA PRIMUS", groups[0].Entity.Name, "the target's name, which is not saved, is looked up")
	})

	t.Run("a name is looked up as EVE spells it, and only of its type", func(t *testing.T) {
		app := intelApp(t)
		found, err := app.LookupEntity("ursa primus", "CORPORATION")
		assert.NoError(t, err)
		assert.Equal(t, "URSA PRIMUS", found.Name)
		assert.Equal(t, 98744491, found.ID)

		_, err = app.LookupEntity("URSA PRIMUS", "ALLIANCE")
		assert.Error(t, err)
		_, err = app.LookupEntity("Jita", "SYSTEM")
		assert.Error(t, err)
	})
}
