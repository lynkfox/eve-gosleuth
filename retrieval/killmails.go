package retrieval

import (
	"errors"
	"fmt"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ErrIncompleteKillmails is returned alongside the killmails that were collected when some could not be retrieved
var ErrIncompleteKillmails = errors.New("not all killmails could be collected")

/*
	CollectKillmails pulls the zkill killmail list for the target, then the full ESI killmail for each.

Parameters:
  - target: the IntelTarget to collect killmails for

Returns:
  - *[]RawKillmail: every killmail that was successfully retrieved from both zkill and the ESI
  - error: nil, or ErrIncompleteKillmails wrapping the underlying errors if the zkill pull was partial or any ESI killmails were skipped - the collected killmails are still returned
*/
func CollectKillmails(target IntelTarget) (*[]RawKillmail, error) {

	zkillClient := target.Clients.ZkillClient
	esiClient := target.Clients.ESIClient

	allRawKillmails := []RawKillmail{}
	var errs []error

	killmails, err := zkillClient.GetZkillResults(target.IDNumber, target.TypeOfEntity)

	if err != nil {
		comm.Logger.WithError(err).Warnf("[Zkill] Only retrieved %d killmails, continuing with partial results", len(*killmails))
		errs = append(errs, err)
	}

	for index, km := range *killmails {

		if (index >= 10 && index%10 == 0) || index == 0 {
			comm.Logger.Infof("[ESI] Fetching killmails: %d/%d", index, len(*killmails))

		}
		esiKM, err := esiClient.GetKillmail(km)
		if err != nil {
			comm.Logger.WithError(err).Warnf("[ESI] Skipping killmail %d", km.KillmailID)
			errs = append(errs, fmt.Errorf("killmail %d: %w", km.KillmailID, err))
		} else {
			allRawKillmails = append(allRawKillmails, RawKillmail{Zkillmail: &km, ESIKillmail: esiKM})
		}

		if index == 10 {
			break
		}
	}

	if len(errs) > 0 {
		return &allRawKillmails, fmt.Errorf("%w: %w", ErrIncompleteKillmails, errors.Join(errs...))
	}

	return &allRawKillmails, nil
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
		NPC:            rkm.Zkillmail.Zkb.NPC,
		Solo:           rkm.Zkillmail.Zkb.Solo,
		Awox:           rkm.Zkillmail.Zkb.Awox,
		Position:       &rkm.ESIKillmail.Victim.Position,
	}

	killmail.System = cached.ExpandSystem(rkm.ESIKillmail.SolarSystemID)

	killmail.Victim = &KillmailVictim{
		Pilot:       cached.ExpandPilot(*rkm.ESIKillmail.Victim.CharacterID, true),
		Ship:        cached.ExpandItem(rkm.ESIKillmail.Victim.ShipTypeID),
		DamageTaken: rkm.ESIKillmail.Victim.DamageTaken,
	}
	if rkm.ESIKillmail.Victim.CorporationID != nil {
		killmail.Victim.CorpAtKill = cached.ExpandCorporation(*rkm.ESIKillmail.Victim.CorporationID, false)
	}
	if rkm.ESIKillmail.Victim.AllianceID != nil {

		killmail.Victim.AllianceAtKill = cached.ExpandAlliance(*rkm.ESIKillmail.Victim.AllianceID)
	}

	attackersExpanded := []*KillmailAttacker{}
	for _, attacker := range rkm.ESIKillmail.Attackers {

		defer func() {
			if r := recover(); r != nil {
				comm.Logger.Errorf("!!!!****!!!! Failed to expand killmail %d on attacker in ship type %d !!!!****!!!!", rkm.ESIKillmail.KillmailID, attacker.ShipTypeID)
			}
		}()

		atkr := &KillmailAttacker{
			DamageDone: attacker.DamageDone,
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

	alreadyExpanded := cached.FetchExpanded(characterID)
	if alreadyExpanded != nil {

		casted, err := CastExpanded[Pilot](alreadyExpanded)
		if err != nil {
			comm.Logger.Errorf("Could not expand Pilot %d: %v", characterID, err)
		}
		return casted

	}

	temp, _ := cached.GetIfNotCached(characterID, types.PILOT)
	pilot := (*temp).(*api.ESICharacter)

	expanded := &Pilot{
		CharacterID: characterID,
		Name:        pilot.Name,
	}

	if expandCorporation && expanded.Corporation == nil {
		expanded.Corporation = cached.ExpandCorporation(pilot.CorporationID, true)
	}

	cached.UpdateExpanded(characterID, expanded)

	return expanded
}

func (cached *CachedEveIDs) ExpandCorporation(corporationID int, expandAlliance bool) *Corporation {

	alreadyExpanded := cached.FetchExpanded(corporationID)

	if alreadyExpanded != nil {

		casted, err := CastExpanded[Corporation](alreadyExpanded)
		if err != nil {
			comm.Logger.Errorf("Could not expand Corporation %d: %v", corporationID, err)
		}
		return casted

	}

	temp, _ := cached.GetIfNotCached(corporationID, types.CORPORATION)

	corp := (*temp).(*api.ESICorporation)

	expanded := &Corporation{
		CorporationID: corporationID,
		Name:          corp.Name,
		CeoID:         corp.CeoID,
		Ticker:        corp.Ticker,
	}

	if expandAlliance && expanded.Alliance == nil {
		expanded.Alliance = cached.ExpandAlliance(corp.AllianceID)
	}

	cached.UpdateExpanded(corporationID, expanded)

	return expanded
}

func (cached *CachedEveIDs) ExpandAlliance(allianceID int) *Alliance {

	if allianceID == 0 {
		return nil
	}
	alreadyExpanded := cached.FetchExpanded(allianceID)
	if alreadyExpanded != nil {

		casted, err := CastExpanded[Alliance](alreadyExpanded)
		if err != nil {
			comm.Logger.Errorf("Could not expand Alliance %d: %v", allianceID, err)
		}
		return casted

	}

	temp, _ := cached.GetIfNotCached(allianceID, types.ALLIANCE)
	alliance := (*temp).(*api.ESIAlliance)

	expanded := &Alliance{
		AllianceID:            allianceID,
		Name:                  alliance.Name,
		ExecutorCorporationID: alliance.ExecutorCorporationID,
		Ticker:                alliance.Ticker,
	}

	return expanded

}

func (cached *CachedEveIDs) ExpandItem(itemID int) *Item {
	alreadyExpanded := cached.FetchExpanded(itemID)
	if alreadyExpanded != nil {

		casted, err := CastExpanded[Item](alreadyExpanded)
		if err != nil {
			comm.Logger.Errorf("Could not expand Item %d: %v", itemID, err)
		}
		return casted

	}

	temp, _ := cached.GetIfNotCached(itemID, types.ITEM)
	item := (*temp).(*api.ESIItem)

	expanded := &Item{
		ItemID:       itemID,
		TypeOfEntity: types.ITEM,
		Name:         item.Name,
	}

	cached.UpdateExpanded(itemID, expanded)

	return expanded
}

func (cached *CachedEveIDs) ExpandSystem(systemID int) *System {
	alreadyExpanded := cached.FetchExpanded(systemID)
	if alreadyExpanded != nil {

		casted, err := CastExpanded[System](alreadyExpanded)
		if err != nil {
			comm.Logger.Errorf("Could not expand System %d: %v", systemID, err)
		}
		return casted

	}
	temp, _ := cached.GetIfNotCached(systemID, types.SYSTEM)
	system := (*temp).(*api.ESISystem)

	expanded := &System{
		SystemID:       systemID,
		Name:           system.Name,
		SecurityStatus: system.SecurityStatus,
	}
	tempCon, _ := cached.Client.GetEsiResponse(system.ConstellationID, types.CONSTELLATION)

	if tempCon != nil {
		constellation := tempCon.(*api.ESIConstellation)
		expanded.Constellation = constellation.Name
		tempReg, _ := cached.Client.GetEsiResponse(constellation.RegionID, types.REGION)

		if tempReg != nil {
			region := tempReg.(*api.ESIRegion)
			expanded.Region = region.Name
		}
	}

	cached.UpdateExpanded(systemID, expanded)

	return expanded

}
