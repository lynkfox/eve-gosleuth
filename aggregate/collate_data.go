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

// IsTargetLoss returns true if the killmail's victim is the target: the pilot itself, or a pilot or structure of the target corp or alliance or one of its holdings
func IsTargetLoss(target *retrieval.IntelTarget, killmail *retrieval.Killmail) bool {
	victim := killmail.Victim

	switch target.TypeOfEntity {
	case types.PILOT:
		return victim.Pilot != nil && victim.Pilot.CharacterID == target.IDNumber
	case types.CORPORATION, types.ALLIANCE:
		return target.IsGroup(groupIDs(victim.CorpAtKill, victim.AllianceAtKill))
	}

	return false
}

/*
IsInternal returns true if a corp or alliance target, counting its holdings, is the victim and every attacker, ie an awox or the main and a holding fighting.
Any attacker from outside, an NPC too, leaves it a loss as usual. An internal killmail is still one of the target's losses, but never one of its kills.
*/
func IsInternal(target *retrieval.IntelTarget, killmail *retrieval.Killmail) bool {
	if target.TypeOfEntity != types.CORPORATION && target.TypeOfEntity != types.ALLIANCE {
		return false
	}
	if len(killmail.Attackers) == 0 || !IsTargetLoss(target, killmail) {
		return false
	}
	for _, attacker := range killmail.Attackers {
		if !target.IsGroup(groupIDs(attacker.CorpAtKill, attacker.AllianceAtKill)) {
			return false
		}
	}
	return true
}

// groupIDs returns a corp's and alliance's ids, 0 for one that is nil
func groupIDs(corp *retrieval.Corporation, alliance *retrieval.Alliance) (corporationID int, allianceID int) {
	if corp != nil {
		corporationID = corp.CorporationID
	}
	if alliance != nil {
		allianceID = alliance.AllianceID
	}
	return corporationID, allianceID
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

	case types.CORPORATION, types.ALLIANCE:
		// the target corp, or a holding corp of a corp or alliance target
		return target.IsGroup(other_id_number, 0)
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
		if alliance != nil && alliance.AllianceID == other_id_number {
			return true
		}
		return target.IsGroup(0, other_id_number) // a holding alliance

	case types.ALLIANCE:
		return target.IsGroup(0, other_id_number)
	}

	return false

}

func CollateAllKillmailData(killmails *[]retrieval.RawKillmail, esiClient *api.ESIClient, cachedData *retrieval.CachedEveIDs) *CollectedData {

	results := &CollectedData{
		PilotsSeen:       make(map[int]PilotSeen),
		SystemsVisted:    make(map[int]SystemVisted),
		ShipsFlown:       make(map[int]ShipFlown),
		BattlesFought:    make(map[string]BattleFought),
		AlliancesSeen:    make(map[int]AllianceSeen),
		CorporationsSeen: make(map[int]CorporationSeen),
	}

	// the battles a killmail can still join, see JoinBattle
	openBattles := []*BattleFought{}

	// looked up together first, in bulk and in parallel, so expanding each killmail is answered from the cache
	cachedData.PrefetchKillmailEntities(*killmails)

	for index, km := range *killmails {

		before := cachedData.Counts()
		expandedKM := km.Expand(esiClient, cachedData)
		comm.ReportProgress("%s", cachedData.Counts().Since(before).Annotate(fmt.Sprintf("Processed killmail %d/%d #%d", index+1, len(*killmails), km.ESIKillmail.KillmailID)))

		if expandedKM == nil {
			comm.Logger.Warnf("Skipping killmail %d, it could not be expanded", km.ESIKillmail.KillmailID)
			continue
		}
		results.Killmails = append(results.Killmails, expandedKM)

		var currentBattle *BattleFought
		currentBattle, openBattles = results.JoinBattle(openBattles, &km, killmailPilots(expandedKM))

		results.UpdateOrAddSystem(&km, expandedKM.System, currentBattle.BattleIDReference)

		results.UpdateOrAddStructures(&km, expandedKM, currentBattle.BattleIDReference)

		// structure and deployable losses have no victim pilot to record
		if expandedKM.Victim.Pilot != nil {
			results.UpdateOrAddPilot(&km, expandedKM, expandedKM.Victim.Pilot, expandedKM.Victim.Ship, expandedKM.Victim.CorpAtKill, expandedKM.Victim.AllianceAtKill, currentBattle.BattleIDReference, true)
		}

		for _, attacker := range expandedKM.Attackers {
			results.UpdateOrAddPilot(&km, expandedKM, attacker.Pilot, attacker.Ship, attacker.CorpAtKill, attacker.AllianceAtKill, currentBattle.BattleIDReference, false)
		}

	}

	// the last battles have no next killmail to close them
	for _, battle := range openBattles {
		results.SaveBattle(battle)
	}

	return results

}

