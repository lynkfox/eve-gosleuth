package gui

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// FitEntry is one of a pilot, corp, or alliance target's losses with its fit, for the Fits tab
type FitEntry struct {
	KillmailID   int       `json:"killmailId"`
	Time         time.Time `json:"time"`
	ShipID       int       `json:"shipId"`
	ShipName     string    `json:"shipName"`
	ShipImageURL string    `json:"shipImageUrl"`
	PilotName    string    `json:"pilotName"` // the pilot, or the owning corp for a structure
	EFT          string    `json:"eft"`
	MissingTypes int       `json:"missingTypes"` // item types not in fit_types.json, so the fit may be incomplete
}

/*
	buildFits lists the target's losses with their fits, by ship name and then newest first.
	Non-combat hulls in fit_ignored_hulls.json (capsules, shuttles, corvettes, haulers, mining ships, structures, deployables) are left out, as is any loss with nothing fitted.

Parameters:
  - killmails: the search's expanded killmails
  - target: the pilot, corp, or alliance searched

Returns:
  - []FitEntry: the losses, empty if there are none
*/
func buildFits(killmails []*retrieval.Killmail, target *retrieval.IntelTarget) []FitEntry {
	fits := []FitEntry{}
	for _, km := range killmails {
		if km.Victim == nil || km.Victim.Fit == nil || km.Victim.Ship == nil || !aggregate.IsTargetLoss(target, km) {
			continue
		}
		if comm.FitHullIgnored(comm.FitTypeOf(km.Victim.Ship.ItemID)) || !km.Victim.Fit.HasModules() {
			continue
		}
		entry := FitEntry{
			KillmailID:   km.KillmailID,
			Time:         km.KillmailTime,
			ShipID:       km.Victim.Ship.ItemID,
			ShipName:     km.Victim.Ship.Name,
			ShipImageURL: api.TypeImageURL(km.Victim.Ship.ItemID),
			EFT:          km.Victim.Fit.EFT(retrieval.EFTTitle(km)),
			MissingTypes: len(km.Victim.Fit.MissingTypes),
		}
		switch {
		case km.Victim.Pilot != nil:
			entry.PilotName = km.Victim.Pilot.Name
		case km.Victim.CorpAtKill != nil:
			entry.PilotName = km.Victim.CorpAtKill.Name
		}
		fits = append(fits, entry)
	}
	slices.SortStableFunc(fits, func(a, b FitEntry) int {
		if byName := strings.Compare(a.ShipName, b.ShipName); byName != 0 {
			return byName
		}
		return b.Time.Compare(a.Time)
	})
	return fits
}

/*
	FitEFT writes the victim's fit on a killmail from the last pilot, corporation, or alliance search in EFT format, for the in-game "Import from clipboard"

Parameters:
  - killmailID: the killmail, which must be in the last search's report

Returns:
  - string: the fit, titled GoSleuth - <pilot>
  - error: a user readable message if the killmail is not in the last search or its victim has no fit
*/
func (a *App) FitEFT(killmailID int) (text string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	// the last search's killmails are kept with its allies, and only pilot, corporation, and alliance searches have fits
	if a.lastAllies == nil {
		return "", errors.New("Search for a pilot, corporation, or alliance first")
	}
	for _, killmail := range a.lastAllies.killmails {
		if killmail.KillmailID != killmailID {
			continue
		}
		if killmail.Victim == nil || killmail.Victim.Fit == nil {
			return "", errors.New("This killmail's victim has no fit")
		}
		return killmail.Victim.Fit.EFT(retrieval.EFTTitle(killmail)), nil
	}
	return "", errors.New("That killmail is not in the last search")
}
