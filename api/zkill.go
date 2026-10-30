package api

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ErrOnlyPartialZkillPull is returned if there is a failure when attempting to retrieve zkill pages, but some values may be in the return as well
var ErrOnlyPartialZkillPull = errors.New("zkill pull failed to complete all pages")

type ZkillClient interface {
	GetZkillResults(entityId int, entityType types.EntityType, paging ZkillPaging) (ZkillList, error)
}

// ZkillPaging is how far GetZkillResults pages through a killmail list
type ZkillPaging struct {
	Killmails int                       // stop once this many killmails are listed, 0 to page to MaxPages
	MaxPages  int                       // never page past this, at most ZKILL_MAX_PAGES
	StartPage int                       // the first page to pull, 0 for page 1, ie to carry on below a list already known
	Listed    int                       // killmails already listed above StartPage, counted in progress messages
	NewerThan time.Time                 // stop after a page that reaches a killmail this old or older, the zero time for no date limit
	Known     func(killmailID int) bool // stop after a page listing a killmail this knows, ie one on a cached list, nil to not
	For       string                    // whose list it is in progress messages, ie "holding URSA Holdings", empty to not say
	// Filter narrows the list with zKillboard's modifiers, put before the entity, ie "losses/groupID/1657" for the entity's citadels lost, empty for its whole list.
	// An entity with a groupID is that entity's own ship or structure in the group, ie kills/corporationID/1/groupID/1657 are kills the corp's own citadels were on.
	Filter string
}

// ZkillList is what GetZkillResults listed
type ZkillList struct {
	Killmails []ZkbResponse // newest first, each once
	Ended     bool          // the list reached the end of zKillboard's, rather than stopping at a paging limit or failing
}

type ZkillConnection struct {
	Client APIClient
}

// NewZkillClient initizes the connection data for an ApiConnection as a ZkillClient interface
func NewZkillClient() *ZkillConnection {

	connection := &ZkillConnection{
		Client: APIConnection{
			BaseURL: ZKILL_API_URL,
			HTTPClient: &http.Client{
				Timeout: 3 * time.Second,
			},
			Headers: map[string]string{
				"Accept-Encoding": "gzip",
				"User-Agent":      USER_AGENT,
			},
			MaxRetries: 3,
			Backoff:    1 * time.Second,
		},
	}

	return connection
}

/*
	GetZkillResults works through zkill responses pages until one comes back short of a full page, or the paging limits are reached.
	zKillboard holds back killmails under five minutes old, the newest, so page 1 can be short without being the last: it goes on to page 2 if nearly full,
	and page 2 failing then is taken as the end of the list. The list also moves as new killmails come in, so a killmail can be on two pages, and only its first is kept.

Parameters:
  - entityID:  the eve ID for the entity being checked on zkill (int)
  - typeOfEntity: the type, such as PILOT or ALLIANCE being checked (types.EntityType)
  - paging: where to start, and how many killmails, how many pages, how far back, or up to which known killmail, to list

Returns:
  - ZkillList: the killmails listed across all pages, and whether that was the end of zKillboard's list
  - error: nill, ErrOnlyPartialZkillPull if it failed to reach the end of the killmail lists - the killmails retrieved will be returned as well
*/
func (z ZkillConnection) GetZkillResults(entityId int, typeOfEntity types.EntityType, paging ZkillPaging) (ZkillList, error) {
	var DispatchMap = map[types.EntityType]string{
		types.PILOT:       "characterID",
		types.SYSTEM:      "systemID",
		types.ALLIANCE:    "allianceID",
		types.CORPORATION: "corporationID",
	}

	endpoint := fmt.Sprintf("%s/%d", DispatchMap[typeOfEntity], entityId)
	if paging.Filter != "" {
		endpoint = paging.Filter + "/" + endpoint
	}
	allZkillmails := &[]ZkbResponse{}

	maxPages := min(max(paging.MaxPages, 1), ZKILL_MAX_PAGES)
	pageNumber := max(paging.StartPage, 1)
	previousShort := false
	ended := false
	for pageNumber <= maxPages {
		if paging.Killmails > 0 && len(*allZkillmails) >= paging.Killmails {
			break
		}
		listFor := ""
		if paging.For != "" {
			listFor = " for " + paging.For
		}
		comm.ReportProgress("Fetching zKillboard page %d%s (live, %d killmails so far)", pageNumber, listFor, paging.Listed+len(*allZkillmails))

		listed := len(*allZkillmails)
		nextPage, err := z.GetZkillPage(endpoint, pageNumber, allZkillmails)
		if err != nil {
			// the page after a short one may be past the end of the list
			if previousShort {
				comm.Logger.WithError(err).Debugf("[Zkill] Page %d after a short page failed, taking it as the end of the list", pageNumber)
				ended = true
				break
			}
			return ZkillList{Killmails: withoutDuplicates(*allZkillmails)}, fmt.Errorf("%w at page %d: %w", ErrOnlyPartialZkillPull, pageNumber, err)
		}
		page := (*allZkillmails)[listed:]
		previousShort = len(page) < ZKILL_PAGE_SIZE
		if nextPage == zkillLastPage {
			ended = true
			break
		}
		if ReachesDate(page, paging.NewerThan) || listsKnown(page, paging.Known) {
			break
		}
		pageNumber = nextPage
	}

	return ZkillList{Killmails: withoutDuplicates(*allZkillmails), Ended: ended}, nil
}

