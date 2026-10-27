package gui

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/retrieval"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestMyIntelSharing(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	// shared is an app with a holding with a note and an ignored ally saved
	shared := func(t *testing.T) *App {
		app := newIntelApp(t)
		_, err := app.SaveHolding(99013330, "ALLIANCE", "URSA.", "High Bear Nation", "ALLIANCE", "supers parked here")
		assert.NoError(t, err)
		assert.NoError(t, app.AddIgnoredAlly("PILOT", 2121139046, "Fraternity.", "ALLIANCE"))
		return app
	}
	exported := func(t *testing.T, app *App, cards ...string) []byte {
		data, err := json.Marshal(app.buildIntelFile(cards))
		assert.NoError(t, err)
		return data
	}
	everyCard := []string{"ALLIANCE:99013330", "PILOT:2121139046"}

	t.Run("an export imported elsewhere gives the same My Intel", func(t *testing.T) {
		from := shared(t)
		to := newIntelApp(t)

		plan, preview, err := to.planImport(exported(t, from, everyCard...))
		assert.NoError(t, err)
		assert.Equal(t, ImportPreview{NewHoldings: 1, NewIgnored: 1, NoteChanges: []IntelNoteChange{}, SkippedExamples: []string{}}, preview)
		to.applyImport(plan, false)

		// tickers are only shown once cached, which the import's checks did in one and not the other
		withoutTickers := func(groups []IntelGroup) []IntelGroup {
			for index := range groups {
				groups[index].Entity.Ticker = ""
				for at := range groups[index].Holdings {
					groups[index].Holdings[at].Ticker = ""
				}
				for at := range groups[index].IgnoredAllies {
					groups[index].IgnoredAllies[at].Ticker = ""
				}
			}
			return groups
		}
		want, _ := from.GetMyIntel()
		got, _ := to.GetMyIntel()
		assert.Equal(t, withoutTickers(want), withoutTickers(got))
	})

	t.Run("only the chosen cards are exported, a holding going with its main", func(t *testing.T) {
		file := shared(t).buildIntelFile([]string{"PILOT:2121139046", "ALLIANCE:99013580"})
		assert.Len(t, file.Cards, 1, "High Bear Nation has no card, as it is only URSA.'s holding, which goes with URSA.")
		assert.Equal(t, "Luxxianna Seraph", file.Cards[0].Entity.Name)
		assert.Equal(t, "Fraternity.", file.Cards[0].IgnoredAllies[0].Name)
	})

	t.Run("a card holds its holdings, with their notes, and its ignored allies", func(t *testing.T) {
		file := shared(t).buildIntelFile(everyCard)
		assert.Equal(t, []intelFileCard{
			{Entity: intelFileEntity{Type: "PILOT", ID: 2121139046, Name: "Luxxianna Seraph"}, IgnoredAllies: []intelFileEntity{{Type: "ALLIANCE", ID: 99003581, Name: "Fraternity."}}},
			{Entity: intelFileEntity{Type: "ALLIANCE", ID: 99013330, Name: "URSA."},
				Holdings: []intelFileHolding{{intelFileEntity: intelFileEntity{Type: "ALLIANCE", ID: 99013580, Name: "High Bear Nation"}, Note: "supers parked here"}}},
		}, file.Cards)
	})

	t.Run("importing what is already saved changes nothing", func(t *testing.T) {
		app := shared(t)
		_, preview, err := app.planImport(exported(t, app, everyCard...))
		assert.NoError(t, err)
		assert.Equal(t, 1, preview.KnownHoldings)
		assert.Equal(t, 1, preview.KnownIgnored)
		assert.Zero(t, preview.NewHoldings+preview.NewIgnored)
	})

	t.Run("a different note keeps the user's unless they take the file's", func(t *testing.T) {
		from := shared(t)
		data := exported(t, from, everyCard...)
		for _, useTheirs := range []bool{false, true} {
			to := newIntelApp(t)
			_, err := to.SaveHolding(99013330, "ALLIANCE", "URSA.", "High Bear Nation", "ALLIANCE", "my own note")
			assert.NoError(t, err)

			plan, preview, err := to.planImport(data)
			assert.NoError(t, err)
			assert.Equal(t, []IntelNoteChange{{Main: "URSA.", Holding: "High Bear Nation", Mine: "my own note", Theirs: "supers parked here"}}, preview.NoteChanges)
			to.applyImport(plan, useTheirs)

			holdings, _ := to.cache.HoldingLinks("ALLIANCE", 99013330)
			if useTheirs {
				assert.Equal(t, "supers parked here", holdings[0].Note)
			} else {
				assert.Equal(t, "my own note", holdings[0].Note)
			}
		}
	})

	t.Run("an import is read at once, taking the file's names, then the check corrects them and removes what does not exist as the type the file says", func(t *testing.T) {
		file := `{"format": "gosleuth-intel", "version": 1, "cards": [
			{"entity": {"type": "ALLIANCE", "id": 99013330, "name": "Goonswarm Federation"}, "holdings": [
				{"type": "ALLIANCE", "id": 99013580, "name": "x"},
				{"type": "CORPORATION", "id": 12345, "name": "made up"}
			]},
			{"entity": {"type": "ALLIANCE", "id": 98744491, "name": "a corp called an alliance"}, "holdings": [{"type": "ALLIANCE", "id": 99013580}]},
			{"entity": {"type": "PILOT", "id": 2121139046}, "ignoredAllies": [{"type": "ALLIANCE", "id": 99003581, "name": "Fake\u202e"}, {"type": "ALLIANCE", "id": 54321}]}
		]}`
		app := newIntelApp(t)

		plan, preview, err := app.planImport([]byte(file))
		assert.NoError(t, err)
		assert.Equal(t, 3, preview.NewHoldings, "nothing checked yet")
		assert.Equal(t, 2, preview.NewIgnored)
		assert.Equal(t, "Goonswarm Federation", plan.newHoldings[0].MainName, "the file's name, until checked")
		assert.Equal(t, "Fake", plan.newIgnored["PILOT:2121139046"][0].Name, "without the invisible right to left mark")
		app.applyImport(plan, false)

		app.runIntelChecks()

		groups, _ := app.GetMyIntel()
		names := []string{}
		for _, group := range groups {
			names = append(names, group.Entity.Name)
		}
		assert.Equal(t, []string{"Luxxianna Seraph", "URSA."}, names, "the corp called an alliance is gone, URSA. named as EVE does")
		assert.Len(t, groups[1].Holdings, 1, "the made up corp is gone")
		assert.Equal(t, "High Bear Nation", groups[1].Holdings[0].Name)
		assert.Equal(t, []string{"Fraternity."}, []string{groups[0].IgnoredAllies[0].Name})
		assert.Len(t, groups[0].IgnoredAllies, 1)
		assert.Empty(t, app.cache.IntelChecks(), "all checked")
	})

	t.Run("a check the ESI can not be asked for, ie while rate limited, keeps everything for the next try", func(t *testing.T) {
		client := unreachableESIClient{}
		cache := retrieval.NewCache(client, filepath.Join(t.TempDir(), "cache.db"))
		t.Cleanup(func() { cache.Close() })
		app := &App{cache: cache, clients: retrieval.CollectionClientPool{ESIClient: cache}}
		plan, _, err := app.planImport(exported(t, shared(t), everyCard...))
		assert.NoError(t, err)
		app.applyImport(plan, false)

		app.runIntelChecks()

		assert.Len(t, app.cache.IntelChecks(), 4, "URSA., High Bear Nation, Luxxianna Seraph, and Fraternity. still to check")
		groups, _ := app.GetMyIntel()
		assert.Len(t, groups, 2, "nothing removed")
	})

	t.Run("a check stops when the app closes, leaving the rest for the next start", func(t *testing.T) {
		app := newIntelApp(t)
		plan, _, err := app.planImport(exported(t, shared(t), everyCard...))
		assert.NoError(t, err)
		app.applyImport(plan, false)
		app.closing.Store(true)

		app.runIntelChecks()

		assert.Len(t, app.cache.IntelChecks(), 4)
	})

	t.Run("malformed entries are left out, each counted once, a card whose own entity is not allowed counting all of its entries", func(t *testing.T) {
		file := `{"format": "gosleuth-intel", "version": 1, "cards": [
			{"entity": {"type": "SYSTEM", "id": 30000142, "name": "Jita"},
			 "holdings": [{"type": "ALLIANCE", "id": 99013580}, {"type": "CORPORATION", "id": 54321}],
			 "ignoredAllies": [{"type": "ALLIANCE", "id": 99003581}, {"type": "PILOT", "id": 2121139046}]},
			{"entity": {"type": "ALLIANCE", "id": 99013330}, "holdings": [{"type": "ALLIANCE", "id": 99013330}, {"type": "ALLIANCE", "id": 0}]}
		]}`

		_, preview, err := newIntelApp(t).planImport([]byte(file))

		assert.NoError(t, err)
		assert.Equal(t, 6, preview.Skipped, "the system's 2 holdings and 2 allies, a self holding, and one with no id")
	})

	t.Run("applying a preview keeps what the user saved since, rather than overwriting or doubling it", func(t *testing.T) {
		app := newIntelApp(t)
		plan, _, err := app.planImport(exported(t, shared(t), everyCard...))
		assert.NoError(t, err)
		// saved after the preview
		_, err = app.SaveHolding(99013330, "ALLIANCE", "URSA.", "High Bear Nation", "ALLIANCE", "my note")
		assert.NoError(t, err)
		assert.NoError(t, app.AddIgnoredAlly("PILOT", 2121139046, "Fraternity.", "ALLIANCE"))

		app.applyImport(plan, false)

		holdings, _ := app.cache.HoldingLinks("ALLIANCE", 99013330)
		assert.Equal(t, "my note", holdings[0].Note)
		assert.Len(t, app.cache.IgnoredAlliesFor("PILOT", 2121139046), 1)
	})

	t.Run("files that are not My Intel exports, or another version of them, are refused", func(t *testing.T) {
		app := newIntelApp(t)
		for _, file := range []string{`not json`, `{"format": "something else", "version": 1}`, `{"format": "gosleuth-intel", "version": 2}`} {
			_, _, err := app.planImport([]byte(file))
			assert.Error(t, err, file)
		}
	})
}

// unreachableESIClient fails every lookup as the ESI does while it is rate limited, not with a 404
type unreachableESIClient struct {
	intelESIClient
}

func (m unreachableESIClient) GetAllianceInfo(id int) (*api.ESIAlliance, error) {
	return nil, api.ErrRateLimited
}

func (m unreachableESIClient) GetCorporationInfo(id int) (*api.ESICorporation, error) {
	return nil, api.ErrRateLimited
}

func (m unreachableESIClient) GetCharacterInfo(id int) (*api.ESICharacter, error) {
	return nil, api.ErrRateLimited
}

func (m unreachableESIClient) GetEntitiesFromEveIDs(ids []int) (map[int]api.EntityIDAndName, error) {
	return nil, api.ErrRateLimited
}
