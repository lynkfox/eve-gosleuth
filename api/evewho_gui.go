package api

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// npc corporation ids are in this range, player corporations are above it
const (
	firstNPCCorporationID = 1000000
	lastNPCCorporationID  = 1999999
)

/*
	FormatShortDate writes a date the way the gui shows every date: short month, ordinal day, and year, ie Oct 24th, 2026. Dates are in EVE time (UTC).

Parameters:
  - date: the date to write

Returns:
  - string: the date, ie Oct 24th, 2026
*/
func FormatShortDate(date time.Time) string {
	date = date.UTC()
	day := date.Day()
	suffix := "th"
	if day < 11 || day > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%s %d%s, %d", date.Format("Jan"), day, suffix, date.Year())
}

// IsNPCCorporation returns true if the corporation id belongs to an npc corporation, such as a starter corp
func IsNPCCorporation(corporationID int) bool {
	return corporationID >= firstNPCCorporationID && corporationID <= lastNPCCorporationID
}

/*
	FormatSpan describes a length of time in years, months, and days, ie 3y 6m 2d. Months are counted as 30 days.

Parameters:
  - span: the length of time

Returns:
  - string: the span, with zero parts left out, or "<1d" for less than a day
*/
func FormatSpan(span time.Duration) string {
	days := int(span.Hours() / 24)
	years, days := days/365, days%365
	months, days := days/30, days%30

	parts := []string{}
	for _, part := range []struct {
		value int
		unit  string
	}{{years, "y"}, {months, "m"}, {days, "d"}} {
		if part.value > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", part.value, part.unit))
		}
	}

	if len(parts) == 0 {
		return "<1d"
	}
	return strings.Join(parts, " ")
}

// FormatDateWithAge shows a date and how long ago it was, ie Apr 18th, 2023 (3y 6m)
func FormatDateWithAge(date time.Time, now time.Time) string {
	return fmt.Sprintf("%s (%s)", FormatShortDate(date), FormatSpan(now.Sub(date)))
}

// stintEnd returns when a corp record ended, or now for the pilot's current corp
func stintEnd(record EveWhoCorpRecord, now time.Time) time.Time {
	if record.EndDate == nil {
		return now
	}
	return record.EndDate.Time
}

// LongestNPCCorpStint returns the longest single stay in an npc corporation in a pilot's corp history
func LongestNPCCorpStint(history []EveWhoCorpRecord, now time.Time) time.Duration {
	var longest time.Duration
	for _, record := range history {
		if IsNPCCorporation(record.CorporationID) {
			longest = max(longest, stintEnd(record, now).Sub(record.StartDate.Time))
		}
	}
	return longest
}

/*
	BuildCorpHistory lists a pilot's corps newest first, with each corp's logo and the dates the pilot was in it.

The zero length npc corp records EveWho shows each time a pilot moves between corps are left out.

Parameters:
  - history: the pilot's EveWho corp history, oldest first
  - corpNames: corporation id to name, ids without a name are shown as their id

Returns:
  - []CorpHistoryEntry: one entry per stint in a corp, newest first
*/
func BuildCorpHistory(history []EveWhoCorpRecord, corpNames map[int]string) []CorpHistoryEntry {
	entries := []CorpHistoryEntry{}
	for _, record := range slices.Backward(history) {
		if record.EndDate != nil && record.EndDate.Equal(record.StartDate.Time) {
			continue
		}

		name, ok := corpNames[record.CorporationID]
		if !ok {
			name = fmt.Sprintf("Corp %d", record.CorporationID)
		}

		end := "present"
		if record.EndDate != nil {
			end = FormatShortDate(record.EndDate.Time)
		}

		entries = append(entries, CorpHistoryEntry{
			CorporationID: record.CorporationID,
			Name:          name,
			IconUrl:       EntityIconURL(types.CORPORATION, record.CorporationID),
			Start:         FormatShortDate(record.StartDate.Time),
			End:           end,
		})
	}
	return entries
}

// corpNamesFor resolves the names of every corp in a pilot's history, returning an empty map if the ESI lookup fails
func corpNamesFor(history []EveWhoCorpRecord, client ESIClient) map[int]string {
	ids := []int{}
	for _, record := range history {
		if !slices.Contains(ids, record.CorporationID) {
			ids = append(ids, record.CorporationID)
		}
	}

	names, err := client.GetNamesFromEveIDs(ids)
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to resolve corp history names")
		return map[int]string{}
	}
	return names
}

// memberCountRow describes how many pilots EveWho has in a corp or alliance
func memberCountRow(count int, err error) DetailRow {
	if err != nil {
		return DetailRow{Label: "Members", Value: "Unavailable from EveWho"}
	}
	return DetailRow{Label: "Members", Value: fmt.Sprintf("%d", count)}
}

/*
	CorpStintsFromESI turns the ESI's corporation history into corp stints, oldest first, each ending when the next begins, so the stint helpers here work on it as on EveWho's history

Parameters:
  - records: the ESI corporationhistory records, in any order

Returns:
  - []EveWhoCorpRecord: the stints oldest first. The current corp has no end date.
*/
func CorpStintsFromESI(records []ESICorporationHistoryItem) []EveWhoCorpRecord {
	ordered := slices.Clone(records)
	slices.SortFunc(ordered, func(a, b ESICorporationHistoryItem) int {
		if order := a.StartDate.Compare(b.StartDate); order != 0 {
			return order
		}
		return a.RecordID - b.RecordID
	})

	stints := []EveWhoCorpRecord{}
	for index, record := range ordered {
		stint := EveWhoCorpRecord{RecordID: record.RecordID, CorporationID: record.CorporationID, StartDate: EveWhoTime{record.StartDate}}
		if index+1 < len(ordered) {
			stint.EndDate = &EveWhoTime{ordered[index+1].StartDate}
		}
		stints = append(stints, stint)
	}
	return stints
}

// JoinedCurrentCorp returns when a pilot joined their current corp, the start of their latest stint in it, and false if their history does not have it
func JoinedCurrentCorp(stints []EveWhoCorpRecord, corporationID int) (time.Time, bool) {
	for index := len(stints) - 1; index >= 0; index-- {
		if stints[index].CorporationID == corporationID {
			return stints[index].StartDate.Time, true
		}
	}
	return time.Time{}, false
}
