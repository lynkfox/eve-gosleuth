package retrieval_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const fitFlagsFixture = `{"slots": [
	{"slot": "cargo", "first_flag": 5, "last_flag": 5, "name": "Cargo"},
	{"slot": "low", "first_flag": 11, "last_flag": 18, "name": "Low slots"},
	{"slot": "mid", "first_flag": 19, "last_flag": 26, "name": "Mid slots"},
	{"slot": "high", "first_flag": 27, "last_flag": 34, "name": "High slots"},
	{"slot": "drone_bay", "first_flag": 87, "last_flag": 87, "name": "Drone bay"},
	{"slot": "booster", "first_flag": 88, "last_flag": 88, "name": "Booster"},
	{"slot": "implant", "first_flag": 89, "last_flag": 89, "name": "Implant"},
	{"slot": "ignored", "first_flag": 90, "last_flag": 90, "name": "Ship maintenance bay"},
	{"slot": "rig", "first_flag": 92, "last_flag": 99, "name": "Rig slots"},
	{"slot": "subsystem", "first_flag": 125, "last_flag": 132, "name": "Subsystem slots"},
	{"slot": "ignored", "first_flag": 155, "last_flag": 155, "name": "Fleet hangar"},
	{"slot": "fighter_bay", "first_flag": 158, "last_flag": 158, "name": "Fighter bay"},
	{"slot": "fighter_tube", "first_flag": 159, "last_flag": 163, "name": "Fighter tubes"},
	{"slot": "service", "first_flag": 164, "last_flag": 171, "name": "Structure services"}
]}`

const fitTypesFixture = `{"sde_build": 1, "cargo_kept": {"categories": [8], "groups": [303, 916]}, "types": {
	"587":   {"name": "Rifter", "group": 25, "category": 6, "high_slots": 4, "mid_slots": 3, "low_slots": 4, "rig_slots": 3},
	"29984": {"name": "Tengu", "group": 963, "category": 6, "rig_slots": 3, "subsystem_slots": 4},
	"35832": {"name": "Astrahus", "group": 1657, "category": 65, "high_slots": 3, "mid_slots": 3, "low_slots": 2, "rig_slots": 3, "service_slots": 3},
	"23":    {"name": "Cargo Container", "group": 12, "category": 2},
	"45589": {"name": "Tengu Core - Augmented Graviton Reactor", "group": 954, "category": 32, "slot": "subsystem", "mid_slot_modifier": 2, "low_slot_modifier": 1},
	"45597": {"name": "Tengu Defensive - Supplemental Screening", "group": 956, "category": 32, "slot": "subsystem", "mid_slot_modifier": 2, "low_slot_modifier": 1},
	"45601": {"name": "Tengu Offensive - Accelerated Ejection Bay", "group": 957, "category": 32, "slot": "subsystem", "high_slot_modifier": 5},
	"45605": {"name": "Tengu Propulsion - Fuel Catalyst", "group": 958, "category": 32, "slot": "subsystem", "mid_slot_modifier": 1, "low_slot_modifier": 1},
	"2410":  {"name": "Heavy Missile Launcher II", "group": 510, "category": 7, "slot": "high"},
	"2873":  {"name": "125mm Gatling AutoCannon II", "group": 55, "category": 7, "slot": "high"},
	"448":   {"name": "Warp Scrambler II", "group": 52, "category": 7, "slot": "mid"},
	"3841":  {"name": "Large Shield Extender II", "group": 38, "category": 7, "slot": "mid"},
	"2048":  {"name": "Damage Control II", "group": 60, "category": 7, "slot": "low"},
	"31790": {"name": "Medium Core Defense Field Extender I", "group": 774, "category": 7, "slot": "rig"},
	"35894": {"name": "Standup Cloning Center I", "group": 1415, "category": 66, "slot": "service"},
	"209":   {"name": "Scourge Heavy Missile", "group": 385, "category": 8},
	"21894": {"name": "Republic Fleet EMP S", "group": 83, "category": 8},
	"2488":  {"name": "Hornet EC-300", "group": 101, "category": 18},
	"40556": {"name": "Templar II", "group": 1652, "category": 87},
	"28668": {"name": "Nanite Repair Paste", "group": 916, "category": 17},
	"15466": {"name": "Synth Crash Booster", "group": 303, "category": 20},
	"10228": {"name": "Zainou 'Gnome' Shield Upgrades SU-601", "group": 300, "category": 20},
	"34":    {"name": "Tritanium", "group": 18, "category": 4},
	"47911": {"name": "Lux Kontos", "group": 4807, "category": 7}
}}`

