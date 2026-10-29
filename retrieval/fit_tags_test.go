package retrieval_test

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// fitTagRulesFixture covers each rule mechanic with small numbers, so the tests don't follow tuning of the real file
const fitTagRulesFixture = `{
"bonus_ignores": ["resistance"],
"exclusive": {"tank style": ["Passive", "Buffer"]},
"tags": {
	"Neut": {"priority": 2, "role": true, "threshold": 3, "signals": [
		{"groups": [71, 68], "weight": 1, "bonused_by": ["Energy Neutralizer"], "bonused_weight": 1.5},
		{"markets": [691], "hull_sizes": [2], "weight": 3}
	]},
	"Logi": {"priority": 1, "role": true, "threshold": 1, "requires": [{"groups": [325]}], "forbids": [{"groups": [53, 74]}], "signals": [
		{"groups": [325], "weight": 1}
	]},
	"Tackle": {"priority": 3, "role": true, "threshold": 1, "signals": [
		{"groups": [52], "weight": 1, "max": 1},
		{"hull": true, "types": [627], "weight": 0.5}
	]},
	"Firewall": {"priority": 4, "role": true, "share_threshold": 1, "signals": [{"groups": [72], "weight": 1}]},
	"Drones": {"hidden": true, "threshold": 1, "signals": [{"groups": [100], "weight": 0.2}]},
	"Passive": {"priority": 5, "threshold": 1, "signals": [{"groups": [39], "weight": 1}]},
	"Buffer": {"priority": 6, "threshold": 1, "signals": [{"groups": [38], "weight": 1}]},
	"Ratting": {"priority": 7, "role": true, "threshold": 1, "tag_weights": {"Buffer": -5}, "signals": [{"groups": [65], "weight": 2}]},
	"Capped": {"hidden": true, "threshold": 1, "signals": [
		{"groups": [71, 68], "weight": 1, "max": 3, "bonused_by": ["Energy Neutralizer"], "bonused_weight": 2}
	]},
	"Swarm": {"hidden": true, "threshold": 2, "signals": [
		{"hull": true, "weight": 0, "bonused_by": ["Drone hitpoints"], "bonused_weight": 1},
		{"groups": [100], "once": true, "weight": 0.5, "bonused_by": ["Drone hitpoints"], "bonused_weight": 1}
	]},
	"Fleet Doctrine": {"priority": 1, "role": true, "fleet_doctrine": {"battle_hulls": 5}}
}}`

// tagFit builds a fit of a hull with the given modules in each rack, padded with empty slots to the hull's counts, and drone stacks by type
func tagFit(t *testing.T, hull int, highs, mids, lows []int, drones map[int]int) *retrieval.Fit {
	t.Helper()
	item := func(typeID int) *retrieval.Item {
		fitType := comm.FitTypeOf(typeID)
		if !assert.NotNil(t, fitType, "type %d", typeID) {
			t.FailNow()
		}
		return &retrieval.Item{ItemID: typeID, Name: fitType.Name}
	}
	rack := func(modules []int, size int) []*retrieval.FittedModule {
		slots := make([]*retrieval.FittedModule, max(size, len(modules)))
		for index, typeID := range modules {
			slots[index] = &retrieval.FittedModule{Module: item(typeID)}
		}
		return slots
	}
	hullType := comm.FitTypeOf(hull)
	fit := &retrieval.Fit{
		Hull:  item(hull),
		Highs: rack(highs, hullType.HighSlots),
		Mids:  rack(mids, hullType.MidSlots),
		Lows:  rack(lows, hullType.LowSlots),
		Rigs:  rack(nil, hullType.RigSlots),
	}
	for typeID, quantity := range drones {
		fit.Drones = append(fit.Drones, &retrieval.FitCargo{Item: item(typeID), Quantity: quantity})
	}
	return fit
}

// tagNames are the names of the tags, in order
func tagNames(tags []retrieval.FitTag) []string {
	names := []string{}
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return names
}

// tagNamed is the tag with the name, nil if the fit does not have it
func tagNamed(tags []retrieval.FitTag, name string) *retrieval.FitTag {
	for index := range tags {
		if tags[index].Name == name {
			return &tags[index]
		}
	}
	return nil
}

