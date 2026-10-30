package gui

import (
	"slices"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBuildFits(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	pilot := &retrieval.Pilot{CharacterID: 10, Name: "Luxxianna Seraph"}
	loss := func(id int, shipID int, shipName string, at time.Time) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: at,
			System:       &retrieval.System{Name: "Jita"},
			Victim: &retrieval.KillmailVictim{
				Pilot: pilot,
				Ship:  &retrieval.Item{ItemID: shipID, Name: shipName},
				Fit: &retrieval.Fit{
					Hull: &retrieval.Item{ItemID: shipID, Name: shipName},
					Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048, Name: "Damage Control II"}}},
				},
			},
		}
	}
	day := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	kill := loss(5, 587, "Rifter", day)
	kill.Victim.Pilot = &retrieval.Pilot{CharacterID: 99, Name: "Someone Else"}
	unfitted := loss(6, 587, "Rifter", day)
	unfitted.Victim.Fit.Lows = []*retrieval.FittedModule{nil, nil}

	// the same Rifter fit with another charge, a different module, and a Rifter whose modules are only in another order
	reloaded := loss(11, 587, "Rifter", day.Add(-time.Hour))
	reloaded.Victim.Fit.Highs = []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2873, Name: "200mm AutoCannon II"}, Charge: &retrieval.Item{ItemID: 12608, Name: "Hail S"}}, nil}
	loaded := loss(12, 587, "Rifter", day.Add(-2*time.Hour))
	loaded.Victim.Fit.Highs = []*retrieval.FittedModule{nil, {Module: &retrieval.Item{ItemID: 2873, Name: "200mm AutoCannon II"}, Charge: &retrieval.Item{ItemID: 12625, Name: "Barrage S"}}}
	refit := loss(13, 587, "Rifter", day)
	refit.Victim.Fit.Lows = []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 1541, Name: "Power Diagnostic System II"}}}
	hurricane := loss(3, 24690, "Hurricane", day)
	hurricane.Victim.CorpAtKill = &retrieval.Corporation{Name: "Sleuth Corp"}
	hurricane.Victim.AllianceAtKill = &retrieval.Alliance{Name: "Sleuth Alliance"}

	fits := buildFits([]*retrieval.Killmail{
		loss(1, 587, "Rifter", day),
		loss(2, 670, "Capsule", day),
		loss(7, 11129, "Gallente Shuttle", day),
		loss(8, 32880, "Venture", day),
		loss(9, 648, "Badger", day),
		loss(10, 35832, "Astrahus", day),
		unfitted,
		hurricane,
		loss(4, 587, "Rifter", day.Add(time.Hour)),
		kill,
		reloaded,
		loaded,
		refit,
	}, target, nil, nil)

	groups := [][]int{}
	for _, fit := range fits {
		ids := []int{}
		for _, loss := range fit.Losses {
			ids = append(ids, loss.KillmailID)
		}
		groups = append(groups, ids)
	}
	assert.Equal(t, [][]int{{3}, {4, 1}, {13}, {11, 12}}, groups, "identical fits merged newest first, by ship name then newest loss, without the kill, the unfitted loss, or the non-combat hulls")
	assert.Contains(t, fits[0].Losses[0].EFT, "[Hurricane, GoSleuth - Luxxianna Seraph]")
	assert.Equal(t, "Jita", fits[0].Losses[0].SystemName)
	assert.Equal(t, []string{"Sleuth Corp", "Sleuth Alliance"}, []string{fits[0].Losses[0].CorpName, fits[0].Losses[0].AllianceName}, "the victim's corp and alliance, for the filter")

	layout := fits[3].Losses[0].Fit
	assert.Equal(t, "Damage Control II", layout.Lows[0].Module.Name)
	assert.Equal(t, "Hail S", layout.Highs[0].Charge.Name, "each loss keeps its own charges")
	assert.Nil(t, layout.Highs[1], "an empty slot")
	assert.Equal(t, "Barrage S", fits[3].Losses[1].Fit.Highs[1].Charge.Name)
}

