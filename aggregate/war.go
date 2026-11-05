package aggregate

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// WarSideKey is which side of a war something counts for
type WarSideKey int

const (
	WarSideNone WarSideKey = iota // neither, shown under Others
	WarSideA
	WarSideB
)

// WarSides are a war's two sides as they are at this run, see retrieval.CachedEveIDs.WarSideGroups, with what the user added from the report
type WarSides struct {
	A, B           []retrieval.TargetGroup
	AddedA, AddedB []retrieval.WarAddedGroup
	Ignored        []retrieval.WarAddedGroup // on no side, and their losses counted nowhere
}

/*
	SideOf is the side a corp and alliance on a killmail belong to at the time of the killmail. An added group counts only from its FromDate.
	A group listing the corp wins over a group listing its alliance, on either side. Between two equally specific, a Member wins over an affiliate, an added group loses to both, and then side A wins.

Parameters:
  - corporationID, allianceID: as on the killmail, 0 for none
  - at: the killmail's time

Returns:
  - WarSideKey: WarSideNone when on neither
*/
func (s WarSides) SideOf(corporationID int, allianceID int, at time.Time) WarSideKey {
	return s.index().of(corporationID, allianceID, at)
}

/*
	ClassifyRawWar picks the killmails a war holds out of those listed and those it already holds, before any lookups, as two large sides list many that have nothing to do with the war.
	Battles are found with FindBattlesOf, leaving out same-side killmails (below). A battle counts when, on at least one of its killmails, one side killed the other, pilot or structure: two sides that only crossed paths in a larger fight have not fought. A lone killmail is a battle of one.

Parameters:
  - raw: the killmails, in any order, each once
  - sides: the war's sides

Returns:
  - []retrieval.RawKillmail: every killmail in a counted battle, every station trash killmail, every same-side killmail (victim and every attacker on one side, which asset denial looks back at) but a capsule only its own stations shot, and every structure a side or a named third party lost (which asset denial and Station ref look for, and which alone counts for nothing)
*/
func ClassifyRawWar(raw []retrieval.RawKillmail, sides WarSides) []retrieval.RawKillmail {
	return classifyRawWar(raw, sides)
}

// WarStructureCount is how many of one structure type a side lost
type WarStructureCount struct {
	Type  *retrieval.Item
	Count int
}

// WarMeasures are the sheet's categories for one side, or for Others. ISKLost is ISKFights + ISKTrash + ISKStructures, and ShipsLost is ShipsFights + ShipsTrash.
type WarMeasures struct {
	Killmails          int
	ISKLost            float64
	ISKFights          float64
	ISKTrash           float64 // station trash and asset denial
	ISKTrashDefinite   float64 // the part of ISKTrash graded definite, the rest being probable
	ISKStructures      float64 // Standup Fighters too
	ShipsLost          int     // category in comm.ShipCategories and not comm.CapsuleGroups. A type fit_types.json lacks counts if the victim has a pilot.
	ShipsFights        int
	ShipsTrash         int
	ShipsTrashDefinite int                 // the part of ShipsTrash graded definite
	Citadels           []WarStructureCount // upwell_structure_groups, most lost first then by name
	POS                int                 // control_tower_groups and poco_groups
	StationRefs        int                 // battles marked Citadel Ref'd for this side
	RefAttempts        int                 // battles marked Station Attacked for this side
	RefAttemptsUnsure  int                 // battles marked Station Attacked? for this side, not among RefAttempts
}

// WarDay is what each side lost on one day, EVE time
type WarDay struct {
	Date string // YYYY-MM-DD
	A, B WarMeasures
}

// WarStationRef marks a battle where a side lost or used Standup Fighters, or a station of its fired and survived, so its station was probably under attack
type WarStationRef struct {
	Side      WarSideKey
	Kind      string    // WarKindCitadel, WarKindPOS, or WarKindPOCO
	Attempt   bool      // no structure of that side died in the system within its timer days
	Destroyed bool      // its station died in this very fight, so it is no ref. The rest of the marks are for one that lived through the fight.
	Unsure    bool      // an attempt on thin evidence: no Standup Fighters, and its station only fired on killmails with few pilots, as one will at a lone ship
	Structure string    // the type of the structure that did, empty for an attempt
	DiedAt    time.Time // when it did, zero for an attempt
	Notes     []WarRefNote
}

// the kinds of structure a battle's marks are for
const (
	WarKindCitadel = "citadel" // any upwell structure
	WarKindPOS     = "pos"
	WarKindPOCO    = "poco"
)

// WarRefNote is one thing that marked a battle: a type of station, battery, or fighter, in one system
type WarRefNote struct {
	Type      string
	Owner     string // its group when that is a third party fighting beside the side, else empty
	System    string
	Lost      bool // lost, not firing
	Killmails int
}

// WarStructureKind is how many of one kind of structure a side lost in a battle
type WarStructureKind struct {
	Name  string // the type's name, or POS for any control tower
	Count int
}

// WarBattleSide is one side's part in a battle, third parties beside it included
type WarBattleSide struct {
	Group              *WarGroup // its largest group in the battle by pilots, nil if it has none there
	Pilots             int
	ISKLost            float64
	ShipsLost          int
	StructureLost      int                // upwell, control tower, and customs office killmails
	StructureKinds     []WarStructureKind // what those were, most lost first then by name
	TrashKillmails     int                // station trash and asset denial attached to this battle, in none of the figures above
	TrashISK           float64
	TrashShips         int
	TrashDefiniteISK   float64 // the part of TrashISK graded definite, the rest being probable
	TrashDefiniteShips int
}

