package retrieval

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// DefaultKillmailLimit is how many killmails are collected unless the user sets another limit in Preferences
const DefaultKillmailLimit = 20

// ErrIncompleteKillmails is returned alongside the killmails that were collected when some could not be retrieved
var ErrIncompleteKillmails = errors.New("not all killmails could be collected")

// ErrTooManyESIKillmails is wrapped when more killmails than MaxESIKillmails needed the ESI, and the rest were skipped
var ErrTooManyESIKillmails = errors.New("too many killmails to fetch from the ESI")

/*
MaxESIKillmails is the most killmails a search fetches from the ESI, those zKillboard sent only in part. zKillboard sends nearly all of them whole,
so this is only reached if it stops doing so. The ESI's killmail rate limit group has 3600 tokens a 15 minute window and a killmail costs 2,
so this leaves some for the rest of the search.
*/
const MaxESIKillmails = 1700

// KillmailLimits are the Preferences that decide which of a target's killmails are collected
type KillmailLimits struct {
	Max           int       // the most killmails to collect from zKillboard's list, newest first, or AllKillmails for every one it lists
	NewerThan     time.Time // only killmails after this, the zero time for no date limit
	IncludeCached bool      // also use killmails already in the cache that are beyond Max, Pages, or NewerThan, ie those listed by earlier searches
	Pages         int       // how many zKillboard pages of killmails to collect when Max is AllKillmails, 1 to api.ZKILL_MAX_PAGES. 0 is DefaultKillmailPages.
	// ConfirmIgnoreDate is asked, when the newest killmail is already older than NewerThan, whether to collect without the date limit for this search.
	// It is given the newest killmail's time, and returns false to keep the limit and collect none. nil always keeps it.
	ConfirmIgnoreDate func(newest time.Time) bool
}

// AllKillmails as KillmailLimits.Max collects every killmail on the first KillmailLimits.Pages pages zKillboard lists, still stopping at NewerThan
const AllKillmails = -1

// DefaultKillmailPages is how many zKillboard pages of 200 are listed for AllKillmails unless the user sets another number in Preferences
const DefaultKillmailPages = 10

/*
paging is how far to page through each zKillboard list for these limits, and how many killmails from the top of each the search needs.
A max count pages until that many are listed, up to every page zKillboard serves. AllKillmails pages to Pages. Either stops at the page reaching NewerThan.
*/
func (limits KillmailLimits) paging() (api.ZkillPaging, int) {
	pages := limits.Pages
	if pages <= 0 {
		pages = DefaultKillmailPages
	}
	if limits.Max == AllKillmails {
		return api.ZkillPaging{MaxPages: pages, NewerThan: limits.NewerThan}, pages * api.ZKILL_PAGE_SIZE
	}
	return api.ZkillPaging{Killmails: limits.Max, MaxPages: api.ZKILL_MAX_PAGES, NewerThan: limits.NewerThan}, limits.Max
}

// DefaultKillmailLimits collects the newest DefaultKillmailLimit killmails of any age
func DefaultKillmailLimits() KillmailLimits {
	return KillmailLimits{Max: DefaultKillmailLimit}
}

// killmailCache is a client that can say whether it already holds a killmail without fetching it, and keep one it did not fetch, ie CachedEveIDs
type killmailCache interface {
	CachedKillmail(killmailID int) *api.ESIKillmail
	StoreKillmail(killmail *api.ESIKillmail)
}

// killmailWorkers is how many killmails zKillboard sent only in part are fetched from the ESI at once. The api.RateGuard keeps them within the ESI's limits.
const killmailWorkers = 10

