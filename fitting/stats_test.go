package fitting_test

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// the made-up dogma.json these tests run on: attribute and effect ids are arbitrary, as code finds them by name
var s4testAttributeNames = []string{
	"duration", "capacitorNeed", "shieldCapacity", "armorHP", "hp", "shieldRechargeRate",
	"shieldEmDamageResonance", "shieldThermalDamageResonance", "shieldKineticDamageResonance", "shieldExplosiveDamageResonance",
	"armorEmDamageResonance", "armorThermalDamageResonance", "armorKineticDamageResonance", "armorExplosiveDamageResonance",
	"emDamageResonance", "thermalDamageResonance", "kineticDamageResonance", "explosiveDamageResonance",
	"shieldBonus", "armorDamageAmount", "structureDamageAmount", "powerTransferAmount", "chargedArmorDamageMultiplier", "resistanceShiftAmount",
	"scanRadarStrength", "scanLadarStrength", "scanMagnetometricStrength", "scanGravimetricStrength",
	"ECMResistance", "remoteRepairImpedance", "energyNeutralizerAmount",
}

// attr is a test attribute's id, from its name
func s4attr(name string) int {
	for i, n := range s4testAttributeNames {
		if n == name {
			return 100 + i
		}
	}
	panic("no test attribute " + name)
}

var s4testEffectNames = map[string]struct{ id, category int }{
	"shieldBoosting": {4, 1}, "armorRepair": {27, 1}, "structureRepair": {26, 1}, "fueledShieldBoosting": {4936, 1}, "fueledArmorRepair": {5275, 1},
	"shipModuleRemoteShieldBooster": {6186, 2}, "shipModuleRemoteArmorRepairer": {6188, 2}, "shipModuleRemoteHullRepairer": {6185, 2},
	"shipModuleAncillaryRemoteArmorRepairer": {6651, 2}, "shipModuleAncillaryRemoteShieldBooster": {6652, 2},
	"ShipModuleRemoteArmorMutadaptiveRepairer": {7166, 2}, "shipModuleRemoteCapacitorTransmitter": {6184, 2},
	"adaptiveArmorHardener": {4928, 1}, "online": {16, 4},
	"energyNeutralizerFalloff": {6187, 2}, "energyNosferatuFalloff": {6197, 2},
}

// ids of the made-up types
const (
	s4hullType       = 1000
	s4characterType  = 1
	s4prepType       = 2000 // a module with only the online effect
	s4pasteType      = 3000
	s4rahType        = 4000
	s4rahType2       = 4001
	s4shieldBoosterT = 5000
	s4ancShieldT     = 5001
	s4armorRepT      = 5002
	s4ancArmorT      = 5003
	s4hullRepT       = 5004
	s4remShieldT     = 6000
	s4remArmorT      = 6001
	s4remHullT       = 6002
	s4remAncArmorT   = 6003
	s4remMutT        = 6004
	s4remCapT        = 6005
	s4neutT          = 7000
	s4nosT           = 7001
	s4oddHullType    = 1001
	s4tieHullType    = 1002
	s4bareHullType   = 1003
)

// s4typeSpec is one test type: its attributes by name and its effects by name
type s4typeSpec struct {
	attrs   map[string]float64
	effects []string
}

