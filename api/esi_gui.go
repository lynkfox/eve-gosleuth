package api

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

/*
	CollectDisplayValues is an orchestrator for TargetDisplay object creation from TargetEntity interfaces.

Parameters:
  - name: Name of the target.
  - typeOfEntity: models.EntityType of target.
  - getRelated: a function that returns the entities shown beside the target (Ie: Corp and Alliance for Character Target).
  - getLeftDetail: a function that returns the title of the Left Details section and its label and value rows (Ie: Born and Security status for Character Target)
  - getRightDetail: a function that returns two strings, one for the title of the section and the value of the Right Details display (Ie: Nothing for Character Target)

Returns:
  - TargetDisplay struct
*/
func CollectDisplayValues(name string, typeOfEntity types.EntityType, getImageUrl func() string, getRelated func() []RelatedEntity, getLeftDetails func() (string, []DetailRow), getRightDetails func() (string, string)) TargetDisplay {
	lDTitle, lDRows := getLeftDetails()
	rDTitle, rDValue := getRightDetails()

	return TargetDisplay{
		Name:              name,
		Type:              typeOfEntity,
		TargetImageUrl:    getImageUrl(),
		Related:           getRelated(),
		LeftDetailsTitle:  lDTitle,
		LeftDetailsValue:  detailText(lDRows),
		DetailRows:        lDRows,
		RightDetailsTitle: rDTitle,
		RightDetailsValue: rDValue,
	}
}

// detailText writes detail rows as plain lines, ie "Region: The Forge", for places that show text rather than columns
func detailText(rows []DetailRow) string {
	lines := []string{}
	for _, row := range rows {
		lines = append(lines, row.Label+": "+row.Value)
	}
	return strings.Join(lines, "\n")
}

/* BuildDisplay creates the display titles and values for the gui for an ESICharacter Object, with security status and corp history from EveWho
 */
func (e ESICharacter) BuildDisplay(client ESIClient, eveWho EveWhoClient) TargetDisplay {
	now := time.Now().UTC()

	eveWhoData, eveWhoErr := eveWho.GetCharacter(e.CharacterID)
	corpHistory := []CorpHistoryEntry{}
	if eveWhoErr != nil {
		comm.Logger.WithError(eveWhoErr).Warnf("Unable to pull EveWho data for %s", e.Name)
	} else {
		corpHistory = BuildCorpHistory(eveWhoData.History, corpNamesFor(eveWhoData.History, client))
	}

	display := CollectDisplayValues(
		e.Name,
		types.PILOT,
		func() string {
			return "Get Img Url"
		},
		func() []RelatedEntity {
			corpName := "(Unknown)"
			corpData, err := client.GetCorporationInfo(e.CorporationID)
			if err != nil {
				comm.Logger.WithError(err).Warnf("Unable to pull Corp ID %d for pilot %s", e.CorporationID, e.Name)
			} else {
				corpName = corpData.Name
			}
			related := []RelatedEntity{NewRelatedEntity("Corporation", corpName, e.CorporationID, types.CORPORATION)}

			if e.AllianceID != 0 {
				allianceName := "(Unknown)"
				alliData, err := client.GetAllianceInfo(e.AllianceID)
				if err != nil {
					comm.Logger.WithError(err).Warnf("Unable to pull Alliance ID %d for pilot %s", e.AllianceID, e.Name)
				} else {
					allianceName = alliData.Name
				}
				related = append(related, NewRelatedEntity("Alliance", allianceName, e.AllianceID, types.ALLIANCE))
			}
			return related
		},
		func() (string, []DetailRow) {
			// EveWho's security status is preferred, the ESI one is the fallback
			security := fmt.Sprintf("%.1f", e.SecurityStatus)
			if eveWhoErr == nil {
				security = eveWhoData.Info[0].SecStatus
			}

			rows := []DetailRow{
				{Label: "Born", Value: FormatDateWithAge(e.Birthday, now)},
				{Label: "Security status", Value: security},
			}
			if eveWhoErr == nil {
				rows = append(rows, DetailRow{Label: "Longest NPC corp time", Value: FormatSpan(LongestNPCCorpStint(eveWhoData.History, now))})
			}
			return "Details", rows
		},
		func() (string, string) {
			if eveWhoErr != nil {
				return "Corp History", "Unavailable from EveWho"
			}
			return "Corp History", "" // shown from CorpHistory
		},
	)
	display.CorpHistory = corpHistory

	return display
}