/*
	CollectKillmails pulls the zkill killmail list for the target and reads each killmail from it, newest first, until the limits are reached.
	Each list is paged only as far as the limits need, and no further than the cached list of it already knows, see listEntity.
	zKillboard sends the whole ESI killmail with each entry, cached like a fetched one when it is listed. Only an entry missing part of it, that the cache does not hold,
	is fetched from the ESI, at most MaxESIKillmails of them. Those are fetched killmailWorkers at a time before the date limit is known,
	so up to killmailWorkers past NewerThan may be fetched, and cached, before it is seen.
	Names, pilots, corps, and the rest the killmails refer to are looked up from the ESI afterwards, see PrefetchKillmailEntities.

Parameters:
  - target: the IntelTarget to collect killmails for
  - limits: how many killmails, and how far back, to collect. With IncludeCached, killmails beyond the limits are still used if they are in the cache.
    When even the newest killmail is older than NewerThan, ConfirmIgnoreDate decides whether to drop the date limit for this search, listing again further back if so.

Returns:
  - *[]RawKillmail: every killmail that was read from zkill, the cache, or the ESI
  - error: nil, or ErrIncompleteKillmails wrapping the underlying errors if the zkill pull was partial or any killmail fetched from the ESI was skipped - the collected killmails are still returned.
    A 429 from the ESI skips the rest of the killmails it was needed for, wrapping api.ErrRateLimited. A halted api.RateGuard stops the collection there, wrapping api.ErrStopped.
    Those past MaxESIKillmails are skipped too, wrapping ErrTooManyESIKillmails.
*/
func CollectKillmails(target IntelTarget, limits KillmailLimits) (*[]RawKillmail, error) {

	esiClient := target.Clients.ESIClient
	cache, _ := esiClient.(killmailCache)

	allRawKillmails := []RawKillmail{}
	var errs []error

	var killmails *[]api.ZkbResponse
	var via []*TargetGroup
	var within int
	list := func() {
		comm.ReportProgress("Fetching killmail list from zKillboard (live, only as far as the cached list does not have it)")
		paging, need := limits.paging()
		var err error
		killmails, via, within, err = listKillmails(target, paging, need)
		if err != nil {
			comm.Logger.WithError(err).Warnf("[Zkill] Only retrieved %d killmails, continuing with partial results", len(*killmails))
			comm.ReportProgress("zKillboard pull incomplete (%s), continuing with %d killmails", api.FailureReason(err), len(*killmails))
			errs = append(errs, err)
		}
	}
	list()

	// paging stopped at the date limit, so when even the newest killmail is older, the limit may be dropped and the list read further back
	if !limits.NewerThan.IsZero() && limits.ConfirmIgnoreDate != nil && within > 0 {
		if newest := listedTime((*killmails)[0], cache); !newest.IsZero() && !newest.After(limits.NewerThan) {
			ask := limits.ConfirmIgnoreDate
			limits.ConfirmIgnoreDate = nil // asked once
			if ask(newest) {
				comm.ReportProgress("The newest killmail is older than %s, collecting without the date limit for this search", api.FormatShortDate(limits.NewerThan))
				limits.NewerThan = time.Time{}
				errs = nil
				list()
			}
		}
	}

	total := within
	if limits.Max != AllKillmails {
		total = min(total, limits.Max)
	}

	// what the cache held before any fetching, so a killmail fetched ahead of the NewerThan limit is not then used as if it had been cached.
	// Only those within Max are looked up, and the rest only to use them from cache.
	cached := make([]*api.ESIKillmail, len(*killmails))
	lookedUp := total
	if limits.IncludeCached {
		lookedUp = len(*killmails)
	}
	if cache != nil {
		for index := range lookedUp {
			cached[index] = cache.CachedKillmail((*killmails)[index].KillmailID)
		}
	}

	// each killmail is the cache's copy, else zKillboard's, else fetched from the ESI
	bodies := make([]*api.ESIKillmail, total)
	fromZkill := make([]bool, total)
	missing := []api.ZkbResponse{}
	missingAt := map[int]int{} // a killmail's index in the list, to its index in missing
	overLimit := 0             // killmails that would need the ESI past MaxESIKillmails, skipped
	for index := range total {
		if cached[index] != nil {
			bodies[index] = cached[index]
		} else if body := (*killmails)[index].Killmail(); body != nil {
			bodies[index], fromZkill[index] = body, true
		} else if len(missing) < MaxESIKillmails {
			missingAt[index] = len(missing)
			missing = append(missing, (*killmails)[index])
		} else {
			overLimit++
		}
	}
	if overLimit > 0 {
		comm.ReportProgress("Skipping %d killmails zKillboard sent only in part, past the %d a search fetches from the ESI", overLimit, MaxESIKillmails)
		errs = append(errs, fmt.Errorf("%w: %d skipped", ErrTooManyESIKillmails, overLimit))
	}
	stop := make(chan struct{})
	defer close(stop)
	results := fetchKillmails(esiClient, missing, stop)

	var readCached, readZkill, fetched, rateLimited int
	beyondLimits := total // where the killmails only used from cache begin
	for index := range total {
		km := (*killmails)[index]
		killmail := bodies[index]

		if position, found := missingAt[index]; found {
			result := <-results[position]

			progress := fmt.Sprintf("Killmail details from the ESI %d/%d #%d", position+1, len(missing), km.KillmailID)
			source := CacheCounts{Fetched: 1}
			if result.err != nil {
				progress = fmt.Sprintf("Skipped killmail %d/%d #%d", position+1, len(missing), km.KillmailID)
				source = CacheCounts{Failed: 1, Reason: api.FailureReason(result.err)}
			}
			comm.ReportProgress("%s", source.Annotate(progress))

			// a stopped search sends nothing more, so it ends here
			if errors.Is(result.err, api.ErrStopped) {
				comm.ReportProgress("Stopped collecting killmails, continuing with the %d collected", len(allRawKillmails))
				return &allRawKillmails, fmt.Errorf("%w: %w", ErrIncompleteKillmails, result.err)
			}
			// the ESI holds the killmail group for a while after a 429, so every killmail still to fetch fails too. Those zKillboard sent whole are still used.
			if errors.Is(result.err, api.ErrRateLimited) {
				if rateLimited == 0 {
					errs = append(errs, result.err)
				}
				rateLimited++
				continue
			}
			if result.err != nil {
				comm.Logger.WithError(result.err).Warnf("[ESI] Skipping killmail %d", km.KillmailID)
				errs = append(errs, fmt.Errorf("killmail %d: %w", km.KillmailID, result.err))
				continue
			}
			killmail = result.killmail
		} else if killmail == nil {
			continue // past MaxESIKillmails
		}

		// zKillboard lists newest first, so once one killmail is too old the rest are too. When even the newest is, the date limit may be dropped instead.
		if !limits.NewerThan.IsZero() && !killmail.KillmailTime.After(limits.NewerThan) && len(allRawKillmails) == 0 &&
			limits.ConfirmIgnoreDate != nil && limits.ConfirmIgnoreDate(killmail.KillmailTime) {
			comm.ReportProgress("The newest killmail is older than %s, collecting without the date limit for this search", api.FormatShortDate(limits.NewerThan))
			limits.NewerThan = time.Time{}
		}
		if !limits.NewerThan.IsZero() && !killmail.KillmailTime.After(limits.NewerThan) {
			comm.ReportProgress("Reached killmails older than %s", api.FormatShortDate(limits.NewerThan))
			beyondLimits = index
			break
		}

		switch {
		case fromZkill[index]:
			readZkill++ // cached when it was listed, see storeBodies
		case cached[index] != nil:
			readCached++
		default:
			fetched++
		}
		allRawKillmails = append(allRawKillmails, RawKillmail{Zkillmail: &km, ESIKillmail: killmail, Via: via[index]})
	}

	if rateLimited > 0 {
		comm.ReportProgress("The ESI rate limited killmail requests, skipped %d killmails zKillboard sent only in part", rateLimited)
	}
	comm.ReportProgress("Killmails read: %d from zKillboard, %d from cache, %d from the ESI", readZkill, readCached, fetched)

	// beyond the limits, only killmails the cache already held are used
	extraFromCache := 0
	if limits.IncludeCached {
		for index := beyondLimits; index < len(*killmails); index++ {
			if cached[index] != nil {
				allRawKillmails = append(allRawKillmails, RawKillmail{Zkillmail: &(*killmails)[index], ESIKillmail: cached[index], Via: via[index]})
				extraFromCache++
			}
		}
	}

	if extraFromCache > 0 {
		comm.ReportProgress("Added %d more killmails from cache beyond the limits", extraFromCache)
	}

	// the holdings' lists and the cached killmails beyond the limits are each in order, but not with each other
	slices.SortStableFunc(allRawKillmails, func(a, b RawKillmail) int { return b.ESIKillmail.KillmailTime.Compare(a.ESIKillmail.KillmailTime) })

	if len(errs) > 0 {
		return &allRawKillmails, fmt.Errorf("%w: %w", ErrIncompleteKillmails, errors.Join(errs...))
	}

	return &allRawKillmails, nil
}

