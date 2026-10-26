package retrieval

import (
	"errors"
	"fmt"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// DefaultKillmailLimit is how many killmails are collected unless the user sets another limit in Preferences
const DefaultKillmailLimit = 20

// ErrIncompleteKillmails is returned alongside the killmails that were collected when some could not be retrieved
var ErrIncompleteKillmails = errors.New("not all killmails could be collected")

// KillmailLimits are the Preferences that decide which of a target's killmails are collected
type KillmailLimits struct {
	Max           int       // the most killmails to collect from zKillboard's list, newest first
	NewerThan     time.Time // only killmails after this, the zero time for no date limit
	IncludeCached bool      // also use killmails already in the cache that are beyond Max or NewerThan
}

// DefaultKillmailLimits collects the newest DefaultKillmailLimit killmails of any age
func DefaultKillmailLimits() KillmailLimits {
	return KillmailLimits{Max: DefaultKillmailLimit}
}

// killmailCache is a client that can say whether it already holds a killmail without fetching it, ie CachedEveIDs
type killmailCache interface {
	CachedKillmail(killmailID int) *api.ESIKillmail
}

// killmailWorkers is how many killmails are fetched from the ESI at once. The api.RateGuard keeps them within the ESI's limits.
const killmailWorkers = 10

/*
	CollectKillmails pulls the zkill killmail list for the target, then the full ESI killmail for each, newest first, until the limits are reached.
	Killmails are fetched killmailWorkers at a time and used in list order, so up to killmailWorkers-1 past the NewerThan limit may be fetched, and cached, before it is seen.

Parameters:
  - target: the IntelTarget to collect killmails for
  - limits: how many killmails, and how far back, to collect. With IncludeCached, killmails beyond the limits are still used if they are in the cache, without calling the ESI.

Returns:
  - *[]RawKillmail: every killmail that was successfully retrieved from both zkill and the ESI
  - error: nil, or ErrIncompleteKillmails wrapping the underlying errors if the zkill pull was partial or any ESI killmails were skipped - the collected killmails are still returned
*/
func CollectKillmails(target IntelTarget, limits KillmailLimits) (*[]RawKillmail, error) {

	zkillClient := target.Clients.ZkillClient
	esiClient := target.Clients.ESIClient
	cache, _ := esiClient.(killmailCache)

	allRawKillmails := []RawKillmail{}
	var errs []error

	comm.ReportProgress("Fetching killmail list from zKillboard (live, zKillboard is not cached)")

	killmails, err := zkillClient.GetZkillResults(target.IDNumber, target.TypeOfEntity)

	if err != nil {
		comm.Logger.WithError(err).Warnf("[Zkill] Only retrieved %d killmails, continuing with partial results", len(*killmails))
		comm.ReportProgress("zKillboard pull incomplete (%s), continuing with %d killmails", api.FailureReason(err), len(*killmails))
		errs = append(errs, err)
	}

	// what the cache held before any fetching, so a killmail fetched ahead of the NewerThan limit is not then used as if it had been cached
	cached := make([]*api.ESIKillmail, len(*killmails))
	if cache != nil {
		for index, km := range *killmails {
			cached[index] = cache.CachedKillmail(km.KillmailID)
		}
	}

	total := min(len(*killmails), limits.Max)
	stop := make(chan struct{})
	defer close(stop)
	results := fetchKillmails(esiClient, (*killmails)[:total], stop)

	beyondLimits := total // where the killmails only used from cache begin
	for index, km := range (*killmails)[:total] {
		result := <-results[index]

		progress := fmt.Sprintf("Killmail details %d/%d #%d", index+1, total, km.KillmailID)
		source := CacheCounts{Fetched: 1}
		if result.err != nil {
			progress = fmt.Sprintf("Skipped killmail %d/%d #%d", index+1, total, km.KillmailID)
			source = CacheCounts{Failed: 1, Reason: api.FailureReason(result.err)}
		} else if cached[index] != nil {
			source = CacheCounts{Cached: 1}
		}
		comm.ReportProgress("%s", source.Annotate(progress))

		if result.err != nil {
			comm.Logger.WithError(result.err).Warnf("[ESI] Skipping killmail %d", km.KillmailID)
			errs = append(errs, fmt.Errorf("killmail %d: %w", km.KillmailID, result.err))
			continue
		}

		// zKillboard lists newest first, so once one killmail is too old the rest are too
		if !limits.NewerThan.IsZero() && !result.killmail.KillmailTime.After(limits.NewerThan) {
			comm.ReportProgress("Reached killmails older than %s", api.FormatShortDate(limits.NewerThan))
			beyondLimits = index
			break
		}

		allRawKillmails = append(allRawKillmails, RawKillmail{Zkillmail: &km, ESIKillmail: result.killmail})
	}

	// beyond the limits, only killmails the cache already held are used
	extraFromCache := 0
	if limits.IncludeCached {
		for index := beyondLimits; index < len(*killmails); index++ {
			if cached[index] != nil {
				allRawKillmails = append(allRawKillmails, RawKillmail{Zkillmail: &(*killmails)[index], ESIKillmail: cached[index]})
				extraFromCache++
			}
		}
	}

	if extraFromCache > 0 {
		comm.ReportProgress("Added %d more killmails from cache beyond the limits", extraFromCache)
	}

	if len(errs) > 0 {
		return &allRawKillmails, fmt.Errorf("%w: %w", ErrIncompleteKillmails, errors.Join(errs...))
	}

	return &allRawKillmails, nil
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
	}

	killmail.System = cached.ExpandSystem(rkm.ESIKillmail.SolarSystemID)

	killmail.Victim = &KillmailVictim{
		Ship:        cached.ExpandItem(rkm.ESIKillmail.Victim.ShipTypeID),
		DamageTaken: rkm.ESIKillmail.Victim.DamageTaken,
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
