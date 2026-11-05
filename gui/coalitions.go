package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// maxCoalitionName is the longest name a coalition has
const maxCoalitionName = 100

// coalitionType is the type name of a coalition target, in searches, ignored allies, and My Intel keys
var coalitionType = types.COALITION.String()

// errNoCoalition is shown when a coalition was removed since the frontend last listed it
var errNoCoalition = errors.New("That coalition no longer exists")

// CoalitionCard is a coalition as My Intel's Coalitions tab and the target panel show it
type CoalitionCard struct {
	ID                int         `json:"id"` // negative, local to this cache, never an eve id
	Name              string      `json:"name"`
	Note              string      `json:"note"`
	IncludeStructures bool        `json:"includeStructures"`
	Members           []EntityRef `json:"members"`    // by name
	Affiliates        []EntityRef `json:"affiliates"` // the members' own affiliates, which its search also pulls, by name
}

/*
	coalitionsFollowAffiliate changes every coalition with a new affiliate as a direct member: its main takes its place as a member, and it becomes the main's affiliate there, which CoalitionGroups brings in.

Parameters:
  - link: the holding link just saved

Returns:
  - []string: the names of the coalitions it changed, never nil
*/
func (a *App) coalitionsFollowAffiliate(link retrieval.HoldingLink) []string {
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	moved := []string{}
	for _, coalition := range a.cache.Coalitions() {
		isHolding := func(member retrieval.CoalitionMember) bool {
			return member.Type == link.HoldingType && member.ID == link.HoldingID
		}
		if !slices.ContainsFunc(coalition.Members, isHolding) {
			continue
		}
		coalition.Members = slices.DeleteFunc(coalition.Members, isHolding)
		if !slices.ContainsFunc(coalition.Members, func(member retrieval.CoalitionMember) bool {
			return member.Type == link.MainType && member.ID == link.MainID
		}) {
			coalition.Members = append(coalition.Members, retrieval.CoalitionMember{Type: link.MainType, ID: link.MainID, Name: link.MainName})
		}
		a.cache.SaveCoalition(coalition)
		moved = append(moved, coalition.Name)
	}
	return moved
}

// coalitionCard builds a coalition's card, with tickers only from the cache so it opens at once
func (a *App) coalitionCard(coalition retrieval.Coalition) CoalitionCard {
	card := CoalitionCard{ID: coalition.ID, Name: coalition.Name, Note: coalition.Note, IncludeStructures: coalition.IncludeStructures,
		Members: []EntityRef{}, Affiliates: []EntityRef{}}
	withTicker := func(entityType types.EntityType, id int, name string) EntityRef {
		ref := *entityRef(entityType, id, name)
		ref.Ticker = a.cachedTicker(ref)
		return ref
	}
	for _, group := range a.cache.CoalitionGroups(coalition, true) {
		if group.Member {
			card.Members = append(card.Members, withTicker(group.Type, group.ID, group.Name))
		} else {
			card.Affiliates = append(card.Affiliates, withTicker(group.Type, group.ID, group.Name))
		}
	}
	return card
}

// validCoalitionName trims a coalition's name and checks it: not empty, at most maxCoalitionName characters, and not another coalition's, ignoring case
func (a *App) validCoalitionName(name string, ownID int) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", errors.New("Enter a name for the coalition")
	case len([]rune(name)) > maxCoalitionName:
		return "", fmt.Errorf("A coalition's name can be at most %d characters", maxCoalitionName)
	}
	if other, found := a.cache.CoalitionNamed(name); found && other.ID != ownID {
		return "", fmt.Errorf("There is already a coalition named %s", other.Name)
	}
	return name, nil
}

// changeCoalition applies a change to a saved coalition, one at a time as it reads then saves, and returns its card
func (a *App) changeCoalition(id int, change func(*retrieval.Coalition) error) (CoalitionCard, error) {
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	coalition, found := a.cache.CoalitionByID(id)
	if !found {
		return CoalitionCard{}, errNoCoalition
	}
	if err := change(&coalition); err != nil {
		return CoalitionCard{}, err
	}
	saved := a.cache.SaveCoalition(coalition)
	a.saveCacheQuietly()
	return a.coalitionCard(saved), nil
}

/*
	GetCoalitions returns every coalition, by name, for My Intel's Coalitions tab. Like GetMyIntel it does not wait for a running search.

Returns:
  - []CoalitionCard: the coalitions
*/
func (a *App) GetCoalitions() (cards []CoalitionCard, err error) {
	defer recoverAsError(&err)
	cards = []CoalitionCard{}
	for _, coalition := range a.cache.Coalitions() {
		cards = append(cards, a.coalitionCard(coalition))
	}
	return cards, nil
}

