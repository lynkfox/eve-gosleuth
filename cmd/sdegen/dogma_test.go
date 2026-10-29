package main

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/stretchr/testify/assert"
)

// the ids of the tiny SDE below
const (
	testHull         = 100
	testSkill        = 200
	testHecate       = 300
	testOldModule    = 400
	testNoDogmaOld   = 401
	testNPC          = 500
	testUnpubSkill   = 201
	testModeDefense  = 310
	testModeSharp    = 311
	testModeProp     = 312
	testLonelyHull   = 320
	testLonelyMode   = 330
	attrResonance    = 271
	attrResonanceCap = 1527
	attrHardener     = 984
	attrHardenerBase = 990
	attrBonusMF      = 587
	attrRoleBonus    = 588
	attrUnrelated    = 999
)

// jsonl joins lines into a jsonl file
func jsonl(lines ...string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(strings.Join(lines, "\n") + "\n")}
}

// tinySDE is an SDE with one of each thing the dogma extract treats differently
func tinySDE() fstest.MapFS {
	return fstest.MapFS{
		"_sde.jsonl": jsonl(`{"_key":"sde","buildNumber":42}`),
		"groups.jsonl": jsonl(
			`{"_key":25,"categoryID":6}`, `{"_key":1305,"categoryID":6}`, `{"_key":1306,"categoryID":7}`,
			`{"_key":60,"categoryID":7}`, `{"_key":257,"categoryID":16}`, `{"_key":1,"categoryID":1}`, `{"_key":99,"categoryID":11}`),
		"types.jsonl": jsonl(
			fmt.Sprintf(`{"_key":%d,"groupID":25,"name":{"en":"Rifter"},"published":true}`, testHull),
			fmt.Sprintf(`{"_key":%d,"groupID":257,"name":{"en":"Minmatar Frigate"},"published":true}`, testSkill),
			fmt.Sprintf(`{"_key":%d,"groupID":257,"name":{"en":"Retired Skill"},"published":false}`, testUnpubSkill),
			fmt.Sprintf(`{"_key":%d,"groupID":1305,"name":{"en":"Hecate"},"published":true}`, testHecate),
			fmt.Sprintf(`{"_key":%d,"groupID":1306,"name":{"en":"Hecate Defense Mode"},"published":false}`, testModeDefense),
			fmt.Sprintf(`{"_key":%d,"groupID":1306,"name":{"en":"Hecate Sharpshooter Mode"},"published":false}`, testModeSharp),
			fmt.Sprintf(`{"_key":%d,"groupID":1306,"name":{"en":"Hecate Propulsion Mode"},"published":false}`, testModeProp),
			fmt.Sprintf(`{"_key":%d,"groupID":1305,"name":{"en":"Lonely"},"published":true}`, testLonelyHull),
			fmt.Sprintf(`{"_key":%d,"groupID":1306,"name":{"en":"Lonely Defense Mode"},"published":false}`, testLonelyMode),
			fmt.Sprintf(`{"_key":%d,"groupID":60,"name":{"en":"Old Plate"},"published":false}`, testOldModule),
			fmt.Sprintf(`{"_key":%d,"groupID":60,"name":{"en":"Never Had Dogma"},"published":false}`, testNoDogmaOld),
			fmt.Sprintf(`{"_key":%d,"groupID":99,"name":{"en":"Sansha Thing"},"published":true}`, testNPC),
			`{"_key":1373,"groupID":1,"name":{"en":"CharacterType"},"published":false}`),
		"dogmaAttributes.jsonl": jsonl(
			`{"_key":271,"name":"shieldEmDamageResonance","defaultValue":1.0,"stackable":false,"highIsGood":false,"maxAttributeID":1527}`,
			`{"_key":1527,"name":"maxShieldResonance","defaultValue":1.0,"stackable":true}`,
			`{"_key":984,"name":"emDamageResistanceBonus","defaultValue":0.0,"stackable":false,"highIsGood":true}`,
			`{"_key":990,"name":"hardenerBase","defaultValue":0.0,"stackable":true}`,
			`{"_key":587,"name":"shipBonusMF","defaultValue":0.0,"stackable":true}`,
			`{"_key":588,"name":"shipBonusRole1","defaultValue":0.0,"stackable":true}`,
			`{"_key":999,"name":"unrelated","defaultValue":0.0,"stackable":true}`,
			`{"_key":182,"name":"requiredSkill1","defaultValue":0.0,"stackable":true}`,
			`{"_key":280,"name":"skillLevel","defaultValue":0.0,"stackable":true}`,
			`{"_key":263,"name":"shieldCapacity","defaultValue":0.0,"stackable":true}`,
			`{"_key":73,"name":"duration","defaultValue":0.0,"stackable":true}`,
			`{"_key":6,"name":"capacitorNeed","defaultValue":0.0,"stackable":true}`),
		// effect 1 only matters because effect 2 uses attribute 984, so the closure takes two passes in id order
		"dogmaEffects.jsonl": jsonl(
			`{"_key":1,"name":"hardenerBase","effectCategoryID":4,"modifierInfo":[`+
				`{"domain":"itemID","func":"ItemModifier","modifiedAttributeID":984,"modifyingAttributeID":990,"operation":0},`+
				`{"domain":"targetID","func":"ItemModifier","modifiedAttributeID":984,"modifyingAttributeID":990,"operation":0},`+
				`{"domain":"target","func":"EffectStopper","effectID":6441},`+
				`{"domain":"itemID","func":"ItemModifier","modifiedAttributeID":999,"modifyingAttributeID":990,"operation":0}]}`,
			`{"_key":2,"name":"hardener","effectCategoryID":1,"durationAttributeID":73,"dischargeAttributeID":6,"modifierInfo":[`+
				`{"domain":"shipID","func":"LocationRequiredSkillModifier","modifiedAttributeID":271,"modifyingAttributeID":984,"operation":6,"skillTypeID":200}]}`,
			`{"_key":3,"name":"irrelevant","effectCategoryID":0,"modifierInfo":[{"domain":"shipID","func":"ItemModifier","modifiedAttributeID":999,"modifyingAttributeID":990,"operation":6}]}`,
			`{"_key":4,"name":"shieldBoosting","effectCategoryID":1,"durationAttributeID":73}`,
			`{"_key":5,"name":"projectedOnly","effectCategoryID":2,"modifierInfo":[{"domain":"targetID","func":"ItemModifier","modifiedAttributeID":271,"modifyingAttributeID":984,"operation":6}]}`,
			`{"_key":6,"name":"bonusMF","effectCategoryID":0,"modifierInfo":[{"domain":"shipID","func":"LocationRequiredSkillModifier","modifiedAttributeID":999,"modifyingAttributeID":587,"operation":6,"skillTypeID":200}]}`),
		"typeDogma.jsonl": jsonl(
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":271,"value":0.5},{"attributeID":263,"value":0.0},{"attributeID":587,"value":10.0},{"attributeID":588,"value":5.0},{"attributeID":999,"value":3.0},{"attributeID":182,"value":200.0}],"dogmaEffects":[{"effectID":2,"isDefault":true},{"effectID":3,"isDefault":false},{"effectID":4,"isDefault":false}]}`, testHull),
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":280,"value":1.0},{"attributeID":999,"value":3.0}]}`, testSkill),
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":999,"value":1.0}]}`, testHecate),
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":984,"value":-25.5}],"dogmaEffects":[{"effectID":1,"isDefault":false},{"effectID":5,"isDefault":true}]}`, testOldModule),
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":999,"value":1.0}]}`, testNPC),
			`{"_key":1373,"dogmaAttributes":[{"attributeID":999,"value":1.0}]}`,
			fmt.Sprintf(`{"_key":%d,"dogmaAttributes":[{"attributeID":271,"value":0.8}]}`, testModeDefense)),
		"typeBonus.jsonl": jsonl(
			fmt.Sprintf(`{"_key":%d,"roleBonuses":[{"bonus":5.0}],"types":[{"_key":%d,"_value":[{"bonus":10.0}]}]}`, testHull, testSkill)),
	}
}

