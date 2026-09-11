package aggregate

import (
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

func IsSameGroupAsTarget(target *retrieval.IntelTarget, other_entity any) bool {

	switch target.TypeOfEntity {
	case types.ALLIANCE:

		if e, ok := other_entity.(retrieval.Alliance); ok {
			return isSameAlliance(target, e.AllianceID)
		}

	case types.CORPORATION:

		if e, ok := other_entity.(retrieval.Corporation); ok {
			if e.Alliance != nil {
				return IsSameCorp(target, e.CorporationID) || isSameAlliance(target, e.Alliance.AllianceID)
			} else {
				return IsSameCorp(target, e.CorporationID)
			}
		}

	case types.PILOT:

		if e, ok := other_entity.(retrieval.Pilot); ok {

			if e.Corporation != nil {
				if e.Corporation.Alliance != nil {
					return IsSameCorp(target, e.Corporation.CorporationID) || isSameAlliance(target, e.Corporation.Alliance.AllianceID)
				} else {
					return IsSameCorp(target, e.Corporation.CorporationID)
				}
			}
		}

	}

	return false
}

func IsTarget(target *retrieval.IntelTarget, idNumber int) bool {

	return target.IDNumber == idNumber
}

func IsSameCorp(target *retrieval.IntelTarget, other_id_number int) bool {
	switch target.TypeOfEntity {
	case types.PILOT:
		corp := target.Expanded.(*retrieval.Pilot).Corporation
		if corp != nil {
			return corp.CorporationID == other_id_number
		}

	case types.CORPORATION:
		return target.IDNumber == other_id_number
	}

	return false
}

func isSameAlliance(target *retrieval.IntelTarget, other_id_number int) bool {
	switch target.TypeOfEntity {
	case types.PILOT:
		corp := target.Expanded.(*retrieval.Pilot).Corporation
		if corp != nil && corp.Alliance != nil {
			return corp.Alliance.AllianceID == other_id_number
		}

	case types.CORPORATION:
		alliance := target.Expanded.(*retrieval.Corporation).Alliance
		if alliance != nil {
			return target.IDNumber == other_id_number
		}

	case types.ALLIANCE:
		return target.IDNumber == other_id_number
	}

	return false

}

func CollateAllKillmailData(killmails *[]retrieval.RawKillmail, esiClient *api.ESIClient, cachedData *retrieval.CachedEveIDs) *CollectedData {

	results := &CollectedData{
		PilotsSeen:       make(map[int]PilotSeen),
		SystemsVisted:    make(map[int]SystemVisted),
		ShipsFlown:       make(map[int]ShipFlown),
		BattlesFought:    make(map[string]BattleFought),
		CapitalPilots:    map[int]CapitalPilot{},
		AlliancesSeen:    make(map[int]AllianceSeen),
		CorporationsSeen: make(map[int]CorporationSeen),
	}

	currentBattle := &BattleFought{
		TotalKillmails: 0,
		UniquePilots:   make(map[int]struct{}),
		KillmailIDs:    []int{},
	}

	for index, km := range *killmails {

		if (index >= 5 && index%5 == 0) || index == 0 {

			comm.Logger.Infof("Processing KM %d/%d", index, len(*killmails))
		}

		expandedKM := km.Expand(esiClient, cachedData)

		lastKillmailTime := km.ESIKillmail.KillmailTime
		previousKillmailSystem := km.ESIKillmail.SolarSystemID

		if index > 0 {
			lastKillmailTime = (*killmails)[index-1].ESIKillmail.KillmailTime
			previousKillmailSystem = (*killmails)[index-1].ESIKillmail.SolarSystemID
		}

		currentBattle = results.UpdateOrCreateCurrentBattle(lastKillmailTime, &km, previousKillmailSystem, currentBattle)

		results.UpdateOrAddSystem(&km, expandedKM.System, currentBattle.BattleIDReference)

		results.UpdateOrAddPilot(&km, expandedKM, expandedKM.Victim.Pilot, expandedKM.Victim.Ship, currentBattle.BattleIDReference, true)

		for _, attacker := range expandedKM.Attackers {
			results.UpdateOrAddPilot(&km, expandedKM, attacker.Pilot, attacker.Ship, currentBattle.BattleIDReference, false)
		}

	}

	return results

}

func (results *CollectedData) UpdateOrAddPilot(km *retrieval.RawKillmail, expandedKillmail *retrieval.Killmail, character *retrieval.Pilot, ship *retrieval.Item, currentBattleID string, isVictim bool) {
	pilot, found := results.PilotsSeen[character.CharacterID]

	if !found {
		pilot = PilotSeen{
			Pilot:                 character,
			TotalKillmails:        1,
			TimesAttacker:         0,
			TimesVictim:           0,
			AttackerKillmailIDs:   []int{},
			OtherAttackerPilotIDs: []int{},
			VictimKillmailIDs:     []int{},
			KillerPilotIDs:        []int{},
			ShipsFlown:            make(map[int]ShipFlown),
			SystemsVisited:        make(map[int]SystemVisted),
			BattlesFoughtIDs:      make(map[string]struct{}),
			HoursActive:           make(map[int]int),
			DaysActive:            []string{},
		}

	} else {
		pilot.TotalKillmails = 1 + pilot.TotalKillmails
	}

	if isVictim {
		pilot.TimesVictim = 1 + pilot.TimesVictim
		pilot.VictimKillmailIDs = append(pilot.VictimKillmailIDs, expandedKillmail.KillmailID)

		for _, value := range expandedKillmail.Attackers {
			if !slices.Contains(pilot.KillerPilotIDs, value.Pilot.CharacterID) {
				pilot.KillerPilotIDs = append(pilot.KillerPilotIDs, value.Pilot.CharacterID)
			}
		}

	} else {
		pilot.TimesAttacker = 1 + pilot.TimesAttacker
		pilot.AttackerKillmailIDs = append(pilot.AttackerKillmailIDs, expandedKillmail.KillmailID)

		for _, value := range expandedKillmail.Attackers {
			if !slices.Contains(pilot.OtherAttackerPilotIDs, value.Pilot.CharacterID) {
				pilot.OtherAttackerPilotIDs = append(pilot.OtherAttackerPilotIDs, value.Pilot.CharacterID)
			}
		}

	}

	pilotShipFlown, found := pilot.ShipsFlown[ship.ItemID]
	newPilotShipFlown := UpdateShipFlown(found, &pilotShipFlown, expandedKillmail, km, isVictim)
	if newPilotShipFlown != nil {
		pilot.ShipsFlown[ship.ItemID] = *newPilotShipFlown
	}

	shipFlown, found := results.ShipsFlown[ship.ItemID]
	newShipFlown := UpdateShipFlown(found, &shipFlown, expandedKillmail, km, isVictim)
	if newShipFlown != nil {
		results.ShipsFlown[ship.ItemID] = *newShipFlown
		shipFlown = *newShipFlown

	}

	if comm.CapitalShips.Contains(ship.ItemID) {
		results.UpdateCapitalPilot(&pilot, &shipFlown, *expandedKillmail)
	}

	systemVisited, found := pilot.SystemsVisited[km.ESIKillmail.SolarSystemID]

	if !found {
		systemVisited = SystemVisted{
			System:         expandedKillmail.System,
			TotalKillmails: 1,
			KillmailIDs:    []int{km.ESIKillmail.KillmailID},
			Activity:       []time.Time{km.ESIKillmail.KillmailTime},
		}
		pilot.SystemsVisited[km.ESIKillmail.SolarSystemID] = systemVisited
	} else {
		systemVisited.TotalKillmails = 1 + systemVisited.TotalKillmails
		systemVisited.KillmailIDs = append(systemVisited.KillmailIDs, km.ESIKillmail.KillmailID)
		systemVisited.Activity = append(systemVisited.Activity, km.ESIKillmail.KillmailTime)

	}

	results.UpdateOrAddAlliance(&pilot, &systemVisited, expandedKillmail, currentBattleID)
	results.UpdateOrAddCoproration(&pilot, &systemVisited, expandedKillmail, currentBattleID)

	if comm.UpwellStructures.Contains(ship.ItemID) {
		systemVisited.UpdateOrAddStructure(pilotShipFlown, character, expandedKillmail, currentBattleID, isVictim)
	}

	pilot.BattlesFoughtIDs[currentBattleID] = struct{}{}

	activityDay := DayActive(expandedKillmail.KillmailTime)
	if !slices.Contains(pilot.DaysActive, activityDay) {
		pilot.DaysActive = append(pilot.DaysActive, activityDay)
	}

	activityHour := HourActive(expandedKillmail.KillmailTime)
	hoursActive, foundHour := pilot.HoursActive[activityHour]
	if foundHour {
		pilot.HoursActive[activityHour] = hoursActive + 1
	} else {
		pilot.HoursActive[activityHour] = 1
	}

	pilot.CalculateDaysSeenOverTime()
	results.PilotsSeen[character.CharacterID] = pilot

}

func (results *CollectedData) UpdateOrAddAlliance(pilot *PilotSeen, system *SystemVisted, km *retrieval.Killmail, currentBattleId string) {
	if pilot.Pilot.Corporation == nil || pilot.Pilot.Corporation.Alliance == nil {
		return
	}

	alliance, found := results.AlliancesSeen[pilot.Pilot.Corporation.Alliance.AllianceID]

	if !found {
		alliance = AllianceSeen{
			Alliance:         pilot.Pilot.Corporation.Alliance,
			Pilots:           map[int]*PilotSeen{pilot.Pilot.CharacterID: pilot},
			SystemsVisited:   map[int]*SystemVisted{system.System.SystemID: system},
			Activity:         []time.Time{km.KillmailTime},
			TotalKillmails:   0,
			KillmailIDs:      map[int]struct{}{km.KillmailID: struct{}{}},
			BattlesFoughtIDs: map[string]struct{}{currentBattleId: struct{}{}},
		}
		results.AlliancesSeen[pilot.Pilot.Corporation.Alliance.AllianceID] = alliance
	} else {

		if _, found := alliance.KillmailIDs[km.KillmailID]; !found {
			alliance.Activity = append(alliance.Activity, km.KillmailTime)
			alliance.TotalKillmails = alliance.TotalKillmails + 1
		}

		if _, found := alliance.Pilots[pilot.Pilot.CharacterID]; !found {
			alliance.Pilots[pilot.Pilot.CharacterID] = pilot
		}

		if _, found := alliance.SystemsVisited[system.System.SystemID]; !found {
			alliance.SystemsVisited[system.System.SystemID] = system
		}

		alliance.BattlesFoughtIDs[currentBattleId] = struct{}{}

	}
}

func (results *CollectedData) UpdateOrAddCoproration(pilot *PilotSeen, system *SystemVisted, km *retrieval.Killmail, currentBattleId string) {
	if pilot.Pilot.Corporation == nil { // TODO exclude NPC Corporations too
		return
	}

	corporation, found := results.CorporationsSeen[pilot.Pilot.Corporation.CorporationID]

	if !found {
		corporation = CorporationSeen{
			Corporation:      pilot.Pilot.Corporation,
			Pilots:           map[int]*PilotSeen{pilot.Pilot.CharacterID: pilot},
			SystemsVisited:   map[int]*SystemVisted{system.System.SystemID: system},
			Activity:         []time.Time{km.KillmailTime},
			TotalKillmails:   0,
			KillmailIDs:      map[int]struct{}{km.KillmailID: struct{}{}},
			BattlesFoughtIDs: map[string]struct{}{currentBattleId: struct{}{}},
		}
		results.CorporationsSeen[pilot.Pilot.Corporation.CorporationID] = corporation
	} else {

		if _, found := corporation.KillmailIDs[km.KillmailID]; !found {
			corporation.Activity = append(corporation.Activity, km.KillmailTime)
			corporation.TotalKillmails = corporation.TotalKillmails + 1
		}

		if _, found := corporation.Pilots[pilot.Pilot.CharacterID]; !found {
			corporation.Pilots[pilot.Pilot.CharacterID] = pilot
		}

		if _, found := corporation.SystemsVisited[system.System.SystemID]; !found {
			corporation.SystemsVisited[system.System.SystemID] = system
		}

		corporation.BattlesFoughtIDs[currentBattleId] = struct{}{}

	}
}

func UpdateShipFlown(found bool, shipFlown *ShipFlown, expandedKillmail *retrieval.Killmail, km *retrieval.RawKillmail, isVictim bool) *ShipFlown {
	if !found {
		shipFlown = &ShipFlown{
			Ship:                expandedKillmail.Victim.Ship,
			TotalKillmails:      1,
			Activity:            []time.Time{km.ESIKillmail.KillmailTime},
			Fits:                make(map[string][]*retrieval.FitItem),
			Destroyed:           1,
			AttackerKillmailIDs: []int{},
			VictimKillmailIDs:   []int{km.ESIKillmail.KillmailID},
		}

		if isVictim {
			if len(expandedKillmail.Victim.Fit) > 0 {
				shipFlown.Fits[fmt.Sprintf("%d-%d", km.ESIKillmail.KillmailID, expandedKillmail.Victim.Ship.ItemID)] = expandedKillmail.Victim.Fit
			}
			//if len(km.ESIKillmail.Victim.Items) > 0{
			//shipFlown.Fits[fmt.Sprintf("%d-%d", km.ESIKillmail.KillmailID, expandedKillmail.Victim.Ship.ItemID)] = km.ESIKillmail.Victim.Items
			//} // TODO: convert fit to structure for use.
		}

		return shipFlown
	} else {
		shipFlown.TotalKillmails = 1 + shipFlown.TotalKillmails
		shipFlown.Activity = append(shipFlown.Activity, km.ESIKillmail.KillmailTime)
		shipFlown.Destroyed = 1 + shipFlown.Destroyed
		shipFlown.VictimKillmailIDs = append(shipFlown.VictimKillmailIDs, km.ESIKillmail.KillmailID)
		shipFlown.Fits[fmt.Sprintf("%d-%d", km.ESIKillmail.KillmailID, expandedKillmail.Victim.Ship.ItemID)] = expandedKillmail.Victim.Fit
	}

	return nil
}

func (results *CollectedData) UpdateCapitalPilot(pilot *PilotSeen, ship *ShipFlown, expandedKM retrieval.Killmail) {
	capitalPilot, found := results.CapitalPilots[pilot.Pilot.CharacterID]
	if !found {
		capitalPilot = CapitalPilot{
			Pilot:          pilot.Pilot.Name,
			Corporation:    pilot.Pilot.Corporation.Name,
			TotalKillmails: 1,
			KillmailIDs:    []int{expandedKM.KillmailID},
			Activity:       []time.Time{expandedKM.KillmailTime},
			Ships:          make(map[int]*ShipFlown),
		}
		if pilot.Pilot.Corporation.Alliance != nil {
			capitalPilot.Alliance = pilot.Pilot.Corporation.Alliance.Name
		}
	} else {
		capitalPilot.TotalKillmails = capitalPilot.TotalKillmails + 1
		capitalPilot.Activity = append(capitalPilot.Activity, expandedKM.KillmailTime)
		capitalPilot.KillmailIDs = append(capitalPilot.KillmailIDs, expandedKM.KillmailID)
	}

	_, found = capitalPilot.Ships[ship.Ship.ItemID]
	if !found {
		capitalPilot.Ships[ship.Ship.ItemID] = ship
	}

}

// UpdateOrAddSystem either adds a new system to the collected data or updates the total killmails, killmail ids, and activity times of an existing system.
func (results *CollectedData) UpdateOrAddSystem(km *retrieval.RawKillmail, expandedSystem *retrieval.System, currentBattleID string) {
	system, found := results.SystemsVisted[km.ESIKillmail.SolarSystemID]

	if !found {
		system = SystemVisted{
			System:           expandedSystem,
			TotalKillmails:   1,
			KillmailIDs:      []int{km.ESIKillmail.KillmailID},
			BattlesFoughtIDs: map[string]struct{}{currentBattleID: struct{}{}},
			Activity:         []time.Time{km.ESIKillmail.KillmailTime},
			StructuresSeen:   make(map[string]*Structure),
			StructuresBashed: make(map[time.Time]Structure),
		}
		results.SystemsVisted[km.ESIKillmail.KillmailID] = system
	} else {
		_, foundBattle := system.BattlesFoughtIDs[currentBattleID]
		if !foundBattle {
			system.BattlesFoughtIDs[currentBattleID] = struct{}{}
		}
		system.TotalKillmails = 1 + system.TotalKillmails
		system.KillmailIDs = append(system.KillmailIDs, km.ESIKillmail.KillmailID)
		system.Activity = append(system.Activity, km.ESIKillmail.KillmailTime)
	}
}

func (s *SystemVisted) UpdateOrAddStructure(structure ShipFlown, pilot *retrieval.Pilot, expandedKillmail *retrieval.Killmail, currentBattleID string, wasDestroyed bool) {

	identifier := fmt.Sprintf("%d-%d", structure.Ship.ItemID, pilot.Corporation.CorporationID)

	structureFound, found := s.StructuresSeen[identifier]

	if !found {
		structureFound = &Structure{
			Structure:        structure.Ship,
			TotalKillmails:   1,
			KillmailIDs:      []int{expandedKillmail.KillmailID},
			Activity:         []time.Time{expandedKillmail.KillmailTime},
			BattlesFoughtIDs: map[string]struct{}{currentBattleID: struct{}{}},
			Destroyed:        0,
			Owner:            pilot.Corporation.Name,
		}
		s.StructuresSeen[identifier] = structureFound
	} else {
		structureFound.TotalKillmails = structureFound.TotalKillmails + 1
		structureFound.Activity = append(structureFound.Activity, expandedKillmail.KillmailTime)
		structureFound.KillmailIDs = append(structureFound.KillmailIDs, expandedKillmail.KillmailID)
	}

	if wasDestroyed {
		structureFound.Destroyed = structureFound.Destroyed + 1

		fitIDentifier := fmt.Sprintf("%s %s (%s)", pilot.Corporation.Name, structure.Ship.Name, expandedKillmail.KillmailTime.Format("2006-01-02:15:04"))
		structureFound.Fits[fitIDentifier] = expandedKillmail.Victim.Fit

		s.StructuresBashed[expandedKillmail.KillmailTime] = Structure{
			Structure:        structure.Ship,
			Fits:             map[string][]*retrieval.FitItem{fitIDentifier: expandedKillmail.Victim.Fit},
			BattlesFoughtIDs: map[string]struct{}{currentBattleID: struct{}{}},
			KillmailIDs:      []int{expandedKillmail.KillmailID},
		}
	}

}

/*
	UpdateOrCreateCurrentBattle checks if two killmails were withinin 15 mins of each other in the same sy stem, if so then updates the CurrentBattle.

If they are not within 15 mins and same system, it appends the any previous battle (killmails >1) to the battleFought references and returns a clean battle.
*/
func (c *CollectedData) UpdateOrCreateCurrentBattle(lastKillmailTime time.Time, km *retrieval.RawKillmail, previousKillmailSystem int, currentBattle *BattleFought) *BattleFought {
	if WithinFifteenMinutes(lastKillmailTime, km.ESIKillmail.KillmailTime) && previousKillmailSystem == km.ESIKillmail.SolarSystemID {

		if currentBattle.TotalKillmails == 0 {
			currentBattle.BattleIDReference = fmt.Sprintf("%d/%s", km.ESIKillmail.SolarSystemID, km.ESIKillmail.KillmailTime.Format("200601021504"))
			currentBattle.SystemID = km.ESIKillmail.SolarSystemID
		}

		currentBattle.TotalKillmails = 1 + currentBattle.TotalKillmails
		currentBattle.KillmailIDs = append(currentBattle.KillmailIDs, km.ESIKillmail.KillmailID)

	} else {
		if currentBattle.TotalKillmails > 1 {
			lastBattle := currentBattle
			c.BattlesFought[lastBattle.BattleIDReference] = *lastBattle
		}

		currentBattle = &BattleFought{
			TotalKillmails: 0,
			UniquePilots:   make(map[int]struct{}),
			KillmailIDs:    []int{},
		}

	}
	return currentBattle
}

func WithinFifteenMinutes(previousKillmailTime time.Time, currentKillmailTime time.Time) bool {
	diff := currentKillmailTime.Sub(previousKillmailTime)

	if diff < 0 {
		diff = -diff
	}

	return diff <= 15*time.Minute
}

func HourActive(killmailTime time.Time) int {
	return killmailTime.Hour()
}

func DayActive(killmailTime time.Time) string {
	return killmailTime.Format("20060102")
}
