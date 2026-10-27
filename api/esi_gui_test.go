package api_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// DisplayESIClient embeds the interface, answering only the lookups BuildDisplay makes
type DisplayESIClient struct {
	api.ESIClient
}

func (m DisplayESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	return &api.ESICorporation{CorporationID: idNumber, Name: "URSA PRIMUS", Ticker: "URSAP", CeoID: 2117000001}, nil
}

func (m DisplayESIClient) GetAllianceInfo(idNumber int) (*api.ESIAlliance, error) {
	return &api.ESIAlliance{AllianceID: idNumber, Name: "URSA.", Ticker: "URSA."}, nil
}

func (m DisplayESIClient) GetCharacterInfo(idNumber int) (*api.ESICharacter, error) {
	return &api.ESICharacter{CharacterID: idNumber, Name: "Derz"}, nil
}

// alliance member corp lists fail to load, see MemberCorpsESIClient for one that works
func (m DisplayESIClient) GetAllianceCorporations(allianceID int) (*api.ESIAllianceCorporations, error) {
	return nil, api.ErrRequestFailedToComplete
}

// GetAllianceHistory has the corp join URSA. in June 2026, its only alliance
func (m DisplayESIClient) GetAllianceHistory(corporationID int) (*api.ESIAllianceHistory, error) {
	return &api.ESIAllianceHistory{CorporationID: corporationID, Records: []api.ESIAllianceHistoryItem{
		{AllianceID: 99013330, StartDate: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)}}}, nil
}

func (m DisplayESIClient) GetNamesFromEveIDs(idsNumbers []int) (map[int]string, error) {
	return map[int]string{}, nil
}

// OfflineEveWhoClient fails every request, as if EveWho were down
type OfflineEveWhoClient struct{}

func (m OfflineEveWhoClient) GetCharacter(characterID int) (*api.EveWhoCharacterResponse, error) {
	return nil, api.ErrNoEveWhoData
}

func (m OfflineEveWhoClient) GetCorpJoined(corporationID int, page int) (*api.EveWhoJoinList, error) {
	return nil, api.ErrNoEveWhoData
}

func (m OfflineEveWhoClient) GetMemberCount(entityID int, typeOfEntity types.EntityType) (int, error) {
	return 0, api.ErrNoEveWhoData
}

// relatedSummary lists each related entity as title:name:type
func relatedSummary(related []api.RelatedEntity) []string {
	summary := []string{}
	for _, entity := range related {
		summary = append(summary, entity.Title+":"+entity.Name+":"+entity.Type.String())
	}
	return summary
}