func TestBuildFitsMergesWhateverTheOrder(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	module := func(id int, name string) *retrieval.FittedModule {
		return &retrieval.FittedModule{Module: &retrieval.Item{ItemID: id, Name: name}}
	}
	scram, battery, mwd := module(448, "Warp Scrambler II"), module(41218, "Medium Cap Battery II"), module(12076, "50MN Microwarpdrive II")
	loss := func(id int, mids []*retrieval.FittedModule, drones []*retrieval.FitCargo, cargo []*retrieval.FitCargo) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: time.Date(2026, 10, 27, 0, 0, id, 0, time.UTC),
			Victim: &retrieval.KillmailVictim{
				Pilot: &retrieval.Pilot{CharacterID: 10, Name: "Luxxianna Seraph"},
				Ship:  &retrieval.Item{ItemID: 17843, Name: "Vexor Navy Issue"},
				Fit: &retrieval.Fit{
					Hull:   &retrieval.Item{ItemID: 17843, Name: "Vexor Navy Issue"},
					Mids:   mids,
					Drones: drones,
					Cargo:  cargo,
				},
			},
		}
	}
	hammerheads := []*retrieval.FitCargo{{Item: &retrieval.Item{ItemID: 2185, Name: "Hammerhead II"}, Quantity: 5}}
	paste := []*retrieval.FitCargo{{Item: &retrieval.Item{ItemID: 28668, Name: "Nanite Repair Paste"}, Quantity: 50}}
	abyssalScram := module(47732, "Abyssal Warp Scrambler")

	fits := buildFits([]*retrieval.Killmail{
		loss(1, []*retrieval.FittedModule{scram, battery, mwd}, nil, nil),
		loss(2, []*retrieval.FittedModule{mwd, nil, scram, battery}, hammerheads, nil),
		loss(3, []*retrieval.FittedModule{battery, mwd, scram}, nil, paste),
		loss(4, []*retrieval.FittedModule{abyssalScram, battery, mwd}, nil, nil),
	}, target, nil, nil)

	assert.Len(t, fits, 2, "the same mids in any order, with or without drones or cargo, are one fit, but an abyssal scram is not its T2")
	// the abyssal loss is the newest, so its fit comes first
	abyssal, plain := fits[0], fits[1]
	assert.Len(t, abyssal.Losses, 1)
	assert.True(t, abyssal.Abyssal)
	assert.Nil(t, abyssal.Summary)
	assert.Len(t, plain.Losses, 3)
	assert.False(t, plain.Abyssal)

	tagNames := func(tags []retrieval.FitTag) []string {
		names := []string{}
		for _, tag := range tags {
			names = append(names, tag.Name)
		}
		return names
	}
	assert.Equal(t, 2, plain.Losses[1].KillmailID)
	assert.Contains(t, tagNames(plain.Losses[1].Tags), "Close", "each loss has its own tags, from its own drones")
	assert.NotContains(t, tagNames(plain.Losses[2].Tags), "Close", "the first loss had none")
	assert.Contains(t, tagNames(plain.Tags), "Close", "the fit has every tag of its losses")
}

func TestBuildFitsMergesECMFlavors(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	loss := func(id int, mids ...*retrieval.Item) *retrieval.Killmail {
		fitted := []*retrieval.FittedModule{}
		for _, module := range mids {
			fitted = append(fitted, &retrieval.FittedModule{Module: module})
		}
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: time.Date(2026, 10, 27, 0, 0, id, 0, time.UTC),
			Victim: &retrieval.KillmailVictim{
				Pilot: &retrieval.Pilot{CharacterID: 10, Name: "Luxxianna Seraph"},
				Ship:  &retrieval.Item{ItemID: 11957, Name: "Falcon"},
				Fit:   &retrieval.Fit{Hull: &retrieval.Item{ItemID: 11957, Name: "Falcon"}, Mids: fitted},
			},
		}
	}
	ladar, radar := &retrieval.Item{ItemID: 2559, Name: "Ladar ECM II"}, &retrieval.Item{ItemID: 2575, Name: "Radar ECM II"}
	multispectral := &retrieval.Item{ItemID: 2567, Name: "Multispectral ECM II"}
	scram := &retrieval.Item{ItemID: 448, Name: "Warp Scrambler II"}

	fits := buildFits([]*retrieval.Killmail{
		loss(1, ladar, ladar, scram),
		loss(2, radar, multispectral, scram),
		loss(3, ladar, scram, scram),
	}, target, nil, nil)

	assert.Len(t, fits, 2, "any two ECM and a scram are one fit, whatever their flavors; two scrams and one ECM another")
	assert.Len(t, fits[1].Losses, 2, "the newer fit, two scrams, comes first")
	assert.Equal(t, "Radar ECM II", fits[1].Losses[0].Fit.Mids[0].Module.Name, "each loss keeps its own flavors")
}

