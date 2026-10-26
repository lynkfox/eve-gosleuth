package retrieval

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type RawKillmail struct {
	ESIKillmail *api.ESIKillmail
	Zkillmail   *api.ZkbResponse
}

type Killmail struct {
	KillmailID     int
	Hash           string
	KillmailTime   time.Time
	System         *System
	Position       *api.ESIPosition
	FittedValue    float64
	DroppableValue float64  // zKillboard's totalDroppableValue
	Labels         []string // zKillboard's tags, ie tz:eu, loc:w-space
	DestroyedValue float64
	DroppedValue   float64
	TotalValue     float64
	NPC            bool
	Solo           bool
	Awox           bool
	Victim         *KillmailVictim
	Attackers      []*KillmailAttacker
}

type KillmailAttacker struct {
	Pilot          *Pilot
	CorpAtKill     *Corporation
	AllianceAtKill *Alliance
	Ship           *Item
	Weapon         *Item
	DamageDone     int
	FinalBlow      bool
}

type KillmailVictim struct {
	Pilot          *Pilot
	CorpAtKill     *Corporation
	AllianceAtKill *Alliance
	Ship           *Item
	DamageTaken    int
	Fit            []*FitItem
}

type Item struct {
	ItemID       int              `json:"id"`
	TypeOfEntity types.EntityType `json:"-"`
	Name         string           `json:"name"`
	ImageUrl     string           `json:"image"`
}

type FitItem struct {
	Item              *Item  `json:"item"`
	Flag              int    `json:"-"`
	FitLocationName   string `json:"slot"`
	QuantityDestroyed int    `json:"quantity_destroyed"`
	QuantityDropped   int    `json:"quantity_dropped"`
	Singleton         int    `json:"singleton"`
}

type System struct {
	SystemID       int     `json:"id"`
	Name           string  `json:"name"`
	Constellation  string  `json:"constellation_name"`
	Region         string  `json:"region_name"`
	SecurityStatus float64 `json:"security_status"`
}

type Pilot struct {
	CharacterID int          `json:"id"`
	Name        string       `json:"name"`
	ImageUrl    string       `json:"img_url"`
	Corporation *Corporation `json:"corporation,omitempty"`
	NPC         bool         `json:"is_npc"`
}

type Corporation struct {
	CorporationID int       `json:"id"`
	Name          string    `json:"name"`
	ImageUrl      string    `json:"img_url"`
	CeoID         int       `json:"ceo_id"`
	Alliance      *Alliance `json:"alliance"`
	Ticker        string    `json:"ticker"`

	AllianceChecked bool `json:"alliance_checked,omitempty"` // the alliance was looked up, so a nil Alliance means the corp is in none
}

func (c *Corporation) GetCEO(cached *CachedEveIDs) *Pilot {
	// ExpandPilot checks the cache first
	return cached.ExpandPilot(c.CeoID, false)
}

type Alliance struct {
	AllianceID            int
	Name                  string
	ImageUrl              string
	ExecutorCorporationID int
	Ticker                string
}

func (a *Alliance) GetExecutorCorp(cached *CachedEveIDs) *Corporation {
	// ExpandCorporation checks the cache first
	return cached.ExpandCorporation(a.ExecutorCorporationID, false)
}
