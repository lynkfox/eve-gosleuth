package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// EVE_TOOLS_BR_API_URL is the api EVE Tools' battle report pages read. It is the site's own and undocumented, so a change to it is met with a plain message and the user types the sides.
const EVE_TOOLS_BR_API_URL = EVE_TOOLS_BR_URL + "/newapi"

// battleReportPolls is how many times a report still being put together is asked for, battleReportPollWait apart, as the site's own page does
const battleReportPolls = 10
const battleReportPollWait = time.Second

var (
	// ErrNotBattleReportLink is returned for text that is not a link to an EVE Tools battle report
	ErrNotBattleReportLink = errors.New("not an EVE Tools battle report link")
	// ErrBattleReportNoTeams is returned for a link to an automatic report, a system's related kills, which has no saved sides
	ErrBattleReportNoTeams = errors.New("an automatic battle report has no saved sides")
	// ErrBattleReportProcessing is returned when a report was still being put together after every poll
	ErrBattleReportProcessing = errors.New("the battle report is still being put together")
	// ErrBattleReportFailed is returned when the site answers that it could not give the report
	ErrBattleReportFailed = errors.New("the site could not give the battle report")
)

var battleReportID = regexp.MustCompile(`^[0-9A-Za-z]+$`)

/*
	ParseBattleReportLink reads the id of a saved EVE Tools battle report from a link to it

Parameters:
  - text: what was typed, ie https://br.evetools.org/br/6aebcccc8b4155c7de8bb2d8, with or without https:// and www.

Returns:
  - string: the report's id
  - error: ErrBattleReportNoTeams for a link to an automatic report, ErrNotBattleReportLink for anything else
*/
func ParseBattleReportLink(text string) (string, error) {
	text = strings.TrimSpace(text)
	if !strings.Contains(text, "://") {
		text = "https://" + text
	}
	link, err := url.Parse(text)
	if err != nil || strings.TrimPrefix(strings.ToLower(link.Hostname()), "www.") != strings.TrimPrefix(EVE_TOOLS_BR_URL, "https://") {
		return "", ErrNotBattleReportLink
	}
	parts := strings.Split(strings.Trim(link.Path, "/"), "/")
	switch {
	case parts[0] == "related":
		return "", ErrBattleReportNoTeams
	case len(parts) != 2 || parts[0] != "br" || !battleReportID.MatchString(parts[1]):
		return "", ErrNotBattleReportLink
	}
	return parts[1], nil
}

// BattleReportClient reads battle reports from EVE Tools
type BattleReportClient interface {
	GetBattleReport(id string) (*BattleReport, error)
}

// BattleReportConnection is a BattleReportClient over an APIClient
type BattleReportConnection struct {
	Client   APIClient
	PollWait time.Duration // how long between asking for a report still being put together
}

// NewBattleReportClient initizes the connection data for an ApiConnection as a BattleReportClient. A large report is many megabytes, so it has longer than the other sites.
func NewBattleReportClient() *BattleReportConnection {
	return &BattleReportConnection{
		Client: APIConnection{
			BaseURL:    EVE_TOOLS_BR_API_URL,
			HTTPClient: &http.Client{Timeout: 60 * time.Second},
			Headers:    map[string]string{"User-Agent": USER_AGENT, "accept": "application/json"},
			MaxRetries: 2,
			Backoff:    500 * time.Millisecond,
		},
		PollWait: battleReportPollWait,
	}
}

/*
	GetBattleReport reads a saved battle report, asking again while the site is still putting it together

Parameters:
  - id: the report's id, from ParseBattleReportLink

Returns:
  - *BattleReport: the report
  - error: ErrBattleReportProcessing or ErrBattleReportFailed for what the site answers, or the request's error
*/
func (b BattleReportConnection) GetBattleReport(id string) (*BattleReport, error) {
	for poll := 0; poll < battleReportPolls; poll++ {
		if poll > 0 {
			time.Sleep(b.PollWait)
		}
		report, err := b.read(id)
		if err != nil {
			return nil, err
		}
		switch report.Status {
		case "success":
			return report, nil
		case "processing":
			continue
		}
		comm.Logger.Warnf("[EVE Tools] Battle report %s answered status %q", id, report.Status)
		return nil, ErrBattleReportFailed
	}
	return nil, ErrBattleReportProcessing
}