const (
	bhaalgorn             = 17920
	thorax                = 627
	exequror              = 634
	vexor                 = 626
	heron                 = 605
	stratios              = 33470
	heavyNeutralizer      = 12271
	heavyNosferatu        = 12263
	mediumNeutralizer     = 12267
	megaPulse             = 3057
	dualRailgun           = 3114
	mediumRemoteArmorRep  = 26913
	warpScrambler         = 448
	stasisWebifier        = 527
	shieldRecharger       = 394
	largeShieldExtender   = 3841
	hobgoblin             = 2456
	hammerhead            = 2183
	warrior               = 2486
	hobgoblinSD           = 23715
	acolyteEV             = 23659
	cynoGenerator         = 21096
	coreProbeLauncher     = 17938
	scanRangefindingArray = 33180
	dataAnalyzer          = 22175
	largeSmartbomb        = 3995
	steelPlates           = 20353
	largeArmorRepairer    = 3540
	bastionModule         = 33400
	shieldCommandBurst    = 43555
	paladin               = 28659
	nighthawk             = 22470
	kronos                = 28661
	lachesis              = 11971
	archon                = 23757
	legion                = 29986
	parasiticCore         = 45624
	cloakingDevice        = 11370
	capitalMWD            = 41253
	nanofiber             = 2605
	armorCommandBurst     = 43552
	trimarkPump           = 31055
	polarizedBlaster      = 34282
	sensorDampener        = 1969
)