// WarBattle is a counted battle, or a skirmish
type WarBattle struct {
	ID           string // from FindBattlesOf
	Start, End   time.Time
	System       *retrieval.System   // the system with the most of its killmails, the first of them on a tie
	OtherSystems []string            // by name, in the order it reached them
	OtherDetails []*retrieval.System // the same systems, for their menu
	Pilots       int
	Killmails    []*retrieval.Killmail // oldest first
	KillmailIDs  []int                 // every killmail it holds, oldest first, which those of an ignored group or with no expanded form are in but Killmails is not
	Edits        []WarEditMark         // what the user did to it
	Ignored      bool                  // set aside by the user: its figures are its row's alone
	Skirmish     bool                  // fewer than minPilots pilots
	A, B, Others WarBattleSide
	StationRefs  []WarStationRef // side A's first
}

// WarGroup is a corp or alliance on the killmails: the alliance, or the corp when it has none. An NPC corp is never one.
type WarGroup struct {
	Type string // CORPORATION or ALLIANCE
	ID   int
	Name string
}

// WarOther is a group under Others in the whole war, which the user can add to a side
type WarOther struct {
	Group    WarGroup
	Measures WarMeasures
}

// WarSystem is what happened in one system
type WarSystem struct {
	System       *retrieval.System
	Battles      int // the battles and skirmishes with a killmail it counts here. A battle that moved counts in each of its systems.
	A, B, Others WarSystemSide
}

// WarSystemSide is one side's losses in a system
type WarSystemSide struct {
	Killmails int
	ISKLost   float64
}

// WarKillmail is a counted killmail
type WarKillmail struct {
	Killmail   *retrieval.Killmail
	VictimSide WarSideKey // in its battle, a third party counting for the side it fought beside
	BattleID   string     // empty for asset denial with no battle
	Trash      bool       // station trash or asset denial
	Definite   bool       // of Trash, graded definite rather than probable
}

// WarData is what a war's report is built from
type WarData struct {
	A, B, Others WarMeasures
	Battles      []*WarBattle // newest first, skirmishes and ignored ones among them
	BattleCount  int          // those neither Skirmish nor Ignored
	Skirmishes   int
	// IgnoredBattles are those the user set aside, on the list but in no other figure
	IgnoredBattles int
	Systems        []WarSystem   // most killmails first
	Killmails      []WarKillmail // newest first
	OtherGroups    []WarOther    // most ISK lost first
	NPCPilots      WarMeasures   // the part of Others lost by pilots in NPC corps, who have no group and can not join a side
	Days           []WarDay      // oldest first, only days with a loss on either side
}

/*
	BuildWar works out the report's figures from what a war holds. See .claude/docs/war-plan.md's What counts and Measures.

Sides are read from the raw killmail's ids, which are as they were at the kill, and the rest from the expanded killmail.
Battles are found on the raw killmails, as ClassifyRawWar does, leaving out same-side killmails and station trash, and uncounted battles are forgotten.
Station trash and asset denial are their own bucket, attached to the battle their station died in, see .claude/docs/war-plan.md's Stage 3.

A third party group in a counted battle counts in that battle for the side it fought beside: the opposite of whichever side it shot at or was shot by on more killmails.
Even or none, it is under Others. So is any killmail of an added group before its FromDate, whatever it shot at. A killmail's ISK and ship are its victim's side's losses.

Parameters:
  - raw: what the war holds, within its dates, in any order
  - expanded: each killmail's expanded form by id. A killmail not in it is left out, and logged.
  - sides: the war's sides
  - minPilots: the fewest pilots a battle needs not to be a skirmish
  - edits: what the user changed by hand, see warEditedBattles

Returns:
  - *WarData: the figures, never nil
*/
func BuildWar(raw []retrieval.RawKillmail, expanded map[int]*retrieval.Killmail, sides WarSides, minPilots int, edits retrieval.WarEdits) *WarData {
	return buildWar(raw, expanded, sides, minPilots, edits)
}

// how a group came to list a corp, the lower the stronger
const (
	warKindMember = iota
	warKindAffiliate
	warKindAdded
)

// warCandidate is a group that lists a corp or alliance
type warCandidate struct {
	side   WarSideKey
	spec   int // 2 for a group listing the corp, 1 for one listing its alliance
	kind   int
	active time.Time // an added group counts from here, the zero time for always
}

// warSideIndex is WarSides by the ids it lists, so a killmail's side is found without walking every group
type warSideIndex struct {
	byCorp, byAlliance map[int][]warCandidate
	dated              map[int][]time.Time // alliance and corp ids of added groups with a date, to tell a group held under Others by its date
	datedCorp          map[int][]time.Time
	ignored            map[warGroupKey]bool
}

func (s WarSides) index() *warSideIndex {
	idx := &warSideIndex{byCorp: map[int][]warCandidate{}, byAlliance: map[int][]warCandidate{}, dated: map[int][]time.Time{}, datedCorp: map[int][]time.Time{}}
	add := func(side WarSideKey, group retrieval.TargetGroup, kind int, from time.Time) {
		switch group.Type {
		case types.CORPORATION:
			idx.byCorp[group.ID] = append(idx.byCorp[group.ID], warCandidate{side: side, spec: 2, kind: kind, active: from})
		case types.ALLIANCE:
			idx.byAlliance[group.ID] = append(idx.byAlliance[group.ID], warCandidate{side: side, spec: 1, kind: kind, active: from})
		}
	}
	for _, listed := range []struct {
		side   WarSideKey
		groups []retrieval.TargetGroup
		added  []retrieval.WarAddedGroup
	}{{WarSideA, s.A, s.AddedA}, {WarSideB, s.B, s.AddedB}} {
		for _, group := range listed.groups {
			kind := warKindAffiliate
			if group.Member {
				kind = warKindMember
			}
			add(listed.side, group, kind, time.Time{})
		}
		for _, group := range listed.added {
			entityType, ok := warEntityType(group.Type)
			if !ok {
				continue
			}
			var from time.Time
			if group.FromDate != "" {
				if parsed, err := time.Parse("2006-01-02", group.FromDate); err == nil {
					from = parsed.UTC()
				}
			}
			add(listed.side, retrieval.TargetGroup{Type: entityType, ID: group.ID}, warKindAdded, from)
			if !from.IsZero() {
				if entityType == types.CORPORATION {
					idx.datedCorp[group.ID] = append(idx.datedCorp[group.ID], from)
				} else {
					idx.dated[group.ID] = append(idx.dated[group.ID], from)
				}
			}
		}
	}
	idx.ignored = map[warGroupKey]bool{}
	for _, group := range s.Ignored {
		if entityType, ok := warEntityType(group.Type); ok {
			idx.ignored[warGroupKey{Type: entityType, ID: group.ID}] = true
		}
	}
	return idx
}

