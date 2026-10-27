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
	GetZkillResults(entityId int, entityType types.EntityType, paging ZkillPaging) (*[]ZkbResponse, error)
}

// ZkillPaging is how far GetZkillResults pages through a killmail list
type ZkillPaging struct {
	Killmails int    // stop once this many killmails are listed, 0 to page to MaxPages
	MinPages  int    // page at least this far, even past Killmails
	MaxPages  int    // never page past this, at most ZKILL_MAX_PAGES
	For       string // whose list it is in progress messages, ie "holding URSA Holdings", empty to not say
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
  - paging: how many killmails, and how many pages, to list

Returns:
  - responses: a slice containing all there killmails across all pages (*[]ZkbResponse)
  - error: nill, ErrOnlyPartialZkillPull if it failed to reach the end of the killmail lists - the killmails retrieved will be returned as well
*/
func (z ZkillConnection) GetZkillResults(entityId int, typeOfEntity types.EntityType, paging ZkillPaging) (*[]ZkbResponse, error) {
	var DispatchMap = map[types.EntityType]string{
		types.PILOT:       "characterID",
		types.SYSTEM:      "systemID",
		types.ALLIANCE:    "allianceID",
		types.CORPORATION: "corporationID",
	}

	endpoint := fmt.Sprintf("%s/%d", DispatchMap[typeOfEntity], entityId)
	allZkillmails := &[]ZkbResponse{}

	maxPages := min(max(paging.MaxPages, 1), ZKILL_MAX_PAGES)
	pageNumber := 1
	previousShort := false
	for pageNumber <= maxPages {
		if paging.Killmails > 0 && len(*allZkillmails) >= paging.Killmails && pageNumber > paging.MinPages {
			break
		}
		listFor := ""
		if paging.For != "" {
			listFor = " for " + paging.For
		}
		comm.ReportProgress("Fetching zKillboard page %d%s (live, %d killmails so far)", pageNumber, listFor, len(*allZkillmails))

		listed := len(*allZkillmails)
		nextPage, err := z.GetZkillPage(endpoint, pageNumber, allZkillmails)
		if err != nil {
			// the page after a short one may be past the end of the list
			if previousShort {
				comm.Logger.WithError(err).Debugf("[Zkill] Page %d after a short page failed, taking it as the end of the list", pageNumber)
				break
			}
			return withoutDuplicates(allZkillmails), fmt.Errorf("%w at page %d: %w", ErrOnlyPartialZkillPull, pageNumber, err)
		}
		previousShort = len(*allZkillmails)-listed < ZKILL_PAGE_SIZE
		pageNumber = nextPage
	}

	return withoutDuplicates(allZkillmails), nil
}

// withoutDuplicates keeps the first of each killmail id, in order
func withoutDuplicates(killmails *[]ZkbResponse) *[]ZkbResponse {
	seen := make(map[int]bool, len(*killmails))
	unique := make([]ZkbResponse, 0, len(*killmails))
	for _, km := range *killmails {
		if !seen[km.KillmailID] {
			seen[km.KillmailID] = true
			unique = append(unique, km)
		}
	}
	return &unique

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

	var nextPage int = 9999

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