func (results *CollectedData) UpdateOrAddPilot(km *retrieval.RawKillmail, expandedKillmail *retrieval.Killmail, character *retrieval.Pilot, ship *retrieval.Item, corpAtKill *retrieval.Corporation, allianceAtKill *retrieval.Alliance, currentBattleID string, isVictim bool) {
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
			CorporationsAtKill:    make(map[int]struct{}),
			AlliancesAtKill:       make(map[int]struct{}),
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
	pilot.ShipsFlown[ship.ItemID] = UpdateShipFlown(found, pilotShipFlown, ship, expandedKillmail, km, isVictim)

	shipFlown, found := results.ShipsFlown[ship.ItemID]
	results.ShipsFlown[ship.ItemID] = UpdateShipFlown(found, shipFlown, ship, expandedKillmail, km, isVictim)

	systemVisited, found := pilot.SystemsVisited[km.ESIKillmail.SolarSystemID]

	if !found {
		systemVisited = SystemVisted{
			System:         expandedKillmail.System,
			TotalKillmails: 1,
			KillmailIDs:    []int{km.ESIKillmail.KillmailID},
			Activity:       []time.Time{km.ESIKillmail.KillmailTime},
		}
	} else {
		systemVisited.TotalKillmails = 1 + systemVisited.TotalKillmails
		systemVisited.KillmailIDs = append(systemVisited.KillmailIDs, km.ESIKillmail.KillmailID)
		systemVisited.Activity = append(systemVisited.Activity, km.ESIKillmail.KillmailTime)
	}
	pilot.SystemsVisited[km.ESIKillmail.SolarSystemID] = systemVisited

	results.UpdateOrAddAlliance(pilot.Pilot, km.ESIKillmail.SolarSystemID, expandedKillmail, currentBattleID)
	results.UpdateOrAddCoproration(pilot.Pilot, km.ESIKillmail.SolarSystemID, expandedKillmail, currentBattleID)

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

	if corpAtKill != nil {
		pilot.CorporationsAtKill[corpAtKill.CorporationID] = struct{}{}
	}
	if allianceAtKill != nil {
		pilot.AlliancesAtKill[allianceAtKill.AllianceID] = struct{}{}
	}

	// killmails are not collected in time order
	if pilot.FirstSeen.IsZero() || expandedKillmail.KillmailTime.Before(pilot.FirstSeen) {
		pilot.FirstSeen = expandedKillmail.KillmailTime
	}
	if expandedKillmail.KillmailTime.After(pilot.LastSeen) {
		pilot.LastSeen = expandedKillmail.KillmailTime
	}

	pilot.CalculateDaysSeenOverTime()
	results.PilotsSeen[character.CharacterID] = pilot

}

// UpdateOrAddAlliance counts a killmail for the pilot's current alliance, once per killmail however many of its pilots were on it
func (results *CollectedData) UpdateOrAddAlliance(pilot *retrieval.Pilot, systemID int, km *retrieval.Killmail, currentBattleId string) {
	if pilot.Corporation == nil || pilot.Corporation.Alliance == nil {
		return
	}

	allianceID := pilot.Corporation.Alliance.AllianceID
	alliance, found := results.AlliancesSeen[allianceID]
	if !found {
		alliance = AllianceSeen{
			Alliance:         pilot.Corporation.Alliance,
			PilotIDs:         map[int]struct{}{},
			SystemIDs:        map[int]struct{}{},
			KillmailIDs:      map[int]struct{}{},
			BattlesFoughtIDs: map[string]struct{}{},
		}
	}

	if _, counted := alliance.KillmailIDs[km.KillmailID]; !counted {
		alliance.KillmailIDs[km.KillmailID] = struct{}{}
		alliance.Activity = append(alliance.Activity, km.KillmailTime)
		alliance.TotalKillmails = 1 + alliance.TotalKillmails
	}
	alliance.PilotIDs[pilot.CharacterID] = struct{}{}
	alliance.SystemIDs[systemID] = struct{}{}
	alliance.BattlesFoughtIDs[currentBattleId] = struct{}{}

	results.AlliancesSeen[allianceID] = alliance
}