func TestBuildFitsGivesShipsTheirPodsImplants(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	pilot := &retrieval.Pilot{CharacterID: 10, Name: "Luxxianna Seraph"}
	jita, amarr := &retrieval.System{SystemID: 30000142, Name: "Jita"}, &retrieval.System{SystemID: 30002187, Name: "Amarr"}
	start := time.Date(2026, 10, 27, 20, 0, 0, 0, time.UTC)
	ship := func(id int, at time.Time, system *retrieval.System) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID: id, KillmailTime: at, System: system,
			Victim: &retrieval.KillmailVictim{
				Pilot: pilot,
				Ship:  &retrieval.Item{ItemID: 587, Name: "Rifter"},
				Fit: &retrieval.Fit{
					Hull: &retrieval.Item{ItemID: 587, Name: "Rifter"},
					Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048, Name: "Damage Control II"}}},
				},
			},
		}
	}
	pod := func(id int, at time.Time, system *retrieval.System, implant string) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID: id, KillmailTime: at, System: system,
			Victim: &retrieval.KillmailVictim{
				Pilot: pilot,
				Ship:  &retrieval.Item{ItemID: 670, Name: "Capsule"},
				Fit: &retrieval.Fit{
					Hull:     &retrieval.Item{ItemID: 670, Name: "Capsule"},
					Implants: []*retrieval.Item{{ItemID: 1, Name: implant}},
				},
			},
		}
	}

	// the battles the killmails were collated into; 6 is on its own
	battles := map[int]string{1: "a", 2: "a", 3: "b", 4: "c", 5: "d", 7: "e", 8: "e", 9: "f", 10: "f", 11: "f", 12: "g", 13: "g"}
	fits := buildFits([]*retrieval.Killmail{
		ship(1, start, jita),
		pod(2, start.Add(30*time.Second), jita, "Ocular Filter"), // the ship just before it
		ship(3, start.Add(time.Hour), jita),
		pod(4, start.Add(time.Hour+5*time.Minute), jita, "Another battle"),
		ship(5, start.Add(2*time.Hour), jita),
		pod(6, start.Add(2*time.Hour+time.Minute), jita, "No battle"),
		pod(7, start.Add(3*time.Hour-time.Minute), jita, "Too early"), // before the ship
		ship(8, start.Add(3*time.Hour), jita),
		ship(9, start.Add(4*time.Hour), jita),
		ship(10, start.Add(4*time.Hour+time.Minute), jita),
		pod(11, start.Add(4*time.Hour+2*time.Minute), jita, "Latest ship"), // the later of two ships
		ship(12, start.Add(5*time.Hour), jita),
		pod(13, start.Add(5*time.Hour+40*time.Minute), amarr, "Long lived"), // the battle moved on, the capsule with it
	}, target, battles, nil)

	assert.Len(t, fits, 1, "the capsules are not fits of their own")
	implants := map[int][]string{}
	pods := map[int]int{}
	for _, loss := range fits[0].Losses {
		for _, implant := range loss.Fit.Implants {
			implants[loss.KillmailID] = append(implants[loss.KillmailID], implant.Name)
		}
		if loss.PodKillmailID != 0 {
			pods[loss.KillmailID] = loss.PodKillmailID
		}
	}
	assert.Equal(t, map[int][]string{1: {"Ocular Filter"}, 10: {"Latest ship"}, 12: {"Long lived"}}, implants)
	order := []int{}
	for _, loss := range fits[0].Losses {
		order = append(order, loss.KillmailID)
	}
	assert.Equal(t, []int{12, 10, 1, 9, 8, 5, 3}, order, "the losses with implants first, then newest")
	assert.Equal(t, map[int]int{1: 2, 10: 11, 12: 13}, pods)
	assert.True(t, fits[0].Implants)

	// implants by slot: Ocular Filter - Basic (1) before Memory Augmentation - Basic (2) though listed after
	slotted := pod(31, start.Add(7*time.Hour+time.Minute), jita, "")
	slotted.Victim.Fit.Implants = []*retrieval.Item{{ItemID: 9941, Name: "Memory Augmentation - Basic"}, {ItemID: 9899, Name: "Ocular Filter - Basic"}}
	slottedShip := ship(30, start.Add(7*time.Hour), jita)
	fits = buildFits([]*retrieval.Killmail{slottedShip, slotted}, target, map[int]string{30: "h", 31: "h"}, nil)
	assert.Equal(t, "Ocular Filter - Basic", fits[0].Losses[0].Fit.Implants[0].Name)
	assert.Equal(t, "Memory Augmentation - Basic", fits[0].Losses[0].Fit.Implants[1].Name)
	assert.Contains(t, fits[0].Losses[0].EFT, "\n\n\nMemory Augmentation - Basic x1\nOcular Filter - Basic x1", "the capsule's implants are exported in the ship's cargo")
	assert.Empty(t, slottedShip.Victim.Fit.Implants, "the ship's own fit is unchanged")

	// a newer fit of the same hull without implants still lists after it
	refit := ship(20, start.Add(6*time.Hour), jita)
	refit.Victim.Fit.Lows = []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 1541, Name: "Power Diagnostic System II"}}}
	fits = buildFits([]*retrieval.Killmail{ship(1, start, jita), pod(2, start.Add(30*time.Second), jita, "Ocular Filter"), refit}, target, battles, nil)
	assert.Len(t, fits, 2)
	assert.Equal(t, []bool{true, false}, []bool{fits[0].Implants, fits[1].Implants}, "the fit with implants first, though older")

	// two fits with implants list by their newest loss, not their first, which is the one with implants
	refit = ship(21, start.Add(5*time.Hour), jita)
	refit.Victim.Fit.Lows = []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 1541, Name: "Power Diagnostic System II"}}}
	fits = buildFits([]*retrieval.Killmail{
		ship(1, start, jita), pod(2, start.Add(30*time.Second), jita, "Ocular Filter"), ship(3, start.Add(10*time.Hour), jita),
		refit, pod(22, start.Add(5*time.Hour+30*time.Second), jita, "Ocular Filter"),
	}, target, map[int]string{1: "a", 2: "a", 21: "b", 22: "b"}, nil)
	assert.Len(t, fits, 2)
	assert.Equal(t, []int{1, 3}, []int{fits[0].Losses[0].KillmailID, fits[0].Losses[1].KillmailID}, "lost last at 3, so listed before the refit lost at 21")
	assert.Equal(t, 21, fits[1].Losses[0].KillmailID)
	assert.Equal(t, start.Add(10*time.Hour), fits[0].LastLost, "its newest loss, not its first")
}

