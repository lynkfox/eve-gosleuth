package retrieval

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

// ErrWarSideGone is wrapped when a war's side refers to a coalition that no longer exists, naming it
var ErrWarSideGone = errors.New("a side of this war no longer exists")

// WarSide is one side of a War, saved as a reference and resolved at each run (WarSideGroups), so a change to a coalition or an affiliate carries into its wars.
// Later versions will give a side's members from and until dates: add them beside these fields.
type WarSide struct {
	Type           string `json:"type"` // COALITION, CORPORATION, or ALLIANCE
	ID             int    `json:"id"`
	Name           string `json:"name"`
	WithAffiliates bool   `json:"withAffiliates,omitempty"` // a corp or alliance with its affiliates, rather than alone. Not used for a coalition, which always brings its members' affiliates.
}

// WarAddedGroup is a group under Others that the user put on a side from the report
type WarAddedGroup struct {
	Type     string `json:"type"` // CORPORATION or ALLIANCE
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FromDate string `json:"fromDate,omitempty"` // YYYY-MM-DD, empty for from the beginning of the war. Before it, its killmails stay under Others.
}

// WarReach is how far a list of a war's groups reached at its last run, for the report's reach warning
type WarReach struct {
	Group    string    `json:"group"`
	Oldest   time.Time `json:"oldest"`
	Complete bool      `json:"complete"` // it reached the war's start or zKillboard's end
}

// War is two sides tracked against each other, with the killmails it counted held apart from the cache's (WarKillmails)
type War struct {
	ID        int     `json:"id"` // negative, local to this cache, as a coalition's
	Name      string  `json:"name"`
	SideA     WarSide `json:"sideA"`
	SideB     WarSide `json:"sideB"`
	FromDate  string  `json:"fromDate,omitempty"`  // YYYY-MM-DD, its start and always a fixed date, empty for no start
	UntilDate string  `json:"untilDate,omitempty"` // YYYY-MM-DD, its last day, empty while it goes on

	AddedA []WarAddedGroup `json:"addedA,omitempty"` // Others put on side A by hand
	AddedB []WarAddedGroup `json:"addedB,omitempty"`
	// Ignored are groups the user set aside from the report: on no side, and their losses counted nowhere
	Ignored []WarAddedGroup `json:"ignored,omitempty"`

	LastRun        time.Time  `json:"lastRun,omitempty"`        // when it last collected, the zero time before its first run
	NewestKillmail time.Time  `json:"newestKillmail,omitempty"` // the newest killmail it has listed, which an update lists from, less 5 days
	RunKey         string     `json:"runKey,omitempty"`         // RunKey at its last full run, which an update needs to match
	Reach          []WarReach `json:"reach,omitempty"`
	SetAt          time.Time  `json:"setAt"`
}

const warDateLayout = "2006-01-02"

// warRules is in the run key, and changes when a war comes to hold killmails it used to drop, so a saved war reads its lists once more. rules3 added station trash and any structure a side lost.
const warRules = "rules3"

/*
	Start is the war's first moment

Returns:
  - time.Time: midnight UTC at the start of FromDate, the zero time for no start or an unreadable date
*/
func (w War) Start() time.Time {
	if w.FromDate == "" {
		return time.Time{}
	}
	start, err := time.Parse(warDateLayout, w.FromDate)
	if err != nil {
		return time.Time{}
	}
	return start.UTC()
}

/*
	Within is whether a killmail's time is in the war's dates

Parameters:
  - at: the killmail's time

Returns:
  - bool: after Start (a killmail at its very moment counts), and on or before UntilDate (its whole day, UTC), where they are set
*/
func (w War) Within(at time.Time) bool {
	if start := w.Start(); !start.IsZero() && at.Before(start) {
		return false
	}
	if w.UntilDate != "" {
		if until, err := time.Parse(warDateLayout, w.UntilDate); err == nil && !at.Before(until.UTC().AddDate(0, 0, 1)) {
			return false
		}
	}
	return true
}

