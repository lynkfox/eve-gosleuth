package api_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// eveWhoCharacterJSON is the evewho.com/api/character response for Luxxianna Seraph
const eveWhoCharacterJSON = `{"info":[{"character_id":2121139046,"corporation_id":98744491,"alliance_id":99013330,"faction_id":0,"name":"Luxxianna Seraph","sec_status":"-1.1"}],"history":[{"record_id":59982653,"corporation_id":1000045,"start_date":"2023/04/18 04:16","end_date":"2023/04/18 23:31"},{"record_id":59987019,"corporation_id":917701062,"start_date":"2023/04/18 23:31","end_date":"2023/08/11 04:58"},{"record_id":60657435,"corporation_id":1000045,"start_date":"2023/08/11 04:58","end_date":"2023/08/11 04:58"},{"record_id":60657436,"corporation_id":98744491,"start_date":"2023/08/11 04:58","end_date":"2024/09/11 05:03"},{"record_id":63089886,"corporation_id":1000045,"start_date":"2024/09/11 05:03","end_date":"2024/09/11 05:03"},{"record_id":63089887,"corporation_id":98739641,"start_date":"2024/09/11 05:03","end_date":"2024/10/29 02:32"},{"record_id":63375079,"corporation_id":1000045,"start_date":"2024/10/29 02:32","end_date":"2024/10/29 05:07"},{"record_id":63375574,"corporation_id":98744491,"start_date":"2024/10/29 05:07","end_date":"2025/10/25 01:09"},{"record_id":65469975,"corporation_id":1000045,"start_date":"2025/10/25 01:09","end_date":"2025/10/25 01:09"},{"record_id":65469977,"corporation_id":98323701,"start_date":"2025/10/25 01:09","end_date":"2025/12/16 01:18"},{"record_id":65786296,"corporation_id":1000045,"start_date":"2025/12/16 01:18","end_date":"2026/03/31 03:16"},{"record_id":66359687,"corporation_id":98739641,"start_date":"2026/03/31 03:16","end_date":"2026/07/08 17:04"},{"record_id":66924626,"corporation_id":1000045,"start_date":"2026/07/08 17:04","end_date":"2026/07/09 01:22"},{"record_id":66926419,"corporation_id":98744491,"start_date":"2026/07/09 01:22","end_date":null}]}`

const eveWhoCorpListJSON = `{"info":[{"corporation_id":98828090,"name":"Wormhole Campus","memberCount":424}],"characters":[{"character_id":2124238686,"name":"10mn Spai"}],"pagination":{"page":1,"limit":500,"total":424,"pages":1,"has_next":false,"has_previous":false}}`

func TestEveWhoGetCharacter(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("decodes info and corp history, the current corp has no end date", func(t *testing.T) {
		mock := CreateMockResposne(eveWhoCharacterJSON, "GET", "", 200)
		client := api.EveWhoConnection{Client: mock}

		character, err := client.GetCharacter(2121139046)

		assert.NoError(t, err)
		assert.Equal(t, "character/2121139046", mock.Endpoint)
		assert.Equal(t, "-1.1", character.Info[0].SecStatus)
		assert.Len(t, character.History, 14)
		assert.Equal(t, time.Date(2023, 4, 18, 4, 16, 0, 0, time.UTC), character.History[0].StartDate.Time)
		assert.Nil(t, character.History[13].EndDate)
	})

	t.Run("a response with no info is ErrNoEveWhoData", func(t *testing.T) {
		client := api.EveWhoConnection{Client: CreateMockResposne(`{"info":[],"history":[]}`, "GET", "", 200)}

		_, err := client.GetCharacter(1)

		assert.ErrorIs(t, err, api.ErrNoEveWhoData)
	})
}