// listedTime is a listed killmail's time, from zKillboard's copy or the cache's, the zero time when neither has it
func listedTime(km api.ZkbResponse, cache killmailCache) time.Time {
	if !km.KillmailTime.IsZero() || cache == nil {
		return km.KillmailTime
	}
	if cachedCopy := cache.CachedKillmail(km.KillmailID); cachedCopy != nil {
		return cachedCopy.KillmailTime
	}
	return time.Time{}
}

/*
	listKillmails lists the target's killmails, and those of each of its Holdings, merged newest first with each killmail once.
	Each list is newest first, so the newest of the merged list are among the newest of each, and every list is paged alike. See listEntity.
	A killmail with no time is sorted by the cache's copy. Should any have no time at all, the list is sorted by killmail id instead,
	as ids are given out in nearly the order killmails happen.

Parameters:
  - target: the IntelTarget, with any Holdings
  - paging: how far to page through each list
  - need: how many killmails each list needs from its top

Returns:
  - *[]api.ZkbResponse: the merged list, those within the limits first, then those only from the cached lists beyond them. A killmail on the target's own list and a holding's is the target's.
  - []*TargetGroup: for each killmail on the list, the holding it was listed for, nil when it is on the target's own list
  - int: how many from the top of the merged list are within the limits
  - error: nil, or the zkill pull errors, the lists that were pulled are still merged
*/
func listKillmails(target IntelTarget, paging api.ZkillPaging, need int) (*[]api.ZkbResponse, []*TargetGroup, int, error) {
	type listed struct {
		killmail api.ZkbResponse
		via      *TargetGroup
		at       time.Time
		beyond   bool // only from a cached list, beyond the limits
	}
	cache, _ := target.Clients.ESIClient.(killmailCache)
	merged := []listed{}
	seen := map[int]bool{}
	untimed := false
	add := func(list entityList, via *TargetGroup) {
		for index, km := range list.killmails {
			if seen[km.KillmailID] {
				continue
			}
			seen[km.KillmailID] = true
			at := listedTime(km, cache)
			untimed = untimed || at.IsZero()
			merged = append(merged, listed{killmail: km, via: via, at: at, beyond: index >= list.within})
		}
	}

	errs := []error{}
	// with holdings the ticker says whose list each page is, as each list starts again from page 1
	mainPaging := paging
	if len(target.Holdings) > 0 {
		mainPaging.For = target.displayName() + " (main)"
	}
	own, err := listEntity(target, target.TypeOfEntity, target.IDNumber, mainPaging, need)
	if err != nil {
		errs = append(errs, err)
	}
	add(own, nil)
	for index := range target.Holdings {
		holding := &target.Holdings[index]
		comm.ReportProgress("Fetching killmail list for holding %s from zKillboard (live)", holding.Name)
		holdingPaging := paging
		holdingPaging.For = "holding " + holding.Name
		list, err := listEntity(target, holding.Type, holding.ID, holdingPaging, need)
		if err != nil {
			errs = append(errs, fmt.Errorf("holding %s: %w", holding.Name, err))
		}
		add(list, holding)
	}

	newestFirst := func(a, b listed) int { return b.at.Compare(a.at) }
	if untimed && len(target.Holdings) == 0 {
		newestFirst = func(a, b listed) int { return 0 } // a single list keeps zKillboard's order
	} else if untimed {
		newestFirst = func(a, b listed) int { return b.killmail.KillmailID - a.killmail.KillmailID }
	}
	// those within the limits come first, then those beyond them
	slices.SortStableFunc(merged, func(a, b listed) int {
		if a.beyond != b.beyond {
			if b.beyond {
				return -1
			}
			return 1
		}
		return newestFirst(a, b)
	})
	all := make([]api.ZkbResponse, len(merged))
	via := make([]*TargetGroup, len(merged))
	within := 0
	for index, entry := range merged {
		all[index], via[index] = entry.killmail, entry.via
		if !entry.beyond {
			within++
		}
	}
	return &all, via, within, errors.Join(errs...)
}