// s4loadTestDogma loads a dogma.json made of the given types, plus the usual attributes and effects
func s4loadTestDogma(t *testing.T, specs map[int]s4typeSpec) {
	t.Helper()
	comm.InitLogger(logrus.ErrorLevel)
	file := comm.DogmaFile{SDEBuild: 0, CharacterType: s4characterType, Attributes: map[int]*comm.DogmaAttribute{}, Effects: map[int]*comm.DogmaEffect{}, Types: map[int]*comm.DogmaType{}, BonusSkills: map[int]int{}, Modes: map[int][]comm.DogmaMode{}}
	for _, name := range s4testAttributeNames {
		def := 0.0
		if len(name) > 9 && name[len(name)-9:] == "Resonance" {
			def = 1
		}
		file.Attributes[s4attr(name)] = &comm.DogmaAttribute{Name: name, Default: def}
	}
	for name, e := range s4testEffectNames {
		effect := &comm.DogmaEffect{Name: name, Category: e.category}
		if e.category != 4 {
			effect.Duration = s4attr("duration")
			effect.Discharge = s4attr("capacitorNeed")
		}
		file.Effects[e.id] = effect
	}
	file.Types[s4characterType] = &comm.DogmaType{Group: 1, Category: 1, Attributes: map[int]float64{}}
	for id, spec := range specs {
		dt := &comm.DogmaType{Group: 1, Category: 6, Attributes: map[int]float64{}}
		for name, value := range spec.attrs {
			dt.Attributes[s4attr(name)] = value
		}
		for _, name := range spec.effects {
			dt.Effects = append(dt.Effects, s4testEffectNames[name].id)
		}
		file.Types[id] = dt
	}
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := comm.LoadDogma(fstest.MapFS{"dogma.json": {Data: data}}, "dogma.json"); err != nil {
		t.Fatal(err)
	}
}

// module is a fitted module in a test fit
func s4module(typeID, charge int) fitting.Module {
	return fitting.Module{Type: typeID, Charge: charge, Slot: comm.SlotMid}
}

func s4compute(t *testing.T, in fitting.Input) *fitting.Stats {
	t.Helper()
	stats, err := fitting.Compute(in)
	if err != nil {
		t.Fatal(err)
	}
	return stats
}

func s4baseSpecs() map[int]s4typeSpec {
	return map[int]s4typeSpec{
		s4hullType: {attrs: map[string]float64{
			"shieldCapacity": 1000, "armorHP": 2000, "hp": 500, "shieldRechargeRate": 500000,
			"shieldEmDamageResonance": 1, "shieldThermalDamageResonance": 0.8, "shieldKineticDamageResonance": 0.6, "shieldExplosiveDamageResonance": 0.4,
			"armorEmDamageResonance": 0.5, "armorThermalDamageResonance": 0.5, "armorKineticDamageResonance": 0.5, "armorExplosiveDamageResonance": 0.5,
			"emDamageResonance": 1, "thermalDamageResonance": 1, "kineticDamageResonance": 1, "explosiveDamageResonance": 1,
			"scanRadarStrength": 10, "scanLadarStrength": 20, "scanMagnetometricStrength": 5, "scanGravimetricStrength": 8,
			"ECMResistance": 0.6,
		}},
		s4bareHullType:   {attrs: map[string]float64{"shieldCapacity": 100}},
		s4tieHullType:    {attrs: map[string]float64{"scanRadarStrength": 15, "scanLadarStrength": 15, "scanMagnetometricStrength": 5}},
		s4prepType:       {effects: []string{"online"}},
		s4shieldBoosterT: {attrs: map[string]float64{"shieldBonus": 400, "duration": 4000}, effects: []string{"shieldBoosting"}},
		s4ancShieldT:     {attrs: map[string]float64{"shieldBonus": 300, "duration": 3000, "chargedArmorDamageMultiplier": 3}, effects: []string{"fueledShieldBoosting"}},
		s4armorRepT:      {attrs: map[string]float64{"armorDamageAmount": 600, "duration": 5000}, effects: []string{"armorRepair"}},
		s4ancArmorT:      {attrs: map[string]float64{"armorDamageAmount": 200, "duration": 10000, "chargedArmorDamageMultiplier": 3}, effects: []string{"fueledArmorRepair"}},
		s4hullRepT:       {attrs: map[string]float64{"structureDamageAmount": 100, "duration": 2000}, effects: []string{"structureRepair"}},
		s4pasteType:      {},
		s4remShieldT:     {attrs: map[string]float64{"shieldBonus": 500, "duration": 5000}, effects: []string{"shipModuleRemoteShieldBooster"}},
		s4remArmorT:      {attrs: map[string]float64{"armorDamageAmount": 300, "duration": 3000}, effects: []string{"shipModuleRemoteArmorRepairer"}},
		s4remHullT:       {attrs: map[string]float64{"structureDamageAmount": 90, "duration": 3000}, effects: []string{"shipModuleRemoteHullRepairer"}},
		s4remAncArmorT:   {attrs: map[string]float64{"armorDamageAmount": 200, "duration": 4000, "chargedArmorDamageMultiplier": 3}, effects: []string{"shipModuleAncillaryRemoteArmorRepairer"}},
		s4remMutT:        {attrs: map[string]float64{"armorDamageAmount": 250, "duration": 5000}, effects: []string{"ShipModuleRemoteArmorMutadaptiveRepairer"}},
		s4remCapT:        {attrs: map[string]float64{"powerTransferAmount": 800, "duration": 4000}, effects: []string{"shipModuleRemoteCapacitorTransmitter"}},
		s4neutT:          {attrs: map[string]float64{"energyNeutralizerAmount": 600, "duration": 12000}, effects: []string{"energyNeutralizerFalloff"}},
		s4nosT:           {attrs: map[string]float64{"powerTransferAmount": 60, "duration": 6000}, effects: []string{"energyNosferatuFalloff"}},
	}
}

