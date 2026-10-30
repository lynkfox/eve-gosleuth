package gui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// FitGroup is one fit the target lost, with every loss of it merged: the same hull with the same base types of modules in each rack (a variant of a module is the same module), whatever the charges, drones, or cargo
type FitGroup struct {
	ShipID        int       `json:"shipId"`
	ShipName      string    `json:"shipName"`
	ShipImageURL  string    `json:"shipImageUrl"`
	ShipRenderURL string    `json:"shipRenderUrl"`
	ShipEveRefURL string    `json:"shipEveRefUrl"`
	Abyssal       bool      `json:"abyssal"`  // it has a mutated module, whose rolled stats are unknown
	Implants      bool      `json:"implants"` // a loss of it has the implants of its capsule (see podsWithShips). Implants don't decide which fits merge.
	Losses        []FitLoss `json:"losses"`   // most implants first, then newest
	LastLost      time.Time `json:"lastLost"` // its newest loss, which need not be its first

	Tags []retrieval.FitTag `json:"tags"` // every tag any of its losses has (see FitLoss.Tags), so its row shows why the filter matched it

	Summary *FitSummary `json:"summary,omitempty"` // its losses side by side, nil for a fit lost once (see buildSummary)
}

// FitSummary is a merged fit's losses side by side, the fitting window's first page and the PDF's doctrine block: each slot's base type with the variants its losses fitted, and each number's range
type FitSummary struct {
	Fit        FitLayout       `json:"fit"`             // each slot's base type (comm.BaseType) and its Variants, each rack sorted by base type name; no charges; the drones and fighters of the CommonLoss; every implant any loss had, in slot order, its Quantity how many losses had it (summaryImplants); no boosters
	Implanted  int             `json:"implanted"`       // how many of its losses had implants
	EFT        string          `json:"eft"`             // the PDF's EFT: the CommonLoss's, with the largest implant set any loss had (summaryEFT)
	Pilots     int             `json:"pilots"`          // the different pilots who lost it
	CommonLoss int             `json:"commonLoss"`      // the index in Losses of the most common exact fit (its real module types, fit_merged_groups still applied), ties to the earliest in Losses; its EFT is the summary's
	Stats      []FitStatsRange `json:"stats,omitempty"` // each mode's range across the losses with stats, in the order of their Stats, empty when none has any
}

// FitStatsRange is each number's lowest and highest across a fit's losses in one mode. Each number is taken alone, so neither end need be one real loss.
type FitStatsRange struct {
	Mode string `json:"mode,omitempty"`
	// every number its lowest (Min) and highest (Max), each ewar resistance by its key; the rest (Sensor.Type, Assumptions, Unknown) from the CommonLoss, and Boosted when any loss is
	Min *fitting.Stats `json:"min"`
	Max *fitting.Stats `json:"max"`
}

// FitVariant is a real module seen in a summary slot, ie the Heavy Gunnar Compact Stasis Grappler in a Heavy Stasis Grappler I slot
type FitVariant struct {
	FitPart
	Losses int `json:"losses"` // how many losses fitted it in this rack among the modules of its base type
}

// FitLoss is one loss of a fit, with its own charges and cargo
type FitLoss struct {
	KillmailID   int                `json:"killmailId"`
	URL          string             `json:"url"` // the killmail on zKillboard
	Time         time.Time          `json:"time"`
	PilotName    string             `json:"pilotName"`          // the pilot, or the owning corp for a structure
	CorpName     string             `json:"corpName,omitempty"` // the victim's corp and alliance at the time, for the filter box
	AllianceName string             `json:"allianceName,omitempty"`
	SystemName   string             `json:"systemName"`
	EFT          string             `json:"eft"`
	Tags         []retrieval.FitTag `json:"tags"`         // read from its modules and drones (Fit.Tags). Drones don't decide which fits merge, so a fit's losses can differ.
	MissingTypes int                `json:"missingTypes"` // item types not in fit_types.json, so the fit may be incomplete
	Fit          FitLayout          `json:"fit"`

	// the capsule the pilot lost with implants later in the battle, see podsWithShips. Its implants are the fit's.
	PodKillmailID int    `json:"podKillmailId,omitempty"`
	PodURL        string `json:"podUrl,omitempty"`

	// what the dogma engine works out for it (see fitStats): one entry, or one per mode for a tactical destroyer, Defense first. Empty for a structure, or a hull dogma.json doesn't have.
	Stats []FitStats `json:"stats,omitempty"`
}

