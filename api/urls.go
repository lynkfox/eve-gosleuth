package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

const ZKILL_API_URL = "https://zkillboard.com/api"
const ESI_URL = "https://esi.evetech.net/latest"
const IMAGE_SERVER_URL = "https://images.evetech.net"

const ZKILL_URL = "https://zkillboard.com"

// PROJECT_URL is where the app's source lives, given in USER_AGENT as the contact for the services it calls
const PROJECT_URL = "https://github.com/lynkfox/eve-gosleuth"

// USER_AGENT identifies the app, not the person running it, to every service it calls, as CCP and zKillboard ask. It is the same for every copy of the app.
var USER_AGENT = fmt.Sprintf("eve-gosleuth/%s (+%s)", comm.VERSION, PROJECT_URL)

// ZKILL_PAGE_SIZE is how many killmails a full zKillboard api page holds, and ZKILL_MAX_PAGES how many pages zKillboard serves
const ZKILL_PAGE_SIZE = 200
const ZKILL_MAX_PAGES = 100

// ZKILL_MAX_KILLMAILS is the most killmails zKillboard lists for one entity
const ZKILL_MAX_KILLMAILS = ZKILL_PAGE_SIZE * ZKILL_MAX_PAGES
const EVE_WHO_URL = "https://evewho.com"
const EVE_REF_URL = "https://everef.net"
const EVE_WHO_API_URL = EVE_WHO_URL + "/api"

// EntityImageURL returns the EVE image server url for a pilot portrait or corp/alliance logo, or an empty string for other entity types
func EntityImageURL(typeOfEntity types.EntityType, idNumber int) string {
	switch typeOfEntity {
	case types.PILOT:
		return fmt.Sprintf("%s/characters/%d/portrait?size=128", IMAGE_SERVER_URL, idNumber)
	case types.CORPORATION:
		return fmt.Sprintf("%s/corporations/%d/logo?size=128", IMAGE_SERVER_URL, idNumber)
	case types.ALLIANCE:
		return fmt.Sprintf("%s/alliances/%d/logo?size=128", IMAGE_SERVER_URL, idNumber)
	}
	return ""
}

// EntityIconURL returns a small (32px) EVE image server portrait or logo, for icons, or an empty string for other entity types
func EntityIconURL(typeOfEntity types.EntityType, idNumber int) string {
	return strings.Replace(EntityImageURL(typeOfEntity, idNumber), "size=128", "size=32", 1)
}

// TypeImageURL returns the EVE image server icon url for an item type, such as a ship or structure
func TypeImageURL(typeID int) string {
	return fmt.Sprintf("%s/types/%d/icon?size=64", IMAGE_SERVER_URL, typeID)
}

// TypeRenderURL returns the EVE image server render of an item type, such as a system's star
func TypeRenderURL(typeID int) string {
	return TypeRenderURLAt(typeID, 128)
}

/*
	TypeRenderURLAt returns the EVE image server render of an item type at a given size, such as a hull in the fitting window

Parameters:
  - typeID: the item type
  - size: a size the server offers, a power of two from 32 to 1024

Returns:
  - string: the render's url
*/
func TypeRenderURLAt(typeID int, size int) string {
	return fmt.Sprintf("%s/types/%d/render?size=%d", IMAGE_SERVER_URL, typeID, size)
}

/*
	EveRefTypeURL returns EVE Ref's page for an item type, such as a module or a ship

Parameters:
  - typeID: the item type

Returns:
  - string: the page's url
*/
func EveRefTypeURL(typeID int) string {
	return fmt.Sprintf("%s/types/%d", EVE_REF_URL, typeID)
}

// ZkillKillmailURL returns the zKillboard page for a killmail
func ZkillKillmailURL(killmailID int) string {
	return fmt.Sprintf("%s/kill/%d/", ZKILL_URL, killmailID)
}

// zkillTogetherButtons are zKillboard's advanced search defaults, as its blank search has them, but for all time rather than the last week, and with Attackers as or
var zkillTogetherButtons = []string{"togglefilters", "alltime", "attackers-or", "either-and", "victims-and", "items-and", "sort-date", "sort-desc", "page1", "allinvolved"}

// zkillSearchFields are the advanced search's names for each kind of entity
var zkillSearchFields = map[types.EntityType]string{types.PILOT: "characterID", types.CORPORATION: "corporationID", types.ALLIANCE: "allianceID", types.SYSTEM: "solarSystemID"}

// ZkillEntity is a pilot, corporation, alliance, or system for a zKillboard advanced search
type ZkillEntity struct {
	Type types.EntityType
	ID   int
}

// zkillSearchEntity is an entity in an advanced search's list, ie {"type": "allianceID", "id": 99013330}
type zkillSearchEntity struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

// zkillSearch is a zKillboard advanced search, the JSON in its url after #. Its boxes must all match, each box all or any of its entities as its button says.
type zkillSearch struct {
	Buttons           []string            `json:"buttons"`
	Attackers         []zkillSearchEntity `json:"attackers"`
	Neutrals          []zkillSearchEntity `json:"neutrals"` // the Either box: on the killmail as attacker or victim
	IncludeAssociates bool                `json:"includeAssociates"`
}