func warEntityType(name string) (types.EntityType, bool) {
	switch name {
	case types.CORPORATION.String():
		return types.CORPORATION, true
	case types.ALLIANCE.String():
		return types.ALLIANCE, true
	}
	return 0, false
}

// of is SideOf
func (idx *warSideIndex) of(corporationID int, allianceID int, at time.Time) WarSideKey {
	var best warCandidate
	found := false
	consider := func(candidates []warCandidate) {
		for _, candidate := range candidates {
			if !candidate.active.IsZero() && at.Before(candidate.active) {
				continue
			}
			better := !found || candidate.spec > best.spec ||
				(candidate.spec == best.spec && (candidate.kind < best.kind || (candidate.kind == best.kind && candidate.side == WarSideA && best.side == WarSideB)))
			if better {
				best, found = candidate, true
			}
		}
	}
	if corporationID != 0 {
		consider(idx.byCorp[corporationID])
	}
	if allianceID != 0 {
		consider(idx.byAlliance[allianceID])
	}
	if !found {
		return WarSideNone
	}
	return best.side
}

// forcedOthers is whether a corp and alliance sit on no side only because an added group listing them counts from a later date
func (idx *warSideIndex) forcedOthers(corporationID int, allianceID int, at time.Time) bool {
	later := func(dates []time.Time) bool {
		return slices.ContainsFunc(dates, func(from time.Time) bool { return at.Before(from) })
	}
	return (corporationID != 0 && later(idx.datedCorp[corporationID])) || (allianceID != 0 && later(idx.dated[allianceID]))
}

// warGroupKey is a group on a killmail: the alliance, or the corp when it has none
type warGroupKey struct {
	Type types.EntityType
	ID   int
}

// warGroupOf is the group of a corp and alliance, false for none: no corp, or an NPC corp
func warGroupOf(corporationID int, allianceID int) (warGroupKey, bool) {
	if allianceID != 0 {
		return warGroupKey{types.ALLIANCE, allianceID}, true
	}
	if corporationID != 0 && !api.IsNPCCorporation(corporationID) {
		return warGroupKey{types.CORPORATION, corporationID}, true
	}
	return warGroupKey{}, false
}

// warParty is the victim or an attacker of a killmail, as the war reads it
type warParty struct {
	char     int
	side     WarSideKey // by SideOf alone
	forced   bool       // held under Others by an added group's date
	ignored  bool       // a group the user ignored: its losses count nowhere
	group    warGroupKey
	hasGroup bool
	victim   bool
}

// warKill is a raw killmail with its parties resolved
type warKill struct {
	raw      *retrieval.RawKillmail
	id       int
	at       time.Time
	parties  []warParty // the victim first
	sameSide bool
	highsec  bool // only known once the killmail is expanded, for a station's timers
	// beside is the side each third party group fought beside, as the automatic battle it was found in decided, so an edit moves no group's ISK. See warFoughtBeside.
	beside map[warGroupKey]WarSideKey
}

// effective is the side a party on this killmail counts for: its own, or for a third party the side it fought beside
func (kill *warKill) effective(p warParty) WarSideKey {
	if p.side != WarSideNone {
		return p.side
	}
	if p.hasGroup && !p.forced && !p.ignored {
		return kill.beside[p.group]
	}
	return WarSideNone
}