func (b BattleReportConnection) read(id string) (*BattleReport, error) {
	resp, err := b.Client.Request("GET", "br/composition/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	report := &BattleReport{}
	if err := json.NewDecoder(resp.Body).Decode(report); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBattleReportFailed, err)
	}
	return report, nil
}

// battleReportTeamEntry is an entry in a team: an alliance's id, or corp: and a corporation's id
type battleReportTeamEntry string

func (e *battleReportTeamEntry) UnmarshalJSON(data []byte) error {
	*e = battleReportTeamEntry(strings.Trim(string(data), `"`))
	return nil
}

// BattleReportPilot is someone on a battle report's killmail. Char is 0 for a structure or an NPC.
type BattleReportPilot struct {
	Char int `json:"char"`
	Corp int `json:"corp"`
	Ally int `json:"ally"`
}

// BattleReportKillmail is a killmail in a battle report. The site sends a report in one of two spellings, a short one for a large report.
type BattleReportKillmail struct {
	Victim      *BattleReportPilot  `json:"victim"`
	Attackers   []BattleReportPilot `json:"attackers"`
	VictimShort *BattleReportPilot  `json:"vict"`
	AttackShort []BattleReportPilot `json:"atts"`
}

// BattleReportRelated is one system's part of a battle report
type BattleReportRelated struct {
	Killmails []BattleReportKillmail `json:"kms"`
}

// BattleReport is the part of a br.evetools.org report read for its sides
type BattleReport struct {
	Status   string                    `json:"status"`
	Teams    [][]battleReportTeamEntry `json:"teams"`
	Relateds []BattleReportRelated     `json:"relateds"`
}

// BattleReportGroup is a corporation or alliance on a battle report's team, and how many pilots it had on the report's killmails
type BattleReportGroup struct {
	Type   types.EntityType
	ID     int
	Pilots int
}

/*
	Sides lists each team's groups, most pilots first and then in the report's order, without NPC corporations, as listing one would read the killmails of everyone in it

Returns:
  - [][]BattleReportGroup: a list per team
  - int: how many NPC corporations were left out
*/
func (r BattleReport) Sides() ([][]BattleReportGroup, int) {
	allianceChars, corpChars := map[int]map[int]bool{}, map[int]map[int]bool{}
	note := func(seen map[int]map[int]bool, group, char int) {
		if group == 0 || char == 0 {
			return
		}
		if seen[group] == nil {
			seen[group] = map[int]bool{}
		}
		seen[group][char] = true
	}
	notePilot := func(pilot BattleReportPilot) {
		note(allianceChars, pilot.Ally, pilot.Char)
		note(corpChars, pilot.Corp, pilot.Char)
	}
	for _, related := range r.Relateds {
		for _, killmail := range related.Killmails {
			for _, victim := range []*BattleReportPilot{killmail.Victim, killmail.VictimShort} {
				if victim != nil {
					notePilot(*victim)
				}
			}
			for _, attacker := range slices.Concat(killmail.Attackers, killmail.AttackShort) {
				notePilot(attacker)
			}
		}
	}

	npcCorps := 0
	sides := [][]BattleReportGroup{}
	for _, team := range r.Teams {
		side := []BattleReportGroup{}
		for _, entry := range team {
			corpID, isCorp := strings.CutPrefix(string(entry), "corp:")
			id, err := strconv.Atoi(corpID)
			if err != nil || id <= 0 {
				comm.Logger.Warnf("[EVE Tools] Unreadable battle report team entry %q", entry)
				continue
			}
			switch {
			case isCorp && IsNPCCorporation(id):
				npcCorps++
			case isCorp:
				side = append(side, BattleReportGroup{Type: types.CORPORATION, ID: id, Pilots: len(corpChars[id])})
			default:
				side = append(side, BattleReportGroup{Type: types.ALLIANCE, ID: id, Pilots: len(allianceChars[id])})
			}
		}
		slices.SortStableFunc(side, func(a, b BattleReportGroup) int { return cmp.Compare(b.Pilots, a.Pilots) })
		sides = append(sides, side)
	}
	return sides, npcCorps
}
