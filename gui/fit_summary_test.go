package gui

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func summaryLoss(pilot string, mids []int, stats ...FitStats) FitLoss {
	slots := []*FitSlot{}
	for _, id := range mids {
		if id == 0 {
			slots = append(slots, nil)
			continue
		}
		name := map[int]string{41058: "Shadow Serpentis Heavy Stasis Grappler", 41054: "Heavy Gunnar Compact Stasis Grappler", 41057: "Heavy Stasis Grappler II", 448: "Warp Scrambler II"}[id]
		slots = append(slots, &FitSlot{Module: FitPart{ID: id, Name: name}})
	}
	return FitLoss{PilotName: pilot, Fit: FitLayout{Mids: slots}, Stats: stats}
}

func TestBuildSummary(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	assert.Nil(t, buildSummary([]FitLoss{summaryLoss("A", []int{41058})}), "one loss has no summary")

	losses := []FitLoss{
		summaryLoss("A", []int{0, 41058, 448}),
		summaryLoss("B", []int{448, 41054, 0}),
		summaryLoss("A", []int{448, 41054, 0}),
		summaryLoss("C", []int{448, 41057, 0}),
	}
	summary := buildSummary(losses)
	assert.NotNil(t, summary)
	assert.Equal(t, 3, summary.Pilots)
	assert.Equal(t, 1, summary.CommonLoss, "two losses share the Gunnar fit, the first of them is index 1")
	assert.Len(t, summary.Fit.Mids, 3, "the rack keeps the common loss's length")
	assert.NotNil(t, summary.Fit.Mids[0])
	assert.NotNil(t, summary.Fit.Mids[1])
	assert.Nil(t, summary.Fit.Mids[2], "empty slots last")
	for _, slot := range summary.Fit.Mids[:2] {
		if slot.Module.ID == 41040 {
			assert.Len(t, slot.Variants, 3)
			assert.Equal(t, 41054, slot.Variants[0].ID, "most losses first")
			assert.Equal(t, 2, slot.Variants[0].Losses)
			assert.Equal(t, 1, slot.Variants[1].Losses)
		} else {
			assert.Equal(t, 447, slot.Module.ID, "Warp Scrambler I is the base of the II")
			assert.Equal(t, 4, slot.Variants[0].Losses)
		}
	}
	assert.Empty(t, summary.Fit.Boosters)
	assert.NotNil(t, summary.Fit.Boosters)
	assert.NotNil(t, summary.Fit.Implants)
	assert.Empty(t, summary.Fit.Implants)
	assert.Equal(t, 0, summary.Implanted)
}

func TestBuildSummaryImplants(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	// a slot 2 and a slot 1 implant, given out of slot order
	snake := FitPart{ID: 19540, Name: "High-grade Snake Alpha"}
	beta := FitPart{ID: 19551, Name: "High-grade Snake Beta"}
	withImplants := func(pilot string, implants ...FitPart) FitLoss {
		loss := summaryLoss(pilot, []int{448})
		loss.Fit.Implants = implants
		return loss
	}
	summary := buildSummary([]FitLoss{withImplants("A", beta, snake), withImplants("B", snake), withImplants("C")})
	if assert.NotNil(t, summary) && assert.Len(t, summary.Fit.Implants, 2) {
		assert.Equal(t, 2, summary.Implanted, "two of the three losses had implants")
		assert.Equal(t, "High-grade Snake Alpha", summary.Fit.Implants[0].Name, "in slot order, alpha first")
		assert.Equal(t, 2, summary.Fit.Implants[0].Quantity, "on two losses")
		assert.Equal(t, 1, summary.Fit.Implants[1].Quantity)
	}
}

func TestBuildSummaryTies(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	losses := []FitLoss{
		summaryLoss("A", []int{41058}),
		summaryLoss("B", []int{41054}),
	}
	summary := buildSummary(losses)
	assert.Equal(t, 0, summary.CommonLoss, "a tie goes to the earliest")
	assert.Equal(t, 41058, summary.Fit.Mids[0].Variants[0].ID, "a tie goes to the common loss's variant")
	assert.Equal(t, 41054, summary.Fit.Mids[0].Variants[1].ID)
}

