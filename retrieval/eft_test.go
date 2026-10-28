package retrieval_test

import (
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFitEFT(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	loadFitFixtures(t)

	tests := []struct {
		name string
		fit  func(cache *retrieval.CachedEveIDs) *retrieval.Fit
		want string
	}{
		{
			name: "a ship with empty slots, loaded charges, drones, and cargo with its booster",
			fit:  func(cache *retrieval.CachedEveIDs) *retrieval.Fit { return cache.BuildFit(587, rifterItems()) },
			want: `[Rifter, GoSleuth - Pilot - 1]

Damage Control II
[Empty Low slot]
[Empty Low slot]
[Empty Low slot]

Warp Scrambler II
[Empty Med slot]
[Empty Med slot]

125mm Gatling AutoCannon II, Republic Fleet EMP S
125mm Gatling AutoCannon II, Republic Fleet EMP S
125mm Gatling AutoCannon II, Republic Fleet EMP S
[Empty High slot]

[Empty Rig slot]
[Empty Rig slot]
[Empty Rig slot]


Hornet EC-300 x2


Republic Fleet EMP S x200
Nanite Repair Paste x50
Synth Crash Booster x3`,
		},
		{
			name: "a T3 cruiser has its subsystems after its rigs",
			fit:  func(cache *retrieval.CachedEveIDs) *retrieval.Fit { return cache.BuildFit(29984, tenguItems()) },
			want: `[Tengu, GoSleuth - Pilot - 1]

Damage Control II
[Empty Low slot]
[Empty Low slot]

Large Shield Extender II
Large Shield Extender II
Warp Scrambler II
[Empty Med slot]
[Empty Med slot]

Heavy Missile Launcher II, Scourge Heavy Missile
Heavy Missile Launcher II, Scourge Heavy Missile
Heavy Missile Launcher II, Scourge Heavy Missile
Heavy Missile Launcher II, Scourge Heavy Missile
Heavy Missile Launcher II, Scourge Heavy Missile

Medium Core Defense Field Extender I
[Empty Rig slot]
[Empty Rig slot]

Tengu Core - Augmented Graviton Reactor
Tengu Defensive - Supplemental Screening
Tengu Offensive - Accelerated Ejection Bay
Tengu Propulsion - Fuel Catalyst


Scourge Heavy Missile x1000`,
		},
		{
			name: "fighters follow drones, a blank line apart",
			fit: func(cache *retrieval.CachedEveIDs) *retrieval.Fit {
				return cache.BuildFit(23, fitItems([4]int{87, 2488, 2, 0}, [4]int{158, 40556, 6, 0}))
			},
			want: "[Cargo Container, GoSleuth - Pilot - 1]\n\nHornet EC-300 x2\n\nTemplar II x6",
		},
		{
			name: "a structure's services come last",
			fit: func(cache *retrieval.CachedEveIDs) *retrieval.Fit {
				return cache.BuildFit(35832, fitItems([4]int{164, 35894, 1, 0}))
			},
			want: "[Astrahus, GoSleuth - Pilot - 1]\n\n" +
				"[Empty Low slot]\n[Empty Low slot]\n\n" +
				"[Empty Med slot]\n[Empty Med slot]\n[Empty Med slot]\n\n" +
				"[Empty High slot]\n[Empty High slot]\n[Empty High slot]\n\n" +
				"[Empty Rig slot]\n[Empty Rig slot]\n[Empty Rig slot]\n\n" +
				"Standup Cloning Center I\n[Empty Service slot]\n[Empty Service slot]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
			assert.Equal(t, test.want, test.fit(cache).EFT("GoSleuth - Pilot - 1"))
		})
	}

	t.Run("a booster also in the cargo is added to its stack without changing the fit", func(t *testing.T) {
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
		fit := cache.BuildFit(23, fitItems([4]int{5, 15466, 2, 0}, [4]int{88, 15466, 1, 0}))

		assert.Equal(t, "[Cargo Container, GoSleuth - Pilot - 1]\n\nSynth Crash Booster x3", fit.EFT("GoSleuth - Pilot - 1"))
		assert.Equal(t, 2, fit.Cargo[0].Quantity)
	})

	t.Run("stacks of types fit_types.json does not have come after the known ones, whatever order they were listed in", func(t *testing.T) {
		known := &retrieval.FitCargo{Item: &retrieval.Item{ItemID: 2488, Name: "Hornet EC-300"}, Quantity: 5}
		unknownA := &retrieval.FitCargo{Item: &retrieval.Item{ItemID: 99001, Name: "Aardvark Drone"}, Quantity: 1}
		unknownZ := &retrieval.FitCargo{Item: &retrieval.Item{ItemID: 99002, Name: "Zebra Drone"}, Quantity: 1}
		hull := &retrieval.Item{ItemID: 23, Name: "Cargo Container"}
		want := "[Cargo Container, T]\n\nHornet EC-300 x5\nAardvark Drone x1\nZebra Drone x1"

		for _, order := range [][]*retrieval.FitCargo{{unknownZ, known, unknownA}, {unknownA, unknownZ, known}, {known, unknownZ, unknownA}} {
			assert.Equal(t, want, (&retrieval.Fit{Hull: hull, Drones: order}).EFT("T"))
		}
	})
}

func TestEFTTitle(t *testing.T) {
	t.Run("a pilot's loss is titled with the pilot", func(t *testing.T) {
		km := &retrieval.Killmail{KillmailID: 12345, Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{Name: "Luxxianna Seraph"}}}
		assert.Equal(t, "GoSleuth - Luxxianna Seraph", retrieval.EFTTitle(km))
	})

	t.Run("a structure's loss is titled with its corp", func(t *testing.T) {
		km := &retrieval.Killmail{KillmailID: 12345, Victim: &retrieval.KillmailVictim{CorpAtKill: &retrieval.Corporation{Name: "Owners"}}}
		assert.Equal(t, "GoSleuth - Owners", retrieval.EFTTitle(km))
	})
}