func intOf(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func analyseWarKill(raw *retrieval.RawKillmail, idx *warSideIndex) *warKill {
	esi := raw.ESIKillmail
	party := func(corporationID, allianceID, char int, victim bool) warParty {
		p := warParty{char: char, victim: victim}
		p.side = idx.of(corporationID, allianceID, esi.KillmailTime)
		p.forced = p.side == WarSideNone && idx.forcedOthers(corporationID, allianceID, esi.KillmailTime)
		p.group, p.hasGroup = warGroupOf(corporationID, allianceID)
		p.ignored = p.side == WarSideNone && p.hasGroup && idx.ignored[p.group]
		return p
	}
	kill := &warKill{raw: raw, id: esi.KillmailID, at: esi.KillmailTime}
	kill.parties = append(kill.parties, party(intOf(esi.Victim.CorporationID), intOf(esi.Victim.AllianceID), intOf(esi.Victim.CharacterID), true))
	for _, attacker := range esi.Attackers {
		kill.parties = append(kill.parties, party(intOf(attacker.CorporationID), intOf(attacker.AllianceID), intOf(attacker.CharacterID), false))
	}

	victim := kill.parties[0].side
	kill.sameSide = victim != WarSideNone && len(esi.Attackers) > 0
	for _, attacker := range kill.parties[1:] {
		if attacker.side != victim {
			kill.sameSide = false
		}
	}
	return kill
}

// analyseWarKills reads each raw killmail once, those without an ESI killmail left out
func analyseWarKills(raw []retrieval.RawKillmail, idx *warSideIndex) []*warKill {
	kills := make([]*warKill, 0, len(raw))
	seen := map[int]bool{}
	for index := range raw {
		if raw[index].ESIKillmail == nil || seen[raw[index].ESIKillmail.KillmailID] {
			continue
		}
		seen[raw[index].ESIKillmail.KillmailID] = true
		kills = append(kills, analyseWarKill(&raw[index], idx))
	}
	return kills
}

// warBattleOf is a battle found among a war's killmails that are not same-side
type warBattleOf struct {
	fought  *BattleFought // nil for a piece of a split
	kills   []*warKill    // oldest first
	id      string        // set by warEditedBattles
	edited  bool          // merged or split by the user
	marks   []WarEditMark // what the user did to it
	ignored bool          // set aside by the user
}

// counted is whether one side killed the other on any of the battle's killmails
func (b *warBattleOf) counted() bool {
	for _, kill := range b.kills {
		victim := kill.parties[0].side
		if victim == WarSideNone {
			continue
		}
		for _, attacker := range kill.parties[1:] {
			if attacker.side != WarSideNone && attacker.side != victim {
				return true
			}
		}
	}
	return false
}

func warBattlesOf(kills []*warKill) []*warBattleOf {
	byID := map[int]*warKill{}
	inputs := []BattleKillmail{}
	for _, kill := range kills {
		if kill.sameSide {
			continue
		}
		byID[kill.id] = kill
		inputs = append(inputs, RawBattleKillmail(kill.raw))
	}
	battles := []*warBattleOf{}
	for _, fought := range FindBattlesOf(inputs) {
		battle := &warBattleOf{fought: fought}
		for _, id := range fought.KillmailIDs {
			battle.kills = append(battle.kills, byID[id])
		}
		slices.SortStableFunc(battle.kills, func(a, b *warKill) int {
			return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id))
		})
		battles = append(battles, battle)
	}
	return battles
}

func classifyRawWar(raw []retrieval.RawKillmail, sides WarSides) []retrieval.RawKillmail {
	kills := analyseWarKills(raw, sides.index())
	keep := []*warKill{}
	for _, kill := range kills {
		if kill.sameSide && !warOwnPodOnly(kill) {
			keep = append(keep, kill)
		}
	}
	kept := map[int]bool{}
	for _, kill := range keep {
		kept[kill.id] = true
	}
	hold := func(kill *warKill) {
		if !kept[kill.id] {
			kept[kill.id] = true
			keep = append(keep, kill)
		}
	}
	for _, battle := range warBattlesOf(kills) {
		if battle.counted() {
			for _, kill := range battle.kills {
				hold(kill)
			}
		}
	}
	trash := findWarTrash(kills)
	for _, kill := range kills {
		if _, marked := trash.marks[kill.id]; marked {
			hold(kill)
		}
	}
	for _, kill := range kills {
		// a named third party's too, as a Station ref can name it
		if victim := kill.parties[0]; warIsStation(warVictimType(kill)) && (victim.side != WarSideNone || (victim.hasGroup && !victim.ignored)) {
			hold(kill)
		}
	}
	slices.SortFunc(keep, func(a, b *warKill) int { return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id)) })

	held := make([]retrieval.RawKillmail, 0, len(keep))
	for _, kill := range keep {
		held = append(held, *kill.raw)
	}
	return held
}

// warMeasureBuilder adds up a WarMeasures
type warMeasureBuilder struct {
	WarMeasures
	citadels map[int]*WarStructureCount
}

func (m *warMeasureBuilder) result() WarMeasures {
	out := m.WarMeasures
	out.Citadels = []WarStructureCount{}
	for _, count := range m.citadels {
		out.Citadels = append(out.Citadels, *count)
	}
	name := func(c WarStructureCount) string { return strings.ToLower(c.Type.Name) }
	slices.SortFunc(out.Citadels, func(a, b WarStructureCount) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(name(a), name(b)), cmp.Compare(a.Type.ItemID, b.Type.ItemID))
	})
	return out
}

// warLoss is what one killmail costs its victim's side
type warLoss struct {
	isk       float64
	ship      bool
	citadel   bool
	structure bool // upwell, control tower, or customs office
	pos       bool
	fighter   bool // a Standup Fighter: ISK under structures, not a structure lost
	typeID    int
	item      *retrieval.Item
}

func warLossOf(kill *warKill, km *retrieval.Killmail) warLoss {
	loss := warLoss{typeID: kill.raw.ESIKillmail.Victim.ShipTypeID, isk: km.TotalValue}
	if kill.raw.Zkillmail != nil && loss.isk == 0 {
		loss.isk = kill.raw.Zkillmail.Zkb.TotalValue
	}
	loss.item = &retrieval.Item{ItemID: loss.typeID}
	if km.Victim != nil && km.Victim.Ship != nil {
		loss.item = km.Victim.Ship
	}

	loss.citadel = comm.UpwellStructureGroups.ContainsGroupOf(loss.typeID)
	loss.pos = comm.ControlTowerGroups.ContainsGroupOf(loss.typeID) || comm.PocoGroups.ContainsGroupOf(loss.typeID)
	loss.structure = loss.citadel || loss.pos
	loss.fighter = comm.StandupFighterGroups.ContainsGroupOf(loss.typeID)

	if fit := comm.FitTypeOf(loss.typeID); fit != nil {
		loss.ship = comm.ShipCategories.ContainsIDNumber(fit.CategoryID) && !comm.CapsuleGroups.ContainsIDNumber(fit.GroupID)
	} else {
		loss.ship = intOf(kill.raw.ESIKillmail.Victim.CharacterID) != 0
	}
	return loss
}