// tinyFit is the fit_types extract of the tiny SDE: the hull and the Hecate, which fit_types.json would hold
func tinyFit() *comm.FitTypesFile {
	return &comm.FitTypesFile{SDEBuild: 42, Types: map[int]*comm.FitType{testHull: {TypeID: testHull}, testHecate: {TypeID: testHecate}, testLonelyHull: {TypeID: testLonelyHull}}}
}

func TestClassifyBonus(t *testing.T) {
	tests := []struct {
		name  string
		skill string
		kind  bonusKind
	}{
		{"shipBonusMF", "Minmatar Frigate", bonusSkill},
		{"shipBonusMF2", "Minmatar Frigate", bonusSkill},
		{"shipBonus2AF", "Amarr Frigate", bonusSkill},
		{"ShipBonusMC3", "Minmatar Cruiser", bonusSkill},
		{"shipBonusCBC1", "Caldari Battlecruiser", bonusSkill},
		{"shipBonusCB3", "Caldari Battleship", bonusSkill},
		{"shipBonusGI2", "Gallente Hauler", bonusSkill},
		{"shipBonusDreadnoughtG1", "Gallente Dreadnought", bonusSkill},
		{"shipBonusDreadnoughtPC2", "Precursor Dreadnought", bonusSkill},
		{"shipBonusCarrierA4", "Amarr Carrier", bonusSkill},
		{"shipBonusForceAuxiliaryM2", "Minmatar Carrier", bonusSkill},
		{"shipBonusSupercarrierC5", "Caldari Carrier", bonusSkill},
		{"shipBonusTitanA1", "Amarr Titan", bonusSkill},
		{"shipBonusTacticalDestroyerGallente2", "Gallente Tactical Destroyer", bonusSkill},
		{"shipBonusNavyDestroyerAmarr1", "Amarr Destroyer", bonusSkill},
		{"shipBonusStrategicCruiserMinmatar1", "Minmatar Strategic Cruiser", bonusSkill},
		{"shipBonusPBS2", "Precursor Battleship", bonusSkill},
		{"shipBonusUF1", "EDENCOM Frigate", bonusSkill},
		{"shipBonusUH2", "Upwell Hauler", bonusSkill},
		{"shipBonusICS3", "Industrial Command Ships", bonusSkill},
		{"shipBonusCommandCarrierShieldCommand", "Command Carriers", bonusSkill},
		{"eliteBonusHeavyGunship2", "Heavy Assault Cruisers", bonusSkill},
		{"eliteBonusInterceptor2", "Interceptors", bonusSkill},
		{"eliteBonusViolators1", "Marauders", bonusSkill},
		{"subsystemBonusAmarrDefensive2", "Amarr Defensive Systems", bonusSkill},
		{"subsystemBonusGallenteCore", "Gallente Core Systems", bonusSkill},
		{"exhumersBonusOreMiningYield", "Exhumers", bonusSkill},
		{"freighterBonusO1", "ORE Freighter", bonusSkill},
		{"minmatarIndustrialBonusGasHoldCapacity", "Minmatar Hauler", bonusSkill},
		{"shipBonusRole1", "", bonusRole},
		{"roleBonusCBC", "", bonusRole},
		{"eliteBonusViolatorsRole2", "", bonusRole},
		{"eliteBonusInterceptorRole", "", bonusRole},
		{"shipBonusATC1", "", bonusRole},
		{"ShipBonusAllShieldResistAT1", "", bonusRole},
		{"shipBonusHPExtender1", "", bonusRole},      // a hand fix
		{"eliteBonusBarge1", "Exhumers", bonusSkill}, // a hand fix
		{"shipBonusSomethingNew", "", bonusUnplaced},
		{"eliteBonusSomethingNew1", "", bonusUnplaced},
		{"damageMultiplier", "", bonusUnplaced},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			skill, kind := classifyBonus(test.name)
			assert.Equal(t, test.kind, kind)
			assert.Equal(t, test.skill, skill)
		})
	}
}

