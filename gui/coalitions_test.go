package gui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// coalitionWith makes a coalition of the members, by name, through the app's own methods
func coalitionWith(t *testing.T, app *App, name string, members ...[2]string) CoalitionCard {
	card, err := app.CreateCoalition(name)
	assert.NoError(t, err)
	for _, member := range members {
		card, err = app.AddCoalitionMember(card.ID, member[0], member[1])
		assert.NoError(t, err)
	}
	return card
}

func memberNames(refs []EntityRef) []string {
	names := []string{}
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}

func TestCoalitionMethods(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("a name is trimmed, not empty, at most 100 characters, and unique ignoring case", func(t *testing.T) {
		app := newIntelApp(t)
		card, err := app.CreateCoalition("  The Coalition ")
		assert.NoError(t, err)
		assert.Equal(t, "The Coalition", card.Name)
		assert.Less(t, card.ID, 0)
		assert.Equal(t, CoalitionCard{ID: card.ID, Name: "The Coalition", Members: []EntityRef{}, Affiliates: []EntityRef{}}, card)

		_, err = app.CreateCoalition("   ")
		assert.EqualError(t, err, "Enter a name for the coalition")
		_, err = app.CreateCoalition(strings.Repeat("x", 101))
		assert.Error(t, err)
		_, err = app.CreateCoalition(strings.Repeat("x", 100))
		assert.NoError(t, err)
		_, err = app.CreateCoalition("the COALITION")
		assert.EqualError(t, err, "There is already a coalition named The Coalition")

		other, err := app.CreateCoalition("Other")
		assert.NoError(t, err)
		_, err = app.RenameCoalition(other.ID, "THE coalition")
		assert.Error(t, err, "another coalition's name")
		renamed, err := app.RenameCoalition(card.ID, "THE COALITION")
		assert.NoError(t, err, "its own name in another case")
		assert.Equal(t, "THE COALITION", renamed.Name)
		_, err = app.RenameCoalition(card.ID, "")
		assert.Error(t, err)
		_, err = app.RenameCoalition(-1, "Nothing")
		assert.ErrorIs(t, err, errNoCoalition)

		cards, err := app.GetCoalitions()
		assert.NoError(t, err)
		assert.Len(t, cards, 3, "the failed ones left nothing behind")
	})

	t.Run("members are corps and alliances found by name or link, once each", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "The Coalition", [2]string{"ursa.", "ALLIANCE"}, [2]string{"URSA PRIMUS", "CORPORATION"})
		assert.Equal(t, []string{"URSA.", "URSA PRIMUS"}, memberNames(card.Members))

		_, err := app.AddCoalitionMember(card.ID, "URSA.", "ALLIANCE")
		assert.EqualError(t, err, "URSA. is already in The Coalition")
		_, err = app.AddCoalitionMember(card.ID, "https://zkillboard.com/alliance/99013330/", "CORPORATION")
		assert.Error(t, err, "a link to a member is refused too")
		_, err = app.AddCoalitionMember(card.ID, "Luxxianna Seraph", "PILOT")
		assert.Error(t, err, "no pilots")
		_, err = app.AddCoalitionMember(card.ID, "https://zkillboard.com/character/2121139046/", "ALLIANCE")
		assert.Error(t, err, "no pilot links")
		_, err = app.AddCoalitionMember(card.ID, "Jita", "SYSTEM")
		assert.Error(t, err)
		_, err = app.AddCoalitionMember(card.ID, "Nobody", "ALLIANCE")
		assert.Error(t, err)
		_, err = app.AddCoalitionMember(-5, "Fraternity.", "ALLIANCE")
		assert.ErrorIs(t, err, errNoCoalition)

		card, err = app.AddCoalitionMember(card.ID, "evewho.com/alliance/99013580", "CORPORATION")
		assert.NoError(t, err)
		assert.Equal(t, "High Bear Nation", card.Members[2].Name, "a link's own type wins")

		card, err = app.RemoveCoalitionMember(card.ID, "ALLIANCE", 99013330)
		assert.NoError(t, err)
		assert.Equal(t, []string{"URSA PRIMUS", "High Bear Nation"}, memberNames(card.Members))
	})

	t.Run("note, structures, and affiliates of the members are on the card", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "The Coalition", [2]string{"URSA.", "ALLIANCE"})
		_, err := app.SaveHolding(99013330, "ALLIANCE", "URSA.", "URSA PRIMUS", "CORPORATION", "")
		assert.NoError(t, err)

		_, err = app.SetCoalitionNote(card.ID, " "+strings.Repeat("n", 300))
		assert.NoError(t, err)
		_, err = app.SetCoalitionStructures(card.ID, true)
		assert.NoError(t, err)
		assert.Error(t, app.AddIgnoredAlly("COALITION", card.ID, "Fraternity.", "ALLIANCE"), "a coalition has no allies")

		cards, _ := app.GetCoalitions()
		assert.Len(t, cards[0].Note, maxHoldingNote)
		assert.True(t, cards[0].IncludeStructures)
		assert.Equal(t, []string{"URSA PRIMUS"}, memberNames(cards[0].Affiliates))
	})

	t.Run("removed coalitions are restored with their ids and members", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "The Coalition", [2]string{"URSA.", "ALLIANCE"})
		card = mustCoalition(t, app, card.ID)
		coalitionWith(t, app, "Kept")

		assert.NoError(t, app.RemoveCoalitions([]int{card.ID, -77}))
		cards, _ := app.GetCoalitions()
		assert.Equal(t, []string{"Kept"}, coalitionNames(cards))
		fresh := coalitionWith(t, app, "Fresh")
		assert.NotEqual(t, card.ID, fresh.ID, "a removed id is not given again")

		assert.NoError(t, app.RestoreCoalitions([]CoalitionCard{card}))
		restored := mustCoalition(t, app, card.ID)
		assert.Equal(t, card, restored)
	})
}