// warExpandedOrEmpty is a killmail's expanded form, or an empty one when it has none, as a structure that only names a ref may have no lookups
func warExpandedOrEmpty(expanded map[int]*retrieval.Killmail, kill *warKill) *retrieval.Killmail {
	if km := expanded[kill.id]; km != nil {
		return km
	}
	return &retrieval.Killmail{}
}

// structureKind is the name a structure loss goes under in a battle, control towers being one kind
func (loss warLoss) structureKind() string {
	if comm.ControlTowerGroups.ContainsGroupOf(loss.typeID) {
		return "POS"
	}
	if loss.item.Name != "" {
		return loss.item.Name
	}
	if fit := comm.FitTypeOf(loss.typeID); fit != nil && fit.Name != "" {
		return fit.Name
	}
	return "Unknown structure"
}

// addStructureKind counts a structure lost in the battle
func (side *WarBattleSide) addStructureKind(name string) {
	side.StructureLost++
	for index := range side.StructureKinds {
		if side.StructureKinds[index].Name == name {
			side.StructureKinds[index].Count++
			return
		}
	}
	side.StructureKinds = append(side.StructureKinds, WarStructureKind{Name: name, Count: 1})
}

func (m *warMeasureBuilder) add(loss warLoss) {
	m.Killmails++
	m.ISKLost += loss.isk
	if loss.structure || loss.fighter {
		m.ISKStructures += loss.isk
	} else {
		m.ISKFights += loss.isk
	}
	if loss.ship {
		m.ShipsLost++
		m.ShipsFights++
	}
	if loss.citadel {
		if m.citadels == nil {
			m.citadels = map[int]*WarStructureCount{}
		}
		if m.citadels[loss.typeID] == nil {
			m.citadels[loss.typeID] = &WarStructureCount{Type: loss.item}
		}
		m.citadels[loss.typeID].Count++
	}
	if loss.pos {
		m.POS++
	}
}

// addTrash adds a station trash or asset denial killmail, never a structure
func (m *warMeasureBuilder) addTrash(loss warLoss, definite bool) {
	m.Killmails++
	m.ISKLost += loss.isk
	m.ISKTrash += loss.isk
	if definite {
		m.ISKTrashDefinite += loss.isk
	}
	if loss.ship {
		m.ShipsLost++
		m.ShipsTrash++
		if definite {
			m.ShipsTrashDefinite++
		}
	}
}

// warSideAbout is one battle side while it is added up
type warSideAbout struct {
	pilots       map[int]struct{}
	groupPilots  map[warGroupKey]map[int]struct{}
	structGroups map[warGroupKey]int // groups of structures lost with no pilot in them
	side         WarBattleSide
}

func newWarSideAbout() *warSideAbout {
	return &warSideAbout{pilots: map[int]struct{}{}, groupPilots: map[warGroupKey]map[int]struct{}{}, structGroups: map[warGroupKey]int{}}
}

// warTally is every figure the report adds up apart from its rows
type warTally struct {
	measures    map[WarSideKey]*warMeasureBuilder
	otherGroups map[warGroupKey]*warMeasureBuilder
	npcPilots   *warMeasureBuilder
	systems     map[int]*WarSystem
	systemOrder []int
	days        map[string]map[WarSideKey]*warMeasureBuilder // what each side lost each day, EVE time
}

func newWarTally() *warTally {
	return &warTally{
		measures:    map[WarSideKey]*warMeasureBuilder{WarSideA: {}, WarSideB: {}, WarSideNone: {}},
		otherGroups: map[warGroupKey]*warMeasureBuilder{},
		npcPilots:   &warMeasureBuilder{},
		systems:     map[int]*WarSystem{},
		days:        map[string]map[WarSideKey]*warMeasureBuilder{},
	}
}

