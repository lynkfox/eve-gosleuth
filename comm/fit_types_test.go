package comm_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const fitTypesFixture = `{"sde_build": 42,
"cargo_kept": {"categories": [8], "groups": [303]},
"types": {
"1": {"name": "Hull", "group": 25, "category": 6, "high_slots": 3, "mid_slots": 2, "low_slots": 1, "turret_slots": 2},
"2": {"name": "Shield", "group": 38, "category": 7, "slot": "mid"},
"3": {"name": "Ammo", "group": 85, "category": 8},
"4": {"name": "Booster", "group": 303, "category": 20},
"5": {"name": "Implant", "group": 300, "category": 20}
}}`

func TestLoadFitTypes(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("types are read with their id from the key", func(t *testing.T) {
		files := fstest.MapFS{"fit_types.json": {Data: []byte(fitTypesFixture)}}
		assert.NoError(t, comm.LoadFitTypes(files, "fit_types.json"))

		hull := comm.FitTypeOf(1)
		if assert.NotNil(t, hull) {
			assert.Equal(t, comm.FitType{TypeID: 1, Name: "Hull", GroupID: 25, CategoryID: 6, HighSlots: 3, MidSlots: 2, LowSlots: 1, TurretSlots: 2}, *hull)
		}
		assert.Equal(t, comm.SlotMid, comm.FitTypeOf(2).Slot)
		assert.Equal(t, 42, comm.FitTypesSDEBuild())
	})

	t.Run("a type the file does not have is nil", func(t *testing.T) {
		assert.Nil(t, comm.FitTypeOf(999))
	})

	t.Run("cargo is kept by category or group", func(t *testing.T) {
		assert.True(t, comm.KeptInFitCargo(comm.FitTypeOf(3)), "a charge")
		assert.True(t, comm.KeptInFitCargo(comm.FitTypeOf(4)), "a booster")
		assert.False(t, comm.KeptInFitCargo(comm.FitTypeOf(5)), "an implant outside the booster group")
		assert.False(t, comm.KeptInFitCargo(comm.FitTypeOf(2)), "a module")
		assert.False(t, comm.KeptInFitCargo(nil), "an unknown type")
	})

	t.Run("a file that fails to load leaves the previous types", func(t *testing.T) {
		files := fstest.MapFS{"broken.json": {Data: []byte(`{"types": {"1": `)}}

		assert.Error(t, comm.LoadFitTypes(files, "broken.json"))
		assert.Error(t, comm.LoadFitTypes(files, "missing.json"))
		assert.Equal(t, "Hull", comm.FitTypeOf(1).Name)
		assert.Equal(t, 42, comm.FitTypesSDEBuild())
	})

	t.Run("a file without types is an error, and leaves the previous types", func(t *testing.T) {
		files := fstest.MapFS{"empty.json": {Data: []byte(`{}`)}, "null.json": {Data: []byte(`{"types": null}`)}}

		assert.Error(t, comm.LoadFitTypes(files, "empty.json"))
		assert.Error(t, comm.LoadFitTypes(files, "null.json"))
		assert.Equal(t, "Hull", comm.FitTypeOf(1).Name)
		assert.Equal(t, 42, comm.FitTypesSDEBuild())
	})
}

func TestEmbeddedFitTypes(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.LoadFitTypes(staticdata.Files, "fit_types.json"))

	assert.Greater(t, comm.FitTypesSDEBuild(), 0)

	t.Run("a T3 cruiser hull has subsystem and rig slots but no fixed module slots", func(t *testing.T) {
		tengu := comm.FitTypeOf(29984)
		if assert.NotNil(t, tengu) {
			assert.Equal(t, "Tengu", tengu.Name)
			assert.Equal(t, 29984, tengu.TypeID)
			assert.Equal(t, 6, tengu.CategoryID)
			assert.Positive(t, tengu.SubsystemSlots)
			assert.Equal(t, 3, tengu.RigSlots)
			assert.Zero(t, tengu.HighSlots+tengu.MidSlots+tengu.LowSlots)
		}
	})

	t.Run("a subsystem adds slots to its hull", func(t *testing.T) {
		node := comm.FitTypeOf(45591) // Tengu Defensive - Amplification Node
		if assert.NotNil(t, node) {
			assert.Equal(t, comm.SlotSubsystem, node.Slot)
			assert.Equal(t, []int{0, 3, 1}, []int{node.HighSlotModifier, node.MidSlotModifier, node.LowSlotModifier})
		}
	})

	t.Run("modules have their slot", func(t *testing.T) {
		assert.Equal(t, comm.SlotMid, comm.FitTypeOf(3841).Slot, "Large Shield Extender II")
	})

	t.Run("a charge has no slot and is kept in cargo", func(t *testing.T) {
		missile := comm.FitTypeOf(209) // Scourge Heavy Missile
		if assert.NotNil(t, missile) {
			assert.Empty(t, missile.Slot)
			assert.True(t, comm.KeptInFitCargo(missile))
		}
	})

	t.Run("boosters and nanite paste are kept in cargo, drones are not", func(t *testing.T) {
		assert.True(t, comm.KeptInFitCargo(comm.FitTypeOf(3898)), "Quafe Zero Classic")
		assert.True(t, comm.KeptInFitCargo(comm.FitTypeOf(28668)), "Nanite Repair Paste")
		assert.False(t, comm.KeptInFitCargo(comm.FitTypeOf(2185)), "Hammerhead II")
	})

	t.Run("a type is in its market group and every group above it", func(t *testing.T) {
		pulse := comm.FitTypeOf(3057) // Mega Pulse Laser II, under Large (573) under Pulse Lasers (558) under Energy Turrets (88)
		assert.True(t, comm.InMarketGroup(pulse, 573))
		assert.True(t, comm.InMarketGroup(pulse, 558))
		assert.True(t, comm.InMarketGroup(pulse, 88))
		assert.False(t, comm.InMarketGroup(pulse, 557), "Beam Lasers")
		assert.True(t, comm.InMarketGroup(comm.FitTypeOf(28211), 911), "Garde II, a sentry drone")
		assert.False(t, comm.InMarketGroup(comm.FitTypeOf(2185), 911), "Hammerhead II is not a sentry")
		assert.False(t, comm.InMarketGroup(nil, 88))
	})

	t.Run("a hull has its size and its bonuses as text", func(t *testing.T) {
		ashimmu := comm.FitTypeOf(17922)
		if assert.NotNil(t, ashimmu) {
			assert.Equal(t, 2, ashimmu.RigSize, "a cruiser takes medium rigs")
			assert.Contains(t, ashimmu.Bonuses, "bonus to Energy Nosferatu and Energy Neutralizer drain amount")
		}
	})
}