// loadFitFixtures loads the inventory flags and item types above for FitSlotOf and FitTypeOf
func loadFitFixtures(t *testing.T) {
	t.Helper()
	files := fstest.MapFS{
		"inventory_flags.json": {Data: []byte(fitFlagsFixture)},
		"fit_types.json":       {Data: []byte(fitTypesFixture)},
	}
	assert.NoError(t, comm.LoadInventoryFlags(files, "inventory_flags.json"))
	assert.NoError(t, comm.LoadFitTypes(files, "fit_types.json"))
}

// fitItems builds a victim's items as the ESI lists them: flag, type, destroyed, dropped
func fitItems(entries ...[4]int) []api.ESIKillmailItem {
	items := []api.ESIKillmailItem{}
	for _, entry := range entries {
		item := api.ESIKillmailItem{Flag: entry[0], ItemTypeID: entry[1]}
		if destroyed := entry[2]; destroyed > 0 {
			item.QuantityDestroyed = &destroyed
		}
		if dropped := entry[3]; dropped > 0 {
			item.QuantityDropped = &dropped
		}
		items = append(items, item)
	}
	return items
}

// slotNames lists a slot group's modules as "Module" or "Module, Charge", with "" for an empty slot
func slotNames(slots []*retrieval.FittedModule) []string {
	names := []string{}
	for _, fitted := range slots {
		switch {
		case fitted == nil:
			names = append(names, "")
		case fitted.Charge != nil:
			names = append(names, fitted.Module.Name+", "+fitted.Charge.Name)
		default:
			names = append(names, fitted.Module.Name)
		}
	}
	return names
}

// stackNames lists stacks as name to quantity
func stackNames(stacks []*retrieval.FitCargo) map[string]int {
	names := map[string]int{}
	for _, stack := range stacks {
		names[stack.Item.Name] = stack.Quantity
	}
	return names
}