func buildWar(raw []retrieval.RawKillmail, expanded map[int]*retrieval.Killmail, sides WarSides, minPilots int, edits retrieval.WarEdits) *WarData {
	data := &WarData{Battles: []*WarBattle{}, Systems: []WarSystem{}, Killmails: []WarKillmail{}, OtherGroups: []WarOther{}}
	idx := sides.index()
	kills := analyseWarKills(raw, idx)

	// the names of the groups on the expanded killmails
	names := map[warGroupKey]string{}
	for _, km := range expanded {
		if km == nil {
			continue
		}
		parties := []struct {
			corp *retrieval.Corporation
			ally *retrieval.Alliance
		}{}
		if km.Victim != nil {
			parties = append(parties, struct {
				corp *retrieval.Corporation
				ally *retrieval.Alliance
			}{km.Victim.CorpAtKill, km.Victim.AllianceAtKill})
		}
		for _, attacker := range km.Attackers {
			if attacker != nil {
				parties = append(parties, struct {
					corp *retrieval.Corporation
					ally *retrieval.Alliance
				}{attacker.CorpAtKill, attacker.AllianceAtKill})
			}
		}
		for _, p := range parties {
			if p.ally != nil && p.ally.AllianceID != 0 && names[warGroupKey{types.ALLIANCE, p.ally.AllianceID}] == "" {
				names[warGroupKey{types.ALLIANCE, p.ally.AllianceID}] = p.ally.Name
			}
			if p.corp != nil && p.corp.CorporationID != 0 && names[warGroupKey{types.CORPORATION, p.corp.CorporationID}] == "" {
				names[warGroupKey{types.CORPORATION, p.corp.CorporationID}] = p.corp.Name
			}
		}
	}
	groupOf := func(key warGroupKey) WarGroup {
		return WarGroup{Type: key.Type.String(), ID: key.ID, Name: names[key]}
	}

	// an ignored battle's figures go to their own tally, as its row shows them but nothing else counts them
	totals, ignoredTotals := newWarTally(), newWarTally()
	tally := totals
	missing := 0

	dayOf := func(at time.Time, side WarSideKey) *warMeasureBuilder {
		date := at.UTC().Format("2006-01-02")
		if tally.days[date] == nil {
			tally.days[date] = map[WarSideKey]*warMeasureBuilder{WarSideA: {}, WarSideB: {}, WarSideNone: {}}
		}
		return tally.days[date][side]
	}
	// systemBucket is a side's part of a system in the Systems tab
	systemBucket := func(system *retrieval.System, side WarSideKey) (*WarSystem, *WarSystemSide) {
		entry := tally.systems[system.SystemID]
		if entry == nil {
			entry = &WarSystem{System: system}
			tally.systems[system.SystemID] = entry
			tally.systemOrder = append(tally.systemOrder, system.SystemID)
		}
		switch side {
		case WarSideA:
			return entry, &entry.A
		case WarSideB:
			return entry, &entry.B
		}
		return entry, &entry.Others
	}

	for _, kill := range kills {
		if km := expanded[kill.id]; km != nil && km.System != nil && api.SecurityBand(km.System.SecurityStatus) == api.SECURITY_HIGH {
			kill.highsec = true
		}
	}
	trash, battles, ignoredMarks := warIgnoring(kills, edits.Edits)
	battleOfStation := map[int]*WarBattle{} // the battle each station with trash died in

	for _, battle := range battles {
		tally = totals
		if battle.ignored {
			tally = ignoredTotals
		}
		about := map[WarSideKey]*warSideAbout{WarSideA: newWarSideAbout(), WarSideB: newWarSideAbout(), WarSideNone: newWarSideAbout()}
		battleOthers := map[warGroupKey]bool{}
		for _, kill := range battle.kills {
			for _, p := range kill.parties {
				if p.ignored {
					continue
				}
				side := kill.effective(p)
				if side == WarSideNone && p.hasGroup {
					battleOthers[p.group] = true
				}
				if p.char != 0 {
					about[side].pilots[p.char] = struct{}{}
					if p.hasGroup {
						if about[side].groupPilots[p.group] == nil {
							about[side].groupPilots[p.group] = map[int]struct{}{}
						}
						about[side].groupPilots[p.group][p.char] = struct{}{}
					}
				} else if p.victim && p.hasGroup {
					about[side].structGroups[p.group]++
				}
			}
		}
		for group := range battleOthers {
			if tally.otherGroups[group] == nil {
				tally.otherGroups[group] = &warMeasureBuilder{}
			}
		}

		// the pilots it had, each once, which leaves out an ignored group's
		pilots := map[int]struct{}{}
		for _, part := range about {
			maps.Copy(pilots, part.pilots)
		}

		// the losses
		wb := &WarBattle{
			ID:        battle.id,
			Edits:     slices.Clone(battle.marks),
			Ignored:   battle.ignored,
			Start:     battle.kills[0].at,
			End:       battle.kills[len(battle.kills)-1].at,
			Pilots:    len(pilots),
			Killmails: []*retrieval.Killmail{},
		}
		wb.Skirmish = wb.Pilots < minPilots
		inBattle := map[WarSideKey]*WarBattleSide{WarSideA: &wb.A, WarSideB: &wb.B, WarSideNone: &wb.Others}
		countedIn := map[int]*WarSystem{}
		systemKills := map[int]int{}
		systemFirst := []*retrieval.System{}
		fighters := []*warRefFighters{}
		noteRef := func(kill *warKill, km *retrieval.Killmail, side WarSideKey, party warParty, ship int, lost, sure bool) {
			system := kill.raw.ESIKillmail.SolarSystemID
			kind := warRefKind(ship)
			index := slices.IndexFunc(fighters, func(f *warRefFighters) bool { return f.side == side && f.system == system && f.kind == kind })
			if index < 0 {
				fighters = append(fighters, &warRefFighters{kind: kind, side: side, listed: party.side, group: party.group, system: system})
				index = len(fighters) - 1
			}
			fighters[index].last = kill.at
			fighters[index].sure = fighters[index].sure || sure
			note := WarRefNote{Type: warRefTypeName(ship), Lost: lost}
			if party.side == WarSideNone {
				note.Owner = names[party.group]
			}
			if km.System != nil {
				note.System = km.System.Name
			}
			fighters[index].addNote(note)
		}
		// the structures each side lost in this battle, by kind, and the systems its citadels died in: fighters there are taken for the dying citadel's, as nothing says which launched them
		destroyed := map[WarSideKey]map[string]*warRefFighters{}
		destroyedIn := map[WarSideKey]map[int]bool{}
		for _, kill := range battle.kills {
			victim := kill.parties[0]
			if side := kill.effective(victim); side != WarSideNone && !victim.ignored && comm.UpwellStructureGroups.ContainsGroupOf(warVictimType(kill)) {
				if destroyedIn[side] == nil {
					destroyedIn[side] = map[int]bool{}
				}
				destroyedIn[side][kill.raw.ESIKillmail.SolarSystemID] = true
			}
		}
		// the stations that died in this battle, by owner and type, as one firing before it dies is not one that was reffed
		stationDied := map[warGroupKey]map[int]bool{}
		for _, kill := range battle.kills {
			if victim := kill.parties[0]; victim.hasGroup && warIsStation(warVictimType(kill)) {
				if stationDied[victim.group] == nil {
					stationDied[victim.group] = map[int]bool{}
				}
				stationDied[victim.group][warVictimType(kill)] = true
			}
		}
		for _, kill := range battle.kills {
			wb.KillmailIDs = append(wb.KillmailIDs, kill.id)
			if trash.anchors[kill.id] {
				battleOfStation[kill.id] = wb
			}
			km := expanded[kill.id]
			if km == nil {
				missing++
				continue
			}
			victim := kill.parties[0]
			side := kill.effective(victim)
			loss := warLossOf(kill, km)

			if !victim.ignored {
				wb.Killmails = append(wb.Killmails, km)
				tally.measures[side].add(loss)
				dayOf(kill.at, side).add(loss)
				system := kill.raw.ESIKillmail.SolarSystemID
				if loss.fighter && side != WarSideNone && !destroyedIn[side][system] {
					noteRef(kill, km, side, victim, loss.typeID, true, true)
				}
				if kind := warDestroyedKind(loss.typeID); side != WarSideNone && kind != "" {
					if destroyed[side] == nil {
						destroyed[side] = map[string]*warRefFighters{}
					}
					if destroyed[side][kind] == nil {
						destroyed[side][kind] = &warRefFighters{}
					}
					note := WarRefNote{Type: warRefTypeName(loss.typeID), Lost: true}
					if victim.side == WarSideNone {
						note.Owner = names[victim.group]
					}
					if km.System != nil {
						note.System = km.System.Name
					}
					destroyed[side][kind].addNote(note)
				}
				pilotsOn := warPilotsOn(kill)
				noted := map[warRefFired]bool{}
				for index, attacker := range kill.raw.ESIKillmail.Attackers {
					shooter := kill.parties[index+1]
					shooterSide := kill.effective(shooter)
					fired := warRefFired{shooterSide, attacker.ShipTypeID}
					isFighter := comm.StandupFighterGroups.ContainsGroupOf(attacker.ShipTypeID)
					if shooterSide != WarSideNone && !noted[fired] && !(isFighter && destroyedIn[shooterSide][system]) && warStationFiring(attacker.ShipTypeID, stationDied[shooter.group]) {
						noted[fired] = true
						sure := isFighter || pilotsOn >= warRefSurePilots
						noteRef(kill, km, shooterSide, shooter, attacker.ShipTypeID, false, sure)
					}
				}
				if side == WarSideNone && victim.hasGroup && tally.otherGroups[victim.group] != nil {
					tally.otherGroups[victim.group].add(loss)
				}
				if side == WarSideNone && !victim.hasGroup {
					tally.npcPilots.add(loss)
				}
				target := inBattle[side]
				target.ISKLost += loss.isk
				if loss.ship {
					target.ShipsLost++
				}
				if loss.structure {
					target.addStructureKind(loss.structureKind())
				}

				if !battle.ignored {
					data.Killmails = append(data.Killmails, WarKillmail{Killmail: km, VictimSide: side, BattleID: wb.ID})
				}
			}

			if km.System != nil {
				if _, known := systemKills[km.System.SystemID]; !known {
					systemFirst = append(systemFirst, km.System)
				}
				systemKills[km.System.SystemID]++
				if victim.ignored {
					continue
				}
				entry, bucket := systemBucket(km.System, side)
				bucket.Killmails++
				bucket.ISKLost += loss.isk
				countedIn[km.System.SystemID] = entry
			}
		}
		for _, entry := range countedIn {
			entry.Battles++
		}

		// a station under attack, for each side with its fighters or a station in the fight
		wb.StationRefs = []WarStationRef{}
		for _, side := range []WarSideKey{WarSideA, WarSideB} {
			for _, kind := range []string{WarKindCitadel, WarKindPOS, WarKindPOCO} {
				if died := destroyed[side][kind]; died != nil {
					wb.StationRefs = append(wb.StationRefs, WarStationRef{Side: side, Kind: kind, Destroyed: true, Structure: died.notes[0].Type, Notes: died.notes})
				}
			}
			for _, kind := range []string{WarKindCitadel, WarKindPOS} {
				lost, sure := false, false
				notes := []WarRefNote{}
				var confirmed *warKill
				for _, f := range fighters {
					if f.side == side && f.kind == kind {
						lost = true
						sure = sure || f.sure
						notes = append(notes, f.notes...)
						// only an upwell structure's timers are known, so a tower is never Ref'd
						if confirmed == nil && kind == WarKindCitadel {
							confirmed = f.confirmedBy(trash.stations)
						}
					}
				}
				if !lost {
					continue
				}
				ref := WarStationRef{Side: side, Kind: kind, Attempt: confirmed == nil, Unsure: confirmed == nil && !sure, Notes: notes}
				if confirmed != nil {
					ref.Structure = warLossOf(confirmed, warExpandedOrEmpty(expanded, confirmed)).structureKind()
					ref.DiedAt = confirmed.at
					tally.measures[side].StationRefs++
				} else if ref.Unsure {
					tally.measures[side].RefAttemptsUnsure++
				} else {
					tally.measures[side].RefAttempts++
				}
				wb.StationRefs = append(wb.StationRefs, ref)
			}
		}

		// where it was fought
		var most *retrieval.System
		for _, system := range systemFirst {
			if most == nil || systemKills[system.SystemID] > systemKills[most.SystemID] {
				most = system
			}
		}
		wb.System = most
		wb.OtherSystems = []string{}
		for _, system := range systemFirst {
			if system != most && system.SystemID != most.SystemID {
				wb.OtherSystems = append(wb.OtherSystems, system.Name)
				wb.OtherDetails = append(wb.OtherDetails, system)
			}
		}

		// who fought
		for side, target := range inBattle {
			a := about[side]
			target.Pilots = len(a.pilots)
			slices.SortFunc(target.StructureKinds, func(x, y WarStructureKind) int {
				return cmp.Or(cmp.Compare(y.Count, x.Count), cmp.Compare(x.Name, y.Name))
			})
			pick := func(counts map[warGroupKey]int) *WarGroup {
				var best *warGroupKey
				for group, count := range counts {
					group := group
					if best == nil {
						best = &group
						continue
					}
					bestCount := counts[*best]
					order := cmp.Or(cmp.Compare(count, bestCount)*-1,
						cmp.Compare(strings.ToLower(names[group]), strings.ToLower(names[*best])), cmp.Compare(int(group.Type), int(best.Type)), cmp.Compare(group.ID, best.ID))
					if order < 0 {
						best = &group
					}
				}
				if best == nil {
					return nil
				}
				chosen := groupOf(*best)
				return &chosen
			}
			counts := map[warGroupKey]int{}
			for group, pilots := range a.groupPilots {
				counts[group] = len(pilots)
			}
			if len(a.pilots) > 0 {
				target.Group = pick(counts)
			} else {
				target.Group = pick(a.structGroups)
			}
		}

		data.Battles = append(data.Battles, wb)
	}

	// station trash and asset denial, in their own bucket
	for _, kill := range kills {
		mark, marked := trash.marks[kill.id]
		tally = totals
		if !marked {
			if mark, marked = ignoredMarks[kill.id]; !marked {
				continue
			}
			tally = ignoredTotals
		}
		km := expanded[kill.id]
		if km == nil {
			missing++
			continue
		}
		victim := kill.parties[0]
		if victim.ignored {
			continue
		}
		// a neutral at the station stood beside its owner
		side := victim.side
		if side == WarSideNone && victim.hasGroup && !victim.forced {
			side = mark.owner
		}
		loss := warLossOf(kill, km)
		tally.measures[side].addTrash(loss, mark.definite)
		dayOf(kill.at, side).addTrash(loss, mark.definite)
		if side == WarSideNone {
			if victim.hasGroup {
				if tally.otherGroups[victim.group] == nil {
					tally.otherGroups[victim.group] = &warMeasureBuilder{}
				}
				tally.otherGroups[victim.group].addTrash(loss, mark.definite)
			} else {
				tally.npcPilots.addTrash(loss, mark.definite)
			}
		}
		battleID := ""
		if wb := battleOfStation[mark.anchor]; wb != nil {
			battleID = wb.ID
			target := map[WarSideKey]*WarBattleSide{WarSideA: &wb.A, WarSideB: &wb.B, WarSideNone: &wb.Others}[side]
			target.TrashKillmails++
			target.TrashISK += loss.isk
			if mark.definite {
				target.TrashDefiniteISK += loss.isk
			}
			if loss.ship {
				target.TrashShips++
				if mark.definite {
					target.TrashDefiniteShips++
				}
			}
		}
		if km.System != nil {
			_, bucket := systemBucket(km.System, side)
			bucket.Killmails++
			bucket.ISKLost += loss.isk
		}
		if tally == totals {
			data.Killmails = append(data.Killmails, WarKillmail{Killmail: km, VictimSide: side, BattleID: battleID, Trash: true, Definite: mark.definite})
		}
	}

	if missing > 0 {
		comm.Logger.Warnf("%d killmails of a war's battles have no expanded form and are left out", missing)
	}

	// the war as a whole
	tally = totals
	slices.SortFunc(data.Battles, func(a, b *WarBattle) int { return cmp.Or(b.Start.Compare(a.Start), cmp.Compare(a.ID, b.ID)) })
	for _, battle := range data.Battles {
		if battle.Ignored {
			data.IgnoredBattles++
		} else if battle.Skirmish {
			data.Skirmishes++
		} else {
			data.BattleCount++
		}
	}
	slices.SortFunc(data.Killmails, func(a, b WarKillmail) int {
		return cmp.Or(b.Killmail.KillmailTime.Compare(a.Killmail.KillmailTime), cmp.Compare(b.Killmail.KillmailID, a.Killmail.KillmailID))
	})

	for _, id := range tally.systemOrder {
		data.Systems = append(data.Systems, *tally.systems[id])
	}
	total := func(s WarSystem) int { return s.A.Killmails + s.B.Killmails + s.Others.Killmails }
	slices.SortStableFunc(data.Systems, func(a, b WarSystem) int {
		return cmp.Or(cmp.Compare(total(b), total(a)), cmp.Compare(strings.ToLower(a.System.Name), strings.ToLower(b.System.Name)), cmp.Compare(a.System.SystemID, b.System.SystemID))
	})

	data.A, data.B, data.Others = tally.measures[WarSideA].result(), tally.measures[WarSideB].result(), tally.measures[WarSideNone].result()
	data.NPCPilots = tally.npcPilots.result()
	data.Days = []WarDay{}
	for date, sides := range tally.days {
		day := WarDay{Date: date, A: sides[WarSideA].result(), B: sides[WarSideB].result()}
		if day.A.Killmails > 0 || day.B.Killmails > 0 {
			data.Days = append(data.Days, day)
		}
	}
	slices.SortFunc(data.Days, func(a, b WarDay) int { return cmp.Compare(a.Date, b.Date) })
	for group, builder := range tally.otherGroups {
		data.OtherGroups = append(data.OtherGroups, WarOther{Group: groupOf(group), Measures: builder.result()})
	}
	slices.SortFunc(data.OtherGroups, func(a, b WarOther) int {
		return cmp.Or(cmp.Compare(b.Measures.ISKLost, a.Measures.ISKLost), cmp.Compare(strings.ToLower(a.Group.Name), strings.ToLower(b.Group.Name)),
			cmp.Compare(a.Group.Type, b.Group.Type), cmp.Compare(a.Group.ID, b.Group.ID))
	})
	return data
}
