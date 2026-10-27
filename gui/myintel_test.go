package gui

import (
	"fmt"
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

// GetEntitiesFromEveIDs answers as universe/names does: the names and kinds of every id, or a 404 for the whole batch if any id does not exist
func (m intelESIClient) GetEntitiesFromEveIDs(ids []int) (map[int]api.EntityIDAndName, error) {
	found := map[int]api.EntityIDAndName{}
	for _, id := range ids {
		known := false
		for _, entity := range intelEntities {
			if entity.ID == id {
				found[id] = api.EntityIDAndName{ID: id, Name: entity.Name, Category: strings.TrimSuffix(strings.ReplaceAll(entity.Category, "corporations", "corporation"), "s")}
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "esi.evetech.net", Status: 404, Message: "Ensure all IDs are valid"})
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

// name is an entity's name if it is of the category, else a 404 as the ESI answers, as it only answers an alliance lookup for an alliance's id
func (m intelESIClient) name(id int, category string) (string, error) {
	for _, entity := range intelEntities {
		if entity.ID == id && entity.Category == category {
			return entity.Name, nil
		}
	}
	return "", fmt.Errorf("%w: %w", api.ErrRequestFailedToComplete, &api.StatusError{Host: "esi.evetech.net", Status: 404, Message: "Not found"})
}

func (m intelESIClient) GetAllianceInfo(id int) (*api.ESIAlliance, error) {
	name, err := m.name(id, "alliances")
	if err != nil {
		return nil, err
	}
	return &api.ESIAlliance{AllianceID: id, Name: name, Ticker: strings.ToUpper(strings.Fields(name)[0])}, nil
}

func (m intelESIClient) GetCorporationInfo(id int) (*api.ESICorporation, error) {
	name, err := m.name(id, "corporations")
	if err != nil {
		return nil, err
	}
	return &api.ESICorporation{CorporationID: id, Name: name}, nil
}

func (m intelESIClient) GetCharacterInfo(id int) (*api.ESICharacter, error) {
	name, err := m.name(id, "characters")
	if err != nil {
		return nil, err
	}
	return &api.ESICharacter{CharacterID: id, Name: name}, nil
}

// newIntelApp is an app with an empty cache, whose ESI is intelESIClient
func newIntelApp(t *testing.T) *App {
	client := intelESIClient{}
	cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cache.db"))
	t.Cleanup(func() { cache.Close() })
	// as in the app, every ESI call goes through the cache
	return &App{cache: cache, clients: retrieval.CollectionClientPool{ESIClient: cache}, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings()}
}

func TestMyIntel(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	intelApp := newIntelApp

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
		highBear := *entityRef(searchTypes["ALLIANCE"], 99013580, "High Bear Nation")
		highBear.Ticker = "HIGH"
		assert.Equal(t, []IntelLink{{EntityRef: highBear, Note: "supers parked here"}}, ursa.Holdings)
		assert.Empty(t, ursa.Entity.Ticker, "URSA. was never looked up, only named by id, so its ticker is left blank rather than fetched")
		assert.Empty(t, groups[1].Entity.Ticker, "a pilot has none")

		_, err = app.LookupEntity("URSA.", "ALLIANCE")
		assert.NoError(t, err)
		groups, _ = app.GetMyIntel()
		assert.Equal(t, "URSA.", groups[2].Entity.Ticker, "once looked up it is cached")
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

	t.Run("several cards are removed at once, and keys that are not cards skipped", func(t *testing.T) {
		app := intelApp(t)
		assert.NoError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "Fraternity.", "ALLIANCE"))
		assert.NoError(t, app.AddIgnoredAlly("PILOT", 2121139046, "Fraternity.", "ALLIANCE"))
		_, err := app.SaveHolding(99013580, "ALLIANCE", "High Bear Nation", "URSA PRIMUS", "CORPORATION", "")
		assert.NoError(t, err)

		assert.NoError(t, app.RemoveMyIntelGroups([]string{"ALLIANCE:99013330", "PILOT:2121139046", "nonsense", "SYSTEM:30000142"}))

		groups, _ := app.GetMyIntel()
		assert.Len(t, groups, 1)
		assert.Equal(t, "High Bear Nation", groups[0].Entity.Name)
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

	t.Run("a link is looked up by the id in it, its own type winning, but only a kind allowed there", func(t *testing.T) {
		app := intelApp(t)
		found, err := app.LookupEntity("https://zkillboard.com/alliance/99013330/", "PILOT")
		assert.NoError(t, err)
		assert.Equal(t, "URSA.", found.Name)
		assert.Equal(t, "ALLIANCE", found.Type, "the link's type, not the dropdown's")

		// a holding added by link is saved as the link's type, here an alliance though the dropdown said corporation
		_, err = app.SaveHolding(98744491, "CORPORATION", "URSA PRIMUS", "evewho.com/alliance/99013580", "CORPORATION", "")
		assert.NoError(t, err)
		holdings, _ := app.cache.HoldingLinks("CORPORATION", 98744491)
		assert.Equal(t, "ALLIANCE", holdings[0].HoldingType)
		assert.Equal(t, "High Bear Nation", holdings[0].HoldingName)

		assert.EqualError(t, app.AddIgnoredAlly("ALLIANCE", 99013330, "https://zkillboard.com/character/2121139046/", "ALLIANCE"), "That link is to a pilot, which can not be used here")
		_, err = app.LookupEntity("https://zkillboard.com/alliance/12345/", "ALLIANCE")
		assert.Error(t, err, "an id the ESI does not know")
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
