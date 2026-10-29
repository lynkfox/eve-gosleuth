package aggregate

import (
	"fmt"
	"maps"
	"slices"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// battlePilot is a pilot seen flying a ship in a battle, as victim or attacker
type battlePilot struct {
	pilot      int
	ship       int
	corpID     int
	allianceID int
}

/*
	FleetDoctrineInBattle finds the target's losses flown as a fleet doctrine in their battle: with at least battle_hulls pilots of the loss's hull on its side, the victim among them, and at least battle_logi in logi hulls (ships.json's logi_groups and logi_types).
	The victim's side is the target and its holdings, and the victim's alliance, or corp if it has none, so a pilot target's fleetmates count. The enemy flying the same hull does not.
	The numbers are the fleet_doctrine of the tag that has one in fit_tag_rules.json.

Parameters:
  - killmails: the search's expanded killmails
  - target: the pilot, corp, or alliance searched
  - battleOf: each killmail in a battle to its battle's id, see CollectedData.BattleOfKillmail

Returns:
  - map[int]retrieval.FitTag: each qualifying loss's killmail id to its Fleet Doctrine tag, whose reasons are the hulls and logi counted. Empty if none qualify or no tag has a fleet_doctrine.
*/
func FleetDoctrineInBattle(killmails []*retrieval.Killmail, target *retrieval.IntelTarget, battleOf map[int]string) map[int]retrieval.FitTag {
	tags := map[int]retrieval.FitTag{}
	name, rule := fleetDoctrineRule()
	if rule == nil || rule.FleetDoctrine.BattleHulls <= 0 {
		return tags
	}
	doctrine := rule.FleetDoctrine

	pilotsIn := map[string][]battlePilot{}
	losses := []*retrieval.Killmail{}
	for _, km := range killmails {
		battle := battleOf[km.KillmailID]
		if battle == "" {
			continue
		}
		pilotsIn[battle] = append(pilotsIn[battle], killmailBattlePilots(km)...)
		if km.Victim != nil && km.Victim.Pilot != nil && km.Victim.Ship != nil && IsTargetLoss(target, km) {
			losses = append(losses, km)
		}
	}

	for _, loss := range losses {
		victimCorp, victimAlliance := groupIDs(loss.Victim.CorpAtKill, loss.Victim.AllianceAtKill)
		onSide := func(seen battlePilot) bool {
			if target.IsGroup(seen.corpID, seen.allianceID) {
				return true
			}
			if victimAlliance != 0 {
				return seen.allianceID == victimAlliance
			}
			return victimCorp != 0 && seen.corpID == victimCorp
		}
		hulls, logi := map[int]bool{loss.Victim.Pilot.CharacterID: true}, map[int]bool{}
		for _, seen := range pilotsIn[battleOf[loss.KillmailID]] {
			if !onSide(seen) {
				continue
			}
			if seen.ship == loss.Victim.Ship.ItemID {
				hulls[seen.pilot] = true
			}
			if comm.LogiGroups.ContainsGroupOf(seen.ship) || comm.LogiTypes.ContainsIDNumber(seen.ship) {
				logi[seen.pilot] = true
			}
		}
		if len(hulls) < doctrine.BattleHulls || len(logi) < doctrine.BattleLogi {
			continue
		}
		reasons := []retrieval.FitTagReason{{Name: fmt.Sprintf("%s in its battle", loss.Victim.Ship.Name), Count: len(hulls), Needed: doctrine.BattleHulls}}
		if doctrine.BattleLogi > 0 {
			reasons = append(reasons, retrieval.FitTagReason{Name: "Logi in its battle", Count: len(logi), Needed: doctrine.BattleLogi})
		}
		tags[loss.KillmailID] = fleetDoctrineTag(name, rule, reasons)
	}
	return tags
}

/*
	FleetDoctrineRepeated reports whether one fit is a fleet doctrine by its losses: at least repeat_losses of them within one alliance, by at least repeat_pilots pilots.
	Losses count by the victim's alliance at the time, or corp if it had none, and the target and its holdings count as one.

Parameters:
  - losses: the losses of one fit, ie one merged row of the Fits tab
  - target: the pilot, corp, or alliance searched

Returns:
  - retrieval.FitTag: its Fleet Doctrine tag, whose reasons are the losses and pilots counted
  - bool: false if the fit is not one, or no tag has a fleet_doctrine
*/
func FleetDoctrineRepeated(losses []*retrieval.Killmail, target *retrieval.IntelTarget) (retrieval.FitTag, bool) {
	name, rule := fleetDoctrineRule()
	if rule == nil || rule.FleetDoctrine.RepeatLosses <= 0 {
		return retrieval.FitTag{}, false
	}
	doctrine := rule.FleetDoctrine

	type count struct {
		losses int
		pilots map[int]bool
	}
	byGroup := map[string]*count{}
	for _, km := range losses {
		if km.Victim == nil || km.Victim.Pilot == nil {
			continue
		}
		corpID, allianceID := groupIDs(km.Victim.CorpAtKill, km.Victim.AllianceAtKill)
		group := ""
		switch {
		case target.IsGroup(corpID, allianceID):
			group = "target"
		case allianceID != 0:
			group = fmt.Sprintf("alliance %d", allianceID)
		case corpID != 0:
			group = fmt.Sprintf("corp %d", corpID)
		default:
			continue
		}
		if byGroup[group] == nil {
			byGroup[group] = &count{pilots: map[int]bool{}}
		}
		byGroup[group].losses++
		byGroup[group].pilots[km.Victim.Pilot.CharacterID] = true
	}

	// sorted, so the same group's counts are the reasons each time ("target" after the ids, which it can't clash with)
	for _, group := range slices.Sorted(maps.Keys(byGroup)) {
		counted := byGroup[group]
		if counted.losses >= doctrine.RepeatLosses && len(counted.pilots) >= doctrine.RepeatPilots {
			return fleetDoctrineTag(name, rule, []retrieval.FitTagReason{
				{Name: "Losses of this fit in one alliance", Count: counted.losses, Needed: doctrine.RepeatLosses},
				{Name: "Pilots who lost it", Count: len(counted.pilots), Needed: doctrine.RepeatPilots},
			}), true
		}
	}
	return retrieval.FitTag{}, false
}

// fleetDoctrineRule finds the tag in fit_tag_rules.json read from battles and other losses, nil if there is none
func fleetDoctrineRule() (string, *comm.FitTagRule) {
	for name, rule := range comm.FitTagRules().Tags {
		if rule.FleetDoctrine != nil {
			return name, rule
		}
	}
	return "", nil
}

// fleetDoctrineTag makes the Fleet Doctrine tag with its reasons, each a count against the number it needed rather than a weight
func fleetDoctrineTag(name string, rule *comm.FitTagRule, reasons []retrieval.FitTagReason) retrieval.FitTag {
	return retrieval.FitTag{Name: name, Meaning: rule.Meaning, Priority: rule.Priority, Role: rule.Role, Hidden: rule.Hidden, Reasons: reasons}
}

// killmailBattlePilots are the pilots on a killmail with their ships, victim and attackers, leaving out NPCs, structures, and those with no ship
func killmailBattlePilots(km *retrieval.Killmail) []battlePilot {
	pilots := []battlePilot{}
	add := func(pilot *retrieval.Pilot, ship *retrieval.Item, corp *retrieval.Corporation, alliance *retrieval.Alliance) {
		if pilot == nil || pilot.CharacterID == 0 || ship == nil {
			return
		}
		corpID, allianceID := groupIDs(corp, alliance)
		pilots = append(pilots, battlePilot{pilot: pilot.CharacterID, ship: ship.ItemID, corpID: corpID, allianceID: allianceID})
	}
	if km.Victim != nil {
		add(km.Victim.Pilot, km.Victim.Ship, km.Victim.CorpAtKill, km.Victim.AllianceAtKill)
	}
	for _, attacker := range km.Attackers {
		add(attacker.Pilot, attacker.Ship, attacker.CorpAtKill, attacker.AllianceAtKill)
	}
	return pilots
}
