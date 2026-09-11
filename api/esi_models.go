package api

import (
	"time"
)

type TargetEntity interface {
	BuildDisplay() TargetDisplay
}

type ESIUniverseIDsResponse struct {
	Alliances      *[]EntityIDAndName `json:"alliances,omitempty"`
	Characters     *[]EntityIDAndName `json:"characters,omitempty"`
	Corporations   *[]EntityIDAndName `json:"corporations,omitempty"`
	InventoryTypes *[]EntityIDAndName `json:"inventory_types,omitempty"`
	Systems        *[]EntityIDAndName `json:"systems,omitempty"`
}

type EntityIDAndName struct {
	Category string `json:"category,omitempty"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
}

type ESICharacter struct {
	AllianceID     int       `json:"alliance_id,omitempty"`
	Birthday       time.Time `json:"birthday,omitempty"`
	BloodlineID    int       `json:"bloodline_id,omitempty"`
	CorporationID  int       `json:"corporation_id,omitempty"`
	CharacterID    int       `json:"-"`
	Description    string    `json:"description,omitempty"`
	Gender         string    `json:"gender,omitempty"`
	Name           string    `json:"name,omitempty"`
	RaceID         int       `json:"race_id,omitempty"`
	SecurityStatus float32   `json:"security_status,omitempty"`
	DateCached     time.Time `json:"date_cached,omitempty"`
}

type ESICorporation struct {
	CorporationID int       `json:"-"`
	AllianceID    int       `json:"alliance_id,omitempty"`
	CeoID         int       `json:"ceo_id,omitempty"`
	CreatorID     int       `json:"creator_id,omitempty"`
	DateFounded   time.Time `json:"date_founded"`
	Description   string    `json:"description"`
	HomeStationID int       `json:"home_station_id,omitempty"`
	MemberCount   int       `json:"member_count"`
	Name          string    `json:"name"`
	Shares        int       `json:"shares"`
	TaxRate       float64   `json:"tax_rate"`
	Ticker        string    `json:"ticker"`
	URL           string    `json:"url,omitempty"`
	WarEligible   bool      `json:"war_eligible"`
	DateCached    time.Time `json:"date_cached,omitempty"`
}

type ESIAlliance struct {
	AllianceID            int       `json:"-"`
	CreatorCorporationID  int       `json:"creator_corporation_id,omitempty"`
	CreatorID             int       `json:"creator_id,omitempty"`
	DateFounded           time.Time `json:"date_founded,omitempty"`
	ExecutorCorporationID int       `json:"executor_corporation_id,omitempty"`
	Name                  string    `json:"name"`
	Ticker                string    `json:"ticker"`
	DateCached            time.Time `json:"date_cached,omitempty"`
}

type ESIRegion struct {
	Constellations []int  `json:"constellations"`
	Description    string `json:"description"`
	Name           string `json:"name"`
	RegionID       int64  `json:"region_id"`
}

type ESIConstellation struct {
	ConstellationID int         `json:"constellation_id"`
	Name            string      `json:"name"`
	Position        ESIPosition `json:"position"`
	RegionID        int         `json:"region_id"`
	Systems         []int       `json:"systems"`
}

type ESISystem struct {
	ConstellationID int         `json:"constellation_id"`
	Name            string      `json:"name"`
	Planets         []ESIPlanet `json:"planets,omitempty"`
	Position        ESIPosition `json:"position,omitempty"`
	SecurityClass   string      `json:"security_class,omitempty"`
	SecurityStatus  float64     `json:"security_status"`
	StarID          int         `json:"star_id,omitempty"`
	Stargates       []int       `json:"stargates,omitempty"`
	Stations        []int       `json:"stations,omitempty"`
	SystemID        int         `json:"system_id"`
}

type ESIPlanet struct {
	PlanetID int   `json:"planet_id"`
	Moons    []int `json:"moons,omitempty"`
}

type ESIPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// ESIKillmail represents the structure of a killmail JSON response.
type ESIKillmail struct {
	KillmailID    int           `json:"killmail_id"`
	KillmailTime  time.Time     `json:"killmail_time"`
	SolarSystemID int           `json:"solar_system_id"`
	Attackers     []ESIAttacker `json:"attackers"`
	Victim        ESIVictim     `json:"victim"`
}

// ESIAttacker represents the structure of an attacker in the killmail.
type ESIAttacker struct {
	AllianceID     *int    `json:"alliance_id,omitempty"`
	CharacterID    *int    `json:"character_id,omitempty"`
	CorporationID  *int    `json:"corporation_id,omitempty"`
	DamageDone     int     `json:"damage_done"`
	FactionID      *int    `json:"faction_id,omitempty"`
	FinalBlow      bool    `json:"final_blow"`
	SecurityStatus float64 `json:"security_status"`
	ShipTypeID     int     `json:"ship_type_id"`
	WeaponTypeID   *int    `json:"weapon_type_id,omitempty"`
}

// ESIVictim represents the structure of the victim in the killmail.
type ESIVictim struct {
	AllianceID    *int              `json:"alliance_id,omitempty"`
	CharacterID   *int              `json:"character_id,omitempty"`
	CorporationID *int              `json:"corporation_id,omitempty"`
	DamageTaken   int               `json:"damage_taken"`
	Items         []ESIKillmailItem `json:"items"`
	Position      ESIPosition       `json:"position"`
	ShipTypeID    int               `json:"ship_type_id"`
}

// ESIKillmailItem represents the structure of an item involved in the killmail.
type ESIKillmailItem struct {
	Flag              int  `json:"flag"`
	ItemTypeID        int  `json:"item_type_id"`
	QuantityDestroyed *int `json:"quantity_destroyed,omitempty"`
	QuantityDropped   *int `json:"quantity_dropped,omitempty"`
	Singleton         int  `json:"singleton"`
}

type ESIItem struct {
	ItemId      int    `json:"type_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	GroupID     int    `json:"group_id"`
}