// TestLayersAndEHP checks each layer's HP, resonances, and EHP against even damage
func TestLayersAndEHP(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType})

	assert.Equal(t, 1000.0, stats.Shield.HP)
	assert.Equal(t, [4]float64{1, 0.8, 0.6, 0.4}, stats.Shield.Resonance)
	assert.InDelta(t, 1000/0.7, stats.Shield.EHP, 1e-9)
	assert.InDelta(t, 4000.0, stats.Armor.EHP, 1e-9)
	assert.InDelta(t, 500.0, stats.Hull.EHP, 1e-9)
	assert.InDelta(t, 1000/0.7+4000+500, stats.EHP, 1e-9)
	assert.Empty(t, stats.Assumptions)
}

// TestLocalRep checks local repair: cycle time, the paste multiplier, and the ancillary shield booster getting none
func TestLocalRep(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())

	stats := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{
		s4module(s4shieldBoosterT, 0), s4module(s4armorRepT, 0), s4module(s4hullRepT, 0),
	}})
	assert.InDelta(t, 100.0, stats.LocalRep.Shield, 1e-9)
	assert.InDelta(t, 120.0, stats.LocalRep.Armor, 1e-9)
	assert.InDelta(t, 50.0, stats.LocalRep.Hull, 1e-9)
	assert.NotContains(t, stats.Assumptions, "Every module active", "every module active goes without saying")

	// the ancillary armor repairer: x3 with paste, x1 without
	withPaste := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4ancArmorT, s4pasteType)}})
	assert.InDelta(t, 60.0, withPaste.LocalRep.Armor, 1e-9)
	noPaste := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4ancArmorT, 0)}})
	assert.InDelta(t, 20.0, noPaste.LocalRep.Armor, 1e-9)

	// the ancillary shield booster ignores the multiplier even with paste loaded
	anc := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4ancShieldT, s4pasteType)}})
	assert.InDelta(t, 100.0, anc.LocalRep.Shield, 1e-9)
	assert.Zero(t, anc.RemoteRep)
}

// TestPassiveRegen checks the peak shield recharge
func TestPassiveRegen(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType})
	assert.InDelta(t, 2.5*1000/500.0, stats.PassiveRegen, 1e-9)

	// a hull with no recharge time has no regen, rather than dividing by zero
	assert.Zero(t, s4compute(t, fitting.Input{Hull: s4bareHullType}).PassiveRegen)
}