// FitStats are a fit's numbers in one tactical destroyer mode, or its only numbers for any other hull
type FitStats struct {
	Mode string `json:"mode,omitempty"` // ie Defense, empty for a hull without modes
	*fitting.Stats
}

// FitLayout is a fit as the fitting window draws it. A nil slot is an empty one.
type FitLayout struct {
	Highs      []*FitSlot `json:"highs"`
	Mids       []*FitSlot `json:"mids"`
	Lows       []*FitSlot `json:"lows"`
	Rigs       []*FitSlot `json:"rigs"`
	Subsystems []*FitSlot `json:"subsystems"`
	Drones     []FitPart  `json:"drones"`
	Fighters   []FitPart  `json:"fighters"`
	Boosters   []FitPart  `json:"boosters"`
	Implants   []FitPart  `json:"implants"`
}

// FitSlot is a fitted module and its loaded charge
type FitSlot struct {
	Module   FitPart      `json:"module"`
	Charge   *FitPart     `json:"charge"`             // nil if unloaded
	Variants []FitVariant `json:"variants,omitempty"` // only in a FitSummary: the real modules its losses fitted, most losses first, ties to the CommonLoss's, then by name. Every slot of one base type in a rack has the same list.
}

// FitPart is an item type in a fit, with how many for a stack
type FitPart struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	EveRefURL string `json:"eveRefUrl"`
	Quantity  int    `json:"quantity,omitempty"`
}

