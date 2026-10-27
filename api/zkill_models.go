package api

import "time"

// Zkb represents the nested structure within the ZkbResponse
type Zkb struct {
	LocationID     int     `json:"locationID"`
	Hash           string  `json:"hash"`
	FittedValue    float64 `json:"fittedValue"`
	DroppedValue   float64 `json:"droppedValue"`
	DestroyedValue float64 `json:"destroyedValue"`
	TotalValue     float64 `json:"totalValue"`
	// TotalDroppableValue is the value of everything on the victim that could have dropped, before the drop roll
	TotalDroppableValue float64  `json:"totalDroppableValue"`
	Points              int      `json:"points"`
	NPC                 bool     `json:"npc"`
	Solo                bool     `json:"solo"`
	Awox                bool     `json:"awox"`
	Labels              []string `json:"labels"` // zKillboard's tags for the killmail, ie tz:eu, loc:w-space, #:5+, capital
}

// ZkbResponse represents each object in the response array on responses from ZKillboard api calls. zKillboard sends the whole ESI killmail with its own zkb details.
type ZkbResponse struct {
	KillmailID    int           `json:"killmail_id"`
	Zkb           Zkb           `json:"zkb"`
	KillmailTime  time.Time     `json:"killmail_time"`
	SolarSystemID int           `json:"solar_system_id"`
	Attackers     []ESIAttacker `json:"attackers"`
	Victim        ESIVictim     `json:"victim"`
}

/*
	Killmail returns the ESI killmail zKillboard sent with the entry, so it need not be fetched from the ESI

Returns:
  - *ESIKillmail: the killmail, or nil when the entry is missing its time, system, victim's ship, or attackers, and the ESI's copy should be fetched instead
*/
func (z ZkbResponse) Killmail() *ESIKillmail {
	if z.KillmailID == 0 || z.KillmailTime.IsZero() || z.SolarSystemID == 0 || z.Victim.ShipTypeID == 0 || len(z.Attackers) == 0 {
		return nil
	}
	return &ESIKillmail{
		KillmailID:    z.KillmailID,
		KillmailTime:  z.KillmailTime,
		SolarSystemID: z.SolarSystemID,
		Attackers:     z.Attackers,
		Victim:        z.Victim,
	}
}