func TestBuildFitsFleetDoctrine(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const alliance = 99000001
	target := &retrieval.IntelTarget{IDNumber: alliance, TypeOfEntity: types.ALLIANCE}
	start := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	ferox := &retrieval.Item{ItemID: 16227, Name: "Ferox"}
	loss := func(id int, pilot int) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: start.Add(time.Duration(id) * time.Hour),
			Victim: &retrieval.KillmailVictim{
				Pilot:          &retrieval.Pilot{CharacterID: pilot},
				AllianceAtKill: &retrieval.Alliance{AllianceID: alliance},
				Ship:           ferox,
				Fit:            &retrieval.Fit{Hull: ferox, Mids: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 3841, Name: "Large Shield Extender II"}}}},
			},
		}
	}
	// the first in a battle with four other Feroxes and an Osprey on its side
	inBattle := loss(1, 1)
	escort := &retrieval.Killmail{
		KillmailID: 100, KillmailTime: start,
		Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: 900}, Ship: &retrieval.Item{ItemID: 587, Name: "Rifter"}},
	}
	for pilot, shipID := range map[int]int{11: 16227, 12: 16227, 13: 16227, 14: 16227, 15: 620} {
		escort.Attackers = append(escort.Attackers, &retrieval.KillmailAttacker{
			Pilot: &retrieval.Pilot{CharacterID: pilot}, Ship: &retrieval.Item{ItemID: shipID}, AllianceAtKill: &retrieval.Alliance{AllianceID: alliance},
		})
	}
	battles := map[int]string{1: "b", 100: "b"}

	t.Run("a loss in a doctrine fleet gives its fit the tag", func(t *testing.T) {
		fits := buildFits([]*retrieval.Killmail{inBattle, escort, loss(2, 2)}, target, battles, nil)

		assert.Equal(t, "Fleet Doctrine", fits[0].Tags[0].Name)
		named := func(loss FitLoss) bool {
			return slices.ContainsFunc(loss.Tags, func(tag retrieval.FitTag) bool { return tag.Name == "Fleet Doctrine" })
		}
		assert.Equal(t, []bool{false, true}, []bool{named(fits[0].Losses[0]), named(fits[0].Losses[1])}, "only the loss in that battle")
	})

	t.Run("lost again and again, every loss has it, after its battle's reasons", func(t *testing.T) {
		fits := buildFits([]*retrieval.Killmail{inBattle, escort, loss(2, 2), loss(3, 3), loss(4, 4), loss(5, 5)}, target, battles, nil)

		for _, each := range fits[0].Losses {
			assert.Equal(t, "Fleet Doctrine", each.Tags[0].Name, "loss %d", each.KillmailID)
		}
		first := fits[0].Losses[len(fits[0].Losses)-1]
		assert.Equal(t, 1, first.KillmailID)
		reasons := []string{}
		for _, reason := range first.Tags[0].Reasons {
			reasons = append(reasons, reason.Name)
		}
		assert.Equal(t, []string{"Ferox in its battle", "Logi in its battle", "Losses of this fit in one alliance", "Pilots who lost it"}, reasons)
	})
}