func TestEveWhoGetMemberCount(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("corporations use corplist", func(t *testing.T) {
		mock := CreateMockResposne(eveWhoCorpListJSON, "GET", "", 200)

		count, err := api.EveWhoConnection{Client: mock}.GetMemberCount(98828090, types.CORPORATION)

		assert.NoError(t, err)
		assert.Equal(t, 424, count)
		assert.Equal(t, "corplist/98828090", mock.Endpoint)
	})

	t.Run("alliances use allilist", func(t *testing.T) {
		mock := CreateMockResposne(eveWhoCorpListJSON, "GET", "", 200)

		_, err := api.EveWhoConnection{Client: mock}.GetMemberCount(99014851, types.ALLIANCE)

		assert.NoError(t, err)
		assert.Equal(t, "allilist/99014851", mock.Endpoint)
	})

	t.Run("pilots have no member list", func(t *testing.T) {
		mock := CreateMockResposne(eveWhoCorpListJSON, "GET", "", 200)

		_, err := api.EveWhoConnection{Client: mock}.GetMemberCount(1, types.PILOT)

		assert.Error(t, err)
		assert.Equal(t, 0, mock.Called)
	})
}

func TestEveWhoDisplayFormatting(t *testing.T) {
	var character api.EveWhoCharacterResponse
	assert.NoError(t, json.Unmarshal([]byte(eveWhoCharacterJSON), &character))
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)

	t.Run("corp history is newest first, without the zero length npc corp records", func(t *testing.T) {
		names := map[int]string{98744491: "Hard Knocks", 1000045: "Science and Trade Institute", 98739641: "Corp B", 98323701: "Corp C", 917701062: "Corp A"}

		entries := api.BuildCorpHistory(character.History, names)

		assert.Len(t, entries, 11)
		assert.Equal(t, api.CorpHistoryEntry{
			CorporationID: 98744491,
			Name:          "Hard Knocks",
			IconUrl:       "https://images.evetech.net/corporations/98744491/logo?size=32",
			Start:         "Jul 9th, 2026",
			End:           "present",
		}, entries[0])
		assert.Equal(t, "Science and Trade Institute", entries[10].Name)
		assert.Equal(t, []string{"Apr 18th, 2023", "Apr 18th, 2023"}, []string{entries[10].Start, entries[10].End})
	})

	t.Run("corps missing a name are shown by id", func(t *testing.T) {
		entries := api.BuildCorpHistory(character.History, map[int]string{})

		assert.Equal(t, "Corp 917701062", entries[9].Name)
	})

	t.Run("the longest npc corp stint", func(t *testing.T) {
		longest := api.LongestNPCCorpStint(character.History, now)

		assert.Equal(t, "3m 15d", api.FormatSpan(longest))
	})

	t.Run("spans", func(t *testing.T) {
		assert.Equal(t, "<1d", api.FormatSpan(5*time.Hour))
		assert.Equal(t, "1y 2m 3d", api.FormatSpan((365+60+3)*24*time.Hour))
		assert.Equal(t, "Apr 18th, 2023 (3y 6m 10d)", api.FormatDateWithAge(time.Date(2023, 4, 18, 4, 16, 0, 0, time.UTC), now))
	})
}

func TestFormatShortDate(t *testing.T) {
	t.Run("short month, ordinal day, comma, and year", func(t *testing.T) {
		days := []string{}
		for _, day := range []int{1, 2, 3, 4, 11, 12, 13, 21, 22, 23, 24, 31} {
			days = append(days, api.FormatShortDate(time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)))
		}
		assert.Equal(t, []string{"Oct 1st, 2026", "Oct 2nd, 2026", "Oct 3rd, 2026", "Oct 4th, 2026", "Oct 11th, 2026", "Oct 12th, 2026", "Oct 13th, 2026",
			"Oct 21st, 2026", "Oct 22nd, 2026", "Oct 23rd, 2026", "Oct 24th, 2026", "Oct 31st, 2026"}, days)
	})

	t.Run("dates are in EVE time whatever zone the time is in", func(t *testing.T) {
		newYork := time.FixedZone("EDT", -4*60*60)
		assert.Equal(t, "Oct 25th, 2026", api.FormatShortDate(time.Date(2026, 10, 24, 22, 0, 0, 0, newYork)))
	})
}