/*
	CurrentRunKey sums up what a full run depends on: both sides, each added group by type and id, and FromDate. Not UntilDate, or the added groups' dates,
	since those are applied to what the war holds.

Returns:
  - string: differs from RunKey when the next run must read every list from the war's start
*/
func (w War) CurrentRunKey() string {
	side := func(s WarSide) string {
		return fmt.Sprintf("%s:%d:%t", s.Type, s.ID, s.WithAffiliates && s.Type != types.COALITION.String())
	}
	added := func(groups []WarAddedGroup) string {
		keys := make([]string, 0, len(groups))
		for _, group := range groups {
			keys = append(keys, fmt.Sprintf("%s:%d", group.Type, group.ID))
		}
		slices.Sort(keys)
		return strings.Join(slices.Compact(keys), ",")
	}
	return fmt.Sprintf("%s|A=%s+[%s]|B=%s+[%s]|from=%s", warRules, side(w.SideA), added(w.AddedA), side(w.SideB), added(w.AddedB), w.FromDate)
}

/*
	RunKeyFor is CurrentRunKey with the groups its sides resolve to now, so a member or affiliate added to a coalition has its list read from the war's start at the next run.

Parameters:
  - sideA, sideB: the groups each side resolves to, see WarSideGroups

Returns:
  - string: differs from RunKey when the next run must read every list from the war's start
*/
func (w War) RunKeyFor(sideA []TargetGroup, sideB []TargetGroup) string {
	keys := []string{}
	for _, group := range slices.Concat(sideA, sideB) {
		keys = append(keys, fmt.Sprintf("%s:%d", group.Type.String(), group.ID))
	}
	slices.Sort(keys)
	return w.CurrentRunKey() + "|groups=" + strings.Join(slices.Compact(keys), ",")
}

// warKey is a war's key in SavedWars: its id
func warKey(id int) string {
	return fmt.Sprint(id)
}

// copyOf is a war with its own added groups and reach, so a caller changing them leaves the cache's copy alone
func (w War) copyOf() War {
	w.AddedA = slices.Clone(w.AddedA)
	w.AddedB = slices.Clone(w.AddedB)
	w.Ignored = slices.Clone(w.Ignored)
	w.Reach = slices.Clone(w.Reach)
	return w
}

/*
loadWars reads every saved war into SavedWars the first time they are wanted. The caller must hold c.mu.
Like coalitions they are read all at once, as they are listed and looked up by name. Their killmails are not: see WarKillmails.
*/
func (c *CachedEveIDs) loadWars() {
	if c.warsLoaded || c.db == nil {
		return
	}
	c.warsLoaded = true

	rows, err := c.db.Query("SELECT key, cached_at, value FROM cache WHERE section = ?", string(sectionWars))
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to read the wars")
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var cachedAt int64
		var data []byte
		if err := rows.Scan(&key, &cachedAt, &data); err != nil {
			comm.Logger.WithError(err).Warn("Unable to read a war")
			continue
		}
		war, err := decodeValue[War](sectionWars, data)
		if err != nil || war.ID >= 0 || warKey(war.ID) != key {
			comm.Logger.WithError(err).Warnf("War %s can not be read, it is skipped", key)
			continue
		}
		if _, found := c.SavedWars[key]; !found {
			c.SavedWars[key] = &CacheEntry{Value: war, CachedAt: timeOf(cachedAt), Permanent: true}
		}
		c.warFloor = min(c.warFloor, war.ID)
	}
}

