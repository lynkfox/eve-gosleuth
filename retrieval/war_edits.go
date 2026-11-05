package retrieval

import (
	"slices"
	"time"
)

// the kinds of edit, see WarBattleEdit
const (
	WarEditMerge = "merge" // Groups: one list of killmail ids per battle
	WarEditSplit = "split" // Groups: what stays, then what moved
)

// WarBattleEdit is one change the user made to a war's battles. It is saved as killmail ids, not a battle's id, as a later run can change those. See .claude/docs/war-plan.md's Stage 4.
type WarBattleEdit struct {
	ID     int       `json:"id"` // unique within its war, how it is undone
	Kind   string    `json:"kind"`
	Groups [][]int   `json:"groups"`
	At     time.Time `json:"at"`
}

// WarEdits is what the user changed by hand on a war's report, applied again at every build. It is apart from the War, which every save rewrites.
type WarEdits struct {
	Edits []WarBattleEdit `json:"edits,omitempty"` // in the order they are applied, the oldest first
}

// copyOf is a copy that shares nothing with the original
func (e WarEdits) copyOf() WarEdits {
	out := e
	out.Edits = slices.Clone(e.Edits)
	for index := range out.Edits {
		groups := make([][]int, len(out.Edits[index].Groups))
		for group, ids := range out.Edits[index].Groups {
			groups[group] = slices.Clone(ids)
		}
		out.Edits[index].Groups = groups
	}
	return out
}

/*
	AddEdit adds an edit to the end of the list, with an id none in the list has.

Parameters:
  - kind: WarEditMerge or the like
  - groups: its killmail ids, as its kind says
*/
func (e *WarEdits) AddEdit(kind string, groups [][]int) {
	id := 1
	for _, edit := range e.Edits {
		id = max(id, edit.ID+1)
	}
	e.Edits = append(e.Edits, WarBattleEdit{ID: id, Kind: kind, Groups: groups, At: time.Now().UTC()})
}

/*
	RemoveEdit takes an edit off the list, undoing it at the next build.

Parameters:
  - id: the edit

Returns:
  - bool: false if it had no such edit
*/
func (e *WarEdits) RemoveEdit(id int) bool {
	before := len(e.Edits)
	e.Edits = slices.DeleteFunc(e.Edits, func(edit WarBattleEdit) bool { return edit.ID == id })
	return len(e.Edits) != before
}

/*
	WarEditsOf returns what the user changed by hand on a war's report.

Parameters:
  - id: the war

Returns:
  - WarEdits: a copy of its edits, empty if it has none
*/
func (c *CachedEveIDs) WarEditsOf(id int) WarEdits {
	c.mu.Lock()
	defer c.mu.Unlock()

	if edits := cachedAs[WarEdits](c, sectionWarEdits, c.WarEdits, warKey(id)); edits != nil {
		return edits.copyOf()
	}
	return WarEdits{}
}

/*
	SaveWarEdits replaces what the user changed by hand on a war's report. It is Permanent and one of userSections, so a clear leaves it unless told not to.

Parameters:
  - id: the war
  - edits: all of them
*/
func (c *CachedEveIDs) SaveWarEdits(id int, edits WarEdits) {
	c.mu.Lock()
	defer c.mu.Unlock()

	saved := edits.copyOf()
	put(c, sectionWarEdits, c.WarEdits, warKey(id), &CacheEntry{Value: &saved, CachedAt: time.Now().UTC(), Permanent: true})
}
