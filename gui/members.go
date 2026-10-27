package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// maxMembersPerRequest caps how many members' details one GetMembers call looks up, the frontend asks for three pages of 50 at a time
const maxMembersPerRequest = 150

// memberDetailLookups caps how many members are looked up on the ESI at once
const memberDetailLookups = 10

// MemberRow is one member of a corp or alliance target on the Members tab
type MemberRow struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	ImageURL    string     `json:"imageUrl"`
	LastSeen    *time.Time `json:"lastSeen"`    // their most recent killmail among the collected ones, null if they are on none
	Corporation *EntityRef `json:"corporation"` // their corp now, null when it could not be looked up
	Born        string     `json:"born"`        // ie Apr 18th, 2023 (3y 6m), or Unavailable
	TimeInCorp  string     `json:"timeInCorp"`  // how long since they joined their current corp, ie 3m 2d
	NPCTime     string     `json:"npcTime"`     // their longest stay in an NPC corp, ie 1y 2m, or None
	Left        bool       `json:"left"`        // EveWho still lists them, but the ESI has them out of the corp, or out of the alliance for an alliance target
	ZkillURL    string     `json:"zkillUrl"`
	EveWhoURL   string     `json:"evewhoUrl"`
}

// MemberCorpRow is one member corporation of an alliance target on the Members tab
type MemberCorpRow struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	IconURL        string    `json:"iconUrl"`
	MemberCount    int       `json:"memberCount"`
	TimeInAlliance string    `json:"timeInAlliance"` // how long since it joined the alliance, ie 2y 4m, or Unavailable
	ZkillURL       string    `json:"zkillUrl"`
	EveWhoURL      string    `json:"evewhoUrl"`
	joined         time.Time // when it joined the alliance, zero if not known, to order the corps by
}

// MemberPage is the rows the frontend asked for, and how many there are in all. A corp target's rows are pilots, an alliance target's are its corporations.
type MemberPage struct {
	Rows         []MemberRow     `json:"rows"`
	Corporations []MemberCorpRow `json:"corporations"`
	Total        int             `json:"total"`   // the member count, at least as many as are known
	HasMore      bool            `json:"hasMore"` // more exist after these rows
}

/*
	roster is the Members tab list of the last corp or alliance searched.

A corp's members: the current members seen on the killmails, most recent first, then the rest newest to the corp first, from EveWho's join records.
EveWho's records are fetched a page (500 joins) at a time, only as far as the rows asked for need.

An alliance's members are its corporations, newest to the alliance first, from the ESI's alliance histories, all looked up the first time they are asked for.
*/
type roster struct {
	mu          sync.Mutex // held while the list grows, so the App's lock is never held while EveWho or the ESI are asked
	targetID    int
	targetType  types.EntityType
	seen        []MemberRow      // a corp's current members on the collected killmails, most recent first, without their details
	listedIDs   map[int]struct{} // every pilot already in the list, so EveWho's records add each pilot once
	joined      []MemberRow      // a corp's other members from EveWho, newest join first, as far as its pages have been read
	pagesRead   int
	hasNextPage bool  // EveWho has more join records
	memberCount int   // the corp's member count from the ESI, 0 if not known
	eveWhoErr   error // set if EveWho's join records could not be read, the list then stops where it got to
	corps       []MemberCorpRow
	corpsLoaded bool
}

// isCurrentMember reports whether a pilot's current corp is the corp target, or in the alliance target
func isCurrentMember(pilot *retrieval.Pilot, targetID int, targetType types.EntityType) bool {
	corp := pilot.Corporation
	if corp == nil {
		return false
	}
	switch targetType {
	case types.CORPORATION:
		return corp.CorporationID == targetID
	case types.ALLIANCE:
		return corp.Alliance != nil && corp.Alliance.AllianceID == targetID
	}
	return false
}

