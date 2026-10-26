package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// PIPilot is a pilot in a system target's PI Pilots tab: someone who lost a hauler near one of its planets, or any ship with PI materials aboard
type PIPilot struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	ZkillURL  string `json:"zkillUrl"`
	EveWhoURL string `json:"evewhoUrl"`
	// the corp and alliance at their latest loss, empty when in none
	Corporation     string   `json:"corporation"`
	Alliance        string   `json:"alliance"`
	CorporationIcon string   `json:"corporationIcon"`
	AllianceIcon    string   `json:"allianceIcon"`
	Losses          []PILoss `json:"losses"` // newest first
}

// PILoss is one of a PI pilot's losses: a hauler near a planet, or any ship with PI aboard
type PILoss struct {
	ID           int       `json:"id"`
	URL          string    `json:"url"`
	Time         time.Time `json:"time"`
	ShipName     string    `json:"shipName"`
	ShipImageURL string    `json:"shipImageUrl"`
	Planet       string    `json:"planet"` // ie Jita IV · Barren, empty when it was not nearest a planet
	TotalValue   float64   `json:"totalValue"`
	Cargo        []PICargo `json:"cargo"` // the PI materials on it, empty when it carried none
}

// PICargo is a PI material on a lost hauler, destroyed and dropped together
type PICargo struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

/*
	buildPIPilots lists the pilots with PI losses in a system target, for its PI Pilots tab: haulers near its planets, and any ship with PI materials aboard

Parameters:
  - systemID: the system target
  - killmails: its expanded killmails

Returns:
  - []PIPilot: the pilot with the latest loss first. Empty if there are none, or if the system's planets could not be looked up.
*/
func (a *App) buildPIPilots(systemID int, killmails []*retrieval.Killmail) []PIPilot {
	esi := a.clients.ESIClient
	system, err := esi.GetSystemInfo(systemID)
	if err != nil || system == nil {
		comm.Logger.WithError(err).Warnf("Unable to look up the planets of system %d, its PI Pilots tab is left empty", systemID)
		return []PIPilot{}
	}
	planetIDs := map[int]bool{}
	for _, planet := range system.Planets {
		planetIDs[planet.PlanetID] = true
	}

	planetNames := map[int]string{}
	planetName := func(planetID int) string {
		if _, found := planetNames[planetID]; !found {
			planetNames[planetID] = planetLabel(esi, planetID)
		}
		return planetNames[planetID]
	}

	// every victim's items are checked for PI materials, so look up the types not cached yet together first
	if cache, ok := esi.(*retrieval.CachedEveIDs); ok {
		typeIDs := []int{}
		for _, km := range killmails {
			if km.Victim != nil {
				for _, item := range km.Victim.Items {
					typeIDs = append(typeIDs, item.ItemTypeID)
				}
			}
		}
		cache.PrefetchItemTypes(typeIDs)
	}

	pilots := []PIPilot{}
	for _, found := range aggregate.GetPIPilots(killmails, planetIDs, esi) {
		pilot := PIPilot{
			ID:        found.Pilot.CharacterID,
			Name:      found.Pilot.Name,
			ImageURL:  api.EntityImageURL(types.PILOT, found.Pilot.CharacterID),
			ZkillURL:  api.ZkillEntityURL(types.PILOT, found.Pilot.CharacterID),
			EveWhoURL: api.EveWhoURL(types.PILOT, found.Pilot.CharacterID),
			Losses:    []PILoss{},
		}
		latest := found.Losses[0].Killmail.Victim
		pilot.Corporation, pilot.Alliance = groupNames(latest.CorpAtKill, latest.AllianceAtKill)
		pilot.CorporationIcon, pilot.AllianceIcon = groupIcons(latest.CorpAtKill, latest.AllianceAtKill)

		for _, loss := range found.Losses {
			km := loss.Killmail
			card := PILoss{
				ID:           km.KillmailID,
				URL:          api.ZkillKillmailURL(km.KillmailID),
				Time:         km.KillmailTime,
				ShipName:     km.Victim.Ship.Name,
				ShipImageURL: api.TypeImageURL(km.Victim.Ship.ItemID),
				TotalValue:   km.TotalValue,
				Cargo:        []PICargo{},
			}
			if loss.PlanetID != 0 {
				card.Planet = planetName(loss.PlanetID)
			}
			for _, cargo := range loss.Cargo {
				card.Cargo = append(card.Cargo, PICargo{Name: cargo.Item.Name, Quantity: cargo.Quantity})
			}
			pilot.Losses = append(pilot.Losses, card)
		}
		pilots = append(pilots, pilot)
	}
	return pilots
}

// planetLabel names a planet with its type, ie Jita IV · Barren, falling back to the name alone or its id when they can not be looked up
func planetLabel(esi api.ESIClient, planetID int) string {
	planet, err := esi.GetPlanetInfo(planetID)
	if err != nil || planet == nil {
		comm.Logger.WithError(err).Warnf("Unable to look up planet %d", planetID)
		return fmt.Sprintf("Planet %d", planetID)
	}
	planetType, err := esi.GetItemInfo(planet.TypeID)
	if err != nil || planetType == nil {
		return planet.Name
	}
	// planet types are named like Planet (Barren)
	kind := strings.TrimSuffix(strings.TrimPrefix(planetType.Name, "Planet ("), ")")
	return planet.Name + " · " + kind
}
