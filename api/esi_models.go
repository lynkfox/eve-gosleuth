package api

import (
	"time"
)

type TargetEntity interface {
	BuildDisplay(client ESIClient, eveWho EveWhoClient) TargetDisplay
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
	CharacterID    int       `json:"character_id,omitempty"` // not in the ESI response, set from the id that was asked for
	Description    string    `json:"description,omitempty"`
	Gender         string    `json:"gender,omitempty"`
	Name           string    `json:"name,omitempty"`
	RaceID         int       `json:"race_id,omitempty"`
	SecurityStatus float32   `json:"security_status,omitempty"`
	DateCached     time.Time `json:"date_cached,omitempty"`
	Deleted        bool      `json:"deleted,omitempty"` // not in the ESI response, set when the ESI says the character no longer exists
}

// ESIAffiliation is a character's current corporation and alliance, from the bulk characters/affiliation endpoint.
// A deleted character is still answered, in Doomheim (DOOMHEIM_CORPORATION_ID).
type ESIAffiliation struct {
	CharacterID   int `json:"character_id"`
	CorporationID int `json:"corporation_id"`
	AllianceID    int `json:"alliance_id,omitempty"`
	FactionID     int `json:"faction_id,omitempty"`
}

// DOOMHEIM_CORPORATION_ID is the NPC corporation deleted characters are moved to
const DOOMHEIM_CORPORATION_ID = 1000001

// ESI_BULK_IDS is the most ids the ESI takes in one universe/names or characters/affiliation request
const ESI_BULK_IDS = 1000

// DELETED_CHARACTER_NAME is shown for a character the ESI no longer has, ie one biomassed after appearing on a killmail
const DELETED_CHARACTER_NAME = "Deleted character"

type ESICorporation struct {
	CorporationID int       `json:"corporation_id,omitempty"` // not in the ESI response, set from the id that was asked for
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
	AllianceID            int       `json:"alliance_id,omitempty"` // not in the ESI response, set from the id that was asked for
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

// ESIStargate is the universe/stargates response, a gate in one system leading to a gate in another
type ESIStargate struct {
	StargateID  int    `json:"stargate_id"`
	Name        string `json:"name"` // ie Stargate (Maurasi)
	SystemID    int    `json:"system_id"`
	Destination struct {
		StargateID int `json:"stargate_id"`
		SystemID   int `json:"system_id"`
	} `json:"destination"`
}

// ESIPlanetInfo is the universe/planets response. Its type id is the planet type, ie Planet (Barren).
type ESIPlanetInfo struct {
	PlanetID int    `json:"planet_id"`
	Name     string `json:"name"` // ie Jita IV
	SystemID int    `json:"system_id"`
	TypeID   int    `json:"type_id"`
}

// ESIMoon is the universe/moons response
type ESIMoon struct {
	MoonID   int    `json:"moon_id"`
	Name     string `json:"name"` // ie Jita IV - Moon 4
	SystemID int    `json:"system_id"`
}

// ESICategory is the universe/categories response, ie 6 is Ship. zKillboard's cat: labels name a category by its id.
type ESICategory struct {
	CategoryID int    `json:"category_id"`
	Name       string `json:"name"`
}

// ESIStar is the universe/stars response, the sun of a system. Its type id picks the star's image.
type ESIStar struct {
	Name          string `json:"name"` // ie Jita - Star
	SolarSystemID int    `json:"solar_system_id"`
	SpectralClass string `json:"spectral_class"` // ie K7 V
	TypeID        int    `json:"type_id"`
}

// ESIAllianceCorporations is the alliances/{id}/corporations response, a bare list of corporation ids, with the alliance it was asked for
type ESIAllianceCorporations struct {
	AllianceID     int   `json:"alliance_id"` // not in the ESI response, set from the id that was asked for
	CorporationIDs []int `json:"corporation_ids"`
}

// ESICorporationHistory is the characters/{id}/corporationhistory response, a bare list of corp stints, with the character it was asked for
type ESICorporationHistory struct {
	CharacterID int                         `json:"character_id"` // not in the ESI response, set from the id that was asked for
	Records     []ESICorporationHistoryItem `json:"records"`
}

// ESICorporationHistoryItem is one stint in a corporation. It has no end date, the next stint's start ends it.
type ESICorporationHistoryItem struct {
	CorporationID int       `json:"corporation_id"`
	RecordID      int       `json:"record_id"`
	StartDate     time.Time `json:"start_date"`
	IsDeleted     bool      `json:"is_deleted,omitempty"` // the corporation has closed
}

// ESIAllianceHistory is the corporations/{id}/alliancehistory response, a bare list of alliance stints, with the corporation it was asked for
type ESIAllianceHistory struct {
	CorporationID int                      `json:"corporation_id"` // not in the ESI response, set from the id that was asked for
	Records       []ESIAllianceHistoryItem `json:"records"`
}

// ESIAllianceHistoryItem is the corp joining an alliance, or leaving one when AllianceID is 0. The next record's start ends it.
type ESIAllianceHistoryItem struct {
	AllianceID int       `json:"alliance_id,omitempty"`
	RecordID   int       `json:"record_id"`
	StartDate  time.Time `json:"start_date"`
	IsDeleted  bool      `json:"is_deleted,omitempty"` // the alliance has closed
}

/*
	JoinedAlliance returns when a corporation joined its current alliance: the start of its latest record in it

Parameters:
  - allianceID: the alliance

Returns:
  - time.Time: when it joined
  - bool: false if the history has no record of the alliance
*/
func (h ESIAllianceHistory) JoinedAlliance(allianceID int) (time.Time, bool) {
	var joined time.Time
	found := false
	for _, record := range h.Records {
		if record.AllianceID == allianceID && (!found || record.StartDate.After(joined)) {
			joined, found = record.StartDate, true
		}
	}
	return joined, found
}