// Wars returns every saved war, by name ignoring case
func (c *CachedEveIDs) Wars() []War {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadWars()

	all := []War{}
	for _, entry := range c.SavedWars {
		all = append(all, entry.Value.(*War).copyOf())
	}
	slices.SortFunc(all, func(a, b War) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return all
}

// WarByID returns a saved war by its id, false when there is none
func (c *CachedEveIDs) WarByID(id int) (War, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadWars()

	if entry, found := c.SavedWars[warKey(id)]; found {
		return entry.Value.(*War).copyOf(), true
	}
	return War{}, false
}

// WarNamed returns a saved war by its name, ignoring case, false when there is none
func (c *CachedEveIDs) WarNamed(name string) (War, bool) {
	for _, war := range c.Wars() {
		if strings.EqualFold(war.Name, name) {
			return war, true
		}
	}
	return War{}, false
}

/*
	SaveWar adds a war, or replaces the one with its id. Wars are the user's own, so cache clears leave them unless asked to clear them too.

Parameters:
  - war: an ID of 0 (or any that is not negative) gets a new one, as SaveCoalition gives. Its SetAt is set here.

Returns:
  - War: what was saved, with its ID
*/
func (c *CachedEveIDs) SaveWar(war War) War {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadWars()

	if war.ID >= 0 {
		war.ID = min(-int(time.Now().UnixMilli()), c.warFloor-1)
		for c.SavedWars[warKey(war.ID)] != nil {
			war.ID--
		}
	}
	c.warFloor = min(c.warFloor, war.ID)
	war = war.copyOf()
	war.SetAt = time.Now().UTC()
	put(c, sectionWars, c.SavedWars, warKey(war.ID), &CacheEntry{Value: &war, CachedAt: war.SetAt, Permanent: true})
	return war.copyOf()
}

// RemoveWar removes a war, its edits, and every killmail it holds, from this session and the database. One that is not saved changes nothing.
func (c *CachedEveIDs) RemoveWar(id int) {
	key := warKey(id)
	prefix := warKillmailPrefix(id)
	c.mu.Lock()
	c.loadWars()
	entry := c.SavedWars[key]
	cachedAs[WarEdits](c, sectionWarEdits, c.WarEdits, key) // so a saved row is in the map for removeWhere
	edits := c.WarEdits[key]
	c.mu.Unlock()

	c.removeWhere(
		func(s section, candidate *CacheEntry) bool {
			return s == sectionWars && candidate == entry || s == sectionWarEdits && candidate == edits
		},
		"(section = ? AND key = ?) OR (section = ? AND key = ?) OR (section = ? AND substr(key, 1, ?) = ?)",
		string(sectionWars), key, string(sectionWarEdits), key, string(sectionWarKillmails), len(prefix), prefix)

	// rows added since removeWhere flushed, or never flushed for want of a database
	c.mu.Lock()
	defer c.mu.Unlock()
	for rowKey := range c.dirty {
		if rowKey.section == sectionWarKillmails && strings.HasPrefix(rowKey.key, prefix) {
			delete(c.dirty, rowKey)
		}
	}
}

// heldKillmail is a killmail a war holds as it is saved: the ESI killmail and zKillboard's figures, which is all RawKillmail.Expand reads
type heldKillmail struct {
	ESI *api.ESIKillmail `json:"esi"`
	Zkb api.Zkb          `json:"zkb"`
}

// warKillmailPrefix starts the key of every killmail a war holds. Its slash keeps war -5 from matching war -55.
func warKillmailPrefix(id int) string {
	return fmt.Sprintf("%d/", id)
}

/*
heldKillmails reads the rows a war holds: the database's, with the unsaved ones over them. The caller must hold c.mu.
Returns the killmails by key, and false if the database could not be read.
*/
func (c *CachedEveIDs) heldKillmails(id int, withValues bool) (map[string]*heldKillmail, bool) {
	prefix := warKillmailPrefix(id)
	held := map[string]*heldKillmail{}
	ok := true

	if c.db != nil {
		rows, err := c.db.Query("SELECT key, value FROM cache WHERE section = ? AND substr(key, 1, ?) = ?", string(sectionWarKillmails), len(prefix), prefix)
		if err != nil {
			comm.Logger.WithError(err).Warnf("Unable to read the killmails of war %d", id)
			ok = false
		} else {
			defer rows.Close()
			for rows.Next() {
				var key string
				var data []byte
				if err := rows.Scan(&key, &data); err != nil {
					comm.Logger.WithError(err).Warn("Unable to read a war's killmail")
					continue
				}
				if !withValues {
					held[key] = nil
					continue
				}
				value, err := decodeValue[heldKillmail](sectionWarKillmails, data)
				if err != nil || value.ESI == nil {
					comm.Logger.WithError(err).Warnf("War killmail %s can not be read, it is skipped", key)
					continue
				}
				held[key] = value
			}
		}
	}
	for key, entry := range c.dirty {
		if key.section == sectionWarKillmails && strings.HasPrefix(key.key, prefix) {
			held[key.key], _ = entry.Value.(*heldKillmail)
		}
	}
	return held, ok
}

/*
	WarKillmails returns the killmails a war holds

Parameters:
  - id: the war

Returns:
  - []RawKillmail: oldest first, with ESIKillmail and Zkillmail (its id, time, and Zkb) set, and no Via. Empty, not nil, for none.
*/
func (c *CachedEveIDs) WarKillmails(id int) []RawKillmail {
	c.mu.Lock()
	held, _ := c.heldKillmails(id, true)
	c.mu.Unlock()

	all := make([]RawKillmail, 0, len(held))
	for _, value := range held {
		if value == nil || value.ESI == nil {
			continue
		}
		esi := *value.ESI
		all = append(all, RawKillmail{
			ESIKillmail: &esi,
			Zkillmail:   &api.ZkbResponse{KillmailID: esi.KillmailID, KillmailTime: esi.KillmailTime, Zkb: value.Zkb},
		})
	}
	slices.SortFunc(all, func(a, b RawKillmail) int {
		return cmp.Or(a.ESIKillmail.KillmailTime.Compare(b.ESIKillmail.KillmailTime), cmp.Compare(a.ESIKillmail.KillmailID, b.ESIKillmail.KillmailID))
	})
	return all
}

/*
	AddWarKillmails adds killmails to those a war holds, one row each, section war_killmail. One it already holds is replaced. Once held, a killmail stays held until the war is removed.

Parameters:
  - id: the war
  - killmails: with ESIKillmail and Zkillmail set, others are skipped
*/
func (c *CachedEveIDs) AddWarKillmails(id int, killmails []RawKillmail) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	for _, raw := range killmails {
		if raw.ESIKillmail == nil || raw.Zkillmail == nil {
			continue
		}
		esi := *raw.ESIKillmail
		key := rowKey{section: sectionWarKillmails, key: fmt.Sprintf("%s%d", warKillmailPrefix(id), esi.KillmailID)}
		c.dirty[key] = &CacheEntry{Value: &heldKillmail{ESI: &esi, Zkb: raw.Zkillmail.Zkb}, CachedAt: now, Permanent: true}
	}
}

