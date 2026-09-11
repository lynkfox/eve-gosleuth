package api

import (
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

/*
	CollectDisplayValues is an orchestrator for TargetDisplay object creation from TargetEntity interfaces.

Parameters:
  - name: Name of the target.
  - typeOfEntity: models.EntityType of target.
  - getLeftMajor: a function that returns two strings, for the title of the section and the value of Left Major Item display section (Ie: Corp for Character Target).
  - getRightMajor: a function that returns two strings, for the title of the section and the value of Right Major Item display section (Ie: Alliance for Character Target).
  - getLeftDetail: a function that returns two strings, for the title of the section and the value of the Left Details display (Ie: corp history for Character Target)
  - getRightDetail: a function that returns two strings, one for the title of the section and the value of the Right Details display (Ie: Nothing for Character Target)

Returns:
  - TargetDisplay struct
*/
func CollectDisplayValues(name string, typeOfEntity types.EntityType, getImageUrl func() string, getLeftMajor func() (string, string), getRightMajor func() (string, string), getLeftDetails func() (string, string), getRightDetails func() (string, string)) TargetDisplay {
	lMTitle, lMValue := getLeftMajor()
	rMTitle, rMValue := getRightMajor()
	lDTitle, lDValue := getLeftDetails()
	rDTitle, rDValue := getRightDetails()

	return TargetDisplay{
		Name:              name,
		Type:              typeOfEntity,
		TargetImageUrl:    getImageUrl(),
		LeftMajorTitle:    lMTitle,
		LeftMajorValue:    lMValue,
		RightMajorTitle:   rMTitle,
		RightMajorValue:   rMValue,
		LeftDetailsTitle:  lDTitle,
		LeftDetailsValue:  lDValue,
		RightDetailsTitle: rDTitle,
		RightDetailsValue: rDValue,
	}
}

/* BuildDisplay creates the display titles and values for the gui for an ESICharacter Object
 */
func (e ESICharacter) BuildDisplay(client ESIClient) TargetDisplay {
	return CollectDisplayValues(
		e.Name,
		types.PILOT,
		func() string {
			return "Get Img Url"
		},
		func() (string, string) {
			corpData, _ := client.GetCorporationInfo(e.CorporationID)

			return "Corporation", corpData.Name
		},
		func() (string, string) {
			allianceName := "(No Alliance)"
			if e.AllianceID != 0 {
				alliData, _ := client.GetAllianceInfo(e.AllianceID)
				allianceName = alliData.Name
			}
			return "Alliance", allianceName
		},
		func() (string, string) {
			return "Details", "Age: Years,Months,Days\nLongest NPC Corp Time: Years, Months, Days"
		},
		func() (string, string) {
			return "Corp History", "To Be Built"
		},
	)
}

/* BuildDisplay creates the display titles and values for the gui for an ESICorporation Object
 */
func (e ESICorporation) BuildDisplay(client ESIClient) TargetDisplay {
	return CollectDisplayValues(
		e.Name,
		types.CORPORATION,
		func() string {
			return "Get Img Url"
		},
		func() (string, string) {
			ceoName := "(Unknown)"
			ceo, err := client.GetCharacterInfo(e.CeoID)

			if err != nil {
				comm.Logger.WithError(err).Errorf("Unable to pull CEO ID %d for corp %s", e.CeoID, e.Name)
			} else {
				ceoName = ceo.Name
			}
			return "CEO", ceoName
		},
		func() (string, string) {
			allianceName := "(No Alliance)"
			if e.AllianceID != 0 {
				alliData, err := client.GetAllianceInfo(e.AllianceID)
				if err != nil {
					comm.Logger.WithError(err).Warnf("Unable to pull Alliance ID %d for corp %s", e.AllianceID, e.Name)

				} else {
					allianceName = alliData.Name
				}
			}
			return "Alliance", allianceName
		},
		func() (string, string) {
			return "Details", "Created: Date\nMembers: #\n"
		},
		func() (string, string) {
			return "Alliance History", "AllianceA: Date->Date\nAllianceB: Date->Date(Closed)"
		},
	)
}

/* BuildDisplay creates the display titles and values for the gui for an ESIAlliance Object
 */
func (e ESIAlliance) BuildDisplay(client ESIClient) TargetDisplay {
	executorCorpName := "(Unknown)"
	executorCEOName := "(Unknown)"
	executor, err := client.GetCorporationInfo(e.ExecutorCorporationID)
	if err != nil {
		comm.Logger.WithError(err).Errorf("Unable to pull Executor Corp ID %d for Alliance %s", e.ExecutorCorporationID, e.Name)
	} else {
		executorCorpName = executor.Name

		ceo, err := client.GetCharacterInfo(executor.CeoID)
		if err != nil {
			comm.Logger.WithError(err).Errorf("Unable to pull Executor CEO ID %d for corp %s in Alliance %s", executor.CeoID, executor.Name, e.Name)
		} else {
			executorCEOName = ceo.Name
		}
	}

	return CollectDisplayValues(
		e.Name,
		types.ALLIANCE,
		func() string {
			return "Get Img Url"
		},
		func() (string, string) {

			return "Executor Corp", executorCorpName
		},
		func() (string, string) {
			return "Executor CEO", executorCEOName
		},
		func() (string, string) {
			return "Details", "Created: Date\nCorp Members: #\n"
		},
		func() (string, string) {
			return "Nothing Yet", ""
		},
	)
}

/* BuildDisplay creates the display titles and values for the gui for an ESISystem Object
 */
func (e ESISystem) BuildDisplay(client ESIClient) TargetDisplay {
	return CollectDisplayValues(
		e.Name,
		types.SYSTEM,
		func() string {
			return "Get Img Url"
		},
		func() (string, string) {

			return "Constellation/Region", "E-C00024 / E-R00025"
		},
		func() (string, string) {
			return "Class/Weather or Sov", "C5 - Vanilla || Pandemic Horde"
		},
		func() (string, string) {
			return "Connections", "Wormholes get Statics, Kspace gets Gatenames"
		},
		func() (string, string) {
			return "System Details", "Star Type \nPlanets: #\n Moons: #\nBelts: #\nNPC Stations: #"
		},
	)
}
