package gui

import (
	"errors"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// AllySummary is the frontend representation of an alliance, or a corporation in none, that attacked alongside the target outside its alliance
type AllySummary struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // ALLIANCE or CORPORATION, for a search
	ImageURL   string    `json:"imageUrl"`
	ZkillURL   string    `json:"zkillUrl"`
	EveWhoURL  string    `json:"evewhoUrl"`
	Killmails  int       `json:"killmails"` // the target's kills it was on
	Killed     int       `json:"killed"`    // the target's kills where the victim was in it
	Pilots     int       `json:"pilots"`
	UniqueDays int       `json:"uniqueDays"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
}

// allyLimit is how many allies the intel report ranks
const allyLimit = 10

// alliesSource is what a search's Allies are ranked from
type alliesSource struct {
	target    retrieval.IntelTarget
	killmails []*retrieval.Killmail
}

// AlliesUpdate is the Allies tab after SaveIgnoredAllies: the allies ranked again, and the ones left out
type AlliesUpdate struct {
	Allies        []AllySummary           `json:"allies"` // nil when the saved target is not the last one searched, so there is nothing to rank again
	IgnoredAllies []retrieval.IgnoredAlly `json:"ignoredAllies"`
}

// rankAllies ranks a search's allies without those the user ignored for its target, returning them and the ignored. The caller must hold a.mu.
func (a *App) rankAllies(source *alliesSource) ([]AllySummary, []retrieval.IgnoredAlly) {
	ignored := a.cache.IgnoredAlliesFor(source.target.TypeOfEntity.String(), source.target.IDNumber)
	// a holding is the same group as the target, so it is left out like the target's own alliance
	skip := a.holdingAllies(&source.target)
	for _, ally := range ignored {
		skip[aggregate.AllyKey{Type: searchTypes[ally.Type], ID: ally.ID}] = true
	}
	return buildAllySummaries(aggregate.GetTopAllies(allyLimit, source.killmails, &source.target, skip, a.GetSettings().HideKilledAllies)), ignored
}

/*
	RankAllies ranks the last search's allies again, ie after the Hide killed allies preference changed

Parameters:
  - targetID: the target's eve id
  - targetType: PILOT, CORPORATION, or ALLIANCE, as in TargetResult.Type

Returns:
  - AlliesUpdate: the target's allies and ignored allies. Allies is nil when the target is not the last one searched.
*/
func (a *App) RankAllies(targetID int, targetType string) (update AlliesUpdate, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if source := a.lastAllies; source != nil && source.target.IDNumber == targetID && source.target.TypeOfEntity.String() == targetType {
		update.Allies, update.IgnoredAllies = a.rankAllies(source)
		return update, nil
	}
	update.IgnoredAllies = a.cache.IgnoredAlliesFor(targetType, targetID)
	return update, nil
}

/*
	SaveIgnoredAllies saves which allies to leave out of a target's Allies, in this and every later search of it, and ranks the last search's allies again

Parameters:
  - targetID: the target's eve id
  - targetType: PILOT, CORPORATION, or ALLIANCE, as in TargetResult.Type
  - ignored: every ally to leave out, replacing those saved before. Empty to leave none out.

Returns:
  - AlliesUpdate: the target's allies ranked again without them, and the ignored ones as saved
  - error: a user readable message if the target type has no Allies
*/
func (a *App) SaveIgnoredAllies(targetID int, targetType string, ignored []retrieval.IgnoredAlly) (update AlliesUpdate, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if entityType, ok := searchTypes[targetType]; !ok || entityType == types.SYSTEM {
		return update, errors.New("Only pilots, corporations, and alliances have allies")
	}
	kept := []retrieval.IgnoredAlly{}
	for _, ally := range ignored {
		if ally.Type == types.ALLIANCE.String() || ally.Type == types.CORPORATION.String() {
			kept = append(kept, ally)
		}
	}
	a.cache.SetIgnoredAllies(targetType, targetID, kept)
	a.saveCache()

	update.IgnoredAllies = kept
	if source := a.lastAllies; source != nil && source.target.IDNumber == targetID && source.target.TypeOfEntity.String() == targetType {
		update.Allies, update.IgnoredAllies = a.rankAllies(source)
	}
	return update, nil
}

// buildAllySummaries converts ranked allies into their frontend representation
func buildAllySummaries(top []aggregate.AllyActivity) []AllySummary {
	allies := []AllySummary{}
	for _, ally := range top {
		allies = append(allies, AllySummary{
			ID:         ally.ID(),
			Name:       ally.Name(),
			Type:       ally.Type().String(),
			ImageURL:   api.EntityImageURL(ally.Type(), ally.ID()),
			ZkillURL:   api.ZkillEntityURL(ally.Type(), ally.ID()),
			EveWhoURL:  api.EveWhoURL(ally.Type(), ally.ID()),
			Killmails:  ally.Killmails,
			Killed:     ally.Killed,
			Pilots:     len(ally.PilotIDs),
			UniqueDays: ally.UniqueDays(),
			FirstSeen:  ally.FirstSeen(),
			LastSeen:   ally.LastSeen(),
		})
	}
	return allies
}
