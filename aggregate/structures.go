package aggregate

import (
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

/*
StructureUse is one of a corp or alliance target's structures seen on its kills: an upwell structure, or a control tower seen through its batteries.
Killmails give a structure no name or id, so a structure is its type, system, and owning corp: two of one type a corp has in one system are one StructureUse.
*/
type StructureUse struct {
	Structure   *retrieval.Item // the upwell structure's or control tower's type. A tower seen only through its batteries has none, unless its loss names it.
	Battery     *retrieval.Item // a control tower's battery on its newest kill, nil for an upwell structure
	System      *retrieval.System
	Corporation *retrieval.Corporation
	Alliance    *retrieval.Alliance    // the owning corp's alliance on its newest kill, nil for none
	Via         *retrieval.TargetGroup // the holding owning it, nil for the target itself
	Kills       int
	FirstSeen   time.Time
	LastSeen    time.Time
	LatestKill  *retrieval.Killmail
	Loss        *retrieval.Killmail // its loss after its newest kill, nil while it stands
}

/*
	StructuresUsed finds the target's structures on the kills its structures were on, and which of them were lost since. A loss counts for a structure
	when it is the same type (any control tower, for a tower), in the same system, of the same corp, after the structure's newest kill.

Parameters:
  - used: the expanded killmails the target's structures were on, newest first
  - lost: the target's expanded structure losses, newest first
  - target: the corp or alliance target, with any Holdings

Returns:
  - []StructureUse: the structures standing first, then those lost, each newest kill first
*/
func StructuresUsed(used []*retrieval.Killmail, lost []*retrieval.Killmail, target *retrieval.IntelTarget) []StructureUse {
	byKey := map[string]*StructureUse{}
	order := []string{}
	for _, km := range used {
		if km.System == nil {
			continue
		}
		onKill := map[string]bool{} // a tower's batteries count it once a kill
		for _, attacker := range km.Attackers {
			if attacker.Ship == nil || !target.IsGroup(groupIDs(attacker.CorpAtKill, attacker.AllianceAtKill)) {
				continue
			}
			var structure, battery *retrieval.Item
			switch {
			case IsStructure(attacker.Ship):
				structure = attacker.Ship
			case comm.PosBatteryGroups.ContainsGroupOf(attacker.Ship.ItemID):
				battery = attacker.Ship
			default:
				continue
			}
			key := structureKey(structure, km.System.SystemID, attacker.CorpAtKill)
			if onKill[key] {
				continue
			}
			onKill[key] = true

			use, found := byKey[key]
			if !found {
				// killmails are newest first, so the first is its newest
				use = &StructureUse{Structure: structure, Battery: battery, System: km.System, Corporation: attacker.CorpAtKill, Alliance: attacker.AllianceAtKill,
					Via: viaFor(target, attacker.CorpAtKill, attacker.AllianceAtKill), LastSeen: km.KillmailTime, LatestKill: km}
				byKey[key] = use
				order = append(order, key)
			}
			use.Kills++
			use.FirstSeen = km.KillmailTime
		}
	}

	uses := []StructureUse{}
	for _, key := range order {
		use := byKey[key]
		for _, loss := range lost {
			if loss.KillmailTime.After(use.LastSeen) && lossOf(use, loss) {
				use.Loss = loss // lost is newest first, so this ends at the first loss after its newest kill
			}
		}
		if use.Loss != nil && use.Structure == nil {
			use.Structure = use.Loss.Victim.Ship
		}
		uses = append(uses, *use)
	}
	slices.SortStableFunc(uses, func(a, b StructureUse) int {
		if (a.Loss == nil) != (b.Loss == nil) {
			if a.Loss == nil {
				return -1
			}
			return 1
		}
		return b.LastSeen.Compare(a.LastSeen)
	})
	return uses
}

// structureKey keys a structure by its type, or as a control tower when it is one or only its battery was seen, its system, and its owning corp
func structureKey(structure *retrieval.Item, systemID int, corp *retrieval.Corporation) string {
	kind := "tower"
	if structure != nil && !comm.ControlTowerGroups.ContainsGroupOf(structure.ItemID) {
		kind = fmt.Sprint(structure.ItemID)
	}
	corpID, _ := groupIDs(corp, nil)
	return fmt.Sprintf("%s:%d:%d", kind, systemID, corpID)
}

// lossOf reports whether a structure loss is the structure's: the same type, or any control tower for a tower, in its system, of its corp
func lossOf(use *StructureUse, loss *retrieval.Killmail) bool {
	victim := loss.Victim
	if victim.Ship == nil || loss.System == nil || victim.Pilot != nil {
		return false
	}
	return structureKey(victim.Ship, loss.System.SystemID, victim.CorpAtKill) == structureKey(use.Structure, use.System.SystemID, use.Corporation)
}

// viaFor is the holding a corp or alliance on a killmail is, nil when it is the target itself
func viaFor(target *retrieval.IntelTarget, corp *retrieval.Corporation, alliance *retrieval.Alliance) *retrieval.TargetGroup {
	corporationID, allianceID := groupIDs(corp, alliance)
	if (retrieval.TargetGroup{ID: target.IDNumber, Type: target.TypeOfEntity}).Includes(corporationID, allianceID) {
		return nil
	}
	for index := range target.Holdings {
		if target.Holdings[index].Includes(corporationID, allianceID) {
			return &target.Holdings[index]
		}
	}
	return nil
}

// isMinorStructure reports whether a type is a structure with no guns, ie a Metenox moon drill or customs office, one of comm.MinorStructureGroups
func isMinorStructure(item *retrieval.Item) bool {
	return item != nil && comm.MinorStructureGroups.ContainsGroupOf(item.ItemID)
}

// StructureKill is a structure the target took part in destroying
type StructureKill struct {
	Killmail *retrieval.Killmail
	Pilots   int // the target's pilots on it
}

/*
	StructuresDestroyed finds the upwell structures, control towers, and minor structures (comm.MinorStructureGroups) the target took part in destroying, among its killmails

Parameters:
  - killmails: the target's expanded killmails, newest first
  - target: the corp or alliance target, with any Holdings

Returns:
  - []StructureKill: each structure killmail the target, or a holding, was on the attackers of, newest first
*/
func StructuresDestroyed(killmails []*retrieval.Killmail, target *retrieval.IntelTarget) []StructureKill {
	kills := []StructureKill{}
	for _, km := range killmails {
		if km.Victim.Pilot != nil || !(IsStructure(km.Victim.Ship) || isMinorStructure(km.Victim.Ship)) || IsTargetLoss(target, km) {
			continue
		}
		pilots := 0
		for _, attacker := range km.Attackers {
			if attacker.Pilot != nil && !attacker.Pilot.NPC && flewForTarget(attacker.CorpAtKill, attacker.AllianceAtKill, target) {
				pilots++
			}
		}
		if pilots > 0 {
			kills = append(kills, StructureKill{Killmail: km, Pilots: pilots})
		}
	}
	return kills
}

// StructureKiller is who destroyed a structure: the group with the most attackers on it
type StructureKiller struct {
	Corporation *retrieval.Corporation // of the group with the most attackers, one of its attackers' corp for an alliance
	Alliance    *retrieval.Alliance    // nil when that group is a corp in no alliance
	Attackers   int
}

// KillerOf finds who destroyed a structure: attackers are grouped by alliance, or by corp for those in none, the first group with the most winning
func KillerOf(km *retrieval.Killmail) StructureKiller {
	killer := StructureKiller{Attackers: len(km.Attackers)}
	counts := map[string]int{}
	best := 0
	for _, attacker := range km.Attackers {
		corporationID, allianceID := groupIDs(attacker.CorpAtKill, attacker.AllianceAtKill)
		if corporationID == 0 && allianceID == 0 {
			continue
		}
		key := fmt.Sprintf("corp:%d", corporationID)
		if allianceID != 0 {
			key = fmt.Sprintf("alliance:%d", allianceID)
		}
		counts[key]++
		if counts[key] > best {
			best = counts[key]
			killer.Corporation, killer.Alliance = attacker.CorpAtKill, attacker.AllianceAtKill
		}
	}
	return killer
}