func TestFitTagRules(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.LoadFitTypes(staticdata.Files, "fit_types.json"))
	assert.NoError(t, comm.LoadFitTagRules(fstest.MapFS{"rules.json": {Data: []byte(fitTagRulesFixture)}}, "rules.json"))

	t.Run("a hull bonused for a module makes each count its bonused weight", func(t *testing.T) {
		tags := tagFit(t, bhaalgorn, []int{heavyNeutralizer, heavyNosferatu}, nil, nil, nil).Tags()
		neut := tagNamed(tags, "Neut")
		if assert.NotNil(t, neut, "1.5 each on a Bhaalgorn reaches 3") {
			assert.Equal(t, 3.0, neut.Weight)
			assert.Equal(t, []retrieval.FitTagReason{{Name: "Heavy Energy Neutralizer II", Count: 1, Weight: 1.5, Bonus: 0.5}, {Name: "Heavy Energy Nosferatu II", Count: 1, Weight: 1.5, Bonus: 0.5}}, neut.Reasons, "with the part the hull's bonus added")
			assert.Equal(t, 3.0, neut.Threshold)
		}
		assert.Nil(t, tagNamed(tagFit(t, thorax, []int{mediumNeutralizer, mediumNeutralizer}, nil, nil, nil).Tags(), "Neut"), "1 each on a hull without the bonus")
	})

	t.Run("a signal with hull sizes counts only on those hulls", func(t *testing.T) {
		neut := tagNamed(tagFit(t, thorax, []int{heavyNeutralizer}, nil, nil, nil).Tags(), "Neut")
		if assert.NotNil(t, neut, "a heavy neutralizer on a cruiser") {
			assert.Equal(t, 4.0, neut.Weight, "1 as a neutralizer and 3 for its size")
		}
		assert.Nil(t, tagNamed(tagFit(t, bhaalgorn, []int{heavyNeutralizer}, nil, nil, nil).Tags(), "Neut"), "on a battleship it is only a neutralizer")
	})

	t.Run("requires must match and forbids must not", func(t *testing.T) {
		assert.NotNil(t, tagNamed(tagFit(t, exequror, []int{mediumRemoteArmorRep}, nil, nil, nil).Tags(), "Logi"))
		assert.Nil(t, tagNamed(tagFit(t, thorax, []int{mediumRemoteArmorRep, dualRailgun}, nil, nil, nil).Tags(), "Logi"), "a gun with it is spider tanking")
		assert.Nil(t, tagNamed(tagFit(t, exequror, nil, nil, nil, nil).Tags(), "Logi"))
	})

	t.Run("max caps a signal and the hull counts once", func(t *testing.T) {
		tackle := tagNamed(tagFit(t, thorax, nil, []int{warpScrambler, warpScrambler}, nil, nil).Tags(), "Tackle")
		if assert.NotNil(t, tackle) {
			assert.Equal(t, 1.5, tackle.Weight, "two scramblers capped at 1, and the Thorax hull's 0.5")
			assert.Equal(t, []retrieval.FitTagReason{{Name: "Warp Scrambler II", Count: 2, Weight: 1}, {Name: "Thorax", Count: 1, Weight: 0.5}}, tackle.Reasons, "in the order of the signals, the scramblers sharing the capped weight")
			assert.InDelta(t, 2.0/4*0.5+0.5/(14.0/3), tackle.Share, 0.0001, "two of four mids, halved by the cap, and the hull's 0.5 over the average of its 5, 4, and 5 slots")
		}
	})

	t.Run("a drone stack counts each drone", func(t *testing.T) {
		drones := tagNamed(tagFit(t, vexor, nil, nil, nil, map[int]int{hobgoblin: 5}).Tags(), "Drones")
		if assert.NotNil(t, drones) {
			assert.InDelta(t, 1.0, drones.Weight, 0.0001)
			assert.Len(t, drones.Reasons, 1)
			assert.Equal(t, 5, drones.Reasons[0].Count)
			assert.True(t, drones.Hidden)
		}
	})

	t.Run("a once signal's matches add the weight of one, and a zero weight hull signal counts only when bonused", func(t *testing.T) {
		swarm := tagNamed(tagFit(t, vexor, nil, nil, nil, map[int]int{hobgoblin: 5, hammerhead: 5}).Tags(), "Swarm")
		if assert.NotNil(t, swarm, "the Vexor's bonus and its drones") {
			assert.InDelta(t, 2.0, swarm.Weight, 0.0001)
			assert.ElementsMatch(t, []retrieval.FitTagReason{{Name: "Vexor", Count: 1, Weight: 1, Bonus: 1}, {Name: "Hobgoblin II", Count: 5, Weight: 0.5, Bonus: 0.25}, {Name: "Hammerhead I", Count: 5, Weight: 0.5, Bonus: 0.25}}, swarm.Reasons, "the two stacks sharing the one weight")
		}
		assert.Nil(t, tagNamed(tagFit(t, thorax, nil, nil, nil, map[int]int{hobgoblin: 5, hammerhead: 5}).Tags(), "Swarm"), "0.5 for drones on a hull without the bonus")
	})

	t.Run("tags sharing a priority go by how far past their threshold they are", func(t *testing.T) {
		ewar := retrieval.FitTag{Name: "EWAR", Priority: 5, Weight: 1.5, Threshold: 1.5}
		tackle := retrieval.FitTag{Name: "Tackle", Priority: 5, Weight: 4.5, Threshold: 2}
		firewall := retrieval.FitTag{Name: "Firewall", Priority: 5, Share: 1, ShareThreshold: 0.75}
		armor := retrieval.FitTag{Name: "Armor", Priority: 9, Weight: 9, Threshold: 2}
		assert.Equal(t, []string{"Tackle", "Firewall", "EWAR", "Armor"}, tagNames(retrieval.MergeFitTags([]retrieval.FitTag{armor, ewar, firewall, tackle})))
	})

	t.Run("only the first listed of an exclusive set applies, however heavy the rest", func(t *testing.T) {
		assert.Equal(t, []string{"Passive"}, tagNames(tagFit(t, heron, nil, []int{shieldRecharger, largeShieldExtender, largeShieldExtender}, nil, nil).Tags()))
		assert.Equal(t, []string{"Buffer"}, tagNames(tagFit(t, heron, nil, []int{largeShieldExtender, largeShieldExtender}, nil, nil).Tags()))
	})

	t.Run("tags are in priority order, those without one last, and never include Fleet Doctrine", func(t *testing.T) {
		tags := tagFit(t, thorax, nil, []int{warpScrambler, shieldRecharger}, nil, map[int]int{hobgoblin: 5}).Tags()
		assert.Equal(t, []string{"Tackle", "Passive", "Drones"}, tagNames(tags))
	})

	t.Run("a share threshold of 1 needs a full rack", func(t *testing.T) {
		full := []int{largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb}
		assert.Contains(t, tagNames(tagFit(t, bhaalgorn, full, nil, nil, nil).Tags()), "Firewall", "seven smartbombs in seven highs")
		assert.NotContains(t, tagNames(tagFit(t, bhaalgorn, full[1:], nil, nil, nil).Tags()), "Firewall", "six, and an empty slot")
	})

	t.Run("tag weights add the weight of other tags the fit has", func(t *testing.T) {
		assert.Contains(t, tagNames(tagFit(t, thorax, nil, []int{stasisWebifier}, nil, nil).Tags()), "Ratting")
		tags := tagFit(t, thorax, nil, []int{stasisWebifier, largeShieldExtender}, nil, nil).Tags()
		assert.NotContains(t, tagNames(tags), "Ratting", "a buffer tank takes 5 off the web's 2")
		assert.Contains(t, tagNames(tags), "Buffer")
	})

	t.Run("a capped signal's bonus is only what the cap left above the weight without it", func(t *testing.T) {
		capped := tagNamed(tagFit(t, bhaalgorn, []int{heavyNeutralizer, heavyNosferatu}, nil, nil, nil).Tags(), "Capped")
		if assert.NotNil(t, capped) {
			assert.Equal(t, 3.0, capped.Weight, "4 bonused, capped at 3")
			assert.Equal(t, []retrieval.FitTagReason{{Name: "Heavy Energy Neutralizer II", Count: 1, Weight: 1.5, Bonus: 0.5}, {Name: "Heavy Energy Nosferatu II", Count: 1, Weight: 1.5, Bonus: 0.5}}, capped.Reasons, "2 without the bonus, so it gave 1")
		}
		capped = tagNamed(tagFit(t, bhaalgorn, []int{heavyNeutralizer, heavyNeutralizer, heavyNosferatu}, nil, nil, nil).Tags(), "Capped")
		if assert.NotNil(t, capped) {
			for _, reason := range capped.Reasons {
				assert.Zero(t, reason.Bonus, "3 reach the cap without the bonus")
			}
		}
	})

	t.Run("a hull bonus line with an ignored phrase is not a bonus", func(t *testing.T) {
		assert.False(t, comm.HullBonused([]*comm.FitType{{Bonuses: []string{"bonus to Energy Neutralizer resistance"}}}, []string{"Energy Neutralizer"}))
		assert.True(t, comm.HullBonused([]*comm.FitType{{Bonuses: []string{"bonus to Energy Neutralizer drain amount"}}}, []string{"energy neutralizer"}), "ignoring case")
		assert.True(t, comm.HullBonused([]*comm.FitType{nil, {Bonuses: []string{"bonus to Energy Neutralizer drain amount"}}}, []string{"Energy Neutralizer"}), "a subsystem's bonus, the hull unknown")
	})

	t.Run("a nil fit has no tags", func(t *testing.T) {
		var fit *retrieval.Fit
		assert.Empty(t, fit.Tags())
	})
}

