package gui

import (
	"fmt"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// ReportScope is what a search covered, for At a glance's Scope card and the PDF report
type ReportScope struct {
	Limits  []string      `json:"limits"`  // the search limits used, as plain phrases, see scopeLimits
	Sources []ScopeSource `json:"sources"` // the target then each affiliate, in the holdings' order, empty when the search had none. A coalition has no row of its own: each member then each affiliate.
}

// ScopeSource is the killmails one list of the search gave: the target's own, or an affiliate's
type ScopeSource struct {
	Name      string    `json:"name"`
	Main      bool      `json:"main"` // the target itself, never set for a coalition
	Killmails int       `json:"killmails"`
	Oldest    time.Time `json:"oldest"` // zero with no killmails
	Newest    time.Time `json:"newest"`
}

/*
	buildScope says what a search covered: the limits it used and, when it had affiliates, the killmails each list gave

Parameters:
  - target: the searched target, with the holdings the search included
  - cards: the report's killmails, each with the holding it came from (Via), empty for the target's own
  - limits: the search limits used

Returns:
  - ReportScope: the scope, its Sources empty without affiliates
*/
func buildScope(target *retrieval.IntelTarget, cards []KillmailCard, limits SearchLimits) ReportScope {
	scope := ReportScope{Limits: scopeLimits(limits), Sources: []ScopeSource{}}
	if len(target.Holdings) == 0 {
		return scope
	}
	// killmails carry only their holding's name (Via), so two affiliates sharing a name, ie a corp and an alliance, count as one: rare enough to accept
	index := map[string]int{}
	if target.TypeOfEntity != types.COALITION {
		index[""] = 0
		scope.Sources = append(scope.Sources, ScopeSource{Name: target.DisplayName(), Main: true})
	}
	for _, holding := range target.Holdings {
		index[holding.Name] = len(scope.Sources)
		scope.Sources = append(scope.Sources, ScopeSource{Name: holding.Name})
	}
	for _, card := range cards {
		i, found := index[card.Via]
		if !found {
			continue
		}
		source := &scope.Sources[i]
		source.Killmails++
		if source.Oldest.IsZero() || card.Time.Before(source.Oldest) {
			source.Oldest = card.Time
		}
		if card.Time.After(source.Newest) {
			source.Newest = card.Time
		}
	}
	return scope
}

// scopeLimits describes the search limits as plain phrases, worded as the search limits button is. The affiliates are their own Sources, so IncludeHoldings has none.
func scopeLimits(limits SearchLimits) []string {
	phrases := []string{}
	if limits.MaxKillmails == retrieval.AllKillmails && limits.KillmailPages == retrieval.AllPages {
		phrases = append(phrases, "All killmails zKillboard lists")
	} else if limits.MaxKillmails == retrieval.AllKillmails {
		pages := "pages"
		if limits.KillmailPages == 1 {
			pages = "page"
		}
		phrases = append(phrases, fmt.Sprintf("All killmails on %d zKillboard %s", limits.KillmailPages, pages))
	} else {
		phrases = append(phrases, fmt.Sprintf("At most %s killmails", commaInt(limits.MaxKillmails)))
	}
	date, dateErr := time.Parse(newerThanDateLayout, limits.KillmailsNewerThanDate)
	switch days := limits.KillmailsNewerThanDays; {
	case dateErr == nil:
		phrases = append(phrases, "Killmails from "+api.FormatShortDate(date))
	case days <= 0:
		phrases = append(phrases, "Killmails of any age")
	case days == 7:
		phrases = append(phrases, "Killmails newer than 1 week")
	default:
		phrases = append(phrases, fmt.Sprintf("Killmails newer than %d days", days))
	}
	return phrases
}

// commaInt writes a count with thousands separators, ie 20,000
func commaInt(n int) string {
	digits := fmt.Sprint(n)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return sign + digits
}