/*
	CreateCoalition makes an empty coalition. It does not wait for a running search.

Parameters:
  - name: at most 100 characters, not another coalition's, ignoring case

Returns:
  - CoalitionCard: the new coalition
  - error: a user readable message if the name is not allowed
*/
func (a *App) CreateCoalition(name string) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	name, err = a.validCoalitionName(name, 0)
	if err != nil {
		return card, err
	}
	saved := a.cache.SaveCoalition(retrieval.Coalition{Name: name})
	a.saveCacheQuietly()
	return a.coalitionCard(saved), nil
}

/*
	RenameCoalition changes a coalition's name. It does not wait for a running search.

Parameters:
  - id: the coalition's id
  - name: the new name, under the rules of CreateCoalition

Returns:
  - CoalitionCard: the coalition after the change
  - error: a user readable message if the name is not allowed or the coalition is gone
*/
func (a *App) RenameCoalition(id int, name string) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	oldName := ""
	card, err = a.changeCoalition(id, func(coalition *retrieval.Coalition) error {
		oldName = coalition.Name
		valid, err := a.validCoalitionName(name, id)
		coalition.Name = valid
		return err
	})
	if err != nil {
		return card, err
	}
	a.renameRecentCoalition(id, oldName, card.Name)
	return card, nil
}

// renameRecentCoalition makes a renamed coalition's recent search use its new name, as recent searches are kept by name. One it never had stays absent.
func (a *App) renameRecentCoalition(id int, oldName string, newName string) {
	for _, recent := range a.cache.RecentSearches(retrieval.MaxRecentSearches) {
		if recent.Type == coalitionType && recent.ID == id {
			a.cache.RemoveRecentSearch(retrieval.RecentSearch{Name: oldName, Type: coalitionType})
			a.cache.AddRecentSearch(retrieval.RecentSearch{Name: newName, Type: coalitionType, ID: id}, a.currentSettings().MaxRecentSearches)
			a.saveCacheQuietly()
			return
		}
	}
}

/*
	SetCoalitionNote changes a coalition's note, trimmed to the same length as an affiliate's. It does not wait for a running search.

Returns:
  - CoalitionCard: the coalition after the change
  - error: a user readable message if the coalition is gone
*/
func (a *App) SetCoalitionNote(id int, note string) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	return a.changeCoalition(id, func(coalition *retrieval.Coalition) error {
		note = strings.TrimSpace(note)
		if len([]rune(note)) > maxHoldingNote {
			note = string([]rune(note)[:maxHoldingNote])
		}
		coalition.Note = note
		return nil
	})
}

/*
	SetCoalitionStructures sets whether a coalition's search collects its structures: off by default, as the structure lists cost many calls.
	It does not wait for a running search.

Returns:
  - CoalitionCard: the coalition after the change
  - error: a user readable message if the coalition is gone
*/
func (a *App) SetCoalitionStructures(id int, include bool) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	return a.changeCoalition(id, func(coalition *retrieval.Coalition) error {
		coalition.IncludeStructures = include
		return nil
	})
}

/*
	AddCoalitionMember adds a corporation or alliance to a coalition, looked up through the ESI so it is spelled as EVE spells it. It does not wait for a running search.

Parameters:
  - id: the coalition's id
  - name: the member's exact name, ignoring case, or a zKillboard or EveWho link to it
  - memberType: CORPORATION or ALLIANCE, for a name

Returns:
  - CoalitionCard: the coalition after the change
  - error: a user readable message if it is not found, not a corporation or alliance, or already a member
*/
func (a *App) AddCoalitionMember(id int, name string, memberType string) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	if _, found := a.cache.CoalitionByID(id); !found {
		return card, errNoCoalition
	}
	member, err := a.lookupEntity(name, memberType, types.CORPORATION, types.ALLIANCE)
	if err != nil {
		return card, err
	}
	return a.changeCoalition(id, func(coalition *retrieval.Coalition) error {
		if slices.ContainsFunc(coalition.Members, func(other retrieval.CoalitionMember) bool { return other.Type == member.Type && other.ID == member.ID }) {
			return fmt.Errorf("%s is already in %s", member.Name, coalition.Name)
		}
		coalition.Members = append(coalition.Members, retrieval.CoalitionMember{Type: member.Type, ID: member.ID, Name: member.Name})
		return nil
	})
}

