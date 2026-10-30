package retrieval

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// StructureKillmails are the killmails a corp or alliance target's own structures were on, from its zKillboard lists narrowed to structure groups
type StructureKillmails struct {
	Used []RawKillmail // kills the target's upwell structures or control tower batteries were on, newest first
	Lost []RawKillmail // the target's upwell structures and control towers lost, newest first
}

/*
	CollectStructureKillmails lists the kills a corp or alliance target's own structures were on, and the structures it lost, for the Structures tab.
	Each of comm.UpwellStructureGroups, comm.ControlTowerGroups, and comm.PosBatteryGroups is its own zKillboard list (api.ZkillPaging Filter), for the target and each of its Holdings,
	paged and cached like the target's whole list (see listEntity) to the limits' Max or Pages, but never stopping at the date limit, as a structure
	may have stood for years. Control towers are only listed for losses, their batteries stand in for them on kills.
	Each killmail is the cache's copy, else zKillboard's, else fetched from the ESI.

Parameters:
  - target: the corp or alliance IntelTarget, with any Holdings
  - limits: the search's limits, of which Max, Pages, and IncludeCached are used

Returns:
  - StructureKillmails: the killmails found, each once, with Via naming the holding whose list it was on
  - error: nil, or the zkill pull and killmail errors joined, what was collected is still returned
*/
func CollectStructureKillmails(target IntelTarget, limits KillmailLimits) (StructureKillmails, error) {
	limits.NewerThan = time.Time{}
	paging, need := limits.paging()

	type entity struct {
		entityType types.EntityType
		id         int
		via        *TargetGroup
		whose      string
	}
	entities := []entity{{target.TypeOfEntity, target.IDNumber, nil, target.DisplayName()}}
	for index := range target.Holdings {
		holding := &target.Holdings[index]
		entities = append(entities, entity{holding.Type, holding.ID, holding, "affiliate " + holding.Name})
	}

	var found StructureKillmails
	seenUsed, seenLost := map[int]bool{}, map[int]bool{}
	var errs []error
	pull := func(on entity, side string, groupID int, groupName string, into *[]RawKillmail, seen map[int]bool) {
		listPaging := paging
		listPaging.Filter = fmt.Sprintf("%s/groupID/%d", side, groupID)
		listPaging.For = fmt.Sprintf("%s %s %s", on.whose, groupName, side)
		list, err := listEntity(target, on.entityType, on.id, listPaging, need)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s %s: %w", on.whose, groupName, side, err))
		}
		used := list.within
		if limits.IncludeCached {
			used = len(list.killmails)
		}
		for _, km := range list.killmails[:used] {
			if seen[km.KillmailID] {
				continue
			}
			body, err := killmailBody(target, km)
			if err != nil {
				errs = append(errs, fmt.Errorf("killmail %d: %w", km.KillmailID, err))
				continue
			}
			seen[km.KillmailID] = true
			*into = append(*into, RawKillmail{Zkillmail: &km, ESIKillmail: body, Via: on.via})
		}
	}

	comm.ReportProgress("Fetching structure killmail lists from zKillboard (live, only as far as the cached lists do not have them)")
	for _, on := range entities {
		for _, groupID := range comm.UpwellStructureGroups.IDNumbers() {
			name := comm.UpwellStructureGroups.NameOf(groupID)
			pull(on, "kills", groupID, name, &found.Used, seenUsed)
			pull(on, "losses", groupID, name, &found.Lost, seenLost)
		}
		for _, groupID := range comm.ControlTowerGroups.IDNumbers() {
			pull(on, "losses", groupID, comm.ControlTowerGroups.NameOf(groupID), &found.Lost, seenLost) // a tower is never on a killmail's attackers, its batteries are instead
		}
		for _, groupID := range comm.PosBatteryGroups.IDNumbers() {
			pull(on, "kills", groupID, comm.PosBatteryGroups.NameOf(groupID), &found.Used, seenUsed)
		}
	}

	newestFirst := func(a, b RawKillmail) int { return b.ESIKillmail.KillmailTime.Compare(a.ESIKillmail.KillmailTime) }
	slices.SortStableFunc(found.Used, newestFirst)
	slices.SortStableFunc(found.Lost, newestFirst)
	comm.ReportProgress("Found %d kills the target's structures were on, and %d structures lost", len(found.Used), len(found.Lost))
	return found, errors.Join(errs...)
}