// zkillLastPage is what GetZkillPage returns as the next page when the page it pulled was the last
const zkillLastPage = 9999

// ReachesDate reports whether a list, newest first, has a killmail no newer than newerThan. The zero time never is reached.
func ReachesDate(page []ZkbResponse, newerThan time.Time) bool {
	if newerThan.IsZero() {
		return false
	}
	for index := len(page) - 1; index >= 0; index-- {
		if at := page[index].KillmailTime; !at.IsZero() {
			return !at.After(newerThan)
		}
	}
	return false
}

// listsKnown reports whether a page lists a killmail known reports, false without known
func listsKnown(page []ZkbResponse, known func(int) bool) bool {
	if known == nil {
		return false
	}
	for _, km := range page {
		if known(km.KillmailID) {
			return true
		}
	}
	return false
}

// withoutDuplicates keeps the first of each killmail id, in order
func withoutDuplicates(killmails []ZkbResponse) []ZkbResponse {
	seen := make(map[int]bool, len(killmails))
	unique := make([]ZkbResponse, 0, len(killmails))
	for _, km := range killmails {
		if !seen[km.KillmailID] {
			seen[km.KillmailID] = true
			unique = append(unique, km)
		}
	}
	return unique
}

// zkillNearlyFullPage is how many killmails page 1 needs to go on to page 2, as it may be short by the newest killmails zKillboard holds back
const zkillNearlyFullPage = ZKILL_PAGE_SIZE * 3 / 4

// zkillRateLimitPause is how long to wait after zKillboard rate limits a page, before one more try
var zkillRateLimitPause = 30 * time.Second

// isRateLimited reports whether a request failed because the server answered 429 Too Many Requests, after Request's own retries
func isRateLimited(err error) bool {
	var statusErr *StatusError
	return errors.As(err, &statusErr) && statusErr.Status == http.StatusTooManyRequests
}

/*
	GetZkillPage will pull the zkill page # provided for a given entity, combining all the zkill responses into the into the master list.
	Returns the next page to pull, or 9999 when this page was the last or failed: one short of a full page, except page 1 when it is nearly full (zkillNearlyFullPage).

Parameters:
  - endpoint: the zkill endpoint, which includes the entity and entityID (string)
  - page: the page number, from 1 to ZKILL_MAX_PAGES (int)
  - allZkillmails: the currently found killmails from zkillboard to be added too (*[]ZkbResponse)
*/
func (z ZkillConnection) GetZkillPage(endpoint string, page int, allZkillmails *[]ZkbResponse) (int, error) {

	thisPageKillmails := []ZkbResponse{}

	var nextPage int = zkillLastPage

	url := fmt.Sprintf("%s/page/%d/", endpoint, page)
	resp, err := z.Client.Request("GET", url, nil)

	// errors.As would write the found error into a shared sentinel, so sentinels are matched with errors.Is
	if errors.Is(err, ErrRequestFailedToSend) {
		comm.Logger.WithError(err).Error("Failed to initiate request - check body")
		return nextPage, err
	}
	if isRateLimited(err) {
		comm.Logger.WithError(err).Warn("zKillboard is rate limiting - 30 second pause")
		comm.ReportProgress("zKillboard is rate limiting, waiting 30 seconds before retrying")
		time.Sleep(zkillRateLimitPause)
		resp, err = z.Client.Request("GET", url, nil)
	}
	if err != nil {
		return nextPage, err
	}

	defer resp.Body.Close()

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		comm.Logger.WithError(err).Error("Could not decode response from Zkill (gzip decode)")
		return nextPage, err
	}

	defer gzipReader.Close()

	err = json.NewDecoder(gzipReader).Decode(&thisPageKillmails)

	if err != nil {
		comm.Logger.WithError(err).Error("Could not decode response from Zkill")
		return nextPage, err
	}

	// zKillboard holds back killmails under five minutes old, the newest, so only page 1 can be short without being the last
	if len(thisPageKillmails) >= ZKILL_PAGE_SIZE || (page == 1 && len(thisPageKillmails) >= zkillNearlyFullPage) {
		nextPage = page + 1
	}

	*allZkillmails = append(*allZkillmails, thisPageKillmails...)

	return nextPage, nil
}