func TestRenamedCoalitionRecentSearch(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	recentNames := func(app *App) []string {
		names := []string{}
		for _, recent := range app.cache.RecentSearches(retrieval.MaxRecentSearches) {
			names = append(names, recent.Name)
		}
		return names
	}

	t.Run("a renamed coalition's recent search takes its new name, and goes with it when removed", func(t *testing.T) {
		app := newIntelApp(t)
		app.settings.MaxRecentSearches = 5
		card := coalitionWith(t, app, "Alpha")
		app.cache.AddRecentSearch(retrieval.RecentSearch{Name: "Alpha", Type: "COALITION", ID: card.ID}, 5)

		_, err := app.RenameCoalition(card.ID, "Beta")

		assert.NoError(t, err)
		assert.Equal(t, []string{"Beta"}, recentNames(app))
		assert.NoError(t, app.RemoveCoalitions([]int{card.ID}))
		assert.Empty(t, recentNames(app))
	})

	t.Run("a coalition never searched gets no recent search from a rename", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "Alpha")

		_, err := app.RenameCoalition(card.ID, "Beta")

		assert.NoError(t, err)
		assert.Empty(t, recentNames(app))
	})

	t.Run("saving ignored allies for a coalition is refused", func(t *testing.T) {
		app := newIntelApp(t)
		card := coalitionWith(t, app, "Alpha")

		_, err := app.SaveIgnoredAllies(card.ID, "COALITION", nil)

		assert.Error(t, err)
	})
}

func mustCoalition(t *testing.T, app *App, id int) CoalitionCard {
	cards, err := app.GetCoalitions()
	assert.NoError(t, err)
	for _, card := range cards {
		if card.ID == id {
			return card
		}
	}
	t.Fatalf("coalition %d not found", id)
	return CoalitionCard{}
}

func coalitionNames(cards []CoalitionCard) []string {
	names := []string{}
	for _, card := range cards {
		names = append(names, card.Name)
	}
	return names
}

