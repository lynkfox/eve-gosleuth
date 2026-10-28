package retrieval

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// prefetchWorkers is how many lookups PrefetchKillmailEntities runs at once. The api.RateGuard keeps them within the ESI's limits.
const prefetchWorkers = 20

/*
	PrefetchKillmailEntities looks up everything the killmails refer to before they are expanded, so expanding them is answered from the cache.

	- Pilots are looked up in bulk, api.ESI_BULK_IDS at a time: their names with universe/names, and their current corp and alliance with characters/affiliation.
	  Pilots either bulk call failed for are left to ExpandPilot, which looks them up one at a time.
	- Corporations, alliances, ship and weapon types, and systems have no bulk call, so each is looked up on its own, prefetchWorkers at a time.
	- So are the types of victims' fitted items, drones, fighters, implants, and boosters that fit_types.json does not have, which BuildFit names through the cache.
	  Cargo and other holds are not: a fit shows only cargo fit_types.json keeps.

	Only what is not already cached is looked up.

Parameters:
  - killmails: the killmails about to be expanded
*/
func (cached *CachedEveIDs) PrefetchKillmailEntities(killmails []RawKillmail) {
	started := time.Now()
	before := cached.Counts()

	pilots, corporations, alliances, items, systems := idsOnKillmails(killmails)

	needed := []int{}
	for characterID := range pilots {
		if !held[Pilot](cached, sectionExpanded, cached.Expanded, characterID) {
			needed = append(needed, characterID)
		}
	}
	slices.Sort(needed)

	comm.ReportProgress("Looking up %d pilots in bulk, and the corporations, alliances, ships and systems on %d killmails", len(needed), len(killmails))

	// the two bulk calls do not depend on each other
	var names map[int]string
	var affiliations map[int]api.ESIAffiliation
	if len(needed) > 0 {
		var wait sync.WaitGroup
		wait.Go(func() {
			var err error
			if names, err = cached.GetNamesFromEveIDs(needed); err != nil {
				comm.Logger.WithError(err).Warn("[ESI] Bulk pilot names failed, those pilots are looked up one at a time")
			}
		})
		wait.Go(func() {
			var err error
			if affiliations, err = cached.GetCharacterAffiliations(needed); err != nil {
				comm.Logger.WithError(err).Warn("[ESI] Bulk pilot affiliations failed, those pilots are looked up one at a time")
			}
		})
		wait.Wait()
	}

	// pilots are shown with their current corp and alliance, so those are looked up too
	for _, affiliation := range affiliations {
		corporations[affiliation.CorporationID] = struct{}{}
		if affiliation.AllianceID != 0 {
			alliances[affiliation.AllianceID] = struct{}{}
		}
	}

	lookups := []func(){}
	for corporationID := range corporations {
		if !held[api.ESICorporation](cached, sectionKnown, cached.Known, corporationID) {
			lookups = append(lookups, func() { _, _ = cached.GetIfNotCached(corporationID, types.CORPORATION) })
		}
	}
	for allianceID := range alliances {
		if !held[api.ESIAlliance](cached, sectionKnown, cached.Known, allianceID) {
			lookups = append(lookups, func() { _, _ = cached.GetIfNotCached(allianceID, types.ALLIANCE) })
		}
	}
	for itemID := range items {
		if !held[Item](cached, sectionExpanded, cached.Expanded, itemID) && !held[api.ESIItem](cached, sectionKnown, cached.Known, itemID) {
			lookups = append(lookups, func() { _, _ = cached.GetIfNotCached(itemID, types.ITEM) })
		}
	}
	for systemID := range systems {
		if !held[System](cached, sectionExpanded, cached.Expanded, systemID) {
			lookups = append(lookups, func() { cached.prefetchSystem(systemID) })
		}
	}
	runParallel(lookups, prefetchWorkers)

	// every pilot both bulk calls answered for can now be built from the cache
	built := 0
	for _, characterID := range needed {
		name, named := names[characterID]
		affiliation, affiliated := affiliations[characterID]
		if named && affiliated {
			cached.buildPilot(characterID, name, affiliation.CorporationID, true)
			built++
		}
	}

	comm.ReportProgress("%s", cached.Counts().Since(before).Annotate(
		fmt.Sprintf("Looked up %d pilots in bulk and %d other ids in %s", built, len(lookups), time.Since(started).Round(time.Millisecond))))
}