/*
	buildFits lists the target's losses with their fits, identical fits merged (see fitKey), by ship name, those with implants first, and then newest loss first.
	A fit's own losses are the one with the most implants first, then newest, so the fitting window opens on its first.
	Non-combat hulls in ships.json's fit_ignored lists (capsules, shuttles, corvettes, haulers, mining ships, structures, deployables) are left out, as is any loss with nothing fitted.
	A capsule lost with implants after a ship in the same battle gives that ship's loss its implants (see podsWithShips).

Parameters:
  - killmails: the search's expanded killmails
  - target: the pilot, corp, or alliance searched
  - battleOf: each killmail in a battle to its battle's id, see aggregate.CollectedData.BattleOfKillmail
  - esiClient: looks up the NPC ships on each loss, whose rats count toward Ratting (aggregate.PvENPCs)

Returns:
  - []FitGroup: the fits, empty if there are none
*/
func buildFits(killmails []*retrieval.Killmail, target *retrieval.IntelTarget, battleOf map[int]string, esiClient api.ESIClient) []FitGroup {
	ships, pods := []*retrieval.Killmail{}, []*retrieval.Killmail{}
	for _, km := range killmails {
		if km.Victim == nil || km.Victim.Fit == nil || km.Victim.Ship == nil || !aggregate.IsTargetLoss(target, km) {
			continue
		}
		switch {
		// only a capsule holds implants, so an ignored hull with them is one
		case comm.FitHullIgnored(comm.FitTypeOf(km.Victim.Ship.ItemID)):
			if len(km.Victim.Fit.Implants) > 0 && km.Victim.Pilot != nil {
				pods = append(pods, km)
			}
		case km.Victim.Fit.HasModules():
			ships = append(ships, km)
		}
	}
	podOf := podsWithShips(ships, pods, battleOf)
	doctrineIn := aggregate.FleetDoctrineInBattle(killmails, target, battleOf)

	// each loss's fit (with its capsule's implants) and EFT title, for the summary's EFT
	fitOf := map[int]*retrieval.Fit{}
	titleOf := map[int]string{}
	groups := []*FitGroup{}
	byKey := map[string]*FitGroup{}
	lossesOf := map[*FitGroup][]*retrieval.Killmail{}
	for _, km := range ships {
		loss := FitLoss{
			KillmailID:   km.KillmailID,
			URL:          api.ZkillKillmailURL(km.KillmailID),
			Time:         km.KillmailTime,
			MissingTypes: len(km.Victim.Fit.MissingTypes),
			Fit:          fitLayout(km.Victim.Fit),
		}
		switch {
		case km.Victim.Pilot != nil:
			loss.PilotName = km.Victim.Pilot.Name
		case km.Victim.CorpAtKill != nil:
			loss.PilotName = km.Victim.CorpAtKill.Name
		}
		if km.Victim.CorpAtKill != nil {
			loss.CorpName = km.Victim.CorpAtKill.Name
		}
		if km.Victim.AllianceAtKill != nil {
			loss.AllianceName = km.Victim.AllianceAtKill.Name
		}
		if km.System != nil {
			loss.SystemName = km.System.Name
		}
		fit := km.Victim.Fit
		if pod := podOf[km.KillmailID]; pod != nil {
			loss.PodKillmailID = pod.KillmailID
			loss.PodURL = api.ZkillKillmailURL(pod.KillmailID)
			loss.Fit.Implants = bySlot(pod.Victim.Fit.Implants)
			// a copy, so the export carries the capsule's implants without changing the killmail's own fit
			withImplants := *fit
			withImplants.Implants = pod.Victim.Fit.Implants
			fit = &withImplants
		}
		loss.EFT = fit.EFT(retrieval.EFTTitle(km))
		fitOf[km.KillmailID], titleOf[km.KillmailID] = fit, retrieval.EFTTitle(km)
		if !aggregate.IsStructure(km.Victim.Ship) { // a structure's numbers wait on security-scaled rigs and services
			loss.Stats = fitStats(fit, hasAbyssal(fit))
		}
		loss.Tags = fit.TagsWith(retrieval.FitTagContext{NPCs: aggregate.PvENPCs(km.Attackers, esiClient)})
		if doctrine, found := doctrineIn[km.KillmailID]; found {
			loss.Tags = retrieval.MergeFitTags(loss.Tags, []retrieval.FitTag{doctrine})
		}
		if affiliate := affiliateOf(target, km); affiliate != nil {
			loss.Tags = retrieval.MergeFitTags(loss.Tags, []retrieval.FitTag{affiliateTag(*affiliate)})
		}

		key := fitKey(km.Victim.Ship.ItemID, km.Victim.Fit)
		group := byKey[key]
		if group == nil {
			group = &FitGroup{
				ShipID:        km.Victim.Ship.ItemID,
				ShipName:      km.Victim.Ship.Name,
				ShipImageURL:  api.TypeImageURL(km.Victim.Ship.ItemID),
				ShipRenderURL: api.TypeRenderURLAt(km.Victim.Ship.ItemID, 512),
				ShipEveRefURL: api.EveRefTypeURL(km.Victim.Ship.ItemID),
			}
			byKey[key] = group
			groups = append(groups, group)
		}
		group.Losses = append(group.Losses, loss)
		lossesOf[group] = append(lossesOf[group], km)
		group.Implants = group.Implants || loss.PodKillmailID != 0
		group.Abyssal = group.Abyssal || hasAbyssal(km.Victim.Fit)
	}

	fits := make([]FitGroup, 0, len(groups))
	for _, group := range groups {
		// lost again and again makes every loss of it a doctrine, its reasons after those of its battle
		if repeated, found := aggregate.FleetDoctrineRepeated(lossesOf[group], target); found {
			for index := range group.Losses {
				group.Losses[index].Tags = withDoctrineReasons(group.Losses[index].Tags, repeated)
			}
		}
		slices.SortStableFunc(group.Losses, func(a, b FitLoss) int {
			if byImplants := cmp.Compare(len(b.Fit.Implants), len(a.Fit.Implants)); byImplants != 0 {
				return byImplants
			}
			return b.Time.Compare(a.Time)
		})
		lossTags := make([][]retrieval.FitTag, 0, len(group.Losses))
		for _, loss := range group.Losses {
			lossTags = append(lossTags, loss.Tags)
		}
		group.Tags = retrieval.MergeFitTags(lossTags...)
		group.Summary = buildSummary(group.Losses)
		if group.Summary != nil {
			group.Summary.EFT = summaryEFT(group.Losses, group.Summary.CommonLoss, fitOf, titleOf)
		}
		for _, loss := range group.Losses {
			if loss.Time.After(group.LastLost) {
				group.LastLost = loss.Time
			}
		}
		fits = append(fits, *group)
	}
	slices.SortStableFunc(fits, func(a, b FitGroup) int {
		if byName := strings.Compare(a.ShipName, b.ShipName); byName != 0 {
			return byName
		}
		if a.Implants != b.Implants {
			if a.Implants {
				return -1
			}
			return 1
		}
		return b.LastLost.Compare(a.LastLost)
	})
	return fits
}

/*
	summaryEFT is a merged fit's EFT for the PDF: the CommonLoss's fit, with the largest set of implants any of its losses had in place of its own when it has fewer (user's call 2026-10-30), so the doctrine's EFT always shows what its pilots plug in

Parameters:
  - losses: the fit's losses, most implants first (buildFits), so the first has the largest set
  - common: the index of the CommonLoss
  - fitOf: each loss's fit by killmail id, with its capsule's implants
  - titleOf: each loss's EFT title by killmail id

Returns:
  - string: the EFT text, the CommonLoss's own when it already has the largest set
*/
func summaryEFT(losses []FitLoss, common int, fitOf map[int]*retrieval.Fit, titleOf map[int]string) string {
	own := losses[common]
	fullest := fitOf[losses[0].KillmailID]
	fit := fitOf[own.KillmailID]
	if fit == nil || fullest == nil || len(fullest.Implants) <= len(fit.Implants) {
		return own.EFT
	}
	// a copy, so the loss's own fit keeps its own implants
	withImplants := *fit
	withImplants.Implants = fullest.Implants
	return withImplants.EFT(titleOf[own.KillmailID])
}