func TestCoalitionRestoreSkipsClashes(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	app := newIntelApp(t)
	card := coalitionWith(t, app, "The Coalition", [2]string{"URSA.", "ALLIANCE"})
	card = mustCoalition(t, app, card.ID)
	assert.NoError(t, app.RemoveCoalitions([]int{card.ID}))
	taken := coalitionWith(t, app, "the coalition")

	err := app.RestoreCoalitions([]CoalitionCard{card})
	assert.EqualError(t, err, "Could not restore The Coalition")
	cards, _ := app.GetCoalitions()
	assert.Equal(t, []string{"the coalition"}, coalitionNames(cards))
	assert.Equal(t, taken.ID, cards[0].ID)
}

// negativeGuardESI is an ESI that knows nothing, recording every id it is asked for. It fails the test if a negative one reaches it.
type negativeGuardESI struct {
	t     *testing.T
	mu    sync.Mutex
	calls []string
}

func (g *negativeGuardESI) note(call string, ids ...int) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, call)
	for _, id := range ids {
		if id < 0 {
			g.t.Errorf("%s was asked for the negative id %d", call, id)
		}
	}
	return fmt.Errorf("%s: not known to the test", call)
}

func (g *negativeGuardESI) GetEveIDsFromNames(names []string) (map[int]api.EntityIDAndName, error) {
	return nil, g.note("GetEveIDsFromNames")
}
func (g *negativeGuardESI) GetNamesFromEveIDs(ids []int) (map[int]string, error) {
	return nil, g.note("GetNamesFromEveIDs", ids...)
}
func (g *negativeGuardESI) GetEntitiesFromEveIDs(ids []int) (map[int]api.EntityIDAndName, error) {
	return nil, g.note("GetEntitiesFromEveIDs", ids...)
}
func (g *negativeGuardESI) GetCharacterAffiliations(ids []int) (map[int]api.ESIAffiliation, error) {
	return nil, g.note("GetCharacterAffiliations", ids...)
}
func (g *negativeGuardESI) GetCharacterInfo(id int) (*api.ESICharacter, error) {
	return nil, g.note("GetCharacterInfo", id)
}
func (g *negativeGuardESI) GetCorporationInfo(id int) (*api.ESICorporation, error) {
	return nil, g.note("GetCorporationInfo", id)
}
func (g *negativeGuardESI) GetAllianceInfo(id int) (*api.ESIAlliance, error) {
	return nil, g.note("GetAllianceInfo", id)
}
func (g *negativeGuardESI) GetSystemInfo(id int) (*api.ESISystem, error) {
	return nil, g.note("GetSystemInfo", id)
}
func (g *negativeGuardESI) GetItemInfo(id int) (*api.ESIItem, error) {
	return nil, g.note("GetItemInfo", id)
}
func (g *negativeGuardESI) GetStargateInfo(id int) (*api.ESIStargate, error) {
	return nil, g.note("GetStargateInfo", id)
}
func (g *negativeGuardESI) GetStarInfo(id int) (*api.ESIStar, error) {
	return nil, g.note("GetStarInfo", id)
}
func (g *negativeGuardESI) GetPlanetInfo(id int) (*api.ESIPlanetInfo, error) {
	return nil, g.note("GetPlanetInfo", id)
}
func (g *negativeGuardESI) GetMoonInfo(id int) (*api.ESIMoon, error) {
	return nil, g.note("GetMoonInfo", id)
}
func (g *negativeGuardESI) GetCategoryInfo(id int) (*api.ESICategory, error) {
	return nil, g.note("GetCategoryInfo", id)
}
func (g *negativeGuardESI) GetAllianceCorporations(id int) (*api.ESIAllianceCorporations, error) {
	return nil, g.note("GetAllianceCorporations", id)
}
func (g *negativeGuardESI) GetCorporationHistory(id int) (*api.ESICorporationHistory, error) {
	return nil, g.note("GetCorporationHistory", id)
}
func (g *negativeGuardESI) GetAllianceHistory(id int) (*api.ESIAllianceHistory, error) {
	return nil, g.note("GetAllianceHistory", id)
}
func (g *negativeGuardESI) GetEsiResponse(id int, entityType types.EntityType) (interface{}, error) {
	return nil, g.note("GetEsiResponse", id)
}
func (g *negativeGuardESI) GetKillmail(zkillmail api.ZkbResponse) (*api.ESIKillmail, error) {
	return nil, g.note("GetKillmail", zkillmail.KillmailID)
}

