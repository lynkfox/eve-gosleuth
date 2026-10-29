package fitting_test

import (
	"encoding/json"
	"math"
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// attribute ids of the made-up dogma data in these tests
const (
	aTarget   = 1 // the attribute the tests change: default 10, stackable only if a test says so
	aSource   = 2 // what a source modifies with
	aSource2  = 3 // a second, unscaled one
	aCap      = 4 // the cap for aTarget
	aLevel    = 5 // skillLevel
	aReq1     = 182
	aOther    = 6
	hullType  = 100
	charType  = 101
	skillType = 102
)

// modifier builds a DogmaModifier changing aTarget
func modifier(domain, fn string, modifying, operation int) comm.DogmaModifier {
	return comm.DogmaModifier{Domain: domain, Func: fn, Modified: aTarget, Modifying: modifying, Operation: operation}
}

// universe is a made-up dogma.json under construction
type universe struct {
	file comm.DogmaFile
}

func newUniverse(stackable bool, max int) *universe {
	return &universe{file: comm.DogmaFile{
		SDEBuild:      1,
		CharacterType: charType,
		Attributes: map[int]*comm.DogmaAttribute{
			aTarget:  {Name: "target", Default: 10, Stackable: stackable, Max: max},
			aSource:  {Name: "source", Default: 0, Stackable: true},
			aSource2: {Name: "source2", Default: 0, Stackable: true},
			aCap:     {Name: "cap", Default: 15, Stackable: true},
			aLevel:   {Name: "skillLevel", Default: 0, Stackable: true},
			aReq1:    {Name: "requiredSkill1", Default: 0, Stackable: true},
			aOther:   {Name: "other", Default: 3, Stackable: true},
		},
		Effects: map[int]*comm.DogmaEffect{},
		Types: map[int]*comm.DogmaType{
			hullType: {Group: 1, Attributes: map[int]float64{}},
			charType: {Group: 2, Attributes: map[int]float64{}},
		},
		BonusSkills: map[int]int{},
		Modes:       map[int][]comm.DogmaMode{},
	}}
}

func (u *universe) effect(id int, name string, category int, mods ...comm.DogmaModifier) *universe {
	u.file.Effects[id] = &comm.DogmaEffect{Name: name, Category: category, Modifiers: mods}
	return u
}

func (u *universe) typ(id, group int, attrs map[int]float64, effects ...int) *universe {
	u.file.Types[id] = &comm.DogmaType{Group: group, Category: 7, Attributes: attrs, Effects: effects}
	return u
}

// load makes the universe the loaded dogma.json
func (u *universe) load(t *testing.T) {
	t.Helper()
	comm.InitLogger(logrus.ErrorLevel)
	raw, err := json.Marshal(u.file)
	assert.NoError(t, err)
	assert.NoError(t, comm.LoadDogma(fstest.MapFS{"dogma.json": {Data: raw}}, "dogma.json"))
}

func newFit(t *testing.T, in fitting.Input) *fitting.Fit {
	t.Helper()
	fit, err := fitting.NewFit(in)
	assert.NoError(t, err)
	return fit
}

func approx(t *testing.T, want, got float64, msg ...interface{}) {
	t.Helper()
	assert.InDelta(t, want, got, 1e-9, msg...)
}

func TestEngineOperationsAndStageOrder(t *testing.T) {
	type op struct {
		operation int
		value     float64
	}
	cases := map[string]struct {
		ops  []op
		want float64
	}{
		"preMul":      {[]op{{0, 2}}, 20},
		"preDiv":      {[]op{{1, 4}}, 2.5},
		"modAdd":      {[]op{{2, 5}}, 15},
		"modSub":      {[]op{{3, 4}}, 6},
		"postMul":     {[]op{{4, 3}}, 30},
		"postDiv":     {[]op{{5, 2}}, 5},
		"postPercent": {[]op{{6, 25}}, 12.5},
		"preAssign":   {[]op{{-1, 7}}, 7},
		"postAssign":  {[]op{{7, 7}}, 7},
		// 10 * 2 (preMul) + 5 (add) - 1 (sub) = 24, * 3 / 2 * 1.5 (post) = 54
		"stage order": {[]op{{6, 50}, {5, 2}, {4, 3}, {3, 1}, {2, 5}, {0, 2}}, 54},
		// pre-assign comes before the multiplication, post-assign wins over all
		"assign order":     {[]op{{0, 2}, {-1, 4}}, 8},
		"post assign last": {[]op{{7, 9}, {0, 2}, {2, 100}}, 9},
		"last assign wins": {[]op{{7, 1}, {7, 2}, {7, 3}}, 3},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			u := newUniverse(true, 0)
			mods := []comm.DogmaModifier{}
			attrs := map[int]float64{}
			for i, o := range c.ops {
				attr := 100 + i
				u.file.Attributes[attr] = &comm.DogmaAttribute{Name: "v" + string(rune('a'+i)), Stackable: true}
				attrs[attr] = o.value
				mods = append(mods, modifier("itemID", "ItemModifier", attr, o.operation))
			}
			u.effect(1, "self", 0, mods...)
			u.typ(hullType, 1, attrs, 1)
			u.load(t)
			fit := newFit(t, fitting.Input{Hull: hullType})
			approx(t, c.want, fit.Attr(fit.Ship, aTarget))
		})
	}
}

