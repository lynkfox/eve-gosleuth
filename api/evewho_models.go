package api

import (
	"strings"
	"time"
)

// EveWhoCharacterResponse is the evewho.com/api/character response: the pilot's current info and full corp history
type EveWhoCharacterResponse struct {
	Info    []EveWhoCharacterInfo `json:"info"`
	History []EveWhoCorpRecord    `json:"history"` // oldest first
}

type EveWhoCharacterInfo struct {
	CharacterID   int    `json:"character_id"`
	CorporationID int    `json:"corporation_id"`
	AllianceID    int    `json:"alliance_id"`
	FactionID     int    `json:"faction_id"`
	Name          string `json:"name"`
	SecStatus     string `json:"sec_status"`
}

// EveWhoCorpRecord is one stint in a corp. EndDate is nil for the pilot's current corp.
type EveWhoCorpRecord struct {
	RecordID      int         `json:"record_id"`
	CorporationID int         `json:"corporation_id"`
	StartDate     EveWhoTime  `json:"start_date"`
	EndDate       *EveWhoTime `json:"end_date"`
}

// EVEWHO_PAGE_SIZE is how many records EveWho's paged lists, such as corpjoined, hold on each page
const EVEWHO_PAGE_SIZE = 500

// EveWhoJoinList is one page of the evewho.com/api/corpjoined response: join records newest first, and the paging
type EveWhoJoinList struct {
	Characters []EveWhoJoin     `json:"characters"`
	Pagination EveWhoPagination `json:"pagination"`
}

// EveWhoJoin is one pilot joining a corp. EndDate is nil while they are still in it.
type EveWhoJoin struct {
	CharacterID int         `json:"id"`
	StartDate   EveWhoTime  `json:"start_date"`
	EndDate     *EveWhoTime `json:"end_date"`
}

type EveWhoPagination struct {
	Page    int  `json:"page"`
	Total   int  `json:"total"`
	Pages   int  `json:"pages"`
	HasNext bool `json:"has_next"`
}

// EveWhoGroupResponse is the evewho.com/api/corplist and allilist response. Only the info is used, the member list is paginated.
type EveWhoGroupResponse struct {
	Info []EveWhoGroupInfo `json:"info"`
}

type EveWhoGroupInfo struct {
	Name        string `json:"name"`
	MemberCount int    `json:"memberCount"`
}

// EveWhoTime is an EveWho date, which is sent as "2006/01/02 15:04" in EVE time (UTC) rather than RFC 3339
type EveWhoTime struct {
	time.Time
}

const eveWhoTimeLayout = "2006/01/02 15:04"

func (t *EveWhoTime) UnmarshalJSON(data []byte) error {
	value := strings.Trim(string(data), `"`)
	if value == "null" || value == "" {
		return nil
	}

	parsed, err := time.Parse(eveWhoTimeLayout, value)
	if err != nil {
		return err
	}

	t.Time = parsed
	return nil
}