func TestBuildFitsAffiliateTag(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	const main, affiliate = 99000001, 99000002
	target := &retrieval.IntelTarget{IDNumber: main, TypeOfEntity: types.ALLIANCE, Holdings: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: affiliate, Name: "Singularity Syndicate"}}}
	nighthawk := &retrieval.Item{ItemID: 22470, Name: "Nighthawk"}
	loss := func(id int, allianceID int) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: time.Date(2026, 8, 12, 2, id, 0, 0, time.UTC),
			Victim: &retrieval.KillmailVictim{
				Pilot:          &retrieval.Pilot{CharacterID: id},
				AllianceAtKill: &retrieval.Alliance{AllianceID: allianceID},
				Ship:           nighthawk,
				Fit:            &retrieval.Fit{Hull: nighthawk, Lows: []*retrieval.FittedModule{{Module: &retrieval.Item{ItemID: 2048, Name: "Damage Control II"}}}},
			},
		}
	}

	fits := buildFits([]*retrieval.Killmail{loss(1, main), loss(2, affiliate)}, target, nil, nil)

	affiliateTags := map[int]*retrieval.FitTag{}
	for _, each := range fits[0].Losses {
		at := slices.IndexFunc(each.Tags, func(tag retrieval.FitTag) bool { return tag.Name == "Affiliate" })
		if at >= 0 {
			affiliateTags[each.KillmailID] = &each.Tags[at]
		}
	}
	assert.NotContains(t, affiliateTags, 1, "the target's own loss")
	if assert.Contains(t, affiliateTags, 2, "the affiliate's loss") {
		assert.True(t, affiliateTags[2].Hidden, "hidden like the weapon tags")
		assert.Contains(t, affiliateTags[2].Meaning, "Singularity Syndicate")
	}
}

func TestFitKeyMergesVariants(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	fit := func(lows ...int) *retrieval.Fit {
		modules := []*retrieval.FittedModule{}
		for _, id := range lows {
			modules = append(modules, &retrieval.FittedModule{Module: &retrieval.Item{ItemID: id}})
		}
		return &retrieval.Fit{Lows: modules}
	}
	assert.Equal(t, fitKey(1, fit(41058)), fitKey(1, fit(41054)), "grappler variants are one module")
	assert.Equal(t, fitKey(1, fit(41057)), fitKey(1, fit(41058)))
	assert.Equal(t, fitKey(1, fit(25890)), fitKey(1, fit(26290)), "reinforcer I and II are one module")
	assert.NotEqual(t, fitKey(1, fit(18710)), fitKey(1, fit(18809)), "different base types do not merge")
	assert.NotEqual(t, fitKeyOf(1, fit(41058), true), fitKeyOf(1, fit(41054), true), "the exact key keeps variants apart")
	assert.Equal(t, fitKey(1, fit(2559)), fitKey(1, fit(2575)), "ECM flavors still merge")
}

func TestBuildFitsMergesVariantsWithSummary(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	target := &retrieval.IntelTarget{IDNumber: 10, TypeOfEntity: types.PILOT}
	loss := func(id, module int) *retrieval.Killmail {
		return &retrieval.Killmail{
			KillmailID:   id,
			KillmailTime: time.Date(2026, 10, 27, 0, 0, id, 0, time.UTC),
			Victim: &retrieval.KillmailVictim{
				Pilot: &retrieval.Pilot{CharacterID: 10, Name: "Pilot"},
				Ship:  &retrieval.Item{ItemID: 11957, Name: "Falcon"},
				Fit: &retrieval.Fit{Hull: &retrieval.Item{ItemID: 11957, Name: "Falcon"}, Mids: []*retrieval.FittedModule{
					{Module: &retrieval.Item{ItemID: module, Name: "Grappler"}},
				}},
			},
		}
	}
	fits := buildFits([]*retrieval.Killmail{loss(1, 41058), loss(2, 41054)}, target, nil, nil)
	assert.Len(t, fits, 1)
	assert.Len(t, fits[0].Losses, 2)
	if assert.NotNil(t, fits[0].Summary) {
		assert.Equal(t, 41040, fits[0].Summary.Fit.Mids[0].Module.ID)
		assert.Len(t, fits[0].Summary.Fit.Mids[0].Variants, 2)
	}
}