/* BuildDisplay creates the display titles and values for the gui for an ESICorporation Object, with the member count from EveWho
 */
func (e ESICorporation) BuildDisplay(client ESIClient, eveWho EveWhoClient) TargetDisplay {
	members, membersErr := eveWho.GetMemberCount(e.CorporationID, types.CORPORATION)
	if membersErr != nil {
		comm.Logger.WithError(membersErr).Warnf("Unable to pull EveWho member count for %s", e.Name)
	}

	// EveWho has no api for a corp's alliance history, the ESI's is the same data
	allianceHistory := []HistoryEntry{}
	history, historyErr := client.GetAllianceHistory(e.CorporationID)
	if historyErr != nil || history == nil {
		comm.Logger.WithError(historyErr).Warnf("Unable to pull the alliance history of %s", e.Name)
	} else {
		allianceHistory = BuildAllianceHistory(*history, allianceNamesFor(*history, client))
	}

	display := CollectDisplayValues(
		e.Name,
		types.CORPORATION,
		func() string {
			return "Get Img Url"
		},
		func() []RelatedEntity {
			ceoName := "(Unknown)"
			ceo, err := client.GetCharacterInfo(e.CeoID)

			if err != nil {
				comm.Logger.WithError(err).Errorf("Unable to pull CEO ID %d for corp %s", e.CeoID, e.Name)
			} else {
				ceoName = ceo.Name
			}
			related := []RelatedEntity{NewRelatedEntity("CEO", ceoName, e.CeoID, types.PILOT)}

			if e.AllianceID != 0 {
				allianceName := "(Unknown)"
				alliData, err := client.GetAllianceInfo(e.AllianceID)
				if err != nil {
					comm.Logger.WithError(err).Warnf("Unable to pull Alliance ID %d for corp %s", e.AllianceID, e.Name)

				} else {
					allianceName = alliData.Name
				}
				related = append(related, NewRelatedEntity("Alliance", allianceName, e.AllianceID, types.ALLIANCE))
			}
			return related
		},
		func() (string, []DetailRow) {
			return "Details", []DetailRow{{Label: "Founded", Value: FormatDateWithAge(e.DateFounded, time.Now().UTC())}, memberCountRow(members, membersErr)}
		},
		func() (string, string) {
			switch {
			case historyErr != nil || history == nil:
				return "Alliance History", "Unavailable"
			case len(allianceHistory) == 0:
				return "Alliance History", "Never in an alliance"
			}
			return "Alliance History", "" // shown from AllianceHistory
		},
	)
	display.AllianceHistory = allianceHistory
	return display
}

/*
	BuildAllianceHistory lists a corp's alliances newest first, with each alliance's logo and the dates the corp was in it.
	A stint ends when the next record starts, whether that is another alliance or the corp leaving to be in none.

Parameters:
  - history: the corp's ESI alliance history
  - allianceNames: names by alliance id, an alliance missing from it is shown by its id

Returns:
  - []HistoryEntry: one entry per stint in an alliance, newest first
*/
func BuildAllianceHistory(history ESIAllianceHistory, allianceNames map[int]string) []HistoryEntry {
	records := slices.Clone(history.Records)
	slices.SortFunc(records, func(a, b ESIAllianceHistoryItem) int { return a.StartDate.Compare(b.StartDate) })

	entries := []HistoryEntry{}
	for index, record := range records {
		if record.AllianceID == 0 {
			continue // a stint in no alliance
		}

		end := "present"
		if index+1 < len(records) {
			if records[index+1].StartDate.Equal(record.StartDate) {
				continue // replaced at once, as a corp history does for a same day hop
			}
			end = FormatShortDate(records[index+1].StartDate)
		}

		name, ok := allianceNames[record.AllianceID]
		if !ok {
			name = fmt.Sprintf("Alliance %d", record.AllianceID)
		}
		if record.IsDeleted {
			name += " (closed)"
		}

		entries = append(entries, HistoryEntry{
			ID:      record.AllianceID,
			Name:    name,
			IconUrl: EntityIconURL(types.ALLIANCE, record.AllianceID),
			Start:   FormatShortDate(record.StartDate),
			End:     end,
		})
	}
	slices.Reverse(entries)
	return entries
}