/*
	newRoster starts a corp or alliance target's Members tab list

Parameters:
  - targetID, targetType: the corporation or alliance
  - pilots: the PilotsSeen from the collected killmails, with each pilot's current corporation. Only used for a corp.
  - memberCount: the corp's member count, to number the pages before EveWho's records are all read. 0 if not known.

Returns:
  - *roster: for a corp, its current members seen on the killmails, most recently seen first. An alliance's corporations are looked up when first asked for.
*/
func newRoster(targetID int, targetType types.EntityType, pilots map[int]aggregate.PilotSeen, memberCount int) *roster {
	members := &roster{targetID: targetID, targetType: targetType, listedIDs: map[int]struct{}{}, hasNextPage: true, memberCount: memberCount}
	if targetType != types.CORPORATION {
		return members
	}

	for _, pilotSeen := range pilots {
		pilot := pilotSeen.Pilot
		if pilot == nil || pilot.NPC || !isCurrentMember(pilot, targetID, targetType) {
			continue
		}
		lastSeen := pilotSeen.LastSeen
		members.seen = append(members.seen, MemberRow{ID: pilot.CharacterID, Name: pilot.Name, LastSeen: &lastSeen})
		members.listedIDs[pilot.CharacterID] = struct{}{}
	}
	slices.SortFunc(members.seen, func(a, b MemberRow) int {
		if order := b.LastSeen.Compare(*a.LastSeen); order != 0 {
			return order
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return members
}

func (r *roster) known() int {
	return len(r.seen) + len(r.joined)
}

// moreOnEveWho reports whether EveWho has join records not yet read
func (r *roster) moreOnEveWho() bool {
	return r.eveWhoErr == nil && r.hasNextPage
}

// readUpTo reads EveWho's join records, newest first, until the roster knows at least count members or there are no more. Only joins still open are current members.
func (r *roster) readUpTo(count int, eveWho api.EveWhoClient) {
	for r.known() < count && r.moreOnEveWho() {
		page, err := eveWho.GetCorpJoined(r.targetID, r.pagesRead+1)
		if err != nil {
			comm.Logger.WithError(err).Warnf("Unable to read EveWho join records page %d for %d", r.pagesRead+1, r.targetID)
			comm.ReportProgress("EveWho join records unavailable (%s), the member list stops at %d", api.FailureReason(err), r.known())
			r.eveWhoErr = err
			return
		}

		r.pagesRead++
		r.hasNextPage = page.Pagination.HasNext
		for _, join := range page.Characters {
			if join.EndDate != nil {
				continue // they have left since
			}
			if _, listed := r.listedIDs[join.CharacterID]; listed {
				continue
			}
			r.listedIDs[join.CharacterID] = struct{}{}
			r.joined = append(r.joined, MemberRow{ID: join.CharacterID})
		}
	}
}

// rows returns a corp's members from offset, up to count of them, without their details
func (r *roster) rows(offset int, count int) []MemberRow {
	ordered := slices.Concat(r.seen, r.joined)
	if offset >= len(ordered) {
		return []MemberRow{}
	}
	return slices.Clone(ordered[offset:min(offset+count, len(ordered))])
}

// loadCorps looks up an alliance's corporations, their member counts, and when each joined the alliance, once, and orders them newest to the alliance first
func (r *roster) loadCorps(client api.ESIClient, now time.Time) {
	if r.corpsLoaded {
		return
	}
	r.corpsLoaded = true

	members, err := client.GetAllianceCorporations(r.targetID)
	if err != nil || members == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull the corporations of alliance %d", r.targetID)
		return
	}

	before := retrieval.CountsOf(client)
	corps := make([]MemberCorpRow, len(members.CorporationIDs))
	slots := make(chan struct{}, memberDetailLookups)
	var wait sync.WaitGroup
	for index, corporationID := range members.CorporationIDs {
		wait.Add(1)
		go func() {
			defer comm.RecoverGoroutine("looking up a member corporation", nil)
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			corps[index] = memberCorpDetails(corporationID, r.targetID, client, now)
		}()
	}
	wait.Wait()
	comm.ReportProgress("%s", retrieval.CountsOf(client).Since(before).Annotate(fmt.Sprintf("Member corporations of the alliance (%d)", len(corps))))

	slices.SortFunc(corps, func(a, b MemberCorpRow) int {
		if order := b.joined.Compare(a.joined); order != 0 {
			return order // newest first, a corp with no known join date (zero) last
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	r.corps = corps
}

// memberCorpDetails looks up one of an alliance's corporations, leaving Unavailable for what the ESI can not give
func memberCorpDetails(corporationID int, allianceID int, client api.ESIClient, now time.Time) MemberCorpRow {
	corp := MemberCorpRow{ID: corporationID, Name: fmt.Sprintf("Corp %d", corporationID), IconURL: api.EntityIconURL(types.CORPORATION, corporationID), TimeInAlliance: "Unavailable",
		ZkillURL: api.ZkillEntityURL(types.CORPORATION, corporationID), EveWhoURL: api.EveWhoURL(types.CORPORATION, corporationID)}
	if info, err := client.GetCorporationInfo(corporationID); err == nil && info != nil {
		corp.Name, corp.MemberCount = info.Name, info.MemberCount
	}
	if history, err := client.GetAllianceHistory(corporationID); err == nil && history != nil {
		if joined, found := history.JoinedAlliance(allianceID); found {
			corp.joined = joined
			corp.TimeInAlliance = api.FormatSpan(now.Sub(joined))
		}
	}
	return corp
}

/*
	GetMembers returns a page of the last corp or alliance searched's Members tab: a corp's pilots with their details from the ESI, or an alliance's corporations.

The tab asks for three pages at a time, as it reaches them.

Parameters:
  - targetID, targetType: the corporation or alliance, as returned in TargetResult, to check the list is still the one shown
  - offset: the first row, from 0
  - count: how many rows, at most maxMembersPerRequest

Returns:
  - MemberPage: the rows, how many there are in all, and whether there are more
  - error: a user readable message if the list is not for this target
*/
func (a *App) GetMembers(targetID int, targetType string, offset int, count int) (page MemberPage, err error) {
	defer recoverAsError(&err)

	count = min(max(count, 0), maxMembersPerRequest)
	now := time.Now().UTC()

	a.rosterMu.Lock()
	members := a.roster
	a.rosterMu.Unlock()
	if members == nil || members.targetID != targetID || members.targetType.String() != targetType {
		return page, errors.New("The member list is for a different search, search again")
	}

	members.mu.Lock()
	if members.targetType == types.ALLIANCE {
		members.loadCorps(a.clients.ESIClient, now)
		page.Corporations = []MemberCorpRow{}
		if offset < len(members.corps) {
			page.Corporations = slices.Clone(members.corps[offset:min(offset+count, len(members.corps))])
		}
		page.Rows = []MemberRow{}
		page.Total = len(members.corps)
		page.HasMore = offset+len(page.Corporations) < page.Total
		members.mu.Unlock()
		return page, nil
	}

	members.readUpTo(offset+count, a.clients.EveWhoClient)
	rows := members.rows(offset, count)
	page.HasMore = offset+len(rows) < members.known() || members.moreOnEveWho()
	page.Total = members.known()
	if page.HasMore {
		page.Total = max(members.memberCount, members.known()+1)
	}
	members.mu.Unlock()

	if len(rows) > 0 {
		before := retrieval.CountsOf(a.clients.ESIClient)
		fillMemberDetails(rows, members.targetID, members.targetType, a.clients.ESIClient, now)
		comm.ReportProgress("%s", retrieval.CountsOf(a.clients.ESIClient).Since(before).Annotate(fmt.Sprintf("Member details %d-%d", offset+1, offset+len(rows))))
	}
	page.Rows = rows
	page.Corporations = []MemberCorpRow{}
	return page, nil
}

// fillMemberDetails looks up each member's portrait, current corp, birthday, and corp history on the ESI, a few at a time, and whether they have left the target
func fillMemberDetails(rows []MemberRow, targetID int, targetType types.EntityType, client api.ESIClient, now time.Time) {
	slots := make(chan struct{}, memberDetailLookups)
	var wait sync.WaitGroup
	for index := range rows {
		wait.Add(1)
		go func() {
			defer comm.RecoverGoroutine("looking up a member", nil)
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			rows[index] = memberDetails(rows[index], targetID, targetType, client, now)
		}()
	}
	wait.Wait()
}

// memberDetails fills in one member's details, leaving Unavailable for any the ESI can not give
func memberDetails(row MemberRow, targetID int, targetType types.EntityType, client api.ESIClient, now time.Time) MemberRow {
	row.ImageURL = api.EntityIconURL(types.PILOT, row.ID)
	row.ZkillURL, row.EveWhoURL = api.ZkillEntityURL(types.PILOT, row.ID), api.EveWhoURL(types.PILOT, row.ID)
	row.Born, row.TimeInCorp, row.NPCTime = "Unavailable", "Unavailable", "Unavailable"

	character, err := client.GetCharacterInfo(row.ID)
	if err != nil || character == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull Character ID %d for the member list", row.ID)
		if row.Name == "" {
			row.Name = fmt.Sprintf("Character %d", row.ID)
		}
		return row
	}
	if row.Name == "" {
		row.Name = character.Name // EveWho's join records have only the id
	}
	if character.Deleted {
		row.Born, row.TimeInCorp, row.NPCTime = "Deleted", "Deleted", "Deleted"
		return row
	}
	row.Born = api.FormatDateWithAge(character.Birthday, now)

	if corp, err := client.GetCorporationInfo(character.CorporationID); err == nil && corp != nil {
		row.Corporation = entityRef(types.CORPORATION, character.CorporationID, corp.Name)
		// EveWho's list lags behind, the ESI knows where they are now
		switch targetType {
		case types.CORPORATION:
			row.Left = character.CorporationID != targetID
		case types.ALLIANCE:
			row.Left = corp.AllianceID != targetID
		}
	}

	history, err := client.GetCorporationHistory(row.ID)
	if err != nil || history == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull the corp history of Character ID %d for the member list", row.ID)
		return row
	}
	stints := api.CorpStintsFromESI(history.Records)
	if joined, found := api.JoinedCurrentCorp(stints, character.CorporationID); found {
		row.TimeInCorp = api.FormatSpan(now.Sub(joined))
	}
	row.NPCTime = "None"
	if longest := api.LongestNPCCorpStint(stints, now); longest > 0 {
		row.NPCTime = api.FormatSpan(longest)
	}
	return row
}