func TestBuildDisplayRelated(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	esi := DisplayESIClient{}

	t.Run("corps and alliances carry their tickers, as target and beside a target", func(t *testing.T) {
		pilot := api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph", CorporationID: 98744491, AllianceID: 99013330}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, []string{"URSAP", "URSA."}, []string{pilot.Related[0].Ticker, pilot.Related[1].Ticker})
		assert.Empty(t, pilot.Ticker)

		corp := api.ESICorporation{CorporationID: 98744491, Name: "URSA PRIMUS", Ticker: "URSAP", CeoID: 2117000001, AllianceID: 99013330}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, "URSAP", corp.Ticker)

		alliance := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", Ticker: "URSA.", ExecutorCorporationID: 98744491}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, "URSA.", alliance.Ticker)
		assert.Equal(t, "URSAP", alliance.Related[0].Ticker)
	})

	t.Run("a pilot shows their corp and alliance", func(t *testing.T) {
		display := api.ESICharacter{CharacterID: 2121139046, Name: "Luxxianna Seraph", CorporationID: 98744491, AllianceID: 99013330}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Equal(t, []string{"Corporation:URSA PRIMUS:CORPORATION", "Alliance:URSA.:ALLIANCE"}, relatedSummary(display.Related))
		assert.Equal(t, "https://images.evetech.net/corporations/98744491/logo?size=128", display.Related[0].ImageUrl)
		assert.Equal(t, "https://zkillboard.com/corporation/98744491/", display.Related[0].ZkillUrl)
		assert.Equal(t, "https://evewho.com/alliance/99013330", display.Related[1].EveWhoUrl)
		assert.Equal(t, []string{"CORPORATION", "ALLIANCE"}, []string{display.Related[0].TypeName, display.Related[1].TypeName})
	})

	t.Run("a pilot with no alliance only shows their corp", func(t *testing.T) {
		display := api.ESICharacter{CharacterID: 1, Name: "Solo", CorporationID: 1000045}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Equal(t, []string{"Corporation:URSA PRIMUS:CORPORATION"}, relatedSummary(display.Related))
	})

	t.Run("a corp shows its CEO and alliance", func(t *testing.T) {
		display := api.ESICorporation{CorporationID: 98744491, Name: "URSA PRIMUS", CeoID: 2117000001, AllianceID: 99013330}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Equal(t, []string{"CEO:Derz:PILOT", "Alliance:URSA.:ALLIANCE"}, relatedSummary(display.Related))
		assert.Equal(t, "https://images.evetech.net/characters/2117000001/portrait?size=128", display.Related[0].ImageUrl)
		if assert.Len(t, display.AllianceHistory, 1) {
			assert.Equal(t, "Jun 1st, 2026", display.AllianceHistory[0].Start)
			assert.Equal(t, "present", display.AllianceHistory[0].End)
		}
		assert.Equal(t, "Alliance History", display.RightDetailsTitle)
	})

	t.Run("an alliance shows its executor corp and executor CEO", func(t *testing.T) {
		display := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", ExecutorCorporationID: 98744491}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Equal(t, []string{"Executor Corp:URSA PRIMUS:CORPORATION", "Executor CEO:Derz:PILOT"}, relatedSummary(display.Related))
	})

	t.Run("a pilot has no corp history when EveWho is down", func(t *testing.T) {
		display := api.ESICharacter{CharacterID: 1, Name: "Solo", CorporationID: 1000045}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Empty(t, display.CorpHistory)
		assert.Equal(t, "Unavailable from EveWho", display.RightDetailsValue)
	})

	t.Run("details still build when EveWho is down", func(t *testing.T) {
		display := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", ExecutorCorporationID: 98744491}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Contains(t, display.DetailRows, api.DetailRow{Label: "Members", Value: "Unavailable from EveWho"})
	})

	t.Run("every target's details are label and value rows", func(t *testing.T) {
		pilot := api.ESICharacter{CharacterID: 1, Name: "Solo", CorporationID: 1000045, SecurityStatus: 1.26}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, []string{"Born", "Security status"}, rowLabels(pilot.DetailRows))
		assert.Equal(t, "1.3", pilot.DetailRows[1].Value)
		assert.Empty(t, pilot.DetailRows[1].SecurityBand, "a pilot's security status is not a system's, so it is not colored like one")

		corp := api.ESICorporation{CorporationID: 98744491, Name: "URSA PRIMUS", CeoID: 2117000001}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, []string{"Founded", "Members"}, rowLabels(corp.DetailRows))

		alliance := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", ExecutorCorporationID: 98744491}.BuildDisplay(esi, OfflineEveWhoClient{})
		assert.Equal(t, []string{"Founded", "Members"}, rowLabels(alliance.DetailRows))
		assert.Contains(t, alliance.LeftDetailsValue, "Members: Unavailable from EveWho", "the plain text version is kept too")
	})

	t.Run("an alliance whose corp list fails to load says so", func(t *testing.T) {
		display := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", ExecutorCorporationID: 98744491}.BuildDisplay(esi, OfflineEveWhoClient{})

		assert.Empty(t, display.MemberCorps)
		assert.Equal(t, "Member Corporations", display.RightDetailsTitle)
		assert.Equal(t, "Unavailable from the ESI", display.RightDetailsValue)
	})

	t.Run("an alliance lists its member corps largest first, leaving out corps that fail", func(t *testing.T) {
		display := api.ESIAlliance{AllianceID: 99013330, Name: "URSA.", ExecutorCorporationID: 1}.BuildDisplay(MemberCorpsESIClient{}, OfflineEveWhoClient{})

		assert.Equal(t, "Member Corporations (3)", display.RightDetailsTitle)
		assert.Equal(t, []api.MemberCorp{
			{CorporationID: 1, Name: "URSA PRIMUS", IconUrl: "https://images.evetech.net/corporations/1/logo?size=32", MemberCount: 400},
			{CorporationID: 3, Name: "Arctic Bears", IconUrl: "https://images.evetech.net/corporations/3/logo?size=32", MemberCount: 25},
			{CorporationID: 2, Name: "Ursa Minor", IconUrl: "https://images.evetech.net/corporations/2/logo?size=32", MemberCount: 25},
		}, display.MemberCorps)
	})
}

// MemberCorpsESIClient answers an alliance's member corp list: corps 1 to 3 load, corp 4 fails
type MemberCorpsESIClient struct {
	DisplayESIClient
}