// UpdateOrAddCoproration counts a killmail for the pilot's current corporation, once per killmail however many of its pilots were on it
func (results *CollectedData) UpdateOrAddCoproration(pilot *retrieval.Pilot, systemID int, km *retrieval.Killmail, currentBattleId string) {
	if pilot.Corporation == nil { // TODO exclude NPC Corporations too
		return
	}

	corporationID := pilot.Corporation.CorporationID
	corporation, found := results.CorporationsSeen[corporationID]
	if !found {
		corporation = CorporationSeen{
			Corporation:      pilot.Corporation,
			PilotIDs:         map[int]struct{}{},
			SystemIDs:        map[int]struct{}{},
			KillmailIDs:      map[int]struct{}{},
			BattlesFoughtIDs: map[string]struct{}{},
		}
	}

	if _, counted := corporation.KillmailIDs[km.KillmailID]; !counted {
		corporation.KillmailIDs[km.KillmailID] = struct{}{}
		corporation.Activity = append(corporation.Activity, km.KillmailTime)
		corporation.TotalKillmails = 1 + corporation.TotalKillmails
	}
	corporation.PilotIDs[pilot.CharacterID] = struct{}{}
	corporation.SystemIDs[systemID] = struct{}{}
	corporation.BattlesFoughtIDs[currentBattleId] = struct{}{}

	results.CorporationsSeen[corporationID] = corporation
}

/*
	UpdateShipFlown counts a killmail a ship type was on, as the victim's ship (destroyed) or an attacker's

Parameters:
  - found: whether shipFlown is already being counted, if not it starts a new count
  - shipFlown: the count so far
  - ship: the ship type, the victim's or the attacker's
  - expandedKillmail, km: the killmail
  - isVictim: true if the ship was the victim

Returns:
  - ShipFlown: the updated count, to store back in its map
*/
func UpdateShipFlown(found bool, shipFlown ShipFlown, ship *retrieval.Item, expandedKillmail *retrieval.Killmail, km *retrieval.RawKillmail, isVictim bool) ShipFlown {
	if !found {
		shipFlown = ShipFlown{
			Ship:                ship,
			Fits:                make(map[string]*retrieval.Fit),
			AttackerKillmailIDs: []int{},
			VictimKillmailIDs:   []int{},
		}
	}

	shipFlown.TotalKillmails = 1 + shipFlown.TotalKillmails
	shipFlown.Activity = append(shipFlown.Activity, km.ESIKillmail.KillmailTime)

	if isVictim {
		shipFlown.Destroyed = 1 + shipFlown.Destroyed
		shipFlown.VictimKillmailIDs = append(shipFlown.VictimKillmailIDs, km.ESIKillmail.KillmailID)
		if expandedKillmail.Victim.Fit.HasModules() {
			shipFlown.Fits[fmt.Sprintf("%d-%d", km.ESIKillmail.KillmailID, ship.ItemID)] = expandedKillmail.Victim.Fit
		}
	} else {
		shipFlown.AttackerKillmailIDs = append(shipFlown.AttackerKillmailIDs, km.ESIKillmail.KillmailID)
	}

	return shipFlown
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
		results.SystemsVisted[km.ESIKillmail.SolarSystemID] = system
	} else {
		_, foundBattle := system.BattlesFoughtIDs[currentBattleID]
		if !foundBattle {
			system.BattlesFoughtIDs[currentBattleID] = struct{}{}
		}
		system.TotalKillmails = 1 + system.TotalKillmails
		system.KillmailIDs = append(system.KillmailIDs, km.ESIKillmail.KillmailID)
		system.Activity = append(system.Activity, km.ESIKillmail.KillmailTime)
		results.SystemsVisted[km.ESIKillmail.SolarSystemID] = system
	}
}

