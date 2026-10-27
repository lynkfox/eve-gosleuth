package gui

import (
	"encoding/json"
	"testing"

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

	t.Run("an import is untrusted: names come from the ESI, and wrong types, unknown ids, and self links are skipped", func(t *testing.T) {
		file := `{"format": "gosleuth-intel", "version": 1, "cards": [
			{"entity": {"type": "ALLIANCE", "id": 99013330, "name": "Goonswarm Federation"}, "holdings": [
				{"type": "ALLIANCE", "id": 99013580, "name": "x"},
				{"type": "CORPORATION", "id": 12345, "name": "made up"},
				{"type": "ALLIANCE", "id": 99013330}
			]},
			{"entity": {"type": "ALLIANCE", "id": 98744491, "name": "a corp called an alliance"}, "holdings": [{"type": "ALLIANCE", "id": 99013580}]},
			{"entity": {"type": "SYSTEM", "id": 30000142}, "holdings": [{"type": "ALLIANCE", "id": 99013580}]},
			{"entity": {"type": "PILOT", "id": 2121139046}, "ignoredAllies": [{"type": "PILOT", "id": 2121139046}, {"type": "ALLIANCE", "id": 99003581, "name": "Fake"}]}
		]}`
		app := newIntelApp(t)

		plan, preview, err := app.planImport([]byte(file))

		assert.NoError(t, err)
		assert.Equal(t, 1, preview.NewHoldings)
		assert.Equal(t, 1, preview.NewIgnored)
		assert.Equal(t, 5, preview.Skipped)
		assert.Equal(t, "URSA.", plan.newHoldings[0].MainName, "not the file's Goonswarm Federation")
		assert.Equal(t, "Fraternity.", plan.newIgnored["PILOT:2121139046"][0].Name)
	})

	t.Run("files that are not My Intel exports, or another version of them, are refused", func(t *testing.T) {
		app := newIntelApp(t)
		for _, file := range []string{`not json`, `{"format": "something else", "version": 1}`, `{"format": "gosleuth-intel", "version": 2}`} {
			_, _, err := app.planImport([]byte(file))
			assert.Error(t, err, file)
		}
	})
}