// allianceNamesFor resolves the names of every alliance in a corp's history in bulk, returning an empty map if the ESI lookup fails
func allianceNamesFor(history ESIAllianceHistory, client ESIClient) map[int]string {
	ids := []int{}
	for _, record := range history.Records {
		if record.AllianceID != 0 && !slices.Contains(ids, record.AllianceID) {
			ids = append(ids, record.AllianceID)
		}
	}
	if len(ids) == 0 {
		return map[int]string{}
	}

	names, err := client.GetNamesFromEveIDs(ids)
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to resolve alliance history names")
		return map[int]string{}
	}
	return names
}

/* BuildDisplay creates the display titles and values for the gui for an ESIAlliance Object, with the member count from EveWho
 */
func (e ESIAlliance) BuildDisplay(client ESIClient, eveWho EveWhoClient) TargetDisplay {
	members, membersErr := eveWho.GetMemberCount(e.AllianceID, types.ALLIANCE)
	if membersErr != nil {
		comm.Logger.WithError(membersErr).Warnf("Unable to pull EveWho member count for %s", e.Name)
	}

	executorCorpName := "(Unknown)"
	executorCEOName := "(Unknown)"
	executorCEOID := 0
	executor, err := client.GetCorporationInfo(e.ExecutorCorporationID)
	if err != nil {
		comm.Logger.WithError(err).Errorf("Unable to pull Executor Corp ID %d for Alliance %s", e.ExecutorCorporationID, e.Name)
	} else {
		executorCorpName = executor.Name
		executorCEOID = executor.CeoID

		ceo, err := client.GetCharacterInfo(executor.CeoID)
		if err != nil {
			comm.Logger.WithError(err).Errorf("Unable to pull Executor CEO ID %d for corp %s in Alliance %s", executor.CeoID, executor.Name, e.Name)
		} else {
			executorCEOName = ceo.Name
		}
	}

	memberCorps := MemberCorporations(e, client)

	display := CollectDisplayValues(
		e.Name,
		types.ALLIANCE,
		func() string {
			return "Get Img Url"
		},
		func() []RelatedEntity {
			related := []RelatedEntity{NewRelatedEntity("Executor Corp", executorCorpName, e.ExecutorCorporationID, types.CORPORATION)}
			if executorCEOID != 0 {
				related = append(related, NewRelatedEntity("Executor CEO", executorCEOName, executorCEOID, types.PILOT))
			}
			return related
		},
		func() (string, []DetailRow) {
			return "Details", []DetailRow{{Label: "Founded", Value: FormatDateWithAge(e.DateFounded, time.Now().UTC())}, memberCountRow(members, membersErr)}
		},
		func() (string, string) {
			if len(memberCorps) == 0 {
				return "Member Corporations", "Unavailable from the ESI"
			}
			return fmt.Sprintf("Member Corporations (%d)", len(memberCorps)), "" // shown from MemberCorps
		},
	)
	display.MemberCorps = memberCorps

	return display
}

// memberCorpLookups caps how many member corps are looked up at once, as large alliances have hundreds
const memberCorpLookups = 10