// affiliateOf is the affiliate (holding, in the code) whose pilot lost a killmail, nil when the target itself lost it
func affiliateOf(target *retrieval.IntelTarget, km *retrieval.Killmail) *retrieval.TargetGroup {
	corpID, allianceID := 0, 0
	if km.Victim.CorpAtKill != nil {
		corpID = km.Victim.CorpAtKill.CorporationID
	}
	if km.Victim.AllianceAtKill != nil {
		allianceID = km.Victim.AllianceAtKill.AllianceID
	}
	// the target's own pilots, though one of its corps may also be listed as an affiliate
	if (target.TypeOfEntity == types.CORPORATION && corpID == target.IDNumber) || (target.TypeOfEntity == types.ALLIANCE && allianceID == target.IDNumber) {
		return nil
	}
	for index := range target.Holdings {
		if target.Holdings[index].Includes(corpID, allianceID) {
			return &target.Holdings[index]
		}
	}
	return nil
}

// affiliateTag marks a loss flown for one of the target's affiliates, hidden like the weapon and range tags: muted and last in the fitting window, and only found by the filter
func affiliateTag(affiliate retrieval.TargetGroup) retrieval.FitTag {
	return retrieval.FitTag{Name: "Affiliate", Meaning: fmt.Sprintf("Lost flying for %s, one of the target's affiliates", affiliate.Name), Hidden: true}
}

// withDoctrineReasons adds a Fleet Doctrine tag to a loss's tags, or its reasons to the one the loss has from its battle
func withDoctrineReasons(tags []retrieval.FitTag, doctrine retrieval.FitTag) []retrieval.FitTag {
	at := slices.IndexFunc(tags, func(tag retrieval.FitTag) bool { return tag.Name == doctrine.Name })
	if at < 0 {
		return retrieval.MergeFitTags(tags, []retrieval.FitTag{doctrine})
	}
	tags = slices.Clone(tags)
	tags[at].Reasons = append(slices.Clone(tags[at].Reasons), doctrine.Reasons...)
	return tags
}

/*
	podsWithShips pairs each capsule lost with implants to the ship its pilot lost before it in the same battle, the latest if several.
	A capsule can outlive its ship by a long while in a battle, and the battle may have moved through a gate. zKillboard's api does not link the two, so they are matched here.
	A ship gets one capsule at most, and a killmail on its own is in no battle, so pairs with nothing.

Parameters:
  - ships: the target's losses with a fit
  - pods: the target's capsule losses with implants
  - battleOf: each killmail in a battle to its battle's id

Returns:
  - map[int]*retrieval.Killmail: each paired ship's killmail id to its capsule's killmail
*/
func podsWithShips(ships []*retrieval.Killmail, pods []*retrieval.Killmail, battleOf map[int]string) map[int]*retrieval.Killmail {
	// only a ship of the same pilot in the same battle can be a capsule's
	type pilotInBattle struct {
		pilot  int
		battle string
	}
	shipsOf := map[pilotInBattle][]*retrieval.Killmail{}
	for _, ship := range ships {
		if battle := battleOf[ship.KillmailID]; battle != "" && ship.Victim.Pilot != nil {
			key := pilotInBattle{ship.Victim.Pilot.CharacterID, battle}
			shipsOf[key] = append(shipsOf[key], ship)
		}
	}

	podOf := map[int]*retrieval.Killmail{}
	for _, pod := range pods {
		battle := battleOf[pod.KillmailID]
		if battle == "" {
			continue
		}
		var ship *retrieval.Killmail
		for _, candidate := range shipsOf[pilotInBattle{pod.Victim.Pilot.CharacterID, battle}] {
			if podOf[candidate.KillmailID] != nil || candidate.KillmailTime.After(pod.KillmailTime) {
				continue
			}
			if ship == nil || candidate.KillmailTime.After(ship.KillmailTime) {
				ship = candidate
			}
		}
		if ship != nil {
			podOf[ship.KillmailID] = pod
		}
	}
	return podOf
}

