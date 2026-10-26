package api

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

// ZkbResponse represents each object in the response array on responses from ZKillboard api calls
type ZkbResponse struct {
	KillmailID int `json:"killmail_id"`
	Zkb        Zkb `json:"zkb"`
}