/*
	RemoveCoalitionMember takes a member out of a coalition. It does not wait for a running search.

Parameters:
  - id: the coalition's id
  - memberType, memberID: the member's CORPORATION or ALLIANCE and eve id

Returns:
  - CoalitionCard: the coalition after the change
  - error: a user readable message if the coalition is gone
*/
func (a *App) RemoveCoalitionMember(id int, memberType string, memberID int) (card CoalitionCard, err error) {
	defer recoverAsError(&err)
	return a.changeCoalition(id, func(coalition *retrieval.Coalition) error {
		coalition.Members = slices.DeleteFunc(coalition.Members, func(member retrieval.CoalitionMember) bool {
			return member.Type == memberType && member.ID == memberID
		})
		return nil
	})
}

/*
	RemoveCoalitions removes coalitions, with their recent searches. The frontend offers RestoreCoalitions to undo it. It does not wait for a running search.

Parameters:
  - ids: the coalitions' ids. Ones that are not saved are skipped.
*/
func (a *App) RemoveCoalitions(ids []int) (err error) {
	defer recoverAsError(&err)
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	for _, id := range ids {
		coalition, found := a.cache.CoalitionByID(id)
		if !found {
			continue
		}
		a.cache.RemoveCoalition(id)
		a.cache.RemoveRecentSearch(retrieval.RecentSearch{Name: coalition.Name, Type: coalitionType})
	}
	a.saveCacheQuietly()
	return nil
}

/*
	RestoreCoalitions puts back coalitions RemoveCoalitions removed, with their ids and members. It does not wait for a running search.

Parameters:
  - cards: the coalitions as GetCoalitions gave them. One whose id is saved again, or whose name another coalition has taken since, is not restored.

Returns:
  - error: a user readable message naming the coalitions that could not be restored, the rest are
*/
func (a *App) RestoreCoalitions(cards []CoalitionCard) (err error) {
	defer recoverAsError(&err)
	a.coalitionMu.Lock()
	defer a.coalitionMu.Unlock()

	failed := []string{}
	for _, card := range cards {
		if _, saved := a.cache.CoalitionByID(card.ID); saved || card.ID >= 0 {
			failed = append(failed, card.Name)
			continue
		}
		name, nameErr := a.validCoalitionName(card.Name, 0)
		if nameErr != nil {
			failed = append(failed, card.Name)
			continue
		}
		coalition := retrieval.Coalition{ID: card.ID, Name: name, Note: card.Note, IncludeStructures: card.IncludeStructures, Members: []retrieval.CoalitionMember{}}
		for _, member := range card.Members {
			if _, ok := searchTypes[member.Type]; ok && canHold(member.Type) {
				coalition.Members = append(coalition.Members, retrieval.CoalitionMember{Type: member.Type, ID: member.ID, Name: member.Name})
			}
		}
		a.cache.SaveCoalition(coalition)
	}
	a.saveCacheQuietly()
	if len(failed) > 0 {
		return fmt.Errorf("Could not restore %s", strings.Join(failed, ", "))
	}
	return nil
}

/*
	coalitionResult is a coalition search's result, made from the cache with no ESI call: a coalition is not an EVE entity. The caller must hold a.mu.

Parameters:
  - coalition: the coalition searched
*/
func (a *App) coalitionResult(coalition retrieval.Coalition) TargetResult {
	card := a.coalitionCard(coalition)
	comm.ReportProgress("Found coalition %s with %d members", coalition.Name, len(coalition.Members))
	a.cache.AddRecentSearch(retrieval.RecentSearch{Name: coalition.Name, Type: coalitionType, ID: coalition.ID}, a.currentSettings().MaxRecentSearches)

	result := TargetResult{ID: coalition.ID, Type: coalitionType, Display: api.TargetDisplay{Name: coalition.Name}, Coalition: &card}
	a.noteDebugSearch(func(search *debugSearch) {
		search.TargetID, search.TargetType, search.TargetName = result.ID, result.Type, coalition.Name
	})
	kept := result
	a.lastTarget = &kept
	return result
}

// searchCoalition finds a coalition by its id, else its name, for SearchTarget
func (a *App) searchCoalition(name string, id int) (TargetResult, error) {
	var coalition retrieval.Coalition
	var found bool
	if id != 0 {
		coalition, found = a.cache.CoalitionByID(id)
	} else {
		coalition, found = a.cache.CoalitionNamed(name)
	}
	if !found {
		return TargetResult{}, errNoCoalition
	}
	return a.coalitionResult(coalition), nil
}