// TestEmbeddedFitTagRules checks the decisions behind the real fit_tag_rules.json, so tuning it can't quietly undo them
func TestEmbeddedFitTagRules(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("a Bhaalgorn with neuts and nosferatus is a neut boat first, and webber", func(t *testing.T) {
		tags := tagFit(t, bhaalgorn, []int{heavyNeutralizer, heavyNeutralizer, heavyNosferatu, heavyNosferatu, megaPulse, megaPulse, megaPulse}, []int{stasisWebifier, stasisWebifier}, nil, nil).Tags()
		names := tagNames(tags)
		if assert.NotEmpty(t, names) {
			assert.Equal(t, "Neut", names[0])
		}
		assert.Contains(t, names, "Webber")
		assert.Contains(t, names, "Lasers", "weighed, though hidden")
	})

	t.Run("a cyno outranks neut", func(t *testing.T) {
		tags := tagFit(t, bhaalgorn, []int{heavyNeutralizer, heavyNeutralizer, heavyNosferatu, heavyNosferatu, cynoGenerator}, nil, nil, nil).Tags()
		assert.Equal(t, []string{"Cyno", "Neut"}, tagNames(tags)[:2])
	})

	t.Run("a bastion with smartbombs is ratting", func(t *testing.T) {
		tags := tagFit(t, paladin, []int{megaPulse, bastionModule, megaPulse, largeSmartbomb, largeSmartbomb, megaPulse, megaPulse}, nil, nil, nil).Tags()
		assert.Equal(t, "Ratting", tags[0].Name)
		assert.NotContains(t, tagNames(tagFit(t, paladin, []int{megaPulse, bastionModule, megaPulse, megaPulse}, nil, nil, nil).Tags()), "Ratting", "a bastion alone")
		plated := tagFit(t, paladin, []int{megaPulse, bastionModule, megaPulse, largeSmartbomb, largeSmartbomb, megaPulse, megaPulse}, nil, []int{steelPlates, steelPlates}, nil).Tags()
		assert.NotContains(t, tagNames(plated), "Ratting", "a buffer tank counts hard against it")
	})

	t.Run("any repairer makes the tank active, whatever its buffer", func(t *testing.T) {
		tags := tagNames(tagFit(t, bhaalgorn, nil, nil, []int{steelPlates, steelPlates, steelPlates, largeArmorRepairer}, nil).Tags())
		assert.Contains(t, tags, "Active")
		assert.NotContains(t, tags, "Buffer")
	})

	t.Run("smartbombs in most of the highs are a firewaller", func(t *testing.T) {
		bombs := []int{largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb, largeSmartbomb}
		assert.Contains(t, tagNames(tagFit(t, bhaalgorn, bombs, nil, nil, nil).Tags()), "Firewaller", "six of seven")
		assert.NotContains(t, tagNames(tagFit(t, bhaalgorn, bombs[1:], nil, nil, nil).Tags()), "Firewaller", "five of seven")
	})

	t.Run("a polarized weapon names the row before anything else", func(t *testing.T) {
		tags := tagFit(t, kronos, []int{polarizedBlaster, bastionModule, polarizedBlaster, heavyNosferatu, heavyNosferatu}, nil, nil, nil).Tags()
		assert.Equal(t, "Polarized", tags[0].Name)
	})

	t.Run("one ewar module on a hull bonused for it is ewar, two anywhere else", func(t *testing.T) {
		assert.Contains(t, tagNames(tagFit(t, lachesis, nil, []int{sensorDampener}, nil, nil).Tags()), "EWAR")
		assert.NotContains(t, tagNames(tagFit(t, thorax, nil, []int{sensorDampener}, nil, nil).Tags()), "EWAR")
		assert.Contains(t, tagNames(tagFit(t, thorax, nil, []int{sensorDampener, sensorDampener}, nil, nil).Tags()), "EWAR")
	})

	t.Run("command bursts are links", func(t *testing.T) {
		assert.Equal(t, "Links", tagFit(t, nighthawk, []int{shieldCommandBurst}, nil, nil, nil).Tags()[0].Name)
	})

	t.Run("weapons and range are hidden, and can't name a row", func(t *testing.T) {
		for _, tag := range tagFit(t, thorax, []int{dualRailgun, dualRailgun, dualRailgun}, nil, nil, nil).Tags() {
			if tag.Name == "Hybrids" || tag.Name == "Long" {
				assert.True(t, tag.Hidden, tag.Name)
				assert.False(t, tag.Role, tag.Name)
			}
		}
	})

	t.Run("a remote repairer is logi only without guns", func(t *testing.T) {
		assert.Contains(t, tagNames(tagFit(t, exequror, []int{mediumRemoteArmorRep}, nil, nil, nil).Tags()), "Logi")
		assert.NotContains(t, tagNames(tagFit(t, thorax, []int{mediumRemoteArmorRep, dualRailgun, dualRailgun}, nil, nil, nil).Tags()), "Logi")
	})

	t.Run("a scanner has a scanning array with its probes, or an analyzer", func(t *testing.T) {
		assert.Contains(t, tagNames(tagFit(t, stratios, []int{coreProbeLauncher}, []int{scanRangefindingArray}, nil, nil).Tags()), "Scanner")
		assert.Contains(t, tagNames(tagFit(t, stratios, nil, []int{dataAnalyzer}, nil, nil).Tags()), "Scanner")
		assert.NotContains(t, tagNames(tagFit(t, stratios, []int{coreProbeLauncher}, nil, nil, nil).Tags()), "Scanner", "probes alone, ie combat probing")
	})

	t.Run("a capital is never nano or cloaky", func(t *testing.T) {
		tags := tagNames(tagFit(t, archon, []int{cloakingDevice}, []int{capitalMWD}, []int{nanofiber, nanofiber}, nil).Tags())
		assert.NotContains(t, tags, "Nano")
		assert.NotContains(t, tags, "Cloaky")
		assert.Contains(t, tagNames(tagFit(t, thorax, []int{cloakingDevice}, nil, nil, nil).Tags()), "Cloaky", "a cruiser still is")
	})

	t.Run("a roller outranks links", func(t *testing.T) {
		assert.Equal(t, "Roller", tagFit(t, archon, []int{armorCommandBurst}, []int{capitalMWD, capitalMWD}, nil, nil).Tags()[0].Name)
	})

	t.Run("a subsystem's bonus counts as the hull's", func(t *testing.T) {
		neuts := func(core bool) *retrieval.FitTag {
			fit := tagFit(t, legion, []int{mediumNeutralizer, mediumNeutralizer}, nil, nil, nil)
			if core {
				fit.Subsystems = []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: parasiticCore, Name: "Legion Core - Energy Parasitic Complex"}}}
			}
			return tagNamed(fit.Tags(), "Neut")
		}
		if neut := neuts(true); assert.NotNil(t, neut, "1.5 each with the neut bonused core") {
			assert.Equal(t, 1.0, neut.Reasons[0].Bonus)
		}
		assert.Nil(t, neuts(false), "1 each without it")
	})

	t.Run("implants nudge a tag, but never make one", func(t *testing.T) {
		amulets := []*retrieval.Item{}
		for _, id := range []int{20499, 20501, 20507, 20503, 20505} {
			amulets = append(amulets, &retrieval.Item{ItemID: id, Name: comm.FitTypeOf(id).Name})
		}
		armored := func(implants []*retrieval.Item, plates ...int) []retrieval.FitTag {
			fit := tagFit(t, thorax, nil, nil, plates, nil)
			fit.Rigs[0] = &retrieval.FittedModule{Module: &retrieval.Item{ItemID: trimarkPump, Name: "Medium Trimark Armor Pump I"}}
			fit.Implants = implants
			return fit.Tags()
		}
		assert.NotContains(t, tagNames(armored(nil, steelPlates)), "Armor", "a plate and a rig make 1.5")
		assert.Contains(t, tagNames(armored(amulets, steelPlates)), "Armor", "and five Amulets the last 0.5")
		if armor := tagNamed(armored(append(amulets, amulets...), steelPlates), "Armor"); assert.NotNil(t, armor) {
			assert.Equal(t, 2.0, armor.Weight, "ten implants still add only 0.5")
		}
	})

	t.Run("a drone hull is drones without any on the killmail, drones alone on another hull are not", func(t *testing.T) {
		assert.Contains(t, tagNames(tagFit(t, vexor, nil, nil, nil, nil).Tags()), "Drones")
		assert.NotContains(t, tagNames(tagFit(t, thorax, nil, nil, nil, map[int]int{warrior: 10}).Tags()), "Drones", "ten in the bay count as one kind")
	})

	t.Run("support drones nudge a role, but never make one", func(t *testing.T) {
		assert.NotContains(t, tagNames(tagFit(t, thorax, nil, nil, nil, map[int]int{hobgoblinSD: 5}).Tags()), "EWAR")
		assert.Contains(t, tagNames(tagFit(t, thorax, nil, []int{sensorDampener}, nil, map[int]int{hobgoblinSD: 5}).Tags()), "EWAR", "with one dampener")
		assert.NotContains(t, tagNames(tagFit(t, thorax, nil, nil, nil, map[int]int{acolyteEV: 5}).Tags()), "Neut")
	})
}