// zkillRecorder lists killmails by entity, recording every entity asked for and the filter it was asked with
type zkillRecorder struct {
	t      *testing.T
	mu     sync.Mutex
	lists  map[int][]api.ZkbResponse
	asked  []int
	filter []string // the filters asked with, one per call
}

func (z *zkillRecorder) GetZkillResults(entityId int, entityType types.EntityType, paging api.ZkillPaging) (api.ZkillList, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	if entityId < 0 {
		z.t.Errorf("zKillboard was asked for the negative id %d", entityId)
	}
	z.asked = append(z.asked, entityId)
	z.filter = append(z.filter, paging.Filter)
	return api.ZkillList{Killmails: z.lists[entityId], Ended: true}, nil
}

func (z *zkillRecorder) structureCalls() int {
	z.mu.Lock()
	defer z.mu.Unlock()
	count := 0
	for _, filter := range z.filter {
		if filter != "" {
			count++
		}
	}
	return count
}

func intPtr(n int) *int { return &n }

// coalitionKillmail is a zKillboard killmail of a corp's pilot killing a ship, sent whole
func coalitionKillmail(id int, at time.Time, attackerCorp int, attackerAlliance int, victimCorp int) api.ZkbResponse {
	attacker := api.ESIAttacker{ShipTypeID: 11202, FinalBlow: true, CorporationID: intPtr(attackerCorp), CharacterID: intPtr(2121139046)}
	if attackerAlliance != 0 {
		attacker.AllianceID = intPtr(attackerAlliance)
	}
	return api.ZkbResponse{KillmailID: id, KillmailTime: at, SolarSystemID: 30000142,
		Victim: api.ESIVictim{ShipTypeID: 587, CorporationID: intPtr(victimCorp)}, Attackers: []api.ESIAttacker{attacker}}
}

// newCollectApp is an app whose ESI and zKillboard are recorders that fail the test on a negative id
func newCollectApp(t *testing.T, lists map[int][]api.ZkbResponse) (*App, *zkillRecorder, *negativeGuardESI) {
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	esi := &negativeGuardESI{t: t}
	zkill := &zkillRecorder{t: t, lists: lists}
	cache := retrieval.NewCache(esi, filepath.Join(t.TempDir(), "cache.db"))
	t.Cleanup(func() { cache.Close() })
	app := &App{cache: cache, clients: retrieval.CollectionClientPool{ZkillClient: zkill, ESIClient: cache},
		settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: defaultSettings(), sdeCheckedAt: time.Now()}
	return app, zkill, esi
}