func itemNames(items []*retrieval.Item) []string {
	names := []string{}
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// rifterItems is a Rifter with three loaded autocannons, a scram, a damage control, two hornets, and some cargo
func rifterItems() []api.ESIKillmailItem {
	return fitItems(
		[4]int{27, 2873, 1, 0}, [4]int{27, 21894, 100, 0},
		[4]int{28, 2873, 0, 1}, [4]int{28, 21894, 0, 100},
		[4]int{29, 2873, 1, 0}, [4]int{29, 21894, 100, 0},
		[4]int{19, 448, 1, 0},
		[4]int{11, 2048, 0, 1},
		[4]int{87, 2488, 1, 0}, [4]int{87, 2488, 0, 1},
		[4]int{5, 21894, 150, 50}, [4]int{5, 28668, 50, 0}, [4]int{5, 34, 0, 1000}, [4]int{5, 15466, 2, 0},
		[4]int{88, 15466, 1, 0},
		[4]int{89, 10228, 1, 0},
	)
}

// tenguItems is a Tengu with four subsystems, five loaded launchers, a shield tank, and a rig
func tenguItems() []api.ESIKillmailItem {
	items := fitItems(
		[4]int{125, 45589, 1, 0}, [4]int{126, 45597, 0, 1}, [4]int{127, 45601, 1, 0}, [4]int{128, 45605, 1, 0},
		[4]int{19, 3841, 1, 0}, [4]int{20, 3841, 0, 1}, [4]int{21, 448, 1, 0},
		[4]int{11, 2048, 1, 0},
		[4]int{92, 31790, 1, 0},
		[4]int{5, 209, 1000, 0},
	)
	for flag := 27; flag < 32; flag++ {
		items = append(items, fitItems([4]int{flag, 2410, 1, 0}, [4]int{flag, 209, 0, 30})...)
	}
	return items
}

func TestBuildFit(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	loadFitFixtures(t)

	tests := []struct {
		name   string
		hull   int
		items  []api.ESIKillmailItem
		checks func(t *testing.T, fit *retrieval.Fit)
	}{
		{
			name:  "a T3 cruiser's slots are its hull's plus its subsystems' modifiers",
			hull:  29984,
			items: tenguItems(),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, "Tengu", fit.Hull.Name)
				assert.Equal(t, []string{
					"Tengu Core - Augmented Graviton Reactor", "Tengu Defensive - Supplemental Screening",
					"Tengu Offensive - Accelerated Ejection Bay", "Tengu Propulsion - Fuel Catalyst",
				}, slotNames(fit.Subsystems))
				assert.Len(t, fit.Highs, 5, "the offensive subsystem's five")
				assert.Len(t, fit.Mids, 5, "two, two, and one from the subsystems")
				assert.Len(t, fit.Lows, 3)
				for _, name := range slotNames(fit.Highs) {
					assert.Equal(t, "Heavy Missile Launcher II, Scourge Heavy Missile", name)
				}
				assert.Equal(t, []string{"Large Shield Extender II", "Large Shield Extender II", "Warp Scrambler II", "", ""}, slotNames(fit.Mids))
				assert.Equal(t, []string{"Damage Control II", "", ""}, slotNames(fit.Lows))
				assert.Equal(t, []string{"Medium Core Defense Field Extender I", "", ""}, slotNames(fit.Rigs))
				assert.True(t, fit.Subsystems[1].Dropped)
				assert.False(t, fit.Subsystems[0].Dropped)
				assert.Equal(t, map[string]int{"Scourge Heavy Missile": 1000}, stackNames(fit.Cargo))
				assert.Empty(t, fit.MissingTypes)
			},
		},
		{
			name:  "empty slots are nil and a charge is loaded in its module",
			hull:  587,
			items: rifterItems(),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"125mm Gatling AutoCannon II, Republic Fleet EMP S", "125mm Gatling AutoCannon II, Republic Fleet EMP S", "125mm Gatling AutoCannon II, Republic Fleet EMP S", ""}, slotNames(fit.Highs))
				assert.Equal(t, []string{"Warp Scrambler II", "", ""}, slotNames(fit.Mids))
				assert.Equal(t, []string{"Damage Control II", "", "", ""}, slotNames(fit.Lows))
				assert.Equal(t, []string{"", "", ""}, slotNames(fit.Rigs))
				assert.Empty(t, fit.Subsystems, "not a T3")
				assert.Empty(t, fit.Services)
				assert.True(t, fit.Highs[1].Dropped)
				assert.True(t, fit.Lows[0].Dropped)
				assert.False(t, fit.Highs[0].Dropped)
			},
		},
		{
			name:  "the destroyed and dropped entries of the same module are merged",
			hull:  587,
			items: fitItems([4]int{11, 2048, 1, 0}, [4]int{11, 2048, 0, 1}, [4]int{5, 21894, 150, 0}, [4]int{5, 21894, 0, 50}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"Damage Control II", "", "", ""}, slotNames(fit.Lows))
				assert.True(t, fit.Lows[0].Dropped)
				assert.Equal(t, map[string]int{"Republic Fleet EMP S": 200}, stackNames(fit.Cargo))
			},
		},
		{
			name:  "drones in the bay are merged by type, fighters in the bay and tubes together",
			hull:  587,
			items: fitItems([4]int{87, 2488, 2, 0}, [4]int{87, 2488, 0, 3}, [4]int{158, 40556, 6, 0}, [4]int{159, 40556, 3, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, map[string]int{"Hornet EC-300": 5}, stackNames(fit.Drones))
				assert.Equal(t, map[string]int{"Templar II": 9}, stackNames(fit.Fighters))
			},
		},
		{
			name:  "cargo keeps charges, paste, and boosters, implants and active boosters are listed on their own",
			hull:  587,
			items: rifterItems(),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, map[string]int{"Republic Fleet EMP S": 200, "Nanite Repair Paste": 50, "Synth Crash Booster": 2}, stackNames(fit.Cargo), "no tritanium")
				assert.Equal(t, []string{"Synth Crash Booster"}, itemNames(fit.Boosters))
				assert.Equal(t, []string{"Zainou 'Gnome' Shield Upgrades SU-601"}, itemNames(fit.Implants))
				assert.Equal(t, map[string]int{"Hornet EC-300": 2}, stackNames(fit.Drones))
			},
		},
		{
			name:  "an unknown flag and an ignored hold are left out",
			hull:  587,
			items: fitItems([4]int{999, 2048, 1, 0}, [4]int{155, 3841, 1, 0}, [4]int{12, 2048, 1, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"", "Damage Control II", "", ""}, slotNames(fit.Lows))
				assert.Equal(t, []string{"", "", ""}, slotNames(fit.Mids))
				assert.Empty(t, fit.Cargo)
			},
		},
		{
			name:  "a module beyond the hull's slots still shows",
			hull:  587,
			items: fitItems([4]int{17, 2048, 1, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"", "", "", "", "", "", "Damage Control II"}, slotNames(fit.Lows))
			},
		},
		{
			name:  "a module fit_types.json gives no slot is the module when alone on its flag, and still takes a charge",
			hull:  587,
			items: fitItems([4]int{27, 47911, 1, 0}, [4]int{28, 47911, 1, 0}, [4]int{28, 21894, 100, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"Lux Kontos", "Lux Kontos, Republic Fleet EMP S", "", ""}, slotNames(fit.Highs))
			},
		},
		{
			name: "items nested in a container or packed ship are never placed",
			hull: 587,
			// the ESI nests them under their parent's items, which api.ESIKillmailItem does not decode
			items: func() []api.ESIKillmailItem {
				var items []api.ESIKillmailItem
				assert.NoError(t, json.Unmarshal([]byte(`[{"flag": 90, "item_type_id": 587, "quantity_dropped": 1, "singleton": 0,
					"items": [{"flag": 11, "item_type_id": 2048, "quantity_dropped": 1, "singleton": 0}]}]`), &items))
				return items
			}(),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"", "", "", ""}, slotNames(fit.Lows))
				assert.Empty(t, fit.Cargo)
			},
		},
		{
			name: "types fit_types.json does not have are named through the cache and placed by their flag",
			hull: 587,
			// a new module alone, a new charge in a known module, two new types in one slot, and new cargo
			items: fitItems([4]int{11, 99001, 1, 0}, [4]int{27, 2873, 1, 0}, [4]int{27, 99002, 100, 0}, [4]int{28, 99003, 1, 0}, [4]int{28, 99004, 1, 0}, [4]int{5, 99005, 10, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, []string{"Loki", "", "", ""}, slotNames(fit.Lows), "the test ESI names every type Loki")
				assert.Equal(t, []string{"125mm Gatling AutoCannon II, Loki", "Loki", "", ""}, slotNames(fit.Highs))
				assert.Equal(t, 99003, fit.Highs[1].Module.ItemID, "the first of two it cannot tell apart is the module")
				assert.Empty(t, fit.Cargo, "unknown cargo can not be checked against the cargo rules")
				assert.Equal(t, []int{99001, 99002, 99003, 99004, 99005}, fit.MissingTypes)
			},
		},
		{
			name:  "a hull fit_types.json does not have has its modules by flag and no empty slots",
			hull:  99010,
			items: fitItems([4]int{28, 2873, 1, 0}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, "Loki", fit.Hull.Name)
				assert.Equal(t, []string{"", "125mm Gatling AutoCannon II"}, slotNames(fit.Highs))
				assert.Empty(t, fit.Lows)
				assert.Equal(t, []int{99010}, fit.MissingTypes)
			},
		},
		{
			name:  "a structure's services are in their slots",
			hull:  35832,
			items: fitItems([4]int{164, 35894, 1, 0}, [4]int{27, 2873, 0, 1}),
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, "Astrahus", fit.Hull.Name)
				assert.Equal(t, []string{"Standup Cloning Center I", "", ""}, slotNames(fit.Services))
				assert.Equal(t, []string{"125mm Gatling AutoCannon II", "", ""}, slotNames(fit.Highs))
				assert.Len(t, fit.Rigs, 3)
			},
		},
		{
			name:  "a victim with no items has its hull and empty slots",
			hull:  587,
			items: nil,
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, "Rifter", fit.Hull.Name)
				assert.Equal(t, []string{"", "", "", ""}, slotNames(fit.Highs))
				assert.Empty(t, fit.Drones)
				assert.Empty(t, fit.Cargo)
			},
		},
		{
			name:  "a victim with no slots has an empty fit",
			hull:  23,
			items: nil,
			checks: func(t *testing.T, fit *retrieval.Fit) {
				assert.Equal(t, "Cargo Container", fit.Hull.Name)
				assert.Empty(t, fit.Highs)
				assert.Empty(t, fit.Rigs)
				assert.Empty(t, fit.Services)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))
			fit := cache.BuildFit(test.hull, test.items)
			if assert.NotNil(t, fit) {
				test.checks(t, fit)
			}
		})
	}
}

