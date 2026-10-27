package gui

import (
	"errors"
	"maps"
	"slices"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// intelCheckBatch is how many ids one universe/names call checks, the ESI's most
const intelCheckBatch = 1000

// errChecksClosing stops a check when the app closes, leaving what is left for the next start
var errChecksClosing = errors.New("the app is closing")

// intelCategories are universe/names' kinds of entity, as My Intel's types
var intelCategories = map[string]string{"character": "PILOT", "corporation": "CORPORATION", "alliance": "ALLIANCE"}

/*
runIntelChecks checks the imported pilots, corporations, and alliances still to check (retrieval.IntelChecks) with the ESI, in bulk, without a word to the user
while it does. Names are corrected to EVE's, and holdings and ignored allies with an entity that does not exist as the type the file said are removed, noted in
the ticker only if any were. One runs at a time; if the ESI can not be asked, ie while it is rate limited, the checks are kept for the next run: the next import,
search, or start. A search runs it first, so it uses checked names, which takes a call or two, or nothing when there is none.
*/
func (a *App) runIntelChecks() {
	a.checkMu.Lock()
	defer a.checkMu.Unlock()
	if a.closing.Load() || a.cache == nil {
		return
	}
	checks := a.cache.IntelChecks()
	if len(checks) == 0 {
		return
	}

	ids := []int{}
	for _, check := range checks {
		if !slices.Contains(ids, check.ID) {
			ids = append(ids, check.ID)
		}
	}
	found, err := a.lookupEntities(ids)
	if err != nil {
		comm.Logger.WithError(err).Infof("Could not check %d imported entities with the ESI, they are kept for the next try", len(checks))
		return
	}

	// what each checked entity is: its name as EVE spells it, or "" if it does not exist as the type the file said
	names := map[string]string{}
	for _, check := range checks {
		entity, exists := found[check.ID]
		name := ""
		if exists && intelCategories[entity.Category] == check.Type {
			name = entity.Name
		}
		names[intelGroupKey(check.Type, check.ID)] = name
	}
	removed := a.correctIntel(names)

	a.cache.RemoveIntelChecks(checks)
	a.saveCacheQuietly()
	if removed == 1 {
		comm.ReportProgress("Removed an imported My Intel entry naming a pilot, corporation, or alliance that does not exist")
	} else if removed > 1 {
		comm.ReportProgress("Removed %d imported My Intel entries naming pilots, corporations, or alliances that do not exist", removed)
	}
}

/*
	lookupEntities asks the ESI for ids' names and kinds, intelCheckBatch at a time. As the ESI answers a whole batch with a 404 when any id in it does not exist,
	a batch that does is split in half and each half asked again, down to the ids that do not exist, which are left out.

Returns:
  - map[int]api.EntityIDAndName: those that exist, by id
  - error: the ESI could not be asked, other than a 404, or the app is closing
*/
func (a *App) lookupEntities(ids []int) (map[int]api.EntityIDAndName, error) {
	found := map[int]api.EntityIDAndName{}
	var ask func(batch []int) error
	ask = func(batch []int) error {
		if a.closing.Load() {
			return errChecksClosing
		}
		entities, err := a.clients.ESIClient.GetEntitiesFromEveIDs(batch)
		switch {
		case err == nil:
			maps.Copy(found, entities)
			return nil
		case !api.IsNotFound(err):
			return err
		case len(batch) == 1:
			return nil // it does not exist
		}
		if err := ask(batch[:len(batch)/2]); err != nil {
			return err
		}
		return ask(batch[len(batch)/2:])
	}
	for batch := range slices.Chunk(ids, intelCheckBatch) {
		if err := ask(batch); err != nil {
			return nil, err
		}
	}
	return found, nil
}

/*
	correctIntel brings the holdings and ignored allies naming checked entities in line with the ESI

Parameters:
  - names: each checked entity's name as EVE spells it, by intelGroupKey, or "" for one that does not exist as that type

Returns:
  - int: how many holdings, ignored allies, and targets' lists of them were removed
*/
func (a *App) correctIntel(names map[string]string) int {
	removed := 0
	exists := func(typeName string, id int) (string, bool, bool) {
		name, checked := names[intelGroupKey(typeName, id)]
		return name, checked, !checked || name != ""
	}

	for _, link := range a.cache.AllHoldingLinks() {
		mainName, mainChecked, mainOK := exists(link.MainType, link.MainID)
		holdingName, holdingChecked, holdingOK := exists(link.HoldingType, link.HoldingID)
		if !mainChecked && !holdingChecked {
			continue
		}
		if !mainOK || !holdingOK {
			a.cache.RemoveHoldingLink(link.MainType, link.MainID, link.HoldingType, link.HoldingID)
			removed++
			continue
		}
		corrected := link
		if mainChecked {
			corrected.MainName = mainName
		}
		if holdingChecked {
			corrected.HoldingName = holdingName
		}
		if corrected.MainName != link.MainName || corrected.HoldingName != link.HoldingName {
			a.cache.SaveHoldingLink(corrected)
		}
	}

	for _, target := range a.cache.AllIgnoredAllies() {
		if _, _, ok := exists(target.TargetType, target.TargetID); !ok {
			a.cache.SetIgnoredAllies(target.TargetType, target.TargetID, nil)
			removed++
			continue
		}
		kept := []retrieval.IgnoredAlly{}
		changed := false
		for _, ally := range target.Allies {
			name, checked, ok := exists(ally.Type, ally.ID)
			switch {
			case !ok:
				removed++
				changed = true
			case checked && name != ally.Name:
				ally.Name = name
				kept = append(kept, ally)
				changed = true
			default:
				kept = append(kept, ally)
			}
		}
		if changed {
			a.cache.SetIgnoredAllies(target.TargetType, target.TargetID, kept)
		}
	}
	return removed
}
