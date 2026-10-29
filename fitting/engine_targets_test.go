package fitting_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/stretchr/testify/assert"
)

func TestEngineHullBonusScaling(t *testing.T) {
	const (
		scaled = 300 // a bonus attribute that belongs to a skill
		role   = 301 // a role bonus
	)
	u := newUniverse(true, 0)
	u.file.Attributes[scaled] = &comm.DogmaAttribute{Name: "shipBonus", Stackable: true}
	u.file.Attributes[role] = &comm.DogmaAttribute{Name: "roleBonus", Stackable: true}
	u.file.BonusSkills[scaled] = skillType
	u.effect(1, "bonuses", 0,
		modifier("itemID", "ItemModifier", scaled, 2),
		modifier("itemID", "ItemModifier", role, 2),
	)
	u.typ(hullType, 1, map[int]float64{scaled: 3, role: 4}, 1)
	u.typ(skillType, 255, map[int]float64{}, 1)
	u.file.Skills = []int{skillType}
	u.load(t)

	t.Run("hull bonus times skill level, role bonus unscaled", func(t *testing.T) {
		fit := newFit(t, fitting.Input{Hull: hullType})
		approx(t, 10+3*5+4, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("a skill that isn't loaded counts as V", func(t *testing.T) {
		u.file.BonusSkills[scaled] = 999 // no such skill
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType})
		approx(t, 10+3*5+4, fit.Attr(fit.Ship, aTarget))
	})
}

func TestEngineSkillSelfModifier(t *testing.T) {
	const bonus = 300
	u := newUniverse(true, 0)
	u.file.Attributes[bonus] = &comm.DogmaAttribute{Name: "bonusPerLevel", Stackable: true}
	// the skill multiplies its own bonus by its level, then adds the result to the ship
	u.effect(1, "skill", 0,
		comm.DogmaModifier{Domain: "itemID", Func: "ItemModifier", Modified: bonus, Modifying: aLevel, Operation: 4},
		modifier("shipID", "ItemModifier", bonus, 2),
	)
	u.typ(skillType, 255, map[int]float64{bonus: 2}, 1)
	u.typ(hullType, 1, map[int]float64{})
	u.file.Skills = []int{skillType}
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType})

	approx(t, 5, fit.Attr(fit.Skills[0], aLevel))
	approx(t, 10, fit.Attr(fit.Skills[0], bonus))
	approx(t, 20, fit.Attr(fit.Ship, aTarget))
}

