package aggregate

import (
	"cmp"
	"slices"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// Which ships are haulers, T1 haulers and T2 transports but not freighters, and which items are PI materials, come from static data: comm.HaulerGroups, comm.HaulerExclusions, and comm.PIMaterialGroups

// PICargo is one PI material on a lost hauler, with how many of it were destroyed and dropped together
type PICargo struct {
	Item     *api.ESIItem
	Quantity int
}

// PILoss is a ship a pilot lost in the system that points to Planetary Industry: a hauler near one of its planets, or any ship with PI materials aboard
type PILoss struct {
	Killmail *retrieval.Killmail
	PlanetID int       // the planet zKillboard says the killmail was nearest, 0 when it was not nearest a planet
	Cargo    []PICargo // the PI materials on it, by name, empty when it carried none
}

// PIPilot is a pilot with PI losses in the system, newest first
type PIPilot struct {
	Pilot     *retrieval.Pilot
	Losses    []PILoss
	CarriedPI bool // any of the losses had PI materials on it
}

/*
	GetPIPilots finds the pilots doing Planetary Industry in a system, by their losses: a hauler, such as an Epithal, Squall, or Crane, near one of its planets,
	or any ship that had PI materials aboard, wherever in the system it died

Parameters:
  - killmails: the system target's expanded killmails
  - planetIDs: the system's planets. A killmail is near one when zKillboard's nearest celestial for it is that planet.
  - esiClient: looks up the ship's and each item's group, ie the cache. Every victim's items are checked, so prefetching them first (retrieval.PrefetchItemTypes) saves a lookup at a time.

Returns:
  - []PIPilot: each pilot with their losses newest first, the pilot with the latest loss first. Empty when there are none.
*/
func GetPIPilots(killmails []*retrieval.Killmail, planetIDs map[int]bool, esiClient api.ESIClient) []PIPilot {
	byPilot := map[int]*PIPilot{}

	for _, km := range killmails {
		victim := km.Victim
		if victim == nil || victim.Pilot == nil || victim.Pilot.NPC || victim.Ship == nil {
			continue
		}
		nearPlanet := planetIDs[km.LocationID]
		cargo := piCargo(victim.Items, esiClient)
		if len(cargo) == 0 && !(nearPlanet && isHauler(victim.Ship.ItemID, esiClient)) {
			continue
		}

		loss := PILoss{Killmail: km, Cargo: cargo}
		if nearPlanet {
			loss.PlanetID = km.LocationID
		}
		pilot, found := byPilot[victim.Pilot.CharacterID]
		if !found {
			pilot = &PIPilot{Pilot: victim.Pilot}
			byPilot[victim.Pilot.CharacterID] = pilot
		}
		pilot.Losses = append(pilot.Losses, loss)
		pilot.CarriedPI = pilot.CarriedPI || len(loss.Cargo) > 0
	}

	newestFirst := func(a, b PILoss) int { return b.Killmail.KillmailTime.Compare(a.Killmail.KillmailTime) }
	pilots := []PIPilot{}
	for _, pilot := range byPilot {
		slices.SortFunc(pilot.Losses, newestFirst)
		pilots = append(pilots, *pilot)
	}
	slices.SortFunc(pilots, func(a, b PIPilot) int {
		if byTime := newestFirst(a.Losses[0], b.Losses[0]); byTime != 0 {
			return byTime
		}
		return cmp.Compare(a.Pilot.Name, b.Pilot.Name)
	})

	return pilots
}

// isHauler reports whether a ship type is in one of the hauler groups, leaving out the ones that are not really haulers
func isHauler(shipTypeID int, esiClient api.ESIClient) bool {
	if comm.HaulerExclusions.ContainsIDNumber(shipTypeID) {
		return false
	}
	ship, err := esiClient.GetItemInfo(shipTypeID)
	if err != nil || ship == nil {
		comm.Logger.WithError(err).Warnf("Unable to look up ship type %d, it is not counted as a hauler", shipTypeID)
		return false
	}
	return comm.HaulerGroups.ContainsIDNumber(ship.GroupID)
}

// piCargo returns the PI materials among a victim's items, each type once with its quantities added up, by name
func piCargo(items []api.ESIKillmailItem, esiClient api.ESIClient) []PICargo {
	byType := map[int]*PICargo{}
	for _, item := range items {
		info, err := esiClient.GetItemInfo(item.ItemTypeID)
		if err != nil || info == nil {
			comm.Logger.WithError(err).Warnf("Unable to look up item type %d, it is not checked for PI materials", item.ItemTypeID)
			continue
		}
		if !comm.PIMaterialGroups.ContainsIDNumber(info.GroupID) {
			continue
		}

		cargo, found := byType[item.ItemTypeID]
		if !found {
			cargo = &PICargo{Item: info}
			byType[item.ItemTypeID] = cargo
		}
		if item.QuantityDestroyed != nil {
			cargo.Quantity += *item.QuantityDestroyed
		}
		if item.QuantityDropped != nil {
			cargo.Quantity += *item.QuantityDropped
		}
	}

	cargo := []PICargo{}
	for _, found := range byType {
		cargo = append(cargo, *found)
	}
	slices.SortFunc(cargo, func(a, b PICargo) int { return cmp.Compare(a.Item.Name, b.Item.Name) })
	return cargo
}