/*
	MemberCorporations looks up an alliance's member corporations from the ESI, a few at a time

Parameters:
  - alliance: the alliance whose corps are listed
  - client: the ESI client, cached so each corp is only fetched once a day

Returns:
  - []MemberCorp: the corps with the most members first, then by name. Corps that fail to load are left out.
*/
func MemberCorporations(alliance ESIAlliance, client ESIClient) []MemberCorp {
	members, err := client.GetAllianceCorporations(alliance.AllianceID)
	if err != nil || members == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull the member corporations of %s", alliance.Name)
		return []MemberCorp{}
	}

	found := make([]*MemberCorp, len(members.CorporationIDs))
	slots := make(chan struct{}, memberCorpLookups)

	var wait sync.WaitGroup
	for index, corporationID := range members.CorporationIDs {
		wait.Add(1)
		go func() {
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			corporation, err := client.GetCorporationInfo(corporationID)
			if err != nil {
				comm.Logger.WithError(err).Warnf("Unable to pull Corporation ID %d in alliance %s", corporationID, alliance.Name)
				return
			}

			found[index] = &MemberCorp{CorporationID: corporationID, Name: corporation.Name, IconUrl: EntityIconURL(types.CORPORATION, corporationID), MemberCount: corporation.MemberCount}
		}()
	}
	wait.Wait()

	corps := []MemberCorp{}
	for _, corp := range found {
		if corp != nil {
			corps = append(corps, *corp)
		}
	}
	slices.SortFunc(corps, func(a, b MemberCorp) int {
		if a.MemberCount != b.MemberCount {
			return b.MemberCount - a.MemberCount
		}
		return strings.Compare(a.Name, b.Name)
	})

	return corps
}

/* BuildDisplay creates the display titles and values for the gui for an ESISystem Object: its location, and statics for j-space or infrastructure for k-space
 */
func (e ESISystem) BuildDisplay(client ESIClient, eveWho EveWhoClient) TargetDisplay {
	constellationName, regionName := "(Unknown)", "(Unknown)"
	if temp, err := client.GetEsiResponse(e.ConstellationID, types.CONSTELLATION); err != nil || temp == nil {
		comm.Logger.WithError(err).Warnf("Unable to pull Constellation ID %d for system %s", e.ConstellationID, e.Name)
	} else {
		constellation := temp.(*ESIConstellation)
		constellationName = constellation.Name

		if tempRegion, err := client.GetEsiResponse(constellation.RegionID, types.REGION); err != nil || tempRegion == nil {
			comm.Logger.WithError(err).Warnf("Unable to pull Region ID %d for system %s", constellation.RegionID, e.Name)
		} else {
			regionName = tempRegion.(*ESIRegion).Name
		}
	}

	class := comm.WormholeClass(e.Name, regionName)
	statics := SystemStaticsFor(e.Name)
	stargates := StargateLinks(e, client)

	// j-space has no meaningful security status or stations, its class is shown beside the name
	rows := []DetailRow{}
	if class == "" {
		rows = append(rows, DetailRow{Label: "Security status", Value: fmt.Sprintf("%.1f", e.SecurityStatus), SecurityBand: SecurityBand(e.SecurityStatus)})
	}
	rows = append(rows, DetailRow{Label: "Constellation", Value: constellationName}, DetailRow{Label: "Region", Value: regionName})
	if class == "" {
		rows = append(rows, DetailRow{Label: "Stations", Value: strconv.Itoa(len(e.Stations))})
	}
	rows = append(rows, DetailRow{Label: "Planets", Value: strconv.Itoa(len(e.Planets))})

	display := CollectDisplayValues(
		e.Name,
		types.SYSTEM,
		func() string {
			return starImageURL(e, client)
		},
		func() []RelatedEntity {
			return []RelatedEntity{}
		},
		func() (string, []DetailRow) {
			return "Details", rows
		},
		func() (string, string) {
			if class != "" {
				if len(statics) == 0 {
					return "Statics", "None known"
				}
				return "Statics", "" // shown from Statics
			}
			if len(stargates) == 0 {
				return "Stargates", "None"
			}
			return "Stargates", "" // shown from Stargates
		},
	)
	display.Statics = statics
	display.Stargates = stargates
	display.SpaceLabel = SpaceLabel(class, regionName, e.SecurityStatus)

	return display
}