/*
	ZkillTogetherURL returns a zKillboard advanced search for an ally's killmails with a target: any of the target's groups among the attackers,
	and the ally anywhere on the killmail, so both the kills it made with them and the kills of it they made

Parameters:
  - groups: the target and its holdings, any of which may be attacking
  - ally: the ally, as attacker or victim

Returns:
  - string: the search's url, ie https://zkillboard.com/asearch/#{"buttons":[...],"attackers":[...],"neutrals":[{"type":"allianceID","id":1}],"includeAssociates":true} escaped
*/
func ZkillTogetherURL(groups []ZkillEntity, ally ZkillEntity) string {
	search := zkillSearch{
		Buttons:           zkillTogetherButtons,
		Attackers:         []zkillSearchEntity{},
		Neutrals:          []zkillSearchEntity{{Type: zkillSearchFields[ally.Type], ID: ally.ID}},
		IncludeAssociates: true,
	}
	for _, group := range groups {
		search.Attackers = append(search.Attackers, zkillSearchEntity{Type: zkillSearchFields[group.Type], ID: group.ID})
	}
	query, _ := json.Marshal(search) // plain strings and ints always marshal
	return fmt.Sprintf("%s/asearch/#%s", ZKILL_URL, url.QueryEscape(string(query)))
}

// EveWhoURL returns the EveWho page for a pilot, corporation, or alliance, or the EveWho home page for other entity types
func EveWhoURL(typeOfEntity types.EntityType, idNumber int) string {
	pages := map[types.EntityType]string{types.PILOT: "character", types.CORPORATION: "corporation", types.ALLIANCE: "alliance"}

	page, ok := pages[typeOfEntity]
	if !ok {
		return EVE_WHO_URL
	}
	return fmt.Sprintf("%s/%s/%d", EVE_WHO_URL, page, idNumber)
}

const DOTLAN_URL = "https://evemaps.dotlan.net"

// DotlanSystemURL returns the Dotlan map page for a system, by name with spaces as underscores, ie https://evemaps.dotlan.net/system/Old_Man_Star
func DotlanSystemURL(systemName string) string {
	return fmt.Sprintf("%s/system/%s", DOTLAN_URL, url.PathEscape(strings.ReplaceAll(systemName, " ", "_")))
}

// ZkillEntityURL returns the zKillboard page for a pilot, corporation, alliance, or system, or the zKillboard home page for other entity types
func ZkillEntityURL(typeOfEntity types.EntityType, idNumber int) string {
	pages := map[types.EntityType]string{types.PILOT: "character", types.CORPORATION: "corporation", types.ALLIANCE: "alliance", types.SYSTEM: "system"}

	page, ok := pages[typeOfEntity]
	if !ok {
		return ZKILL_URL
	}
	return fmt.Sprintf("%s/%s/%d/", ZKILL_URL, page, idNumber)
}

// notALink is ParseEntityLink's type for text that is not a link, outside every real type
const notALink = types.EntityType(-1)

// linkPages are the page names zKillboard and EveWho use for each kind of entity, ie zkillboard.com/alliance/99007237/
var linkPages = map[string]map[string]types.EntityType{
	"zkillboard.com": {"character": types.PILOT, "corporation": types.CORPORATION, "alliance": types.ALLIANCE, "system": types.SYSTEM},
	"evewho.com":     {"character": types.PILOT, "corporation": types.CORPORATION, "alliance": types.ALLIANCE},
}

/*
	ParseEntityLink reads a pilot, corporation, alliance, or system from a zKillboard or EveWho link, so it can be found by id, as the ESI can not find every name

Parameters:
  - text: what was typed, ie https://zkillboard.com/alliance/99007237/ or evewho.com/character/2121139046, with or without https:// and www.

Returns:
  - types.EntityType: what the link is to
  - int: its eve id
  - bool: the text is such a link
*/
func ParseEntityLink(text string) (types.EntityType, int, bool) {
	text = strings.TrimSpace(text)
	if !strings.Contains(text, "://") {
		text = "https://" + text
	}
	link, err := url.Parse(text)
	if err != nil {
		return notALink, 0, false
	}
	pages, known := linkPages[strings.TrimPrefix(strings.ToLower(link.Hostname()), "www.")]
	if !known {
		return notALink, 0, false
	}
	parts := strings.Split(strings.Trim(link.Path, "/"), "/")
	if len(parts) < 2 {
		return notALink, 0, false
	}
	entityType, found := pages[strings.ToLower(parts[0])]
	id, idErr := strconv.Atoi(parts[1])
	if !found || idErr != nil || id <= 0 {
		return notALink, 0, false
	}
	return entityType, id, true
}

const EVE_TOOLS_BR_URL = "https://br.evetools.org"
const WARBEACON_URL = "https://warbeacon.net"

// relatedHour is a battle report's time as the related pages take it: the hour, in EVE time, with its minutes 00, ie 202610301200
func relatedHour(at time.Time) string {
	return at.UTC().Truncate(time.Hour).Format("200601021504")
}

// ZkillRelatedURL returns zKillboard's related kills page for a system around a time, ie https://zkillboard.com/related/30005206/202610301200/
func ZkillRelatedURL(systemID int, at time.Time) string {
	return fmt.Sprintf("%s/related/%d/%s/", ZKILL_URL, systemID, relatedHour(at))
}

// EveToolsRelatedURL returns the EVE Tools battle report for a system around a time, ie https://br.evetools.org/related/30005206/202610301200
func EveToolsRelatedURL(systemID int, at time.Time) string {
	return fmt.Sprintf("%s/related/%d/%s", EVE_TOOLS_BR_URL, systemID, relatedHour(at))
}

// WarbeaconRelatedURL returns the Warbeacon battle report for a system around a time, ie https://warbeacon.net/br/related/30005206/202610301200/
func WarbeaconRelatedURL(systemID int, at time.Time) string {
	return fmt.Sprintf("%s/br/related/%d/%s/", WARBEACON_URL, systemID, relatedHour(at))
}