func TestEngineTargets(t *testing.T) {
	const (
		gunGroup    = 50
		otherGroup  = 51
		skillGun    = 900
		skillMissle = 901
		gun         = 200
		missile     = 201
		scanner     = 202
		ammo        = 203
		src         = 210 // module that modifies with each domain/func under test
	)
	mods := func(domain, fn string, group, skill int) *universe {
		u := newUniverse(true, 0)
		m := modifier(domain, fn, aSource, 2)
		m.Group, m.Skill = group, skill
		u.effect(1, "x", 0, m)
		u.typ(hullType, 1, map[int]float64{aSource: 100}, 1)
		u.typ(gun, gunGroup, map[int]float64{aReq1: skillGun}, 1)
		u.typ(missile, otherGroup, map[int]float64{aReq1: skillMissle})
		u.typ(scanner, otherGroup, map[int]float64{})
		u.typ(ammo, 60, map[int]float64{aSource: 7, aReq1: skillGun})
		return u
	}
	in := fitting.Input{Hull: hullType, Modules: []fitting.Module{
		{Type: gun, Slot: comm.SlotHigh, Charge: ammo},
		{Type: missile, Slot: comm.SlotHigh},
		{Type: scanner, Slot: comm.SlotMid},
	}}
	base := 10.0

	t.Run("ItemModifier shipID", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "x", 0, modifier("shipID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(gun, gunGroup, map[int]float64{aSource: 5}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: gun, Slot: comm.SlotHigh}}})
		approx(t, base+5, fit.Attr(fit.Ship, aTarget))
		approx(t, base, fit.Attr(fit.Modules[0], aTarget), "only the ship")
	})
	t.Run("structureID reaches the hull too", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "x", 0, modifier("structureID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(gun, gunGroup, map[int]float64{aSource: 5}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: gun, Slot: comm.SlotHigh}}})
		approx(t, base+5, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("ItemModifier charID reaches the character", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "x", 0, modifier("charID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{aSource: 5}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType})
		approx(t, base+5, fit.Attr(fit.Character, aTarget))
		approx(t, base, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("otherID: module to its charge and charge to its module", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "toCharge", 0, modifier("otherID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(gun, gunGroup, map[int]float64{aSource: 5}, 1)
		u.typ(ammo, 60, map[int]float64{aSource: 7}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: gun, Slot: comm.SlotHigh, Charge: ammo}}})
		module := fit.Modules[0]
		assert.Same(t, module, module.Charge.Module)
		approx(t, base+5, fit.Attr(module.Charge, aTarget), "the module's attribute lands on the charge")
		approx(t, base+7, fit.Attr(module, aTarget), "the charge's attribute lands on the module")
	})
	t.Run("otherID without a charge does nothing", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "toCharge", 0, modifier("otherID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(gun, gunGroup, map[int]float64{aSource: 5}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: gun, Slot: comm.SlotHigh}}})
		approx(t, base, fit.Attr(fit.Modules[0], aTarget))
	})
	t.Run("LocationModifier shipID: every fitted module, not the hull or the charge", func(t *testing.T) {
		u := mods("shipID", "LocationModifier", 0, 0)
		u.load(t)
		fit := newFit(t, in)
		approx(t, base+100, fit.Attr(fit.Modules[0], aTarget))
		approx(t, base+100, fit.Attr(fit.Modules[2], aTarget))
		approx(t, base, fit.Attr(fit.Modules[0].Charge, aTarget))
		approx(t, base, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("LocationGroupModifier", func(t *testing.T) {
		u := mods("shipID", "LocationGroupModifier", gunGroup, 0)
		u.load(t)
		fit := newFit(t, in)
		approx(t, base+100, fit.Attr(fit.Modules[0], aTarget))
		approx(t, base, fit.Attr(fit.Modules[1], aTarget))
		approx(t, base, fit.Attr(fit.Modules[2], aTarget))
	})
	t.Run("LocationRequiredSkillModifier", func(t *testing.T) {
		u := mods("shipID", "LocationRequiredSkillModifier", 0, skillGun)
		u.load(t)
		fit := newFit(t, in)
		approx(t, base+100, fit.Attr(fit.Modules[0], aTarget))
		approx(t, base, fit.Attr(fit.Modules[1], aTarget))
		approx(t, base, fit.Attr(fit.Modules[0].Charge, aTarget), "charges are not in the ship's location")
	})
	t.Run("OwnerRequiredSkillModifier reaches modules and charges but not the hull", func(t *testing.T) {
		u := mods("charID", "OwnerRequiredSkillModifier", 0, skillGun)
		u.typ(hullType, 1, map[int]float64{aSource: 100, aReq1: skillGun}, 1)
		u.load(t)
		fit := newFit(t, in)
		approx(t, base+100, fit.Attr(fit.Modules[0], aTarget))
		approx(t, base+100, fit.Attr(fit.Modules[0].Charge, aTarget))
		approx(t, base, fit.Attr(fit.Modules[1], aTarget))
		approx(t, base, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("LocationRequiredSkillModifier charID reaches charges", func(t *testing.T) {
		u := mods("charID", "LocationRequiredSkillModifier", 0, skillGun)
		u.load(t)
		fit := newFit(t, in)
		approx(t, base+100, fit.Attr(fit.Modules[0].Charge, aTarget))
	})
	t.Run("itemID location is the source's charge", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "x", 0, modifier("itemID", "LocationModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(gun, gunGroup, map[int]float64{aSource: 5}, 1)
		u.typ(ammo, 60, map[int]float64{})
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: gun, Slot: comm.SlotHigh, Charge: ammo}}})
		approx(t, base+5, fit.Attr(fit.Modules[0].Charge, aTarget))
		approx(t, base, fit.Attr(fit.Modules[0], aTarget))
	})
	t.Run("EffectStopper and target domains are ignored", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "x", 0, modifier("targetID", "ItemModifier", aSource, 2), modifier("shipID", "EffectStopper", aSource, 2))
		u.typ(hullType, 1, map[int]float64{aSource: 5}, 1)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType})
		approx(t, base, fit.Attr(fit.Ship, aTarget))
	})
}

func TestEngineApplies(t *testing.T) {
	const module = 200
	u := newUniverse(true, 0)
	cats := map[string]int{"passive": 0, "active": 1, "target": 2, "area": 3, "onlineCat": 4, "overload": 5, "dungeon": 6, "system": 7}
	id := 1
	var effectIDs []int
	for name, c := range cats {
		u.effect(id, name, c)
		effectIDs = append(effectIDs, id)
		id++
	}
	u.effect(id, "online", 1) // the online effect is category 1 in the data
	onlineID := id
	u.typ(hullType, 1, map[int]float64{})
	u.typ(module, 3, map[int]float64{}, append(effectIDs, onlineID)...)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: module, Slot: comm.SlotLow}}})
	m := fit.Modules[0]
	assert.Equal(t, fitting.StateActive, m.State)

	applies := func(state fitting.State) map[string]bool {
		m.State = state
		got := map[string]bool{}
		for _, effectID := range append(effectIDs, onlineID) {
			e := comm.DogmaEffectOf(effectID)
			got[e.Name] = fit.Applies(m, e)
		}
		return got
	}
	assert.Equal(t, map[string]bool{"passive": true, "active": false, "target": false, "area": false, "onlineCat": false, "overload": false, "dungeon": false, "system": false, "online": false}, applies(fitting.StateOffline))
	assert.Equal(t, map[string]bool{"passive": true, "active": false, "target": false, "area": false, "onlineCat": true, "overload": false, "dungeon": false, "system": false, "online": true}, applies(fitting.StateOnline))
	assert.Equal(t, map[string]bool{"passive": true, "active": true, "target": true, "area": false, "onlineCat": true, "overload": false, "dungeon": false, "system": false, "online": true}, applies(fitting.StateActive))
	assert.Equal(t, map[string]bool{"passive": true, "active": true, "target": true, "area": false, "onlineCat": true, "overload": true, "dungeon": false, "system": false, "online": true}, applies(fitting.StateOverheated))

	t.Run("everything but modules and charges always applies", func(t *testing.T) {
		for _, e := range []int{1, 2, 3} {
			assert.True(t, fit.Applies(fit.Ship, comm.DogmaEffectOf(e)))
		}
	})
	t.Run("a charge follows its module being online", func(t *testing.T) {
		charge := &fitting.Item{Role: fitting.RoleCharge, Module: m}
		active := comm.DogmaEffectOf(2)
		m.State = fitting.StateOffline
		assert.False(t, fit.Applies(charge, active))
		m.State = fitting.StateOnline
		assert.True(t, fit.Applies(charge, active))
	})
	t.Run("state decides which modifiers reach the ship", func(t *testing.T) {
		u := newUniverse(true, 0)
		u.effect(1, "passive", 0, modifier("shipID", "ItemModifier", aSource, 2))
		u.effect(2, "active", 1, modifier("shipID", "ItemModifier", aSource, 2))
		u.effect(3, "overload", 5, modifier("shipID", "ItemModifier", aSource, 2))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(module, 3, map[int]float64{aSource: 1}, 1, 2, 3)
		u.load(t)
		for state, want := range map[fitting.State]float64{fitting.StateOffline: 11, fitting.StateOnline: 11, fitting.StateActive: 12, fitting.StateOverheated: 13} {
			fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: module, Slot: comm.SlotLow}}})
			fit.Modules[0].State = state
			approx(t, want, fit.Attr(fit.Ship, aTarget))
		}
	})
}