/*
	SpaceLabel names the kind of space a system is in, as shown beside its name

Parameters:
  - class: the system's wormhole class, empty for k-space
  - regionName: the system's region, to find regions with their own kind of space (comm.Spaces.RegionSpaceLabels), ie Pochven is Triglavian
  - security: the system's true security status. Any positive security shows in game as at least 0.1, so it is lowsec.

Returns:
  - string: the wormhole class, ie C5, the region's own label, ie Triglavian, or High Sec, Low Sec, or Null Sec
*/
func SpaceLabel(class string, regionName string, security float64) string {
	if class != "" {
		return class
	}
	if label := comm.Spaces.RegionSpaceLabels[regionName]; label != "" {
		return label
	}
	return map[string]string{SECURITY_HIGH: "High Sec", SECURITY_LOW: "Low Sec", SECURITY_NULL: "Null Sec"}[SecurityBand(security)]
}

// the security bands, as SecurityBand names them. The frontend colors each as sec-<band>.
const (
	SECURITY_HIGH = "high"
	SECURITY_LOW  = "low"
	SECURITY_NULL = "null"
)

/*
	SecurityBand names the band a true security status falls in, the one place the app decides it. The game rounds security to one decimal,
	so 0.45 and up is highsec, and any positive security shows as at least 0.1, so lowsec.

Returns:
  - string: SECURITY_HIGH, SECURITY_LOW, or SECURITY_NULL (0.0 and below, which includes j-space)
*/
func SecurityBand(security float64) string {
	switch {
	case security >= 0.45:
		return SECURITY_HIGH
	case security > 0:
		return SECURITY_LOW
	}
	return SECURITY_NULL
}

// starImageURL returns the image server render of a system's star, or an empty string if the system has no star or it can not be looked up
func starImageURL(system ESISystem, client ESIClient) string {
	if system.StarID == 0 {
		return ""
	}
	star, err := client.GetStarInfo(system.StarID)
	if err != nil || star == nil || star.TypeID == 0 {
		comm.Logger.WithError(err).Warnf("Unable to pull Star ID %d for system %s", system.StarID, system.Name)
		return ""
	}
	return TypeRenderURL(star.TypeID)
}

/*
	StargateLinks looks up which system each of a system's stargates leads to. The gates are looked up in parallel, two ESI calls each.

Parameters:
  - system: the system whose stargates are followed
  - client: the ESI client

Returns:
  - []SystemLink: the connected systems sorted by name. Gates that could not be looked up are left out.
*/
func StargateLinks(system ESISystem, client ESIClient) []SystemLink {
	found := make([]*SystemLink, len(system.Stargates))

	var wait sync.WaitGroup
	for index, stargateID := range system.Stargates {
		wait.Add(1)
		go func() {
			defer wait.Done()

			stargate, err := client.GetStargateInfo(stargateID)
			if err != nil {
				comm.Logger.WithError(err).Warnf("Unable to pull Stargate ID %d in system %s", stargateID, system.Name)
				return
			}

			destination, err := client.GetSystemInfo(stargate.Destination.SystemID)
			if err != nil {
				comm.Logger.WithError(err).Warnf("Unable to pull System ID %d through a stargate in %s", stargate.Destination.SystemID, system.Name)
				return
			}

			found[index] = &SystemLink{SystemID: stargate.Destination.SystemID, Name: destination.Name, SecurityStatus: destination.SecurityStatus, SecurityBand: SecurityBand(destination.SecurityStatus)}
		}()
	}
	wait.Wait()

	links := []SystemLink{}
	for _, link := range found {
		if link != nil {
			links = append(links, *link)
		}
	}
	slices.SortFunc(links, func(a, b SystemLink) int { return strings.Compare(a.Name, b.Name) })

	return links
}