func TestCollectIntelCoalition(t *testing.T) {
	comm.InitLogger(logrus.PanicLevel)
	now := time.Now().UTC()
	const alpha, beta, affiliate = 99000001, 98000002, 98000009
	lists := func() map[int][]api.ZkbResponse {
		return map[int][]api.ZkbResponse{
			alpha:     {coalitionKillmail(10, now.Add(-time.Hour), 98000100, alpha, 98000777)},
			beta:      {coalitionKillmail(20, now.Add(-2*time.Hour), beta, 0, 98000777), coalitionKillmail(11, now.Add(-3*time.Hour), beta, 0, 98000100)},
			affiliate: {coalitionKillmail(30, now.Add(-4*time.Hour), affiliate, 0, 98000777)},
		}
	}
	// a coalition of alpha and beta, with beta's affiliate
	setup := func(t *testing.T, structures bool) (*App, *zkillRecorder, *negativeGuardESI, CoalitionCard) {
		app, zkill, esi := newCollectApp(t, lists())
		card, err := app.CreateCoalition("The Coalition")
		assert.NoError(t, err)
		app.cache.SaveCoalition(retrieval.Coalition{ID: card.ID, Name: "The Coalition", IncludeStructures: structures, Members: []retrieval.CoalitionMember{
			{Type: "ALLIANCE", ID: alpha, Name: "Alpha"}, {Type: "CORPORATION", ID: beta, Name: "Beta"}}})
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "CORPORATION", MainID: beta, MainName: "Beta", HoldingType: "CORPORATION", HoldingID: affiliate, HoldingName: "Gamma"})
		return app, zkill, esi, card
	}

	t.Run("every member and the affiliate of a member is listed, no id below 0 reaches zKillboard or the ESI, and no structure lists are asked for", func(t *testing.T) {
		app, zkill, esi, card := setup(t, false)

		report, err := app.CollectIntel(card.ID, "COALITION")

		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{alpha, beta, affiliate}, zkill.asked)
		assert.Equal(t, 0, zkill.structureCalls())
		assert.Nil(t, report.Structures)
		assert.Nil(t, report.Structures)
		assert.Nil(t, report.Allies, "a coalition has no Allies")
		assert.Nil(t, report.IgnoredAllies)
		assert.Nil(t, report.Holdings)
		assert.Len(t, report.Killmails, 4, "each once, though one is on two lists")
		for _, km := range report.Killmails {
			assert.NotEmpty(t, km.Via, "every killmail came from a member's or affiliate's list")
		}
		sources := map[string]int{}
		for _, source := range report.Scope.Sources {
			assert.False(t, source.Main, "a coalition has no main row")
			sources[source.Name] = source.Killmails
		}
		assert.Equal(t, map[string]int{"Alpha": 1, "Beta": 2, "Gamma": 1}, sources)
		assert.NotEmpty(t, esi.calls, "names were looked up")
	})

	t.Run("without Include affiliates only the members are listed", func(t *testing.T) {
		app, zkill, _, card := setup(t, false)
		app.settings.Preferences.IncludeHoldings = false

		_, err := app.CollectIntel(card.ID, "COALITION")

		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{alpha, beta}, zkill.asked)
	})

	t.Run("opted in, the structure lists are asked for", func(t *testing.T) {
		app, zkill, _, card := setup(t, true)

		report, err := app.CollectIntel(card.ID, "COALITION")

		assert.NoError(t, err)
		assert.Greater(t, zkill.structureCalls(), 0)
		assert.NotNil(t, report.Structures)
		assert.False(t, report.MemberList)
	})

	t.Run("a coalition that is gone is refused", func(t *testing.T) {
		app, zkill, _, _ := setup(t, false)
		_, err := app.CollectIntel(-5, "COALITION")
		assert.ErrorIs(t, err, errNoCoalition)
		assert.Empty(t, zkill.asked)
	})

	t.Run("a member searched alone does not pull in the others", func(t *testing.T) {
		app, zkill, _, _ := setup(t, false)
		app.cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: alpha, MainName: "Alpha", HoldingType: "CORPORATION", HoldingID: 98000555, HoldingName: "Alpha Holdings"})

		_, err := app.CollectIntel(alpha, "ALLIANCE")

		assert.NoError(t, err)
		assert.ElementsMatch(t, []int{alpha, 98000555}, slices.Compact(slices.Sorted(slices.Values(zkill.asked))), "its own affiliate, but not the coalition's other members")
	})
}