func TestEngineAttrExcluding(t *testing.T) {
	u := newUniverse(true, 0)
	u.effect(1, "ship", 0, modifier("shipID", "ItemModifier", aSource, 2))
	// aSource2 depends on the same module through a second attribute, to show dependencies are worked out again
	u.effect(2, "chain", 0, comm.DogmaModifier{Domain: "shipID", Func: "ItemModifier", Modified: aSource, Modifying: aSource2, Operation: 2})
	u.effect(3, "hull", 0, modifier("itemID", "ItemModifier", aSource, 2))
	u.typ(hullType, 1, map[int]float64{aSource: 1}, 3)
	u.typ(200, 3, map[int]float64{aSource: 5, aSource2: 100}, 1)
	u.typ(201, 3, map[int]float64{aSource: 2}, 1)
	u.typ(202, 3, map[int]float64{aSource2: 10}, 2)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{
		{Type: 200, Slot: comm.SlotLow}, {Type: 201, Slot: comm.SlotLow}, {Type: 202, Slot: comm.SlotLow},
	}})
	// the ship's aSource = 1 + 10 (from 202); aTarget = 10 + (that) + 5 + 2
	approx(t, 10+11+5+2, fit.Attr(fit.Ship, aTarget))
	approx(t, 10+11+2, fit.AttrExcluding(fit.Ship, aTarget, fit.Modules[0]))
	approx(t, 10+1+5+2, fit.AttrExcluding(fit.Ship, aTarget, fit.Modules[2]), "the dependency is worked out without it too")
	approx(t, 10+11+5+2, fit.Attr(fit.Ship, aTarget), "the cached value is untouched")
}

