package aggregate

import (
	"cmp"
	"slices"
	"sync"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// Which weapons are smartbombs comes from static data: comm.SmartbombGroups

// RatKind is how sure a loss is to be a ratting loss, by who else was on it. The lower, the surer.
type RatKind int

const (
	RatsOnly       RatKind = iota // only NPCs were on the killmail
	RatsSmartbombs                // NPCs, and players who only hit it with smartbombs, ie a bomber clearing rats who caught the victim too
	RatsAndPlayers                // NPCs and players with other weapons, ie a ratter caught by a gang while the rats were on them
)

// RatCount is one kind of NPC on a ratting loss, by its ship type, and how many of them were on it
type RatCount struct {
	Ship  *retrieval.Item
	Count int
}

// RatLoss is a ship a pilot lost in the system with NPCs on the killmail
type RatLoss struct {
	Killmail *retrieval.Killmail
	Kind     RatKind
	Rats     []RatCount // the NPCs on it, the most numerous first
	Players  int        // the players on it
}

// Ratter is a pilot with ratting losses in the system, newest first
type Ratter struct {
	Pilot    *retrieval.Pilot
	Losses   []RatLoss
	BestKind RatKind // the surest of their losses
}

/*
	GetRatters finds the pilots ratting in a system, by their losses with NPCs on the killmail.
	Losses only NPCs were on are the surest, then those where the only players used smartbombs, then those with other players on them too.

Parameters:
  - killmails: the system target's expanded killmails
  - esiClient: looks up each player attacker's weapon group, ie the cache. The weapon types are prefetched with the killmails.

Returns:
  - []Ratter: each pilot with their losses newest first. The pilots with the surest ratting losses come first, then the one with the latest loss. Empty when there are none.
*/
func GetRatters(killmails []*retrieval.Killmail, esiClient api.ESIClient) []Ratter {
	byPilot := map[int]*Ratter{}

	for _, km := range killmails {
		victim := km.Victim
		if victim == nil || victim.Pilot == nil || victim.Pilot.NPC || victim.Ship == nil {
			continue
		}
		loss, isRatting := ratLoss(km, esiClient)
		if !isRatting {
			continue
		}

		pilot, found := byPilot[victim.Pilot.CharacterID]
		if !found {
			pilot = &Ratter{Pilot: victim.Pilot, BestKind: loss.Kind}
			byPilot[victim.Pilot.CharacterID] = pilot
		}
		pilot.Losses = append(pilot.Losses, loss)
		pilot.BestKind = min(pilot.BestKind, loss.Kind)
	}

	newestFirst := func(a, b RatLoss) int { return b.Killmail.KillmailTime.Compare(a.Killmail.KillmailTime) }
	pilots := []Ratter{}
	for _, pilot := range byPilot {
		slices.SortFunc(pilot.Losses, newestFirst)
		pilots = append(pilots, *pilot)
	}
	slices.SortFunc(pilots, func(a, b Ratter) int {
		if byKind := cmp.Compare(a.BestKind, b.BestKind); byKind != 0 {
			return byKind
		}
		if byTime := newestFirst(a.Losses[0], b.Losses[0]); byTime != 0 {
			return byTime
		}
		return cmp.Compare(a.Pilot.Name, b.Pilot.Name)
	})

	return pilots
}

// ratLoss sorts a killmail's attackers into NPCs and players, returning false when no NPC was on it
func ratLoss(km *retrieval.Killmail, esiClient api.ESIClient) (RatLoss, bool) {
	loss := RatLoss{Killmail: km, Kind: RatsOnly, Rats: []RatCount{}}
	rats := map[int]*RatCount{}
	for _, attacker := range km.Attackers {
		if IsRat(attacker) {
			// sentry guns and police shoot players who aren't ratting, so are neither a rat nor a player
			if attacker.Ship == nil || !ratShip(attacker.Ship, esiClient) {
				continue
			}
			rat, found := rats[attacker.Ship.ItemID]
			if !found {
				rat = &RatCount{Ship: attacker.Ship}
				rats[attacker.Ship.ItemID] = rat
			}
			rat.Count++
			continue
		}

		loss.Players++
		if loss.Kind != RatsAndPlayers && !isSmartbomb(attacker.Weapon, esiClient) {
			loss.Kind = RatsAndPlayers
		} else if loss.Kind == RatsOnly {
			loss.Kind = RatsSmartbombs
		}
	}
	if len(rats) == 0 {
		return loss, false
	}

	for _, rat := range rats {
		loss.Rats = append(loss.Rats, *rat)
	}
	slices.SortFunc(loss.Rats, func(a, b RatCount) int {
		if byCount := cmp.Compare(b.Count, a.Count); byCount != 0 {
			return byCount
		}
		return cmp.Compare(a.Ship.Name, b.Ship.Name)
	})
	return loss, true
}

/*
	IsRat reports whether an attacker is an NPC: no character, and in an NPC corporation or none. Structures have no character either, but belong to a player corp.
	Gate sentries, CONCORD, and police are NPCs too, see IsPvENPC.

Parameters:
  - attacker: one of a killmail's attackers

Returns:
  - bool: true for an NPC
*/
func IsRat(attacker *retrieval.KillmailAttacker) bool {
	if attacker.Pilot != nil && !attacker.Pilot.NPC {
		return false
	}
	return attacker.CorpAtKill == nil || api.IsNPCCorporation(attacker.CorpAtKill.CorporationID)
}

/*
	IsPvENPC reports whether an attacker is an NPC met in PvE: a rat (IsRat) whose ship is not in ships.json's npc_not_rat_groups, so not a gate or station sentry gun, CONCORD, police, or a faction navy

Parameters:
  - attacker: one of a killmail's attackers
  - esiClient: looks up the NPC ship's group, which fit_types.json does not have

Returns:
  - bool: true for a rat
*/
func IsPvENPC(attacker *retrieval.KillmailAttacker, esiClient api.ESIClient) bool {
	return IsRat(attacker) && (attacker.Ship == nil || ratShip(attacker.Ship, esiClient))
}

/*
	PvENPCs counts the attackers that are NPCs met in PvE (IsPvENPC), which the Ratting tag reads

Parameters:
  - attackers: a killmail's attackers
  - esiClient: looks up the NPC ships' groups

Returns:
  - int: how many
*/
func PvENPCs(attackers []*retrieval.KillmailAttacker, esiClient api.ESIClient) int {
	count := 0
	for _, attacker := range attackers {
		if IsPvENPC(attacker, esiClient) {
			count++
		}
	}
	return count
}

// unknownNPCShips are the NPC ship types already warned of as not looked up, so a rate limited search warns once a type, not once an attacker
var unknownNPCShips sync.Map

// ratShip reports whether an NPC's ship is a rat's, not one in npc_not_rat_groups. A ship the ESI can't look up, or with no client to ask, counts as a rat's, as most NPCs are.
func ratShip(ship *retrieval.Item, esiClient api.ESIClient) bool {
	if esiClient == nil {
		return true
	}
	info, err := esiClient.GetItemInfo(ship.ItemID)
	if err != nil || info == nil {
		if _, warned := unknownNPCShips.LoadOrStore(ship.ItemID, true); !warned {
			comm.Logger.WithError(err).Warnf("Unable to look up NPC ship type %d, it is counted as a rat", ship.ItemID)
		}
		return true
	}
	return !comm.NPCNotRatGroups.ContainsIDNumber(info.GroupID)
}

// isSmartbomb reports whether a player attacker's weapon is a smartbomb
func isSmartbomb(weapon *retrieval.Item, esiClient api.ESIClient) bool {
	if weapon == nil {
		return false
	}
	info, err := esiClient.GetItemInfo(weapon.ItemID)
	if err != nil || info == nil {
		comm.Logger.WithError(err).Warnf("Unable to look up weapon type %d, it is not counted as a smartbomb", weapon.ItemID)
		return false
	}
	return comm.SmartbombGroups.ContainsIDNumber(info.GroupID)
}