func TestEveWhoGetCorpJoined(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("asks for the page and decodes the join records, newest first, open while still in the corp", func(t *testing.T) {
		mock := CreateMockResposne(`{"characters":[{"id":2017699927,"start_date":"2026/10/15 23:56","end_date":null},{"id":822843042,"start_date":"2026/09/11 00:36","end_date":"2026/10/08 06:06"}],"pagination":{"page":2,"limit":500,"total":1363,"pages":3,"has_next":true,"has_previous":true}}`, "GET", "", 200)

		page, err := api.EveWhoConnection{Client: mock}.GetCorpJoined(98744491, 2)

		assert.NoError(t, err)
		assert.Equal(t, "corpjoined/98744491/page/2", mock.Endpoint)
		assert.Equal(t, 2017699927, page.Characters[0].CharacterID)
		assert.Equal(t, time.Date(2026, 10, 15, 23, 56, 0, 0, time.UTC), page.Characters[0].StartDate.Time)
		assert.Nil(t, page.Characters[0].EndDate)
		assert.NotNil(t, page.Characters[1].EndDate)
		assert.Equal(t, api.EveWhoPagination{Page: 2, Total: 1363, Pages: 3, HasNext: true}, page.Pagination)
	})
}

func TestCorpStintsFromESI(t *testing.T) {
	start := func(month int, day int) time.Time {
		return time.Date(2026, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}
	records := []api.ESICorporationHistoryItem{
		{CorporationID: 98744491, RecordID: 3, StartDate: start(7, 9)},
		{CorporationID: 1000045, RecordID: 1, StartDate: start(1, 1)},
		{CorporationID: 98744491, RecordID: 2, StartDate: start(3, 1)},
		{CorporationID: 1000045, RecordID: 4, StartDate: start(5, 1)},
	}

	t.Run("orders the ESI's records oldest first, each ending when the next starts", func(t *testing.T) {
		stints := api.CorpStintsFromESI(records)

		assert.Equal(t, []int{1000045, 98744491, 1000045, 98744491}, []int{stints[0].CorporationID, stints[1].CorporationID, stints[2].CorporationID, stints[3].CorporationID})
		assert.Equal(t, start(3, 1), stints[0].EndDate.Time)
		assert.Nil(t, stints[3].EndDate, "the current corp has no end")
	})

	t.Run("joined the current corp at the start of the latest stint in it", func(t *testing.T) {
		joined, found := api.JoinedCurrentCorp(api.CorpStintsFromESI(records), 98744491)
		assert.True(t, found)
		assert.Equal(t, start(7, 9), joined)

		_, found = api.JoinedCurrentCorp(api.CorpStintsFromESI(records), 1)
		assert.False(t, found)
	})

	t.Run("the longest npc stint works on the ESI history too", func(t *testing.T) {
		assert.Equal(t, 69*24*time.Hour, api.LongestNPCCorpStint(api.CorpStintsFromESI(records), start(10, 25)))
	})
}

func TestJoinedAlliance(t *testing.T) {
	history := api.ESIAllianceHistory{CorporationID: 98744491, Records: []api.ESIAllianceHistoryItem{
		{AllianceID: 99013330, RecordID: 3, StartDate: time.Date(2024, 6, 14, 19, 8, 0, 0, time.UTC)},
		{RecordID: 2, StartDate: time.Date(2024, 6, 8, 11, 2, 0, 0, time.UTC)},
		{AllianceID: 99013330, RecordID: 1, StartDate: time.Date(2023, 9, 6, 21, 7, 0, 0, time.UTC)},
	}}

	joined, found := history.JoinedAlliance(99013330)
	assert.True(t, found)
	assert.Equal(t, time.Date(2024, 6, 14, 19, 8, 0, 0, time.UTC), joined, "the latest time it joined")

	_, found = history.JoinedAlliance(1)
	assert.False(t, found)
}
