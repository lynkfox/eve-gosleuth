package gui

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHoldingMain(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	app := func(includeHoldings bool) *App {
		cache := retrieval.NewCache(api.NewESIClient(), filepath.Join(t.TempDir(), "cache.db"))
		settings := defaultSettings()
		settings.IncludeHoldings = includeHoldings
		a := &App{cache: cache, settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: settings}
		for _, main := range []string{"Zeta Alliance", "Alpha Alliance"} {
			cache.SaveHoldingLink(retrieval.HoldingLink{MainType: "ALLIANCE", MainID: len(main), MainName: main, HoldingType: "CORPORATION", HoldingID: 98000002, HoldingName: "Holding Corp"})
		}
		return a
	}
	holding := &retrieval.IntelTarget{IDNumber: 98000002, TypeOfEntity: types.CORPORATION}

	t.Run("a holding is searched as its main, the first by name when it has several", func(t *testing.T) {
		main, found := app(true).holdingMain(holding)

		assert.True(t, found)
		assert.Equal(t, "Alpha Alliance", main.MainName)
	})

	t.Run("not while the Include holdings preference is off", func(t *testing.T) {
		_, found := app(false).holdingMain(holding)
		assert.False(t, found)
	})

	t.Run("a corp that holds for none, or a pilot, is searched itself", func(t *testing.T) {
		a := app(true)
		_, found := a.holdingMain(&retrieval.IntelTarget{IDNumber: 98000009, TypeOfEntity: types.CORPORATION})
		assert.False(t, found)
		_, found = a.holdingMain(&retrieval.IntelTarget{IDNumber: 98000002, TypeOfEntity: types.PILOT})
		assert.False(t, found)
	})
}