/*
	PrefetchItemTypes looks up item types that are not cached yet, prefetchWorkers at a time, ie everything the victims of a system's killmails carried
	before checking it for PI materials. Each type is cached until a newer SDE, so later searches look up only types not seen before.

Parameters:
  - typeIDs: the item types, repeats are looked up once
*/
func (cached *CachedEveIDs) PrefetchItemTypes(typeIDs []int) {
	started := time.Now()
	before := cached.Counts()

	seen := map[int]struct{}{}
	lookups := []func(){}
	for _, typeID := range typeIDs {
		if _, done := seen[typeID]; done || typeID == 0 {
			continue
		}
		seen[typeID] = struct{}{}
		if !held[api.ESIItem](cached, sectionKnown, cached.Known, typeID) {
			lookups = append(lookups, func() { _, _ = cached.GetIfNotCached(typeID, types.ITEM) })
		}
	}
	if len(lookups) == 0 {
		return
	}

	comm.ReportProgress("Looking up %d item types", len(lookups))
	runParallel(lookups, prefetchWorkers)
	comm.ReportProgress("%s", cached.Counts().Since(before).Annotate(fmt.Sprintf("Looked up %d item types in %s", len(lookups), time.Since(started).Round(time.Millisecond))))
}

/*
	PrefetchMoons looks up moons that are not cached yet, prefetchWorkers at a time, ie the moons of a corp's control towers, as the ESI has no bulk call
	that names a moon. Each moon is cached until a newer SDE.

Parameters:
  - moonIDs: the moons, repeats are looked up once
*/
func (cached *CachedEveIDs) PrefetchMoons(moonIDs []int) {
	seen := map[int]struct{}{}
	lookups := []func(){}
	for _, moonID := range moonIDs {
		if _, done := seen[moonID]; done || moonID == 0 {
			continue
		}
		seen[moonID] = struct{}{}
		if !held[api.ESIMoon](cached, sectionKnown, cached.Known, moonID) {
			lookups = append(lookups, func() { _, _ = cached.GetIfNotCached(moonID, types.MOON) })
		}
	}
	if len(lookups) == 0 {
		return
	}
	comm.ReportProgress("Looking up %d moons", len(lookups))
	runParallel(lookups, prefetchWorkers)
}

// idsOnKillmails collects the unique ids the killmails refer to, with the victims' item types fit_types.json does not have. NPC attackers have no character.
func idsOnKillmails(killmails []RawKillmail) (pilots, corporations, alliances, items, systems map[int]struct{}) {
	pilots, corporations, alliances, items, systems = map[int]struct{}{}, map[int]struct{}{}, map[int]struct{}{}, map[int]struct{}{}, map[int]struct{}{}
	add := func(set map[int]struct{}, id *int) {
		if id != nil && *id != 0 {
			set[*id] = struct{}{}
		}
	}

	for _, km := range killmails {
		esiKM := km.ESIKillmail
		add(systems, &esiKM.SolarSystemID)
		add(items, &esiKM.Victim.ShipTypeID)
		add(pilots, esiKM.Victim.CharacterID)
		add(corporations, esiKM.Victim.CorporationID)
		add(alliances, esiKM.Victim.AllianceID)
		// a fit names its items from fit_types.json, so only types newer than it, in the places BuildFit names them, are looked up
		for _, item := range esiKM.Victim.Items {
			if comm.FitTypeOf(item.ItemTypeID) == nil && namedOnFit(item.Flag) {
				add(items, &item.ItemTypeID)
			}
		}
		for _, attacker := range esiKM.Attackers {
			add(items, &attacker.ShipTypeID)
			add(items, attacker.WeaponTypeID)
			add(pilots, attacker.CharacterID)
			add(corporations, attacker.CorporationID)
			add(alliances, attacker.AllianceID)
		}
	}
	return
}

// namedOnFit reports whether BuildFit names an item at this flag even when fit_types.json does not have its type: a slot, the drone or fighter bay, an implant, or a booster
func namedOnFit(flag int) bool {
	slot, _ := comm.FitSlotOf(flag)
	return slot != comm.SlotCargo && slot != comm.SlotIgnored && slot != comm.SlotUnknown
}

// prefetchSystem looks up a system and the constellation and region ExpandSystem names it with
func (cached *CachedEveIDs) prefetchSystem(systemID int) {
	found, err := cached.GetIfNotCached(systemID, types.SYSTEM)
	system, _ := (*found).(*api.ESISystem)
	if err != nil || system == nil {
		return
	}
	constellation, _ := cached.GetEsiResponse(system.ConstellationID, types.CONSTELLATION)
	if constellation, ok := constellation.(*api.ESIConstellation); ok && constellation != nil {
		_, _ = cached.GetEsiResponse(constellation.RegionID, types.REGION)
	}
}

// held reports whether the cache already has a current T for an id, without counting it as a lookup
func held[T any](c *CachedEveIDs, s section, entries map[int]*CacheEntry, idNumber int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := cachedAs[T](c, s, entries, idNumber)
	return value != nil && !missingID(value)
}

// runParallel runs every task, at most workers at once, and waits for them all
func runParallel(tasks []func(), workers int) {
	var wait sync.WaitGroup
	slots := make(chan struct{}, workers)
	for _, task := range tasks {
		slots <- struct{}{}
		wait.Go(func() {
			defer func() { <-slots }()
			task()
		})
	}
	wait.Wait()
}