func TestEngineDefaultsCapsAndNames(t *testing.T) {
	u := newUniverse(true, aCap)
	u.effect(1, "self", 0, modifier("itemID", "ItemModifier", aSource, 4))
	u.typ(hullType, 1, map[int]float64{aSource: 3}, 1)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType})

	approx(t, 15, fit.Attr(fit.Ship, aTarget), "30 is capped by aCap's default 15")
	approx(t, 15, fit.AttrNamed(fit.Ship, "target"))
	approx(t, 3, fit.Attr(fit.Ship, aOther), "an unlisted attribute reads its default")
	approx(t, 0, fit.AttrNamed(fit.Ship, "noSuchAttribute"))
	approx(t, 0, fit.Attr(fit.Ship, 9999), "an attribute dogma.json lacks reads 0")
}

// stackingFit has module types 200+i giving aSource = values[i] through a shipID modifier with the operation, for aTarget on the hull
func stackingFit(t *testing.T, stackable bool, operation int, role fitting.Role, values ...float64) (*fitting.Fit, float64) {
	u := newUniverse(stackable, 0)
	u.effect(1, "ship", 0, modifier("shipID", "ItemModifier", aSource, operation))
	u.typ(hullType, 1, map[int]float64{})
	in := fitting.Input{Hull: hullType}
	for i, v := range values {
		id := 200 + i
		u.typ(id, 3, map[int]float64{aSource: v}, 1)
		slot := comm.SlotLow
		if role == fitting.RoleSubsystem {
			slot = comm.SlotSubsystem
		}
		in.Modules = append(in.Modules, fitting.Module{Type: id, Slot: slot})
	}
	u.load(t)
	fit := newFit(t, in)
	return fit, fit.Attr(fit.Ship, aTarget)
}

func chain(factors ...float64) float64 {
	product := 1.0
	for i, f := range factors {
		product *= 1 + (f-1)*math.Exp(-math.Pow(float64(i)/2.67, 2))
	}
	return product
}