// fetchedKillmail is the ESI's answer for one killmail
type fetchedKillmail struct {
	killmail *api.ESIKillmail
	err      error
}

/*
	fetchKillmails starts fetching killmails from the ESI in the background, killmailWorkers at a time, in list order

Parameters:
  - client: the ESI client, normally the cache
  - killmails: the zKillboard list entries to fetch
  - stop: closing it starts no more fetches, those already started still finish

Returns:
  - []chan fetchedKillmail: one per killmail, in the same order, each receiving its result once fetched
*/
func fetchKillmails(client api.ESIClient, killmails []api.ZkbResponse, stop <-chan struct{}) []chan fetchedKillmail {
	results := make([]chan fetchedKillmail, len(killmails))
	for index := range results {
		results[index] = make(chan fetchedKillmail, 1)
	}

	go func() {
		workers := make(chan struct{}, killmailWorkers)
		for index, km := range killmails {
			select {
			case <-stop:
				return
			case workers <- struct{}{}:
			}
			go func() {
				defer func() { <-workers }()
				killmail, err := client.GetKillmail(km)
				results[index] <- fetchedKillmail{killmail: killmail, err: err}
			}()
		}
	}()

	return results
}

func (rkm *RawKillmail) Expand(esiClient *api.ESIClient, cached *CachedEveIDs) *Killmail {
	defer func() {
		if r := recover(); r != nil {
			comm.Logger.Errorf("Failed to Expand Killmail %d / %s", rkm.ESIKillmail.KillmailID, rkm.Zkillmail.Zkb.Hash)
		}
	}()

	killmail := Killmail{
		KillmailID:     rkm.Zkillmail.KillmailID,
		Hash:           rkm.Zkillmail.Zkb.Hash,
		KillmailTime:   rkm.ESIKillmail.KillmailTime,
		FittedValue:    rkm.Zkillmail.Zkb.FittedValue,
		DroppedValue:   rkm.Zkillmail.Zkb.DroppedValue,
		DestroyedValue: rkm.Zkillmail.Zkb.DestroyedValue,
		TotalValue:     rkm.Zkillmail.Zkb.TotalValue,
		DroppableValue: rkm.Zkillmail.Zkb.TotalDroppableValue,
		Labels:         rkm.Zkillmail.Zkb.Labels,
		NPC:            rkm.Zkillmail.Zkb.NPC,
		Solo:           rkm.Zkillmail.Zkb.Solo,
		Awox:           rkm.Zkillmail.Zkb.Awox,
		Position:       &rkm.ESIKillmail.Victim.Position,
		LocationID:     rkm.Zkillmail.Zkb.LocationID,
		Via:            rkm.Via,
	}

	killmail.System = cached.ExpandSystem(rkm.ESIKillmail.SolarSystemID)

	killmail.Victim = &KillmailVictim{
		Ship:        cached.ExpandItem(rkm.ESIKillmail.Victim.ShipTypeID),
		DamageTaken: rkm.ESIKillmail.Victim.DamageTaken,
		Items:       rkm.ESIKillmail.Victim.Items,
	}
	// structure and deployable losses have no pilot, only the owning corp and alliance
	if rkm.ESIKillmail.Victim.CharacterID != nil {
		killmail.Victim.Pilot = cached.ExpandPilot(*rkm.ESIKillmail.Victim.CharacterID, true)
	}
	if rkm.ESIKillmail.Victim.CorporationID != nil {
		killmail.Victim.CorpAtKill = cached.ExpandCorporation(*rkm.ESIKillmail.Victim.CorporationID, false)
	}
	if rkm.ESIKillmail.Victim.AllianceID != nil {

		killmail.Victim.AllianceAtKill = cached.ExpandAlliance(*rkm.ESIKillmail.Victim.AllianceID)
	}

	attackersExpanded := []*KillmailAttacker{}
	for _, attacker := range rkm.ESIKillmail.Attackers {
		atkr := &KillmailAttacker{
			DamageDone: attacker.DamageDone,
			FinalBlow:  attacker.FinalBlow,
			Ship:       cached.ExpandItem(attacker.ShipTypeID),
		}

		if attacker.CharacterID != nil {
			atkr.Pilot = cached.ExpandPilot(*attacker.CharacterID, true)
		} else {
			atkr.Pilot = &Pilot{
				CharacterID: attacker.ShipTypeID,
				NPC:         true,
			}
		}
		if attacker.CorporationID != nil {
			atkr.CorpAtKill = cached.ExpandCorporation(*attacker.CorporationID, false)
		}

		if attacker.AllianceID != nil {
			atkr.AllianceAtKill = cached.ExpandAlliance(*attacker.AllianceID)
		}

		if attacker.WeaponTypeID != nil {
			atkr.Weapon = cached.ExpandItem(*attacker.WeaponTypeID)
		}

		attackersExpanded = append(attackersExpanded, atkr)
	}

	killmail.Attackers = attackersExpanded

	return &killmail

}