/*
	UpdateOrAddStructures records every structure on a killmail against the killmail's system: a victim with no pilot (upwell structure, control tower, or deployable) as destroyed, and any upwell structure or control tower attacker as seen.

Parameters:
  - km: the raw killmail, its system must already be in SystemsVisted
  - expandedKillmail: the expanded form of km
  - currentBattleID: the battle the killmail belongs to
*/
func (results *CollectedData) UpdateOrAddStructures(km *retrieval.RawKillmail, expandedKillmail *retrieval.Killmail, currentBattleID string) {
	// StructuresSeen and StructuresBashed are maps, so updating them through this copy of the system is kept
	system, found := results.SystemsVisted[km.ESIKillmail.SolarSystemID]
	if !found {
		return
	}

	victim := expandedKillmail.Victim
	if victim.Pilot == nil {
		system.UpdateOrAddStructure(victim.Ship, victim.CorpAtKill, victim.AllianceAtKill, expandedKillmail, currentBattleID, true)
	}

	for _, attacker := range expandedKillmail.Attackers {
		if IsStructure(attacker.Ship) {
			system.UpdateOrAddStructure(attacker.Ship, attacker.CorpAtKill, attacker.AllianceAtKill, expandedKillmail, currentBattleID, false)
		}
	}
}

// IsStructure returns true if the item is an upwell structure or control tower
func IsStructure(item *retrieval.Item) bool {
	return item != nil && (comm.UpwellStructures.Contains(item.ItemID) || comm.ControlTowers.Contains(item.ItemID))
}

// StructureIdentifier is the StructuresSeen key for a structure type owned by a corp and alliance combination. A nil corp or alliance is keyed as 0.
func StructureIdentifier(structureType *retrieval.Item, corp *retrieval.Corporation, alliance *retrieval.Alliance) string {
	corpID, allianceID := 0, 0
	if corp != nil {
		corpID = corp.CorporationID
	}
	if alliance != nil {
		allianceID = alliance.AllianceID
	}
	return fmt.Sprintf("%d-%d-%d", structureType.ItemID, corpID, allianceID)
}

/*
	UpdateOrAddStructure adds or updates a structure in the system, tracked separately for each corp and alliance combination that owned it, since corps change alliances over time.

Parameters:
  - structureType: the structure's item type
  - corp: the owning corp at the time of the killmail
  - alliance: the owning corp's alliance at the time of the killmail, nil if it had none
  - expandedKillmail: the killmail the structure was seen on
  - currentBattleID: the battle the killmail belongs to
  - wasDestroyed: true if the structure was the victim
*/
func (s *SystemVisted) UpdateOrAddStructure(structureType *retrieval.Item, corp *retrieval.Corporation, alliance *retrieval.Alliance, expandedKillmail *retrieval.Killmail, currentBattleID string, wasDestroyed bool) {

	identifier := StructureIdentifier(structureType, corp, alliance)

	structureFound, found := s.StructuresSeen[identifier]

	if !found {
		structureFound = &Structure{
			Structure:        structureType,
			Corporation:      corp,
			Alliance:         alliance,
			Fits:             make(map[string]*retrieval.Fit),
			TotalKillmails:   1,
			KillmailIDs:      []int{expandedKillmail.KillmailID},
			Activity:         []time.Time{expandedKillmail.KillmailTime},
			BattlesFoughtIDs: map[string]struct{}{currentBattleID: {}},
			Destroyed:        0,
		}
		s.StructuresSeen[identifier] = structureFound
	} else {
		structureFound.TotalKillmails = structureFound.TotalKillmails + 1
		structureFound.Activity = append(structureFound.Activity, expandedKillmail.KillmailTime)
		structureFound.KillmailIDs = append(structureFound.KillmailIDs, expandedKillmail.KillmailID)
		structureFound.BattlesFoughtIDs[currentBattleID] = struct{}{}
	}

	if wasDestroyed {
		structureFound.Destroyed = structureFound.Destroyed + 1

		fitIdentifier := fmt.Sprintf("%s %s (%s)", identifier, structureType.Name, expandedKillmail.KillmailTime.Format("2006-01-02:15:04"))
		structureFound.Fits[fitIdentifier] = expandedKillmail.Victim.Fit

		s.StructuresBashed[expandedKillmail.KillmailTime] = Structure{
			Structure:        structureType,
			Corporation:      corp,
			Alliance:         alliance,
			Fits:             map[string]*retrieval.Fit{fitIdentifier: expandedKillmail.Victim.Fit},
			TotalKillmails:   1,
			KillmailIDs:      []int{expandedKillmail.KillmailID},
			Activity:         []time.Time{expandedKillmail.KillmailTime},
			BattlesFoughtIDs: map[string]struct{}{currentBattleID: {}},
			Destroyed:        1,
		}
	}

}