func TestSearchCoalition(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	newApp := func(t *testing.T) *App {
		app := newIntelApp(t)
		app.sdeCheckedAt = time.Now()
		return app
	}

	t.Run("a coalition is searched by id from the cache, and kept as a recent search", func(t *testing.T) {
		app := newApp(t)
		card := coalitionWith(t, app, "The Coalition", [2]string{"URSA.", "ALLIANCE"})

		result, err := app.SearchTarget("The Coalition", "COALITION", card.ID)

		assert.NoError(t, err)
		assert.Equal(t, "COALITION", result.Type)
		assert.Equal(t, card.ID, result.ID)
		assert.Equal(t, "The Coalition", result.Display.Name)
		assert.Nil(t, result.Holdings)
		if assert.NotNil(t, result.Coalition) {
			assert.Equal(t, []string{"URSA."}, memberNames(result.Coalition.Members))
		}
		recent, _ := app.GetRecentSearches()
		assert.Equal(t, []retrieval.RecentSearch{{Name: "The Coalition", Type: "COALITION", ID: card.ID}}, recent)
		assert.Equal(t, "COALITION", app.lastTarget.Type)

		data, _ := json.Marshal(result)
		assert.Contains(t, string(data), `"coalition":{`)
		plain, _ := json.Marshal(TargetResult{})
		assert.NotContains(t, string(plain), "coalition", "omitted for other targets")
	})

	t.Run("a coalition that no longer exists says so", func(t *testing.T) {
		app := newApp(t)
		_, err := app.SearchTarget("Gone", "COALITION", -5)
		assert.EqualError(t, err, "That coalition no longer exists")
		_, err = app.SearchTarget("Gone", "COALITION", 0)
		assert.EqualError(t, err, "That coalition no longer exists")
	})

	t.Run("a typed name that only a coalition has is searched directly", func(t *testing.T) {
		app := newApp(t)
		card := coalitionWith(t, app, "Nobody Home")

		result, err := app.SearchTarget("nobody home", "", 0)

		assert.NoError(t, err)
		assert.Equal(t, card.ID, result.ID)
		assert.Equal(t, "COALITION", result.Type)
	})

	t.Run("a typed name an ESI match shares with a coalition offers both", func(t *testing.T) {
		app := newApp(t)
		card := coalitionWith(t, app, "ursa.")

		result, err := app.SearchTarget("URSA.", "", 0)

		assert.NoError(t, err)
		if assert.Len(t, result.Choices, 2) {
			assert.Equal(t, TargetChoice{Type: "COALITION", ID: card.ID, Name: "ursa."}, result.Choices[0])
			assert.Equal(t, "ALLIANCE", result.Choices[1].Type)
		}
		assert.Nil(t, result.Coalition)
	})
}

func TestMyIntelWithCoalitions(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	t.Run("an ignored ally row naming a coalition gets no card and no name lookup with its negative id", func(t *testing.T) {
		app := newIntelApp(t)
		app.cache.SetIgnoredAllies("COALITION", -1700000000000, []retrieval.IgnoredAlly{{Type: "ALLIANCE", ID: 99003581, Name: "Fraternity."}})
		assert.NoError(t, app.AddIgnoredAlly("PILOT", 2121139046, "Fraternity.", "ALLIANCE"))
		guard := &negativeGuardESI{t: t}
		app.clients.ESIClient = lookupGuard{intelESIClient: intelESIClient{}, guard: guard}

		groups, err := app.GetMyIntel()

		assert.NoError(t, err)
		if assert.Len(t, groups, 1) {
			assert.Equal(t, "Luxxianna Seraph", groups[0].Entity.Name)
		}
		assert.Equal(t, []string{"GetNamesFromEveIDs"}, guard.calls[:1], "the pilot's name is looked up")
	})
}

// lookupGuard answers as intelESIClient does, noting any negative id it is asked a name for
type lookupGuard struct {
	intelESIClient
	guard *negativeGuardESI
}

func (l lookupGuard) GetNamesFromEveIDs(ids []int) (map[int]string, error) {
	_ = l.guard.note("GetNamesFromEveIDs", ids...)
	return l.intelESIClient.GetNamesFromEveIDs(ids)
}

