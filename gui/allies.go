package gui

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// AllySummary is the frontend representation of an alliance, or a corporation in none, that attacked alongside the target outside its alliance
type AllySummary struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // ALLIANCE or CORPORATION, for a search
	ImageURL    string    `json:"imageUrl"`
	ZkillURL    string    `json:"zkillUrl"`
	EveWhoURL   string    `json:"evewhoUrl"`
	Killmails   int       `json:"killmails"` // the target's kills it was on
	Killed      int       `json:"killed"`    // the target's kills where the victim was in it
	Pilots      int       `json:"pilots"`
	UniqueDays  int       `json:"uniqueDays"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	TogetherURL string    `json:"togetherUrl"` // zKillboard's search for its killmails with the target or a holding searched with it, see api.ZkillTogetherURL
}

// allyLimit is how many allies the intel report ranks
const allyLimit = 10

// alliesSource is what a search's Allies are ranked from
type alliesSource struct {
	target     retrieval.IntelTarget
	killmails  []*retrieval.Killmail
	showKilled bool // the user chose to show, for this search only, the allies the Hide killed allies preference leaves out
}

// AlliesUpdate is the Allies tab after SaveIgnoredAllies: the allies ranked again, and the ones left out
type AlliesUpdate struct {
	Allies        []AllySummary        `json:"allies"` // nil when the saved target is not the last one searched, so there is nothing to rank again
	IgnoredAllies []IgnoredAllySummary `json:"ignoredAllies"`
	KilledAllies  int                  `json:"killedAllies"` // how many allies the target also killed, which the Hide killed allies preference leaves out
	Holdings      []HoldingActivity    `json:"holdings"`     // a corp or alliance target's holdings, with the kills each shared with it
}

// HoldingActivity is one of a corp or alliance target's holdings, for the line at the top of its Allies, with the kills its pilots and the target's were on together
type HoldingActivity struct {
	Type      string `json:"type"` // ALLIANCE or CORPORATION
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ImageURL  string `json:"imageUrl"`
	ZkillURL  string `json:"zkillUrl"`
	EveWhoURL string `json:"evewhoUrl"`
	Killmails int    `json:"killmails"`
}

// holdingActivity lists a corp or alliance target's holdings with the kills each shared with it in a search, most first, then by name. Pilots have none.
func (a *App) holdingActivity(source *alliesSource) []HoldingActivity {
	holdings := a.targetHoldings(&source.target)
	together := aggregate.HoldingKillsTogether(source.killmails, &source.target, holdings)
	activity := []HoldingActivity{}
	for _, holding := range holdings {
		activity = append(activity, HoldingActivity{
			Type:      holding.Type.String(),
			ID:        holding.ID,
			Name:      holding.Name,
			ImageURL:  api.EntityImageURL(holding.Type, holding.ID),
			ZkillURL:  api.ZkillEntityURL(holding.Type, holding.ID),
			EveWhoURL: api.EveWhoURL(holding.Type, holding.ID),
			Killmails: together[aggregate.AllyKey{Type: holding.Type, ID: holding.ID}],
		})
	}
	slices.SortFunc(activity, func(a, b HoldingActivity) int {
		if byKillmails := cmp.Compare(b.Killmails, a.Killmails); byKillmails != 0 {
			return byKillmails
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return activity
}

// IgnoredAllySummary is an ally ignored for the target, with the target's kills it was on in the shown search
type IgnoredAllySummary struct {
	retrieval.IgnoredAlly
	Killmails int `json:"killmails"`
}

// ignoredAllySummaries pairs the ignored allies with the kills each was on, most first, then by name
func ignoredAllySummaries(ignored []retrieval.IgnoredAlly, killmails map[aggregate.AllyKey]int) []IgnoredAllySummary {
	summaries := []IgnoredAllySummary{}
	for _, ally := range ignored {
		summaries = append(summaries, IgnoredAllySummary{IgnoredAlly: ally, Killmails: killmails[aggregate.AllyKey{Type: searchTypes[ally.Type], ID: ally.ID}]})
	}
	slices.SortFunc(summaries, func(a, b IgnoredAllySummary) int {
		if byKillmails := cmp.Compare(b.Killmails, a.Killmails); byKillmails != 0 {
			return byKillmails
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return summaries
}

/*
	rankAllies ranks a search's allies without those the user ignored for its target, nor with the Hide killed allies preference those it killed,
	unless the user chose to show them for this search. The caller must hold a.mu.

Returns:
  - []AllySummary: the top allies
  - []IgnoredAllySummary: the allies ignored for the target, with the kills each was on, most first
  - int: how many allies the target also killed, whether or not they were left out
*/
func (a *App) rankAllies(source *alliesSource) ([]AllySummary, []IgnoredAllySummary, int) {
	ignored := a.cache.IgnoredAlliesFor(source.target.TypeOfEntity.String(), source.target.IDNumber)
	// a holding is the same group as the target, so it is left out like the target's own alliance
	skip := a.holdingAllies(&source.target)
	for _, ally := range ignored {
		skip[aggregate.AllyKey{Type: searchTypes[ally.Type], ID: ally.ID}] = true
	}
	ranking := aggregate.GetTopAllies(allyLimit, source.killmails, &source.target, skip, a.currentSettings().HideKilledAllies && !source.showKilled)
	return buildAllySummaries(ranking.Top, &source.target), ignoredAllySummaries(ignored, ranking.IgnoredKillmails), ranking.KilledAllies
}

/*
	RankAllies ranks the last search's allies again, ie after the Hide killed allies preference changed

Parameters:
  - targetID: the target's eve id
  - targetType: PILOT, CORPORATION, or ALLIANCE, as in TargetResult.Type
  - showKilled: show the allies the Hide killed allies preference leaves out, for this search only. Later rankings of it, ie after SaveIgnoredAllies, keep the choice.

Returns:
  - AlliesUpdate: the target's allies and ignored allies. Allies is nil when the target is not the last one searched.
*/
func (a *App) RankAllies(targetID int, targetType string, showKilled bool) (update AlliesUpdate, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if source := a.lastAllies; source != nil && source.target.IDNumber == targetID && source.target.TypeOfEntity.String() == targetType {
		source.showKilled = showKilled
		update.Allies, update.IgnoredAllies, update.KilledAllies = a.rankAllies(source)
		update.Holdings = a.holdingActivity(source)
		return update, nil
	}
	update.IgnoredAllies = ignoredAllySummaries(a.cache.IgnoredAlliesFor(targetType, targetID), nil)
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

	update.IgnoredAllies = ignoredAllySummaries(kept, nil)
	if source := a.lastAllies; source != nil && source.target.IDNumber == targetID && source.target.TypeOfEntity.String() == targetType {
		update.Allies, update.IgnoredAllies, update.KilledAllies = a.rankAllies(source)
		update.Holdings = a.holdingActivity(source)
	}
	return update, nil
}

// buildAllySummaries converts ranked allies into their frontend representation, with links to their killmails with the target and each of its holdings
func buildAllySummaries(top []aggregate.AllyActivity, target *retrieval.IntelTarget) []AllySummary {
	allies := []AllySummary{}
	for _, ally := range top {
		allies = append(allies, AllySummary{
			ID:          ally.ID(),
			Name:        ally.Name(),
			Type:        ally.Type().String(),
			ImageURL:    api.EntityImageURL(ally.Type(), ally.ID()),
			ZkillURL:    api.ZkillEntityURL(ally.Type(), ally.ID()),
			EveWhoURL:   api.EveWhoURL(ally.Type(), ally.ID()),
			Killmails:   ally.Killmails,
			Killed:      ally.Killed,
			Pilots:      len(ally.PilotIDs),
			UniqueDays:  ally.UniqueDays(),
			FirstSeen:   ally.FirstSeen(),
			LastSeen:    ally.LastSeen(),
			TogetherURL: togetherURL(ally.Type(), ally.ID(), target),
		})
	}
	return allies
}

// togetherURL is an ally's zKillboard search with the target and every holding searched with it
func togetherURL(allyType types.EntityType, allyID int, target *retrieval.IntelTarget) string {
	groups := []api.ZkillEntity{{Type: target.TypeOfEntity, ID: target.IDNumber}}
	for _, holding := range target.Holdings {
		groups = append(groups, api.ZkillEntity{Type: holding.Type, ID: holding.ID})
	}
	return api.ZkillTogetherURL(groups, api.ZkillEntity{Type: allyType, ID: allyID})
}