func TestEngineStackingPenalty(t *testing.T) {
	t.Run("modules are penalized", func(t *testing.T) {
		_, got := stackingFit(t, false, 4, fitting.RoleModule, 1.1, 1.3, 1.2)
		approx(t, 10*chain(1.3, 1.2, 1.1), got)
	})
	t.Run("strongest first for reducing modifiers", func(t *testing.T) {
		_, got := stackingFit(t, false, 4, fitting.RoleModule, 0.9, 0.7, 0.8)
		approx(t, 10*chain(0.7, 0.8, 0.9), got)
	})
	t.Run("stackable attribute is not penalized", func(t *testing.T) {
		_, got := stackingFit(t, true, 4, fitting.RoleModule, 1.1, 1.3, 1.2)
		approx(t, 10*1.1*1.3*1.2, got)
	})
	t.Run("positive and negative chains are apart", func(t *testing.T) {
		_, got := stackingFit(t, false, 4, fitting.RoleModule, 1.2, 0.8, 1.1, 0.9)
		approx(t, 10*chain(1.2, 1.1)*chain(0.8, 0.9), got)
	})
	t.Run("percent is penalized as 1+v/100", func(t *testing.T) {
		_, got := stackingFit(t, false, 6, fitting.RoleModule, 20, 10)
		approx(t, 10*chain(1.2, 1.1), got)
	})
	t.Run("divisions chain as 1/x", func(t *testing.T) {
		_, got := stackingFit(t, false, 5, fitting.RoleModule, 2, 4)
		approx(t, 10*chain(0.25, 0.5), got)
	})
	t.Run("additions are never penalized", func(t *testing.T) {
		_, got := stackingFit(t, false, 2, fitting.RoleModule, 1, 2, 3)
		approx(t, 16, got)
	})
	t.Run("subsystems are never penalized", func(t *testing.T) {
		_, got := stackingFit(t, false, 4, fitting.RoleSubsystem, 1.1, 1.3, 1.2)
		approx(t, 10*1.1*1.3*1.2, got)
	})
	t.Run("pre and post chains are independent", func(t *testing.T) {
		u := newUniverse(false, 0)
		u.effect(1, "pre", 0, modifier("shipID", "ItemModifier", aSource, 0))
		u.effect(2, "post", 0, modifier("shipID", "ItemModifier", aSource, 4))
		u.typ(hullType, 1, map[int]float64{})
		u.typ(200, 3, map[int]float64{aSource: 1.2}, 1)
		u.typ(201, 3, map[int]float64{aSource: 1.2}, 2)
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: 200, Slot: comm.SlotLow}, {Type: 201, Slot: comm.SlotLow}}})
		approx(t, 10*1.2*1.2, fit.Attr(fit.Ship, aTarget), "neither is a second in its chain")
	})
	t.Run("hull and skill sources are not penalized", func(t *testing.T) {
		u := newUniverse(false, 0)
		u.effect(1, "hull", 0, modifier("itemID", "ItemModifier", aSource, 4))
		u.effect(2, "skill", 0, modifier("shipID", "ItemModifier", aSource, 4))
		u.typ(hullType, 1, map[int]float64{aSource: 1.5}, 1)
		u.typ(skillType, 255, map[int]float64{aSource: 1.5}, 2)
		u.file.Skills = []int{skillType}
		u.load(t)
		fit := newFit(t, fitting.Input{Hull: hullType})
		approx(t, 10*1.5*1.5, fit.Attr(fit.Ship, aTarget))
	})
}

func TestEngineHandModifiers(t *testing.T) {
	u := newUniverse(false, 0)
	u.effect(1, "ship", 0, modifier("shipID", "ItemModifier", aSource, 4))
	u.typ(hullType, 1, map[int]float64{})
	u.typ(200, 3, map[int]float64{aSource: 1.2}, 1)
	u.load(t)
	fit := newFit(t, fitting.Input{Hull: hullType, Modules: []fitting.Module{{Type: 200, Slot: comm.SlotLow}}})
	module := fit.Modules[0]
	fit.AddHand(
		fitting.HandModifier{Source: module, Target: fit.Ship, Attribute: aTarget, Operation: 4, Value: func(*fitting.Fit) float64 { return 1.3 }},
		fitting.HandModifier{Source: fit.Ship, Target: fit.Ship, Attribute: aTarget, Operation: 2, Value: func(*fitting.Fit) float64 { return 1 }},
	)
	// the addition comes before the post multiplications, which chain together: the hand one is 1.3, the module's 1.2
	approx(t, 11*chain(1.3, 1.2), fit.Attr(fit.Ship, aTarget), "the hand modifier chains with the data one")
}

func TestEngineRealDataSmoke(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	rifter := comm.DogmaTypeOf(587)
	if rifter == nil {
		t.Skip("dogma.json has no Rifter")
	}
	shield := comm.DogmaAttributeNamed("shieldCapacity")
	if shield == nil {
		t.Skip("dogma.json has no shieldCapacity")
	}
	fit := newFit(t, fitting.Input{Hull: 587})
	// Shield Management at V adds 25%
	approx(t, rifter.Attributes[shield.AttributeID]*1.25, fit.AttrNamed(fit.Ship, "shieldCapacity"), "a bare Rifter's shield is its base with Shield Management V")
}