func TestCoalitionSharing(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	export := func(t *testing.T, app *App, cards ...string) []byte {
		data, err := json.Marshal(app.buildIntelFile(cards))
		assert.NoError(t, err)
		return data
	}

	t.Run("exported, a coalition is added by name to another copy, adding only members", func(t *testing.T) {
		from := newIntelApp(t)
		card := coalitionWith(t, from, "The Coalition", [2]string{"URSA.", "ALLIANCE"}, [2]string{"URSA PRIMUS", "CORPORATION"}, [2]string{"High Bear Nation", "ALLIANCE"})
		_, err := from.SetCoalitionNote(card.ID, "a note")
		assert.NoError(t, err)
		_, err = from.SetCoalitionStructures(card.ID, true)
		assert.NoError(t, err)
		other := coalitionWith(t, from, "Not Chosen")
		data := export(t, from, intelGroupKey("COALITION", card.ID))
		assert.NotContains(t, string(data), other.Name)

		to := newIntelApp(t)
		mine := coalitionWith(t, to, "the coalition", [2]string{"URSA.", "ALLIANCE"}, [2]string{"Fraternity.", "ALLIANCE"})
		plan, preview, err := to.planImport(data)
		assert.NoError(t, err)
		assert.Equal(t, 0, preview.Coalitions, "the user has one by that name")
		assert.Equal(t, 2, preview.CoalitionMembers)
		to.applyImport(plan, false)

		got := mustCoalition(t, to, mine.ID)
		assert.Equal(t, "the coalition", got.Name)
		assert.Equal(t, []string{"URSA.", "Fraternity.", "URSA PRIMUS", "High Bear Nation"}, memberNames(got.Members), "nothing removed, the others added")
		assert.Empty(t, got.Note, "the user's own settings are kept")
		assert.False(t, got.IncludeStructures)
		assert.Len(t, to.cache.IntelChecks(), 3, "every member the file named is still to be checked")

		blank := newIntelApp(t)
		plan, preview, err = blank.planImport(data)
		assert.NoError(t, err)
		assert.Equal(t, 1, preview.Coalitions)
		assert.Equal(t, 3, preview.CoalitionMembers)
		blank.applyImport(plan, false)
		cards, _ := blank.GetCoalitions()
		if assert.Len(t, cards, 1) {
			assert.Equal(t, "a note", cards[0].Note)
			assert.True(t, cards[0].IncludeStructures)
			assert.Len(t, cards[0].Members, 3)
			assert.Less(t, cards[0].ID, 0)
		}
	})

	t.Run("malformed entries are skipped and counted, names cleaned", func(t *testing.T) {
		app := newIntelApp(t)
		file := intelFile{Format: intelFileFormat, Version: intelFileVersion, Cards: []intelFileCard{}, Coalitions: []intelFileCoalition{
			{Name: "  Clean​Me\n ", Members: []intelFileEntity{
				{Type: "ALLIANCE", ID: 99013330, Name: "U\u0000RSA."},
				{Type: "PILOT", ID: 2121139046, Name: "A pilot"},
				{Type: "ALLIANCE", ID: -4, Name: "negative"},
				{Type: "CORPORATION", ID: 98744491, Name: ""},
				{Type: "ALLIANCE", ID: 99013330, Name: "twice"},
			}},
			{Name: " \n ", Members: []intelFileEntity{{Type: "ALLIANCE", ID: 99013580}}},
		}}
		data, _ := json.Marshal(file)

		plan, preview, err := app.planImport(data)

		assert.NoError(t, err)
		assert.Equal(t, 1, preview.Coalitions)
		assert.Equal(t, 2, preview.CoalitionMembers)
		assert.Equal(t, 4, preview.Skipped, "a pilot, a negative id, and a coalition with no name and its member")
		app.applyImport(plan, false)
		cards, _ := app.GetCoalitions()
		if assert.Len(t, cards, 1) {
			assert.Equal(t, "CleanMe", cards[0].Name)
			assert.Equal(t, []string{"URSA.", "corporation 98744491"}, memberNames(cards[0].Members))
		}
	})

	t.Run("a file with no coalitions reads as before, and older copies ignore the list", func(t *testing.T) {
		app := newIntelApp(t)
		data, _ := json.Marshal(app.buildIntelFile(nil))
		assert.NotContains(t, string(data), "coalitions")
		_, preview, err := app.planImport(data)
		assert.NoError(t, err)
		assert.Equal(t, ImportPreview{NoteChanges: []IntelNoteChange{}, SkippedExamples: []string{}}, preview)
	})

	t.Run("the intel check drops a member that does not exist and corrects a name", func(t *testing.T) {
		app := newIntelApp(t)
		file := intelFile{Format: intelFileFormat, Version: intelFileVersion, Cards: []intelFileCard{}, Coalitions: []intelFileCoalition{
			{Name: "Imported", Members: []intelFileEntity{
				{Type: "ALLIANCE", ID: 99013330, Name: "ursa"},
				{Type: "ALLIANCE", ID: 99999999, Name: "Not Real"},
				{Type: "CORPORATION", ID: 99013580, Name: "Wrong Type"},
			}},
		}}
		data, _ := json.Marshal(file)
		plan, _, err := app.planImport(data)
		assert.NoError(t, err)
		app.applyImport(plan, false)

		app.runIntelChecks()

		cards, _ := app.GetCoalitions()
		if assert.Len(t, cards, 1) {
			assert.Equal(t, []string{"URSA."}, memberNames(cards[0].Members))
		}
		assert.Empty(t, app.cache.IntelChecks())
	})
}

