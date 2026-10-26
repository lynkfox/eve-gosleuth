package gui

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// Ratter is a pilot in a system target's Ratters tab: someone who lost a ship with NPCs on the killmail
type Ratter struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	ZkillURL  string `json:"zkillUrl"`
	EveWhoURL string `json:"evewhoUrl"`
	// the corp and alliance at their latest loss, empty when in none
	Corporation     string    `json:"corporation"`
	Alliance        string    `json:"alliance"`
	CorporationIcon string    `json:"corporationIcon"`
	AllianceIcon    string    `json:"allianceIcon"`
	Losses          []RatLoss `json:"losses"` // newest first
}

// RatLoss is one of a ratter's losses
type RatLoss struct {
	ID           int        `json:"id"`
	URL          string     `json:"url"`
	Time         time.Time  `json:"time"`
	ShipName     string     `json:"shipName"`
	ShipImageURL string     `json:"shipImageUrl"`
	Location     string     `json:"location"` // zKillboard's nearest celestial, ie Jita IV - Asteroid Belt 1, empty when it has none
	TotalValue   float64    `json:"totalValue"`
	Kind         string     `json:"kind"`    // who was on it: npc (only NPCs), smartbomb (NPCs and players with only smartbombs), or players (NPCs and players with other weapons)
	Rats         []RatCount `json:"rats"`    // the NPCs on it, the most numerous first
	Players      int        `json:"players"` // the players on it
}

// RatCount is one kind of NPC on a ratting loss, and how many were on it
type RatCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var ratKindNames = map[aggregate.RatKind]string{
	aggregate.RatsOnly:       "npc",
	aggregate.RatsSmartbombs: "smartbomb",
	aggregate.RatsAndPlayers: "players",
}

/*
	buildRatters lists the pilots with ratting losses in a system target, for its Ratters tab: any loss with NPCs on the killmail

Parameters:
  - killmails: the system target's expanded killmails

Returns:
  - []Ratter: the pilots with only NPCs on a loss first, then with smartbombing players, then with other players, each the one with the latest loss first. Empty if there are none.
*/
func (a *App) buildRatters(killmails []*retrieval.Killmail) []Ratter {
	esi := a.clients.ESIClient
	found := aggregate.GetRatters(killmails, esi)

	// the nearest celestials are named together, ie asteroid belts, which have no ESI lookup of their own
	locationIDs := map[int]bool{}
	for _, ratter := range found {
		for _, loss := range ratter.Losses {
			if loss.Killmail.LocationID != 0 {
				locationIDs[loss.Killmail.LocationID] = true
			}
		}
	}
	locationNames := map[int]string{}
	if len(locationIDs) > 0 {
		ids := make([]int, 0, len(locationIDs))
		for id := range locationIDs {
			ids = append(ids, id)
		}
		names, err := esi.GetNamesFromEveIDs(ids)
		if err != nil {
			comm.Logger.WithError(err).Warn("Unable to name the ratting losses' nearest celestials")
		}
		locationNames = names
	}

	ratters := []Ratter{}
	for _, ratter := range found {
		row := Ratter{
			ID:        ratter.Pilot.CharacterID,
			Name:      ratter.Pilot.Name,
			ImageURL:  api.EntityImageURL(types.PILOT, ratter.Pilot.CharacterID),
			ZkillURL:  api.ZkillEntityURL(types.PILOT, ratter.Pilot.CharacterID),
			EveWhoURL: api.EveWhoURL(types.PILOT, ratter.Pilot.CharacterID),
			Losses:    []RatLoss{},
		}
		latest := ratter.Losses[0].Killmail.Victim
		row.Corporation, row.Alliance = groupNames(latest.CorpAtKill, latest.AllianceAtKill)
		row.CorporationIcon, row.AllianceIcon = groupIcons(latest.CorpAtKill, latest.AllianceAtKill)

		for _, loss := range ratter.Losses {
			km := loss.Killmail
			card := RatLoss{
				ID:           km.KillmailID,
				URL:          api.ZkillKillmailURL(km.KillmailID),
				Time:         km.KillmailTime,
				ShipName:     km.Victim.Ship.Name,
				ShipImageURL: api.TypeImageURL(km.Victim.Ship.ItemID),
				Location:     locationNames[km.LocationID],
				TotalValue:   km.TotalValue,
				Kind:         ratKindNames[loss.Kind],
				Rats:         []RatCount{},
				Players:      loss.Players,
			}
			for _, rat := range loss.Rats {
				card.Rats = append(card.Rats, RatCount{Name: rat.Ship.Name, Count: rat.Count})
			}
			row.Losses = append(row.Losses, card)
		}
		ratters = append(ratters, row)
	}
	return ratters
}