// fitKey is what makes two fits identical: the hull and the base type (comm.BaseType) of the modules in each rack, in any order, so a module's T2, meta, faction, and other variants are one module. Charges, drones, and cargo do not count, as they are too easily used up or forgotten by the time the ship dies.
// A module in one of fit_merged_groups counts as its group, so a fit's ECM flavors, picked for the enemy's sensors, don't split it.
func fitKey(shipID int, fit *retrieval.Fit) string {
	return fitKeyOf(shipID, fit, false)
}

// fitKeyOf is fitKey, or with exact the key by each module's own type, which still counts a merged group as one module
func fitKeyOf(shipID int, fit *retrieval.Fit, exact bool) string {
	parts := []string{fmt.Sprint(shipID)}
	for _, rack := range [][]*retrieval.FittedModule{fit.Highs, fit.Mids, fit.Lows, fit.Rigs, fit.Subsystems, fit.Services} {
		ids := []int{}
		for _, fitted := range rack {
			if fitted != nil && fitted.Module != nil {
				ids = append(ids, moduleKey(fitted.Module.ItemID, exact))
			}
		}
		slices.Sort(ids)
		parts = append(parts, strings.Trim(fmt.Sprint(ids), "[]"))
	}
	return strings.Join(parts, "|")
}

// moduleKey is a module's identity when merging: its merged group (negative so it can't clash with a type id), else its base type, or with exact its own type
func moduleKey(id int, exact bool) int {
	if comm.FitMergedGroups.ContainsGroupOf(id) {
		return -comm.TypeGroup(id)
	}
	if exact {
		return id
	}
	return comm.BaseType(id)
}

// hasAbyssal is whether a fit has a mutated module. A merged fit is Abyssal if any of its losses is, as a merged group's flavors may differ.
func hasAbyssal(fit *retrieval.Fit) bool {
	for _, rack := range [][]*retrieval.FittedModule{fit.Highs, fit.Mids, fit.Lows, fit.Rigs, fit.Subsystems, fit.Services} {
		for _, fitted := range rack {
			if fitted == nil || fitted.Module == nil {
				continue
			}
			if fitType := comm.FitTypeOf(fitted.Module.ItemID); fitType != nil && fitType.Abyssal {
				return true
			}
		}
	}
	return false
}

// fitLayout turns a fit into what the fitting window draws
func fitLayout(fit *retrieval.Fit) FitLayout {
	return FitLayout{
		Highs:      fitSlots(fit.Highs),
		Mids:       fitSlots(fit.Mids),
		Lows:       fitSlots(fit.Lows),
		Rigs:       fitSlots(fit.Rigs),
		Subsystems: fitSlots(fit.Subsystems),
		Drones:     fitStacks(fit.Drones),
		Fighters:   fitStacks(fit.Fighters),
		Boosters:   bySlot(fit.Boosters),
		Implants:   bySlot(fit.Implants),
	}
}

func fitSlots(rack []*retrieval.FittedModule) []*FitSlot {
	slots := make([]*FitSlot, len(rack))
	for index, fitted := range rack {
		if fitted == nil || fitted.Module == nil {
			continue
		}
		slots[index] = &FitSlot{Module: fitPart(fitted.Module, 0)}
		if fitted.Charge != nil {
			charge := fitPart(fitted.Charge, 0)
			slots[index].Charge = &charge
		}
	}
	return slots
}

func fitStacks(stacks []*retrieval.FitCargo) []FitPart {
	parts := make([]FitPart, 0, len(stacks))
	for _, stack := range stacks {
		parts = append(parts, fitPart(stack.Item, stack.Quantity))
	}
	return parts
}

// bySlot lists implants or boosters in the order of their slots, those fit_types.json does not have last
func bySlot(items []*retrieval.Item) []FitPart {
	slot := func(item *retrieval.Item) int {
		if fitType := comm.FitTypeOf(item.ItemID); fitType != nil && fitType.HeadSlot > 0 {
			return fitType.HeadSlot
		}
		return math.MaxInt
	}
	sorted := slices.Clone(items)
	slices.SortStableFunc(sorted, func(a, b *retrieval.Item) int { return cmp.Compare(slot(a), slot(b)) })
	parts := make([]FitPart, 0, len(sorted))
	for _, item := range sorted {
		parts = append(parts, fitPart(item, 1))
	}
	return parts
}

func fitPart(item *retrieval.Item, quantity int) FitPart {
	return FitPart{ID: item.ItemID, Name: item.Name, ImageURL: api.TypeImageURL(item.ItemID), EveRefURL: api.EveRefTypeURL(item.ItemID), Quantity: quantity}
}