func (m MemberCorpsESIClient) GetAllianceCorporations(allianceID int) (*api.ESIAllianceCorporations, error) {
	return &api.ESIAllianceCorporations{AllianceID: allianceID, CorporationIDs: []int{2, 4, 1, 3}}, nil
}

func (m MemberCorpsESIClient) GetCorporationInfo(idNumber int) (*api.ESICorporation, error) {
	corps := map[int]api.ESICorporation{1: {Name: "URSA PRIMUS", MemberCount: 400}, 2: {Name: "Ursa Minor", MemberCount: 25}, 3: {Name: "Arctic Bears", MemberCount: 25}}
	corp, ok := corps[idNumber]
	if !ok {
		return nil, api.ErrRequestFailedToComplete
	}
	corp.CorporationID = idNumber
	return &corp, nil
}

// SystemESIClient answers the constellation and region lookups for a system display
type SystemESIClient struct {
	DisplayESIClient
	regionName string
}

// stargates 1 and 2 lead to Perimeter and Maurasi, any other gate fails to load
func (m SystemESIClient) GetStargateInfo(idNumber int) (*api.ESIStargate, error) {
	destinations := map[int]int{1: 30000144, 2: 30000140}
	if _, ok := destinations[idNumber]; !ok {
		return nil, api.ErrRequestFailedToComplete
	}
	stargate := &api.ESIStargate{StargateID: idNumber}
	stargate.Destination.SystemID = destinations[idNumber]
	return stargate, nil
}

// star 40009076 is a known star, any other star fails to load
func (m SystemESIClient) GetStarInfo(idNumber int) (*api.ESIStar, error) {
	if idNumber != 40009076 {
		return nil, api.ErrRequestFailedToComplete
	}
	return &api.ESIStar{SolarSystemID: 30000142, TypeID: 45041}, nil
}

func (m SystemESIClient) GetPlanetInfo(idNumber int) (*api.ESIPlanetInfo, error) {
	return nil, api.ErrRequestFailedToComplete
}

func (m SystemESIClient) GetCategoryInfo(idNumber int) (*api.ESICategory, error) {
	return nil, api.ErrRequestFailedToComplete
}

func (m SystemESIClient) GetSystemInfo(idNumber int) (*api.ESISystem, error) {
	systems := map[int]api.ESISystem{30000144: {Name: "Perimeter", SecurityStatus: 0.95}, 30000140: {Name: "Maurasi", SecurityStatus: 0.66}}
	system := systems[idNumber]
	return &system, nil
}

func (m SystemESIClient) GetEsiResponse(idNumber int, typeOfEntity types.EntityType) (interface{}, error) {
	if typeOfEntity == types.CONSTELLATION {
		return &api.ESIConstellation{Name: "E-C00261", RegionID: 11000025}, nil
	}
	return &api.ESIRegion{Name: m.regionName}, nil
}

func TestSystemBuildDisplay(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	t.Run("a j-space system shows its class and statics", func(t *testing.T) {
		display := api.ESISystem{SystemID: 31002572, Name: "J001025", SecurityStatus: -0.99, Planets: []api.ESIPlanet{{}, {}, {}, {}}}.BuildDisplay(SystemESIClient{regionName: "E-R00024"}, OfflineEveWhoClient{})

		assert.Equal(t, []api.DetailRow{{Label: "Constellation", Value: "E-C00261"}, {Label: "Region", Value: "E-R00024"}, {Label: "Planets", Value: "4"}}, display.DetailRows, "no security status or stations in j-space")
		assert.Equal(t, "C5", display.SpaceLabel, "the class is shown beside the name")
		assert.Equal(t, "Statics", display.RightDetailsTitle)
		assert.Equal(t, []api.SystemStatic{{Code: "H296", Destination: "C5"}, {Code: "U210", Destination: "Lowsec"}}, display.Statics)
		assert.Empty(t, display.Related)
	})

	t.Run("a k-space system shows where its stargates lead, sorted by name, leaving out gates that fail", func(t *testing.T) {
		system := api.ESISystem{SystemID: 30000142, Name: "Jita", SecurityStatus: 0.9459, StarID: 40009076, Stargates: []int{1, 2, 3}, Stations: []int{1, 2}, Planets: []api.ESIPlanet{{}, {}, {}}}
		display := system.BuildDisplay(SystemESIClient{regionName: "The Forge"}, OfflineEveWhoClient{})

		assert.Equal(t, []api.DetailRow{
			{Label: "Security status", Value: "0.9", SecurityBand: api.SECURITY_HIGH},
			{Label: "Constellation", Value: "E-C00261"},
			{Label: "Region", Value: "The Forge"},
			{Label: "Stations", Value: "2"},
			{Label: "Planets", Value: "3"},
		}, display.DetailRows)
		assert.Equal(t, "Security status: 0.9\nConstellation: E-C00261\nRegion: The Forge\nStations: 2\nPlanets: 3", display.LeftDetailsValue)
		assert.Equal(t, "https://images.evetech.net/types/45041/render?size=128", display.TargetImageUrl)
		assert.Equal(t, "High Sec", display.SpaceLabel)
		assert.Equal(t, "Stargates", display.RightDetailsTitle)
		assert.Equal(t, []api.SystemLink{{SystemID: 30000140, Name: "Maurasi", SecurityStatus: 0.66, SecurityBand: api.SECURITY_HIGH}, {SystemID: 30000144, Name: "Perimeter", SecurityStatus: 0.95, SecurityBand: api.SECURITY_HIGH}}, display.Stargates)
		assert.Empty(t, display.Statics)
	})

	t.Run("a j-space system has no stargates", func(t *testing.T) {
		display := api.ESISystem{SystemID: 31002572, Name: "J001025", SecurityStatus: -0.99, StarID: 1}.BuildDisplay(SystemESIClient{regionName: "E-R00024"}, OfflineEveWhoClient{})

		assert.Empty(t, display.Stargates)
		assert.NotContains(t, display.LeftDetailsValue, "Stations")
		assert.Empty(t, display.TargetImageUrl, "a star that fails to load leaves the image out")
	})
}