// WarKillmailCount is how many killmails a war holds, without reading them
func (c *CachedEveIDs) WarKillmailCount(id int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	held, _ := c.heldKillmails(id, false)
	return len(held)
}

/*
	WarSideGroups resolves a side into the groups whose lists are read and whose killmails are its own, as they are now

Parameters:
  - side: the side

Returns:
  - []TargetGroup: Member true for the side's own corp or alliance or a coalition's member, false for an affiliate, each once. A coalition's are CoalitionGroups with affiliates.
    A corp or alliance only has its affiliates with WithAffiliates.
  - error: wraps ErrWarSideGone, naming it, for a coalition no longer saved
*/
func (c *CachedEveIDs) WarSideGroups(side WarSide) ([]TargetGroup, error) {
	if side.Type == types.COALITION.String() {
		coalition, found := c.CoalitionByID(side.ID)
		if !found {
			return nil, fmt.Errorf("%w: the coalition %s", ErrWarSideGone, side.Name)
		}
		return c.CoalitionGroups(coalition, true), nil
	}

	entityType, ok := memberEntityType(side.Type)
	if !ok {
		return nil, fmt.Errorf("a war's side can not be a %q", side.Type)
	}
	groups := []TargetGroup{{Type: entityType, ID: side.ID, Name: side.Name, Member: true}}
	if !side.WithAffiliates {
		return groups, nil
	}
	holdings, _ := c.HoldingLinks(side.Type, side.ID)
	for _, link := range holdings {
		linkType, ok := memberEntityType(link.HoldingType)
		if !ok || slices.ContainsFunc(groups, func(g TargetGroup) bool { return g.Type == linkType && g.ID == link.HoldingID }) {
			continue
		}
		groups = append(groups, TargetGroup{Type: linkType, ID: link.HoldingID, Name: link.HoldingName})
	}
	return groups, nil
}