/*
	ExpandPilot takes a character ID and attempts to find its expanded or raw esi data in the cache, or update from the esi. when found it constructs a Pilot object

Parameters:
  - characterID: the id of the character to expand
  - expandCorporation: a boolean, if true will also expand the corpation

Returns
  - *Pilot - an expanded, human readable pilot object
*/
func (cached *CachedEveIDs) ExpandPilot(characterID int, expandCorporation bool) *Pilot {

	if alreadyExpanded := FetchExpanded[Pilot](cached, characterID); alreadyExpanded != nil {
		return alreadyExpanded
	}

	temp, err := cached.GetIfNotCached(characterID, types.PILOT)
	pilot, _ := (*temp).(*api.ESICharacter)
	if err != nil || pilot == nil {
		// the killmail is still used, with the pilot unnamed for now. Not cached, so the next search tries the ESI again.
		comm.Logger.WithError(err).Warnf("Unable to pull Character ID %d, showing it as unknown", characterID)
		return &Pilot{CharacterID: characterID, Name: "(Unknown pilot)"}
	}

	return cached.buildPilot(characterID, pilot.Name, pilot.CorporationID, expandCorporation)
}

// buildPilot makes and caches the Pilot model from a character's name and current corp. A deleted character's corp may be 0, the killmail still has the corp they flew for.
func (cached *CachedEveIDs) buildPilot(characterID int, name string, corporationID int, expandCorporation bool) *Pilot {
	expanded := &Pilot{
		CharacterID: characterID,
		Name:        name,
	}

	if expandCorporation && corporationID != 0 {
		expanded.Corporation = cached.ExpandCorporation(corporationID, true)
	}

	cached.UpdateExpanded(characterID, expanded)

	return expanded
}