func TestExtractDogma(t *testing.T) {
	dogma, source, err := ExtractDogma(tinySDE(), tinyFit())
	assert.NoError(t, err)

	t.Run("header", func(t *testing.T) {
		assert.Equal(t, 42, dogma.SDEBuild)
		assert.Equal(t, 1373, dogma.CharacterType)
		assert.Equal(t, []int{testSkill}, dogma.Skills, "published skills only")
	})

	t.Run("which types go in", func(t *testing.T) {
		for _, id := range []int{testHull, testHecate, testSkill, testOldModule, 1373, testModeDefense, testModeSharp, testModeProp, testLonelyMode} {
			assert.Contains(t, dogma.Types, id, "type %d", id)
		}
		assert.NotContains(t, dogma.Types, testNoDogmaOld, "an unpublished type with no dogma")
		assert.NotContains(t, dogma.Types, testUnpubSkill)
		assert.NotContains(t, dogma.Types, testNPC)
		assert.Equal(t, 25, dogma.Types[testHull].Group)
		assert.Equal(t, 6, dogma.Types[testHull].Category)
		assert.Equal(t, 16, dogma.Types[testSkill].Category)
	})

	t.Run("a type with nothing relevant is kept", func(t *testing.T) {
		assert.Contains(t, dogma.Types, testHecate)
		assert.Empty(t, dogma.Types[testHecate].Attributes)
		assert.Contains(t, dogma.Types, 1373)
	})

	t.Run("attributes are the closure of the outputs", func(t *testing.T) {
		// 984 modifies the output 271 through effect 2, and 990 modifies 984 through effect 1: found in the second pass
		for _, id := range []int{attrResonance, attrResonanceCap, attrHardener, attrHardenerBase, attrBonusMF, 182, 280, 263, 73, 6} {
			assert.Contains(t, dogma.Attributes, id, "attribute %d", id)
		}
		assert.NotContains(t, dogma.Attributes, attrUnrelated, "999 is only modified by effects that nothing kept reads")
		assert.NotContains(t, dogma.Attributes, attrRoleBonus, "an unscaled role bonus is not an output")
		assert.Equal(t, &comm.DogmaAttribute{Name: "shieldEmDamageResonance", Default: 1, Max: attrResonanceCap}, dogma.Attributes[attrResonance])
		assert.True(t, dogma.Attributes[attrHardener].HighIsGood)
	})

	t.Run("a type keeps only kept attributes that differ from the default", func(t *testing.T) {
		assert.Equal(t, map[int]float64{attrResonance: 0.5, attrBonusMF: 10, 182: 200}, dogma.Types[testHull].Attributes)
		assert.Equal(t, map[int]float64{280: 1}, dogma.Types[testSkill].Attributes)
		assert.Equal(t, map[int]float64{attrHardener: -25.5}, dogma.Types[testOldModule].Attributes)
	})

	t.Run("effects are kept for a kept modifier or because the code handles them", func(t *testing.T) {
		assert.Contains(t, dogma.Effects, 1)
		assert.Contains(t, dogma.Effects, 2)
		assert.Contains(t, dogma.Effects, 4, "shield boosting has no modifiers")
		assert.NotContains(t, dogma.Effects, 3, "it modifies only an unrelated attribute")
		assert.NotContains(t, dogma.Effects, 5, "it only projects onto a target")
		assert.NotContains(t, dogma.Effects, 6, "the hull bonus feeds only the unrelated attribute")
	})

	t.Run("modifiers are mapped and filtered", func(t *testing.T) {
		assert.Equal(t, []comm.DogmaModifier{{Domain: "shipID", Func: "LocationRequiredSkillModifier", Modified: attrResonance, Modifying: attrHardener, Operation: 6, Skill: testSkill}}, dogma.Effects[2].Modifiers)
		assert.Equal(t, []comm.DogmaModifier{{Domain: "itemID", Func: "ItemModifier", Modified: attrHardener, Modifying: attrHardenerBase, Operation: 0}}, dogma.Effects[1].Modifiers, "target domains, the effect stopper, and the unrelated modifier are dropped")
		assert.Equal(t, &comm.DogmaEffect{Name: "hardener", Category: 1, Duration: 73, Discharge: 6, Modifiers: dogma.Effects[2].Modifiers}, dogma.Effects[2])
	})

	t.Run("a type lists its kept effects and the default one", func(t *testing.T) {
		assert.Equal(t, []int{2, 4}, dogma.Types[testHull].Effects)
		assert.Equal(t, 2, dogma.Types[testHull].DefaultEffect)
		assert.Equal(t, []int{1}, dogma.Types[testOldModule].Effects)
		assert.Zero(t, dogma.Types[testOldModule].DefaultEffect, "effect 5 is not kept")
	})

	t.Run("modes are linked by name", func(t *testing.T) {
		assert.Equal(t, []comm.DogmaMode{{TypeID: testModeDefense, Name: "Defense"}, {TypeID: testModeSharp, Name: "Sharpshooter"}, {TypeID: testModeProp, Name: "Propulsion"}}, dogma.Modes[testHecate])
		assert.Len(t, source.modeReport, 1, "the Lonely hull has one mode, not 3")
		assert.Contains(t, source.modeReport[0], "Lonely")
	})

	t.Run("bonus skills leave out role bonuses", func(t *testing.T) {
		assert.Equal(t, map[int]int{attrBonusMF: testSkill}, dogma.BonusSkills)
		assert.Contains(t, source.bonus.role, attrRoleBonus)
	})
}