func TestEngineUnknownsAndModes(t *testing.T) {
	const mode = 400
	u := newUniverse(true, 0)
	u.effect(1, "mode", 0, modifier("shipID", "ItemModifier", aSource, 2))
	u.typ(hullType, 1, map[int]float64{})
	u.typ(200, 3, map[int]float64{})
	u.typ(201, 4, map[int]float64{}) // a charge
	u.typ(300, 5, map[int]float64{aSource: 3}, 1)
	u.typ(mode, 6, map[int]float64{aSource: 4}, 1)
	u.typ(301, 7, map[int]float64{aSource: 1}, 1) // a subsystem
	u.load(t)

	t.Run("unknown module, charge, implant, and mode are left out", func(t *testing.T) {
		fit := newFit(t, fitting.Input{Hull: hullType, Implants: []int{300, 9001}, Mode: 9002, Modules: []fitting.Module{
			{Type: 200, Slot: comm.SlotLow, Charge: 9003},
			{Type: 9004, Slot: comm.SlotLow, Charge: 201},
		}})
		assert.ElementsMatch(t, []int{9001, 9002, 9003, 9004}, fit.Unknown)
		assert.Len(t, fit.Modules, 1)
		assert.Nil(t, fit.Modules[0].Charge)
		assert.Len(t, fit.Implants, 1)
		assert.Nil(t, fit.Mode)
		approx(t, 13, fit.Attr(fit.Ship, aTarget), "the known implant applies")
	})
	t.Run("mode, implant and subsystem roles", func(t *testing.T) {
		fit := newFit(t, fitting.Input{Hull: hullType, Mode: mode, Modules: []fitting.Module{{Type: 301, Slot: comm.SlotSubsystem}, {Type: 200, Slot: comm.SlotLow}}})
		assert.Equal(t, fitting.RoleMode, fit.Mode.Role)
		assert.Equal(t, fitting.RoleSubsystem, fit.Modules[0].Role)
		assert.Equal(t, fitting.RoleModule, fit.Modules[1].Role)
		assert.Equal(t, fitting.RoleCharacter, fit.Character.Role)
		assert.Equal(t, fitting.RoleHull, fit.Ship.Role)
		assert.Equal(t, fitting.StateActive, fit.Modules[1].State)
		approx(t, 10+4+1, fit.Attr(fit.Ship, aTarget))
	})
	t.Run("unknown hull", func(t *testing.T) {
		_, err := fitting.NewFit(fitting.Input{Hull: 424242})
		assert.ErrorIs(t, err, fitting.ErrUnknownHull)
	})
}

func TestEngineCycleGuard(t *testing.T) {
	// aSource of the hull depends on aTarget, which depends on aSource: a loop
	u := newUniverse(true, 0)
	u.effect(1, "loop", 0,
		modifier("itemID", "ItemModifier", aSource, 2),
		comm.DogmaModifier{Domain: "itemID", Func: "ItemModifier", Modified: aSource, Modifying: aTarget, Operation: 2},
	)
	u.typ(hullType, 1, map[int]float64{aSource: 1}, 1)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType})
	done := make(chan float64)
	go func() { done <- fit.Attr(fit.Ship, aTarget) }()
	got := <-done
	assert.False(t, got != got, "a number comes back")

	// a loop's result is not kept as if it were right
	first := fit.Attr(fit.Ship, aSource)
	assert.Equal(t, first, fit.Attr(fit.Ship, aSource))
}

func TestEngineTargetEffectAppliesButChangesNothing(t *testing.T) {
	u := newUniverse(true, 0)
	u.effect(1, "remote", 2, modifier("shipID", "ItemModifier", aSource, 2))
	u.typ(hullType, 1, map[int]float64{})
	u.typ(200, 3, map[int]float64{aSource: 5}, 1)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: 200, Slot: comm.SlotHigh}}})
	assert.True(t, fit.Applies(fit.Modules[0], comm.DogmaEffectOf(1)))
	approx(t, 10, fit.Attr(fit.Ship, aTarget))
}

// a hand modifier whose Value reads the attribute it modifies without its own modifiers, as the Reactive Armor Hardener does
func TestEngineReentrantAttrExcluding(t *testing.T) {
	u := newUniverse(true, 0)
	u.effect(1, "ship", 0, modifier("shipID", "ItemModifier", aSource, 2))
	u.typ(hullType, 1, map[int]float64{})
	u.typ(200, 3, map[int]float64{aSource: 5}, 1)
	u.typ(201, 3, map[int]float64{})
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: 200, Slot: comm.SlotLow}, {Type: 201, Slot: comm.SlotLow}}})
	rah := fit.Modules[1]
	calls := 0
	fit.AddHand(fitting.HandModifier{Source: rah, Target: fit.Ship, Attribute: aTarget, Operation: 4, Value: func(f *fitting.Fit) float64 {
		calls++
		return 1 + f.AttrExcluding(f.Ship, aTarget, rah)/100 // 1.15
	}})
	approx(t, 15*1.15, fit.Attr(fit.Ship, aTarget))
	approx(t, 15*1.15, fit.Attr(fit.Ship, aTarget), "kept")
	assert.Equal(t, 1, calls, "the outer value was cached, and the inner one did not trip the guard or write to it")
	approx(t, 15, fit.AttrExcluding(fit.Ship, aTarget, rah))
}
