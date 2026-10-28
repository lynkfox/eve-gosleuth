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
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// FitGroup is one fit the target lost, with every loss of it merged: the same hull with the same modules in each rack, whatever the charges, drones, or cargo
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
}

// FitLoss is one loss of a fit, with its own charges and cargo
type FitLoss struct {
	KillmailID   int       `json:"killmailId"`
	URL          string    `json:"url"` // the killmail on zKillboard
	Time         time.Time `json:"time"`
	PilotName    string    `json:"pilotName"`          // the pilot, or the owning corp for a structure
	CorpName     string    `json:"corpName,omitempty"` // the victim's corp and alliance at the time, for the filter box
	AllianceName string    `json:"allianceName,omitempty"`
	SystemName   string    `json:"systemName"`
	EFT          string    `json:"eft"`
	MissingTypes int       `json:"missingTypes"` // item types not in fit_types.json, so the fit may be incomplete
	Fit          FitLayout `json:"fit"`

	// the capsule the pilot lost with implants later in the battle, see podsWithShips. Its implants are the fit's.
	PodKillmailID int    `json:"podKillmailId,omitempty"`
	PodURL        string `json:"podUrl,omitempty"`
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
	Module FitPart  `json:"module"`
	Charge *FitPart `json:"charge"` // nil if unloaded
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

Returns:
  - []FitGroup: the fits, empty if there are none
*/
func buildFits(killmails []*retrieval.Killmail, target *retrieval.IntelTarget, battleOf map[int]string) []FitGroup {
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

	groups := []*FitGroup{}
	byKey := map[string]*FitGroup{}
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

		key := fitKey(km.Victim.Ship.ItemID, km.Victim.Fit)
		group := byKey[key]
		if group == nil {
			group = &FitGroup{
				ShipID:        km.Victim.Ship.ItemID,
				ShipName:      km.Victim.Ship.Name,
				ShipImageURL:  api.TypeImageURL(km.Victim.Ship.ItemID),
				ShipRenderURL: api.TypeRenderURLAt(km.Victim.Ship.ItemID, 512),
				ShipEveRefURL: api.EveRefTypeURL(km.Victim.Ship.ItemID),
				Abyssal:       hasAbyssal(km.Victim.Fit),
			}
			byKey[key] = group
			groups = append(groups, group)
		}
		group.Losses = append(group.Losses, loss)
		group.Implants = group.Implants || loss.PodKillmailID != 0
	}

	fits := make([]FitGroup, 0, len(groups))
	for _, group := range groups {
		slices.SortStableFunc(group.Losses, func(a, b FitLoss) int {
			if byImplants := cmp.Compare(len(b.Fit.Implants), len(a.Fit.Implants)); byImplants != 0 {
				return byImplants
			}
			return b.Time.Compare(a.Time)
		})
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

// fitKey is what makes two fits identical: the hull and the module types in each rack, in any order. Charges, drones, and cargo do not count, as they are too easily used up or forgotten by the time the ship dies.
func fitKey(shipID int, fit *retrieval.Fit) string {
	parts := []string{fmt.Sprint(shipID)}
	for _, rack := range [][]*retrieval.FittedModule{fit.Highs, fit.Mids, fit.Lows, fit.Rigs, fit.Subsystems, fit.Services} {
		ids := []int{}
		for _, fitted := range rack {
			if fitted != nil && fitted.Module != nil {
				ids = append(ids, fitted.Module.ItemID)
			}
		}
		slices.Sort(ids)
		parts = append(parts, strings.Trim(fmt.Sprint(ids), "[]"))
	}
	return strings.Join(parts, "|")
}

// hasAbyssal is whether a fit has a mutated module. As the key holds the module types, every loss of a fit has one or none does.
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