func (cached *CachedEveIDs) ExpandCorporation(corporationID int, expandAlliance bool) *Corporation {

	// a corp first cached without its alliance looked up (ie from a killmail) is rebuilt when the alliance is wanted, from the raw esi cache.
	// One in no alliance has been looked up already, AllianceChecked says so.
	if alreadyExpanded := FetchExpanded[Corporation](cached, corporationID); alreadyExpanded != nil && (!expandAlliance || alreadyExpanded.AllianceChecked) {
		return alreadyExpanded
	}

	temp, err := cached.GetIfNotCached(corporationID, types.CORPORATION)
	corp, _ := (*temp).(*api.ESICorporation)
	if corp == nil {
		// not cached, so the next search tries the ESI again
		comm.Logger.WithError(err).Warnf("Unable to pull Corporation ID %d, showing it as unknown", corporationID)
		return &Corporation{CorporationID: corporationID, Name: "(Unknown corporation)"}
	}

	expanded := &Corporation{
		CorporationID: corporationID,
		Name:          corp.Name,
		CeoID:         corp.CeoID,
		Ticker:        corp.Ticker,
	}

	if expandAlliance {
		expanded.Alliance = cached.ExpandAlliance(corp.AllianceID)
		if expanded.Alliance != nil && expanded.Alliance.Name == unknownAllianceName {
			return expanded // not cached with a placeholder alliance, so the next lookup tries the ESI again
		}
		expanded.AllianceChecked = true
	}

	cached.UpdateExpanded(corporationID, expanded)

	return expanded
}