// TestRemoteRep checks the remote repair of each layer, the capacitor, the paste multiplier, and the mutadaptive assumption
func TestRemoteRep(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())

	stats := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{
		s4module(s4remShieldT, 0), s4module(s4remArmorT, 0), s4module(s4remHullT, 0), s4module(s4remCapT, 0), s4module(s4remAncArmorT, s4pasteType),
	}})
	assert.InDelta(t, 100.0, stats.RemoteRep.Shield, 1e-9)
	assert.InDelta(t, 100.0+150.0, stats.RemoteRep.Armor, 1e-9) // 300/3 and 200*3/4
	assert.InDelta(t, 30.0, stats.RemoteRep.Hull, 1e-9)
	assert.InDelta(t, 200.0, stats.RemoteRep.Capacitor, 1e-9)
	assert.Zero(t, stats.LocalRep)
	assert.Empty(t, stats.Assumptions)

	unloaded := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4remAncArmorT, 0)}})
	assert.InDelta(t, 50.0, unloaded.RemoteRep.Armor, 1e-9)

	mutadaptive := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4remMutT, 0)}})
	assert.InDelta(t, 50.0, mutadaptive.RemoteRep.Armor, 1e-9)
	assert.Len(t, mutadaptive.Assumptions, 1)
}

// TestSensor checks the strongest sensor is named, and that a tie is Multispectral
func TestSensor(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	assert.Equal(t, fitting.Sensor{Type: "Ladar", Strength: 20}, s4compute(t, fitting.Input{Hull: s4hullType}).Sensor)
	assert.Equal(t, fitting.Sensor{Type: "Multispectral", Strength: 15}, s4compute(t, fitting.Input{Hull: s4tieHullType}).Sensor)
	assert.Equal(t, "", s4compute(t, fitting.Input{Hull: s4bareHullType}).Sensor.Type)
}

// TestEwarResist checks only the resistance attributes dogma.json has are reported, with the ship's value or the attribute's default
func TestEwarResist(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType})
	assert.Equal(t, 0.6, stats.EwarResist["ECMResistance"])
	assert.Zero(t, stats.EwarResist["remoteRepairImpedance"])
	assert.Len(t, stats.EwarResist, 2) // the test dogma.json has only these two of the nine
	assert.NotContains(t, stats.EwarResist, "sensorDampenerResistance")
}

// TestUnknownReported checks that types dogma.json lacks are listed, not fatal
func TestUnknownReported(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(99999, 0)}})
	assert.Equal(t, []int{99999}, stats.Unknown)
	_, err := fitting.Compute(fitting.Input{Hull: 424242})
	assert.ErrorIs(t, err, fitting.ErrUnknownHull)
}

// TestNotFinite checks a stat that overflows, which only a broken dogma.json can cause, gives an error rather than numbers json can't carry
func TestNotFinite(t *testing.T) {
	specs := s4baseSpecs()
	// a huge shield over near-zero resonances: its EHP overflows
	specs[s4hullType].attrs["shieldCapacity"] = 1e300
	for _, name := range []string{"shieldEmDamageResonance", "shieldThermalDamageResonance", "shieldKineticDamageResonance", "shieldExplosiveDamageResonance"} {
		specs[s4hullType].attrs[name] = 1e-300
	}
	s4loadTestDogma(t, specs)
	_, err := fitting.Compute(fitting.Input{Hull: s4hullType})
	assert.ErrorIs(t, err, fitting.ErrNotFinite)
}

// TestNeut checks neutralizers and nosferatus add up to the capacitor they take each second, and nothing else counts
func TestNeut(t *testing.T) {
	s4loadTestDogma(t, s4baseSpecs())
	stats := s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4neutT, 0), s4module(s4neutT, 0), s4module(s4nosT, 0)}})
	assert.InDelta(t, 2*600/12.0+60/6.0, stats.Neut, 1e-9)
	assert.Zero(t, s4compute(t, fitting.Input{Hull: s4hullType, Modules: []fitting.Module{s4module(s4remCapT, 0)}}).Neut, "a capacitor transmitter gives, it doesn't neut")
}