func TestCoalitionTargetHelpers(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	const coalitionID = -1700000000000
	target := retrieval.IntelTarget{IDNumber: coalitionID, TypeOfEntity: types.COALITION, Name: "The Coalition", Holdings: []retrieval.TargetGroup{
		{Type: types.ALLIANCE, ID: 99000001, Name: "Alpha", Member: true},
		{Type: types.CORPORATION, ID: 98000002, Name: "Beta", Member: true},
		{Type: types.CORPORATION, ID: 98000009, Name: "Gamma"},
	}}

	t.Run("the scope has a row per member and affiliate and none for the coalition", func(t *testing.T) {
		at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		scope := buildScope(&target, []KillmailCard{{Via: "Alpha", Time: at}, {Via: "Gamma", Time: at}, {Via: "Alpha", Time: at.Add(time.Hour)}}, SearchLimits{MaxKillmails: 20})
		names, counts := []string{}, []int{}
		for _, source := range scope.Sources {
			assert.False(t, source.Main)
			names, counts = append(names, source.Name), append(counts, source.Killmails)
		}
		assert.Equal(t, []string{"Alpha", "Beta", "Gamma"}, names)
		assert.Equal(t, []int{2, 0, 1}, counts)
	})

	t.Run("a loss flown for a member is the coalition's own, one for an affiliate is tagged", func(t *testing.T) {
		lostBy := func(corpID int, allianceID int) *retrieval.Killmail {
			km := &retrieval.Killmail{Victim: &retrieval.KillmailVictim{CorpAtKill: &retrieval.Corporation{CorporationID: corpID}}}
			if allianceID != 0 {
				km.Victim.AllianceAtKill = &retrieval.Alliance{AllianceID: allianceID}
			}
			return km
		}
		assert.Nil(t, affiliateOf(&target, lostBy(98000100, 99000001)))
		assert.Nil(t, affiliateOf(&target, lostBy(98000002, 0)))
		if affiliate := affiliateOf(&target, lostBy(98000009, 0)); assert.NotNil(t, affiliate) {
			assert.Equal(t, "Gamma", affiliate.Name)
		}
		assert.Nil(t, affiliateOf(&target, lostBy(98000777, 0)))
	})

	t.Run("the PDF words the type and has no logo to fetch for a coalition", func(t *testing.T) {
		assert.Equal(t, "Coalition", pdfTargetType("COALITION"))
		assert.Equal(t, "ALLIANCE", pdfTargetType("ALLIANCE"))
		assert.Empty(t, pdfImageURLs(types.COALITION, coalitionID, IntelReport{}))
	})
}
