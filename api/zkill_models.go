package api

// Zkb represents the nested structure within the ZkbResponse
type Zkb struct {
	LocationID     int     `json:"locationID"`
	Hash           string  `json:"hash"`
	FittedValue    float64 `json:"fittedValue"`
	DroppedValue   float64 `json:"droppedValue"`
	DestroyedValue float64 `json:"destroyedValue"`
	TotalValue     float64 `json:"totalValue"`
	Points         int     `json:"points"`
	NPC            bool    `json:"npc"`
	Solo           bool    `json:"solo"`
	Awox           bool    `json:"awox"`
}

// ZkbResponse represents each object in the response array on responses from ZKillboard api calls
type ZkbResponse struct {
	KillmailID int `json:"killmail_id"`
	Zkb        Zkb `json:"zkb"`
}
