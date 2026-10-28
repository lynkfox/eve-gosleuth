package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFitEFT(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	fit := &retrieval.Fit{Hull: &retrieval.Item{ItemID: 587, Name: "Rifter"}, Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048, Name: "Damage Control II"}}}}
	killmails := []*retrieval.Killmail{
		{KillmailID: 1, Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{Name: "Luxxianna Seraph"}, Fit: fit}},
		{KillmailID: 2, Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{Name: "Someone"}}},
	}
	app := &App{lastAllies: &alliesSource{killmails: killmails}}

	t.Run("a killmail in the last search is written as EFT", func(t *testing.T) {
		text, err := app.FitEFT(1)
		assert.NoError(t, err)
		assert.Equal(t, "[Rifter, GoSleuth - Luxxianna Seraph]\n\nDamage Control II", text)
	})

	t.Run("a killmail without a fit, or not in the last search, is an error", func(t *testing.T) {
		_, err := app.FitEFT(2)
		assert.Error(t, err)
		_, err = app.FitEFT(3)
		assert.Error(t, err)
	})

	t.Run("before any search there is nothing to export", func(t *testing.T) {
		_, err := (&App{}).FitEFT(1)
		assert.Error(t, err)
	})
}

func TestBuildFits(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	pilot := &retrieval.Pilot{CharacterID: 10, Name: "Luxxianna Seraph"}
	loss := func(id int, shipID int, shipName string, at time.Time) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: at,
			System:       &retrieval.System{Name: "Jita"},
			Victim: &retrieval.KillmailVictim{
				Pilot: pilot,
				Ship:  &retrieval.Item{ItemID: shipID, Name: shipName},
				Fit: &retrieval.Fit{
					Hull: &retrieval.Item{ItemID: shipID, Name: shipName},
					Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048, Name: "Damage Control II"}}},
				},
			},
		}
	}
	day := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	kill := loss(5, 587, "Rifter", day)
	kill.Victim.Pilot = &retrieval.Pilot{CharacterID: 99, Name: "Someone Else"}
	unfitted := loss(6, 587, "Rifter", day)
	unfitted.Victim.Fit.Lows = []*retrieval.FittedModule{nil, nil}

	fits := buildFits([]*retrieval.Killmail{
		loss(1, 587, "Rifter", day),
		loss(2, 670, "Capsule", day),
		loss(7, 11129, "Gallente Shuttle", day),
		loss(8, 32880, "Venture", day),
		loss(9, 648, "Badger", day),
		loss(10, 35832, "Astrahus", day),
		unfitted,
		loss(3, 24690, "Hurricane", day),
		loss(4, 587, "Rifter", day.Add(time.Hour)),
		kill,
	}, target)

	ids := []int{}
	for _, fit := range fits {
		ids = append(ids, fit.KillmailID)
	}
	assert.Equal(t, []int{3, 4, 1}, ids, "by ship name then newest first, without the kill, the unfitted loss, or the non-combat hulls")
	assert.Contains(t, fits[0].EFT, "[Hurricane, GoSleuth - Luxxianna Seraph]")
}