/*
	JoinBattle adds a killmail to the battle it belongs to, or starts a new one, so every killmail belongs to a battle with an id.
	Killmails come in time order (either way), and a battle stays open while each killmail is within 15 minutes of the one before it in that battle.
	Several battles can be open at once, as unrelated fights happen at the same time. A killmail joins an open battle:
	  - in the same system, or
	  - failing that, in another system with pilots in common, as when a fight moves through a gate. It joins the one with the most in common.
	A battle no longer within 15 minutes of the killmail is closed and saved.

Parameters:
  - open: the battles still open
  - km: the killmail to add
  - pilots: the character ids on the killmail, victim and attackers

Returns:
  - *BattleFought: the battle the killmail is now in
  - []*BattleFought: the battles still open, including it
*/
func (c *CollectedData) JoinBattle(open []*BattleFought, km *retrieval.RawKillmail, pilots []int) (*BattleFought, []*BattleFought) {
	at := km.ESIKillmail.KillmailTime
	system := km.ESIKillmail.SolarSystemID

	stillOpen := open[:0]
	for _, battle := range open {
		if WithinFifteenMinutes(battle.LatestTime, at) {
			stillOpen = append(stillOpen, battle)
		} else {
			c.SaveBattle(battle)
		}
	}

	var joined *BattleFought
	mostShared := 0
	for _, battle := range stillOpen {
		if _, here := battle.Systems[system]; here {
			joined = battle
			break
		}
		shared := 0
		for _, pilot := range pilots {
			if _, on := battle.UniquePilots[pilot]; on {
				shared++
			}
		}
		if shared > mostShared {
			joined, mostShared = battle, shared
		}
	}

	if joined == nil {
		joined = &BattleFought{
			BattleIDReference: fmt.Sprintf("%d/%s", system, at.Format("200601021504")),
			SystemID:          system,
			Systems:           make(map[int]struct{}),
			UniquePilots:      make(map[int]struct{}),
			KillmailIDs:       []int{},
		}
		stillOpen = append(stillOpen, joined)
	}

	joined.TotalKillmails++
	joined.KillmailIDs = append(joined.KillmailIDs, km.ESIKillmail.KillmailID)
	joined.Systems[system] = struct{}{}
	for _, pilot := range pilots {
		joined.UniquePilots[pilot] = struct{}{}
	}
	joined.LatestTime = at
	return joined, stillOpen
}

// killmailPilots are the character ids on a killmail, victim and attackers, leaving out NPCs and structures, which have none
func killmailPilots(km *retrieval.Killmail) []int {
	pilots := []int{}
	if km.Victim != nil && km.Victim.Pilot != nil && km.Victim.Pilot.CharacterID != 0 {
		pilots = append(pilots, km.Victim.Pilot.CharacterID)
	}
	for _, attacker := range km.Attackers {
		if attacker.Pilot != nil && attacker.Pilot.CharacterID != 0 {
			pilots = append(pilots, attacker.Pilot.CharacterID)
		}
	}
	return pilots
}

/*
	BattleOfKillmail maps each killmail in a battle of more than one killmail to that battle's id. A killmail on its own is not in it.

Returns:
  - map[int]string: killmail id to battle id
*/
func (c *CollectedData) BattleOfKillmail() map[int]string {
	battleOf := map[int]string{}
	for id, battle := range c.BattlesFought {
		for _, killmailID := range battle.KillmailIDs {
			battleOf[killmailID] = id
		}
	}
	return battleOf
}

// SaveBattle records a battle in BattlesFought if it had more than one killmail, a single killmail is not a battle
func (c *CollectedData) SaveBattle(battle *BattleFought) {
	if battle.TotalKillmails > 1 {
		c.BattlesFought[battle.BattleIDReference] = *battle
	}
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