func TestFitOnExpandedKillmail(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	loadFitFixtures(t)

	t.Run("the victim's fit is built when the killmail is expanded", func(t *testing.T) {
		km := killmailWith(1, 101, 102)
		km.ESIKillmail.Victim.ShipTypeID = 587
		km.ESIKillmail.Victim.Items = rifterItems()
		cache := retrieval.NewCache(&RawESIClient{}, filepath.Join(t.TempDir(), "cache.db"))

		expanded := km.Expand(nil, cache)

		if assert.NotNil(t, expanded.Victim.Fit) {
			assert.Equal(t, "Rifter", expanded.Victim.Fit.Hull.Name)
			assert.Equal(t, []string{"Warp Scrambler II", "", ""}, slotNames(expanded.Victim.Fit.Mids))
		}
	})

	t.Run("victims' item types fit_types.json does not have are prefetched, the rest are not looked up", func(t *testing.T) {
		km := killmailWith(1, 101)
		km.ESIKillmail.Victim.ShipTypeID = 587
		km.ESIKillmail.Victim.Items = fitItems([4]int{11, 2048, 1, 0}, [4]int{12, 99001, 1, 0}, [4]int{5, 99002, 1, 0})
		raw := &RawESIClient{}
		cache := retrieval.NewCache(raw, filepath.Join(t.TempDir(), "cache.db"))

		cache.PrefetchKillmailEntities([]retrieval.RawKillmail{km})
		assert.Equal(t, 3, raw.Calls("item"), "the two new types, and the ship, which is always looked up")
		calls := raw.total()

		cache.BuildFit(587, km.ESIKillmail.Victim.Items)
		assert.Equal(t, calls, raw.total(), "the fit is built from the cache")
	})
}