// PreviousStructureLosses is how many structure losses StructureLosses gives besides a system's last
const PreviousStructureLosses = 5

/*
	StructureLosses finds the structures lost in a system target, however long ago, from page 1 of each zKillboard loss list for the system
	of comm.UpwellStructureGroups, comm.ControlTowerGroups, and comm.MinorStructureGroups.

Parameters:
  - target: the system IntelTarget

Returns:
  - *RawKillmail: the newest upwell structure or control tower lost, never a minor structure, nil when zKillboard lists none
  - []RawKillmail: the PreviousStructureLosses newest other structures lost, minor ones too, newest first
  - error: nil, or the zkill pull and killmail errors joined, what the lists pulled have is still returned
*/
func StructureLosses(target IntelTarget) (*RawKillmail, []RawKillmail, error) {
	var errs []error
	lists := [][]api.ZkbResponse{}
	minor := map[int]bool{} // killmails on the minor structures' lists
	comm.ReportProgress("Fetching the system's structures lost from zKillboard (live)")
	pull := func(groups *comm.StaticValues, minorGroups bool) {
		for _, groupID := range groups.IDNumbers() {
			name := groups.NameOf(groupID)
			paging := api.ZkillPaging{MaxPages: 1, Filter: fmt.Sprintf("losses/groupID/%d", groupID), For: name + " losses"}
			list, err := listEntity(target, target.TypeOfEntity, target.IDNumber, paging, api.ZKILL_PAGE_SIZE)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s losses: %w", name, err))
			}
			lists = append(lists, list.killmails[:list.within])
			for _, km := range list.killmails[:list.within] {
				minor[km.KillmailID] = minorGroups
			}
		}
	}
	pull(comm.UpwellStructureGroups, false)
	pull(comm.ControlTowerGroups, false)
	pull(comm.MinorStructureGroups, true)
	listed := combine(lists...) // each list is newest first, but not with each other

	var last *RawKillmail
	previous := []RawKillmail{}
	for _, km := range listed {
		if last != nil && len(previous) == PreviousStructureLosses {
			break
		}
		if last == nil && minor[km.KillmailID] {
			if len(previous) < PreviousStructureLosses {
				previous = append(previous, RawKillmail{Zkillmail: &km})
			}
			continue
		}
		if last == nil {
			last = &RawKillmail{Zkillmail: &km}
		} else {
			previous = append(previous, RawKillmail{Zkillmail: &km})
		}
	}

	// only the killmails shown need their bodies
	withBody := func(raw *RawKillmail) bool {
		body, err := killmailBody(target, *raw.Zkillmail)
		if err != nil {
			errs = append(errs, fmt.Errorf("killmail %d: %w", raw.Zkillmail.KillmailID, err))
			return false
		}
		raw.ESIKillmail = body
		return true
	}
	if last != nil && !withBody(last) {
		last = nil
	}
	bodied := []RawKillmail{}
	for index := range previous {
		if withBody(&previous[index]) {
			bodied = append(bodied, previous[index])
		}
	}
	return last, bodied, errors.Join(errs...)
}

// killmailBody is a listed killmail's body: the cache's copy, else zKillboard's, else fetched from the ESI
func killmailBody(target IntelTarget, km api.ZkbResponse) (*api.ESIKillmail, error) {
	if cache, ok := target.Clients.ESIClient.(killmailCache); ok {
		if cached := cache.CachedKillmail(km.KillmailID); cached != nil {
			return cached, nil
		}
	}
	if body := km.Killmail(); body != nil {
		return body, nil
	}
	return target.Clients.ESIClient.GetKillmail(km)
}