func TestBuildSummaryStats(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	stats := func(ehp, strength float64, boosted bool, resist float64) *fitting.Stats {
		return &fitting.Stats{
			EHP: ehp, Boosted: boosted,
			Shield:     fitting.Layer{HP: ehp / 2, Resonance: [4]float64{0.5, 0.6, 0.7, 0.8}},
			Sensor:     fitting.Sensor{Type: "Radar", Strength: strength},
			EwarResist: map[string]float64{"ECMResistance": resist},
		}
	}
	first := summaryLoss("A", []int{448}, FitStats{Mode: "Defense", Stats: stats(100, 20, false, 0.5)}, FitStats{Mode: "Sharpshooter", Stats: stats(50, 30, false, 0.1)})
	second := summaryLoss("B", []int{448}, FitStats{Mode: "Defense", Stats: stats(300, 10, true, 0.2)}, FitStats{Mode: "Sharpshooter", Stats: stats(70, 5, false, 0.3)})
	none := summaryLoss("C", []int{448})
	summary := buildSummary([]FitLoss{first, none, second})

	assert.Len(t, summary.Stats, 2)
	defense := summary.Stats[0]
	assert.Equal(t, "Defense", defense.Mode)
	assert.Equal(t, 100.0, defense.Min.EHP)
	assert.Equal(t, 300.0, defense.Max.EHP)
	assert.Equal(t, 50.0, defense.Min.Shield.HP)
	assert.Equal(t, 150.0, defense.Max.Shield.HP)
	assert.Equal(t, 0.5, defense.Min.Shield.Resonance[0])
	assert.Equal(t, 10.0, defense.Min.Sensor.Strength)
	assert.Equal(t, 20.0, defense.Max.Sensor.Strength)
	assert.Equal(t, 0.2, defense.Min.EwarResist["ECMResistance"])
	assert.Equal(t, 0.5, defense.Max.EwarResist["ECMResistance"])
	assert.Equal(t, "Radar", defense.Min.Sensor.Type)
	assert.True(t, defense.Min.Boosted && defense.Max.Boosted)
	assert.Equal(t, "Sharpshooter", summary.Stats[1].Mode)
	assert.Equal(t, 70.0, summary.Stats[1].Max.EHP)

	assert.Equal(t, 100.0, first.Stats[0].EHP, "the losses' own stats are untouched")
	assert.False(t, first.Stats[0].Boosted)

	assert.Empty(t, buildSummary([]FitLoss{none, none}).Stats)
}

func TestBuildSummaryMissingStats(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	none := buildSummary([]FitLoss{summaryLoss("A", []int{448}), summaryLoss("B", []int{448})})
	if assert.NotNil(t, none) {
		assert.Empty(t, none.Stats, "no loss has stats, so the summary has none")
	}

	stats := func(ehp float64, sensor string) *fitting.Stats {
		return &fitting.Stats{EHP: ehp, Sensor: fitting.Sensor{Type: sensor, Strength: 10}}
	}
	// the first loss is the common one (a tie goes to the earliest) but has no stats, so the others give the range and the sensor type
	summary := buildSummary([]FitLoss{
		summaryLoss("A", []int{448}),
		summaryLoss("B", []int{448}, FitStats{Stats: stats(100, "Ladar")}),
		summaryLoss("C", []int{448}, FitStats{Stats: stats(300, "Ladar")}),
	})
	if assert.NotNil(t, summary) && assert.Len(t, summary.Stats, 1) {
		assert.Equal(t, 0, summary.CommonLoss)
		assert.InDelta(t, 100, summary.Stats[0].Min.EHP, 1e-9)
		assert.InDelta(t, 300, summary.Stats[0].Max.EHP, 1e-9)
		assert.Equal(t, "Ladar", summary.Stats[0].Min.Sensor.Type)
	}
}

func TestSummaryEFT(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	scram := &retrieval.FittedModule{Module: &retrieval.Item{ItemID: 448, Name: "Warp Scrambler II"}}
	alpha := &retrieval.Item{ItemID: 19540, Name: "High-grade Snake Alpha"}
	beta := &retrieval.Item{ItemID: 19551, Name: "High-grade Snake Beta"}
	fit := func(implants ...*retrieval.Item) *retrieval.Fit {
		return &retrieval.Fit{Hull: &retrieval.Item{ItemID: 587, Name: "Rifter"}, Mids: []*retrieval.FittedModule{scram}, Implants: implants}
	}
	fitOf := map[int]*retrieval.Fit{1: fit(alpha, beta), 2: fit(), 3: fit(alpha)}
	titleOf := map[int]string{1: "[Rifter, A]", 2: "[Rifter, B]", 3: "[Rifter, C]"}
	losses := []FitLoss{{KillmailID: 1, EFT: "one"}, {KillmailID: 2, EFT: "two"}, {KillmailID: 3, EFT: "three"}}

	eft := summaryEFT(losses, 1, fitOf, titleOf)
	assert.Contains(t, eft, "[Rifter, B]", "the common loss's fit and title")
	assert.Contains(t, eft, "High-grade Snake Alpha", "with the largest implant set, which it lacked")
	assert.Contains(t, eft, "High-grade Snake Beta")
	assert.Empty(t, fitOf[2].Implants, "the loss's own fit is left alone")

	assert.Equal(t, "one", summaryEFT(losses, 0, fitOf, titleOf), "the common loss with the largest set keeps its own EFT")
	fitOf[2] = fit(alpha, beta)
	assert.Equal(t, "two", summaryEFT(losses, 1, fitOf, titleOf), "as does one with a set as large")
}