func TestSecurityBand(t *testing.T) {
	assert.Equal(t, api.SECURITY_HIGH, api.SecurityBand(0.45), "0.45 rounds to 0.5")
	assert.Equal(t, api.SECURITY_LOW, api.SecurityBand(0.44))
	assert.Equal(t, api.SECURITY_LOW, api.SecurityBand(0.01), "any positive security shows as at least 0.1")
	assert.Equal(t, api.SECURITY_NULL, api.SecurityBand(0))
	assert.Equal(t, api.SECURITY_NULL, api.SecurityBand(-0.99))
}

func TestSpaceLabel(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	cases := []struct {
		name     string
		class    string
		region   string
		security float64
		want     string
	}{
		{"j-space shows its class", "C5", "E-R00024", -0.99, "C5"},
		{"highsec", "", "The Forge", 0.9459, "High Sec"},
		{"0.45 rounds to highsec", "", "The Forge", 0.45, "High Sec"},
		{"lowsec", "", "Black Rise", 0.3, "Low Sec"},
		{"a tiny positive security is still lowsec", "", "Black Rise", 0.01, "Low Sec"},
		{"nullsec", "", "Delve", -0.4, "Null Sec"},
		{"Pochven is Triglavian", "", "Pochven", -1.0, "Triglavian"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, api.SpaceLabel(c.class, c.region, c.security))
		})
	}
}

// rowLabels lists the labels of detail rows, in order
func rowLabels(rows []api.DetailRow) []string {
	labels := []string{}
	for _, row := range rows {
		labels = append(labels, row.Label)
	}
	return labels
}

func TestBuildAllianceHistory(t *testing.T) {
	day := func(month time.Month, d int) time.Time { return time.Date(2026, month, d, 12, 0, 0, 0, time.UTC) }
	// the ESI lists newest first: in URSA. now, out of any alliance for a while before, and in a closed alliance first
	history := api.ESIAllianceHistory{CorporationID: 98744491, Records: []api.ESIAllianceHistoryItem{
		{AllianceID: 99013330, StartDate: day(time.June, 1)},
		{AllianceID: 0, StartDate: day(time.March, 1)},
		{AllianceID: 99012589, StartDate: day(time.January, 5), IsDeleted: true},
	}}

	entries := api.BuildAllianceHistory(history, map[int]string{99013330: "URSA."})

	assert.Equal(t, []api.HistoryEntry{
		{ID: 99013330, Name: "URSA.", IconUrl: api.EntityIconURL(types.ALLIANCE, 99013330), Start: "Jun 1st, 2026", End: "present"},
		{ID: 99012589, Name: "Alliance 99012589 (closed)", IconUrl: api.EntityIconURL(types.ALLIANCE, 99012589), Start: "Jan 5th, 2026", End: "Mar 1st, 2026"},
	}, entries, "newest first, a stint ends when the corp leaves, and gaps in no alliance are left out")

	assert.Empty(t, api.BuildAllianceHistory(api.ESIAllianceHistory{}, nil), "a corp never in an alliance")
}