// unknownAllianceName, unknownTypeName, and unknownSystemName stand in for an alliance, type, or system the ESI could not be reached for. They are never cached.
const (
	unknownAllianceName = "(Unknown alliance)"
	unknownTypeName     = "(Unknown type)"
	unknownSystemName   = "(Unknown system)"
)

// noTypeName names type 0, an attacker's ship or weapon the killmail does not give
const noTypeName = "Unknown"

func (cached *CachedEveIDs) ExpandAlliance(allianceID int) *Alliance {

	if allianceID == 0 {
		return nil
	}
	if alreadyExpanded := FetchExpanded[Alliance](cached, allianceID); alreadyExpanded != nil {
		return alreadyExpanded
	}

	temp, err := cached.GetIfNotCached(allianceID, types.ALLIANCE)
	alliance, _ := (*temp).(*api.ESIAlliance)
	if alliance == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull Alliance ID %d, showing it as unknown", allianceID)
		return &Alliance{AllianceID: allianceID, Name: unknownAllianceName}
	}

	expanded := &Alliance{
		AllianceID:            allianceID,
		Name:                  alliance.Name,
		ExecutorCorporationID: alliance.ExecutorCorporationID,
		Ticker:                alliance.Ticker,
	}

	cached.UpdateExpanded(allianceID, expanded)

	return expanded

}

func (cached *CachedEveIDs) ExpandItem(itemID int) *Item {
	// a killmail gives type 0 for an attacker with no known ship or weapon. The ESI answers it with a placeholder that has no id, which is never cached, so it is not asked for.
	if itemID == 0 {
		return &Item{ItemID: 0, TypeOfEntity: types.ITEM, Name: noTypeName}
	}
	if alreadyExpanded := FetchExpanded[Item](cached, itemID); alreadyExpanded != nil {
		return alreadyExpanded
	}

	temp, err := cached.GetIfNotCached(itemID, types.ITEM)
	item, _ := (*temp).(*api.ESIItem)
	if item == nil {
		// the killmail is still used, with the type unnamed. Not cached, so the next lookup tries the ESI again.
		comm.Logger.WithError(err).Warnf("Unable to pull Type ID %d, showing it as unknown", itemID)
		return &Item{ItemID: itemID, TypeOfEntity: types.ITEM, Name: unknownTypeName}
	}

	expanded := &Item{
		ItemID:       itemID,
		TypeOfEntity: types.ITEM,
		Name:         item.Name,
	}

	cached.UpdateExpanded(itemID, expanded)

	return expanded
}

func (cached *CachedEveIDs) ExpandSystem(systemID int) *System {
	if alreadyExpanded := FetchExpanded[System](cached, systemID); alreadyExpanded != nil {
		return alreadyExpanded
	}
	temp, err := cached.GetIfNotCached(systemID, types.SYSTEM)
	system, _ := (*temp).(*api.ESISystem)
	if system == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull System ID %d, showing it as unknown", systemID)
		return &System{SystemID: systemID, Name: unknownSystemName}
	}

	expanded := &System{
		SystemID:       systemID,
		Name:           system.Name,
		SecurityStatus: system.SecurityStatus,
	}

	// without its constellation and region the system is still used, but not cached, so they are tried again next time
	tempCon, _ := cached.GetEsiResponse(system.ConstellationID, types.CONSTELLATION)
	constellation, _ := tempCon.(*api.ESIConstellation)
	if constellation == nil {
		return expanded
	}
	expanded.Constellation = constellation.Name

	tempReg, _ := cached.GetEsiResponse(constellation.RegionID, types.REGION)
	region, _ := tempReg.(*api.ESIRegion)
	if region == nil {
		return expanded
	}
	expanded.Region = region.Name

	cached.UpdateExpanded(systemID, expanded)

	return expanded

}