func TestLinkModesFlagsAnOrphanMode(t *testing.T) {
	sde := tinySDE()
	sde["types.jsonl"] = jsonl(
		`{"_key":300,"groupID":1305,"name":{"en":"Hecate"},"published":true}`,
		`{"_key":310,"groupID":1306,"name":{"en":"Hecate Defense Mode"},"published":false}`,
		`{"_key":311,"groupID":1306,"name":{"en":"Ghost Defense Mode"},"published":false}`)
	_, source, err := ExtractDogma(sde, &comm.FitTypesFile{SDEBuild: 42, Types: map[int]*comm.FitType{}})
	assert.NoError(t, err)
	assert.Contains(t, strings.Join(source.modeReport, "\n"), `"Ghost Defense Mode" has no tactical destroyer hull`)
	assert.Contains(t, strings.Join(source.modeReport, "\n"), "Hecate has 1 modes, not 3")
}

func TestCheckBonuses(t *testing.T) {
	_, source, err := ExtractDogma(tinySDE(), tinyFit())
	assert.NoError(t, err)

	t.Run("a value under the skill's traits agrees", func(t *testing.T) {
		evidence := checkBonuses(source)
		assert.Equal(t, 1, evidence[attrBonusMF].agree)
		assert.Zero(t, evidence[attrBonusMF].conflict)
	})

	t.Run("a value only among the role bonuses disagrees with a skill", func(t *testing.T) {
		source.traits[testHull].Types[0].Bonuses[0].Bonus = 99
		source.traits[testHull].Role[0].Bonus = 10
		evidence := checkBonuses(source)
		assert.Equal(t, 1, evidence[attrBonusMF].conflict)
		assert.Contains(t, evidence[attrBonusMF].conflicts[0], "only as a role bonus")
	})
}

func TestMarshalDogmaLoads(t *testing.T) {
	dogma, _, err := ExtractDogma(tinySDE(), tinyFit())
	assert.NoError(t, err)
	text, err := marshalDogma(dogma)
	assert.NoError(t, err)

	// one entry per line, in id order
	assert.Contains(t, string(text), "\n\"100\":{\"group\":25,\"category\":6,")
	assert.Less(t, strings.Index(string(text), "\n\"100\":"), strings.Index(string(text), "\n\"200\":"))

	err = comm.LoadDogma(fstest.MapFS{"dogma.json": {Data: text}}, "dogma.json")
	assert.NoError(t, err)
	assert.Equal(t, 0.5, comm.DogmaTypeOf(testHull).Attributes[attrResonance])
	assert.Equal(t, "hardener", comm.DogmaEffectOf(2).Name)
	assert.Equal(t, attrResonanceCap, comm.DogmaAttributeNamed("shieldEmDamageResonance").Max)
}
