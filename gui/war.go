package gui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// warType is a war's type in recent searches, choices, and TargetResult.Type
const warType = "WAR"

// errNoWar is shown when a war was removed since the frontend last listed it
var errNoWar = errors.New("That war no longer exists")

// WarSideCard is one side of a war as the frontend sends and shows it
type WarSideCard struct {
	Type           string `json:"type"` // COALITION, CORPORATION, or ALLIANCE
	ID             int    `json:"id"`
	Name           string `json:"name"`
	WithAffiliates bool   `json:"withAffiliates"` // a corp or alliance with its affiliates, as picked from the search bar's list of groups
	Count          int    `json:"count"`          // the groups it resolves to now, sent back only
}

// WarAddedCard is a group the user put on a side from the report
type WarAddedCard struct {
	Type     string `json:"type"` // CORPORATION or ALLIANCE
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FromDate string `json:"fromDate"` // YYYY-MM-DD, empty for from the beginning of the war
}

// WarCard is a saved war, for the setup window's list and the report's versus card
type WarCard struct {
	ID        int            `json:"id"` // 0 for a new war
	Name      string         `json:"name"`
	SideA     WarSideCard    `json:"sideA"`
	SideB     WarSideCard    `json:"sideB"`
	FromDays  int            `json:"fromDays"`  // sent by the age picker's buttons, and made a fixed FromDate on saving. Sent back as 0.
	FromDate  string         `json:"fromDate"`  // YYYY-MM-DD, empty with FromDays 0 for no start
	UntilDate string         `json:"untilDate"` // YYYY-MM-DD, empty while it goes on
	AddedA    []WarAddedCard `json:"addedA"`    // sent back only: change them with AddWarGroup and RemoveWarGroup
	AddedB    []WarAddedCard `json:"addedB"`
	Ignored   []WarAddedCard `json:"ignored"`   // sent back only: change them with IgnoreWarGroup and RemoveWarGroup
	LastRun   time.Time      `json:"lastRun"`   // sent back only, the zero time before its first run
	Killmails int            `json:"killmails"` // sent back only, how many it holds
}

// WarStructureCount is how many of one structure type were lost
type WarStructureCount struct {
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
	Count    int    `json:"count"`
}

// WarMeasures are the sheet's categories for one side. ISKLost is ISKFights + ISKTrash + ISKStructures, and ShipsLost is ShipsFights + ShipsTrash.
type WarMeasures struct {
	Killmails          int                 `json:"killmails"`
	ISKLost            float64             `json:"iskLost"`
	ISKFights          float64             `json:"iskFights"`
	ISKTrash           float64             `json:"iskTrash"`         // station trash and asset denial
	ISKTrashDefinite   float64             `json:"iskTrashDefinite"` // the part of iskTrash graded definite, the rest being probable
	ISKStructures      float64             `json:"iskStructures"`
	ShipsLost          int                 `json:"shipsLost"`
	ShipsFights        int                 `json:"shipsFights"`
	ShipsTrash         int                 `json:"shipsTrash"`
	ShipsTrashDefinite int                 `json:"shipsTrashDefinite"`
	Citadels           []WarStructureCount `json:"citadels"` // never null
	CitadelCount       int                 `json:"citadelCount"`
	POS                int                 `json:"pos"`
	StationRefs        int                 `json:"stationRefs"`       // battles marked Citadel Ref'd for this side
	RefAttempts        int                 `json:"refAttempts"`       // battles marked Station Attacked for this side
	RefAttemptsUnsure  int                 `json:"refAttemptsUnsure"` // battles marked Station Attacked? for this side
	Efficiency         float64             `json:"efficiency"`        // 0 to 100: the other side's ISK lost as a share of both sides' ISK lost. 0 for Others, and when neither lost any.
}

// WarDay is a row of the Timeline tab: what each side lost on one day, EVE time. The frontend adds them up into running totals.
type WarDay struct {
	Date string      `json:"date"` // YYYY-MM-DD
	A    WarMeasures `json:"a"`
	B    WarMeasures `json:"b"`
}

// WarStationRef marks a battle where a side lost or used Standup Fighters, or a station of its fired and survived, so its station was probably under attack
type WarStationRef struct {
	Side        string       `json:"side"`        // a or b
	Attempt     bool         `json:"attempt"`     // no structure of that side died in the system within its timer days
	Destroyed   bool         `json:"destroyed"`   // its station died in this very fight, so it is no ref
	Kind        string       `json:"kind"`        // citadel, pos, or poco
	Unsure      bool         `json:"unsure"`      // an attempt on thin evidence, see aggregate.WarStationRef
	Structure   string       `json:"structure"`   // the type of the structure that did, empty for an attempt
	StructureAt time.Time    `json:"structureAt"` // when it did, zero for an attempt
	Notes       []WarRefNote `json:"notes"`       // what marked it, never null
}

// WarRefNote is one thing that marked a battle: a type of station, battery, or fighter, in one system
type WarRefNote struct {
	Type      string `json:"type"`
	Owner     string `json:"owner"` // its group when that is a third party fighting beside the side, else empty
	System    string `json:"system"`
	Lost      bool   `json:"lost"` // lost, not firing
	Killmails int    `json:"killmails"`
}

// WarStructureKind is how many of one kind of structure a side lost in a battle
type WarStructureKind struct {
	Name  string `json:"name"` // the type's name, or POS for any control tower
	Count int    `json:"count"`
}

// WarBattleSide is a side's part in a row of the Battles tab
type WarBattleSide struct {
	Group              *EntityRef         `json:"group"` // its largest group in the battle by pilots, null if it has none there
	Pilots             int                `json:"pilots"`
	ISKLost            float64            `json:"iskLost"`
	ShipsLost          int                `json:"shipsLost"`
	Structures         int                `json:"structures"`
	StructureKinds     []WarStructureKind `json:"structureKinds"` // never null, most lost first
	TrashKillmails     int                `json:"trashKillmails"` // station trash and asset denial attached to this battle, in none of the figures above
	TrashISK           float64            `json:"trashIsk"`
	TrashShips         int                `json:"trashShips"`
	TrashDefiniteISK   float64            `json:"trashDefiniteIsk"` // the part of trashIsk graded definite, the rest being probable
	TrashDefiniteShips int                `json:"trashDefiniteShips"`
}

// WarBattle is a row of the Battles tab: one fight, as in the sheet
type WarBattle struct {
	ID           string        `json:"id"`
	Start        time.Time     `json:"start"`
	End          time.Time     `json:"end"`
	System       SystemSummary `json:"system"`       // the system with the most of its killmails, which its links are for
	OtherSystems []string      `json:"otherSystems"` // never null
	// the same systems' battle reports, for the row's menu
	OtherSystemReports []WarSystemReport `json:"otherSystemReports"`
	Class              string            `json:"class"` // the system's wormhole class, else its security band
	Pilots             int               `json:"pilots"`
	Killmails          int               `json:"killmails"`
	Skirmish           bool              `json:"skirmish"` // marked beside its date
	Edits              []WarEditMark     `json:"edits"`    // what the user did to it, never null
	Ignored            bool              `json:"ignored"`  // set aside by the user: grayed out, its figures its row's alone
	A                  WarBattleSide     `json:"a"`
	B                  WarBattleSide     `json:"b"`
	Others             WarBattleSide     `json:"others"`
	StationRefs        []WarStationRef   `json:"stationRefs"` // never null
	ZkillURL           string            `json:"zkillUrl"`
	EveToolsURL        string            `json:"evetoolsUrl"`
	WarbeaconURL       string            `json:"warbeaconUrl"`
}

// WarSystemReport is the battle report links of one system of a battle
type WarSystemReport struct {
	Name         string `json:"name"`
	ZkillURL     string `json:"zkillUrl"`
	EveToolsURL  string `json:"evetoolsUrl"`
	WarbeaconURL string `json:"warbeaconUrl"`
}

// WarClassCount is how many of a war's systems are of one class, ie 13 C5
type WarClassCount struct {
	Class string `json:"class"` // a wormhole class, or a security band: high, low, or null
	Count int    `json:"count"`
}

// WarSystemSide is a side's losses in a system
type WarSystemSide struct {
	Killmails int     `json:"killmails"`
	ISKLost   float64 `json:"iskLost"`
}

// WarSystem is a row of the Systems tab
type WarSystem struct {
	System  SystemSummary `json:"system"`
	Class   string        `json:"class"`   // as WarClassCount's
	Battles int           `json:"battles"` // battles and skirmishes here, one that moved counting in each of its systems
	A       WarSystemSide `json:"a"`
	B       WarSystemSide `json:"b"`
	Others  WarSystemSide `json:"others"`
}

// WarKillmail is a killmail a war's report counted. The frontend never lists them, as a war has far too many, so it gets only their count.
type WarKillmail struct {
	ID       int
	Side     string // whose loss it is: a, b, or others
	BattleID string // the WarBattle it is in
}

// WarOtherGroup is a group under Others, which the user can add to a side
type WarOtherGroup struct {
	Group     EntityRef `json:"group"`
	Killmails int       `json:"killmails"` // its losses under Others
	ISKLost   float64   `json:"iskLost"`
	ShipsLost int       `json:"shipsLost"`
}

// WarNPCPilots is what pilots in NPC corps lost, the part of Others with no group, so it can not be added to a side or ignored
type WarNPCPilots struct {
	Killmails int     `json:"killmails"`
	ISKLost   float64 `json:"iskLost"`
	ShipsLost int     `json:"shipsLost"`
}

// WarReachLine is a list that did not reach the war's start at its last run, for the reach warning
type WarReachLine struct {
	Group  string    `json:"group"`
	Oldest time.Time `json:"oldest"`
}

// WarReport is what a war's report shows. New categories join WarMeasures and WarBattleSide, which have room for Stage 3's.
type WarReport struct {
	War            WarCard         `json:"war"`
	A              WarMeasures     `json:"a"`
	B              WarMeasures     `json:"b"`
	Others         WarMeasures     `json:"others"`
	Battles        []WarBattle     `json:"battles"`     // newest first, skirmishes among them. Never null, as all the lists here.
	BattleCount    int             `json:"battleCount"` // those not Skirmish
	Skirmishes     int             `json:"skirmishes"`
	IgnoredBattles int             `json:"ignoredBattles"` // set aside by the user, in Battles but not in BattleCount or Skirmishes
	SystemCount    int             `json:"systemCount"`
	Classes        []WarClassCount `json:"classes"` // most systems first
	Systems        []WarSystem     `json:"systems"` // most killmails first
	Killmails      []WarKillmail   `json:"-"`
	KillmailCount  int             `json:"killmailCount"`
	OtherGroups    []WarOtherGroup `json:"otherGroups"` // most ISK lost first
	NPCPilots      WarNPCPilots    `json:"npcPilots"`
	Days           []WarDay        `json:"days"` // oldest first, only days with a loss on either side
	Reach          []WarReachLine  `json:"reach"`
	FullRunDue     bool            `json:"fullRunDue"` // the sides or start changed since its last full run, so its next run reads every list from its start. Also true before its first run.
	Stopped        bool            `json:"stopped"`
	RateLimited    bool            `json:"rateLimited"`
	Sources        string          `json:"sources"` // as IntelReport's, empty on opening with nothing collected
}

// warDateLayout is how a war's dates are saved and sent
const warDateLayout = "2006-01-02"

// warUpdateOverlap is how far before its newest killmail an update lists, so a killmail posted late is not missed. zKillboard allows itself 3 days, and it is rare for a killmail to be held back longer than a couple of business days.
const warUpdateOverlap = 5 * 24 * time.Hour

// warSideTypes are the types a side can be
var warSideTypes = []string{types.COALITION.String(), types.CORPORATION.String(), types.ALLIANCE.String()}

// warSideFromCard is a side as saved on the war
func warSideFromCard(card WarSideCard) retrieval.WarSide {
	return retrieval.WarSide{Type: card.Type, ID: card.ID, Name: card.Name, WithAffiliates: card.WithAffiliates}
}

// warCard builds a saved war's card. The groups its sides resolve to now are counted from the cache, and a side that is gone is counted as 0.
func (a *App) warCard(war retrieval.War) WarCard {
	sideCard := func(side retrieval.WarSide) WarSideCard {
		card := WarSideCard{Type: side.Type, ID: side.ID, Name: side.Name, WithAffiliates: side.WithAffiliates}
		if groups, err := a.cache.WarSideGroups(side); err == nil {
			card.Count = len(groups)
		}
		return card
	}
	added := func(groups []retrieval.WarAddedGroup) []WarAddedCard {
		cards := []WarAddedCard{}
		for _, group := range groups {
			cards = append(cards, WarAddedCard{Type: group.Type, ID: group.ID, Name: group.Name, FromDate: group.FromDate})
		}
		return cards
	}
	return WarCard{ID: war.ID, Name: war.Name, SideA: sideCard(war.SideA), SideB: sideCard(war.SideB),
		FromDate: war.FromDate, UntilDate: war.UntilDate, AddedA: added(war.AddedA), AddedB: added(war.AddedB), Ignored: added(war.Ignored),
		LastRun: war.LastRun, Killmails: a.cache.WarKillmailCount(war.ID)}
}

// warSideGoneError is the message for a side that no longer exists, naming it
func warSideGoneError(war retrieval.War, side retrieval.WarSide, err error) error {
	if errors.Is(err, retrieval.ErrWarSideGone) {
		return fmt.Errorf("%s, a side of %s, no longer exists. Open the war setup and choose another.", side.Name, war.Name)
	}
	return fmt.Errorf("Could not work out who is on %s of %s: %s", side.Name, war.Name, api.FailureReason(err))
}

// warSides resolves both sides of a war as they are now, and the groups of each
func (a *App) warSides(war retrieval.War) (aggregate.WarSides, error) {
	groupsA, err := a.cache.WarSideGroups(war.SideA)
	if err != nil {
		return aggregate.WarSides{}, warSideGoneError(war, war.SideA, err)
	}
	groupsB, err := a.cache.WarSideGroups(war.SideB)
	if err != nil {
		return aggregate.WarSides{}, warSideGoneError(war, war.SideB, err)
	}
	return aggregate.WarSides{A: groupsA, B: groupsB, AddedA: war.AddedA, AddedB: war.AddedB, Ignored: war.Ignored}, nil
}

// warStandIn is the target a war's killmails are collected and shown for: a coalition of every group on either side and each group added, each once
func (a *App) warStandIn(war retrieval.War, sides aggregate.WarSides) retrieval.IntelTarget {
	holdings := []retrieval.TargetGroup{}
	add := func(group retrieval.TargetGroup) {
		if !slices.ContainsFunc(holdings, func(other retrieval.TargetGroup) bool { return other.Type == group.Type && other.ID == group.ID }) {
			holdings = append(holdings, group)
		}
	}
	for _, group := range sides.A {
		add(group)
	}
	for _, group := range sides.B {
		add(group)
	}
	for _, added := range slices.Concat(sides.AddedA, sides.AddedB) {
		if entityType, ok := searchTypes[added.Type]; ok {
			add(retrieval.TargetGroup{Type: entityType, ID: added.ID, Name: added.Name, Member: true})
		}
	}
	return retrieval.IntelTarget{IDNumber: war.ID, Name: war.Name, TypeOfEntity: types.COALITION, Clients: a.clients, Holdings: holdings}
}

// validWarSide checks a side of a saved war, and returns it as saved with its directly listed members as type and id
func (a *App) validWarSide(card WarSideCard, label string) (retrieval.WarSide, []retrieval.CoalitionMember, error) {
	if card.Type == "" || card.ID == 0 {
		return retrieval.WarSide{}, nil, fmt.Errorf("Choose who is on side %s", label)
	}
	if !slices.Contains(warSideTypes, card.Type) {
		return retrieval.WarSide{}, nil, fmt.Errorf("Side %s can be a coalition, a corporation, or an alliance, not a %s", label, strings.ToLower(card.Type))
	}
	side := warSideFromCard(card)
	if card.Type == coalitionType {
		coalition, found := a.cache.CoalitionByID(card.ID)
		if !found {
			return side, nil, fmt.Errorf("The coalition %s on side %s is no longer saved. Choose another for that side in the War window.", card.Name, label)
		}
		side.Name, side.WithAffiliates = coalition.Name, false
		direct := []retrieval.CoalitionMember{}
		for _, member := range coalition.Members {
			direct = append(direct, member)
		}
		return side, direct, nil
	}
	if card.ID < 0 {
		return side, nil, fmt.Errorf("Side %s is not a corporation or alliance that EVE knows", label)
	}
	if strings.TrimSpace(side.Name) == "" {
		if names, err := a.clients.ESIClient.GetNamesFromEveIDs([]int{card.ID}); err == nil && names[card.ID] != "" {
			side.Name = names[card.ID]
		} else {
			side.Name = fmt.Sprintf("%s %d", strings.ToLower(card.Type), card.ID)
		}
	}
	side.Name = strings.TrimSpace(side.Name)
	return side, []retrieval.CoalitionMember{{Type: card.Type, ID: card.ID, Name: side.Name}}, nil
}

// parseWarDate reads a war's date, empty for none
func parseWarDate(text string, label string) (string, time.Time, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", time.Time{}, nil
	}
	at, err := time.Parse(warDateLayout, text)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("The %s date %q is not a date, use year-month-day", label, text)
	}
	return text, at, nil
}

// renameRecentWar makes a renamed war's recent search use its new name, as recent searches are kept by name. One it never had stays absent.
func (a *App) renameRecentWar(id int, oldName string, newName string) {
	for _, recent := range a.cache.RecentSearches(retrieval.MaxRecentSearches) {
		if recent.Type == warType && recent.ID == id {
			a.cache.RemoveRecentSearch(retrieval.RecentSearch{Name: oldName, Type: warType})
			a.cache.AddRecentSearch(retrieval.RecentSearch{Name: newName, Type: warType, ID: id}, a.currentSettings().MaxRecentSearches)
			return
		}
	}
}

// warClass is a system's wormhole class, else its security band, empty for an unknown system
func warClass(system *retrieval.System) string {
	if system == nil {
		return ""
	}
	if class := comm.WormholeClass(system.Name, system.Region); class != "" {
		return class
	}
	return api.SecurityBand(system.SecurityStatus)
}

/*
	GetWars returns every saved war, by name, for the setup window. It reads only the cache, so it does not wait for a running search.

Returns:
  - []WarCard: the wars
  - error: a user readable message if something went wrong
*/
func (a *App) GetWars() (cards []WarCard, err error) {
	defer recoverAsError(&err)
	a.waitReady()
	cards = []WarCard{}
	for _, war := range a.cache.Wars() {
		cards = append(cards, a.warCard(war))
	}
	return cards, nil
}

/*
	SaveWar adds a war, or changes the one with card's ID, keeping what it holds. It does not run it. It does not wait for a running search.
	A rolling FromDays is made a fixed date here, as a war's start must not slide.

Parameters:
  - card: Name, SideA, SideB, FromDays or FromDate, and UntilDate. An empty Name becomes "A vs B".

Returns:
  - WarCard: what was saved, with its ID
  - error: a user readable message: a side missing or of a wrong type, an entity listed directly on both sides or the same side twice, a coalition no longer saved, a name another war has or too long, Until before From, or a date that can not be read. Nothing is saved.
*/
func (a *App) SaveWar(card WarCard) (saved WarCard, err error) {
	defer recoverAsError(&err)
	a.waitReady()
	a.warMu.Lock()
	defer a.warMu.Unlock()

	war := retrieval.War{}
	if card.ID != 0 {
		var found bool
		if war, found = a.cache.WarByID(card.ID); !found {
			return saved, errNoWar
		}
	}
	oldName := war.Name

	sideA, directA, err := a.validWarSide(card.SideA, "A")
	if err != nil {
		return saved, err
	}
	sideB, directB, err := a.validWarSide(card.SideB, "B")
	if err != nil {
		return saved, err
	}
	if sideA.Type == sideB.Type && sideA.ID == sideB.ID {
		return saved, fmt.Errorf("%s can not be on both sides", sideA.Name)
	}
	for _, member := range directA {
		if slices.ContainsFunc(directB, func(other retrieval.CoalitionMember) bool { return other.Type == member.Type && other.ID == member.ID }) {
			return saved, fmt.Errorf("%s is listed on both sides", member.Name)
		}
	}

	name := strings.TrimSpace(card.Name)
	switch {
	case name == "":
		name = sideA.Name + " vs " + sideB.Name
	case len([]rune(name)) > maxCoalitionName:
		return saved, fmt.Errorf("A war's name can be at most %d characters", maxCoalitionName)
	}
	if len([]rune(name)) > maxCoalitionName {
		name = string([]rune(name)[:maxCoalitionName])
	}
	if other, found := a.cache.WarNamed(name); found && other.ID != war.ID {
		return saved, fmt.Errorf("There is already a war named %s", other.Name)
	}

	fromText := card.FromDate
	if card.FromDays > 0 {
		fromText = time.Now().UTC().AddDate(0, 0, -card.FromDays).Format(warDateLayout)
	}
	fromText, from, err := parseWarDate(fromText, "From")
	if err != nil {
		return saved, err
	}
	untilText, until, err := parseWarDate(card.UntilDate, "Until")
	if err != nil {
		return saved, err
	}
	if !from.IsZero() && !until.IsZero() && until.Before(from) {
		return saved, errors.New("A war can not end before it starts")
	}

	war.Name, war.SideA, war.SideB, war.FromDate, war.UntilDate = name, sideA, sideB, fromText, untilText
	// a group now on a side is no longer ignored
	if sides, err := a.warSides(war); err == nil {
		war.Ignored = slices.DeleteFunc(slices.Clone(war.Ignored), func(ignored retrieval.WarAddedGroup) bool {
			return slices.ContainsFunc(slices.Concat(sides.A, sides.B), func(listed retrieval.TargetGroup) bool {
				return listed.Type.String() == ignored.Type && listed.ID == ignored.ID
			})
		})
	}
	result := a.cache.SaveWar(war)
	if oldName != "" && oldName != result.Name {
		a.renameRecentWar(result.ID, oldName, result.Name)
	}
	a.saveCacheQuietly()
	return a.warCard(result), nil
}

/*
	RemoveWar removes a war and the killmails it holds, and its recent search. It does not wait for a running search.

Parameters:
  - id: the war

Returns:
  - error: a user readable message if something went wrong
*/
func (a *App) RemoveWar(id int) (err error) {
	defer recoverAsError(&err)
	a.waitReady()
	a.warMu.Lock()
	defer a.warMu.Unlock()

	war, found := a.cache.WarByID(id)
	if !found {
		return nil
	}
	a.cache.RemoveWar(id)
	a.cache.RemoveRecentSearch(retrieval.RecentSearch{Name: war.Name, Type: warType})
	a.saveCacheQuietly()
	return nil
}

// beginWarReport readies the app to report on a war, whose report replaces any other: what acts on the last report or its allies and roster must not act on the previous one
func (a *App) beginWarReport(war retrieval.War) {
	a.lastReport, a.lastTarget, a.lastAllies = nil, nil, nil
	a.setRoster(nil)
	a.cache.AddRecentSearch(retrieval.RecentSearch{Name: war.Name, Type: warType, ID: war.ID}, a.currentSettings().MaxRecentSearches)
}

/*
	OpenWar builds a war's report from what it holds, with no network beyond name lookups, and adds it to recent searches. It takes a.mu.

Parameters:
  - id: the war

Returns:
  - WarReport: the report. Its lists are empty before its first run.
  - error: errNoWar, a side no longer existing named, or a user readable message
*/
func (a *App) OpenWar(id int) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		a.finishDebug(func(search *debugSearch) {
			search.Killmails, search.Stopped, search.RateLimited = len(report.Killmails), report.Stopped, report.RateLimited
			if err != nil {
				search.Error = err.Error()
			}
		})
	}()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, found := a.cache.WarByID(id)
	if !found {
		return report, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return report, err
	}
	a.beginWarReport(war)
	report = a.buildWarReport(war, sides)
	a.saveCacheQuietly()
	return report, nil
}

// warRunUpdate is what a run learned, applied to the war as saved when the run ends
type warRunUpdate struct {
	full     bool
	runKey   string
	newest   time.Time
	reach    []retrieval.WarReach
	complete bool // the run was not stopped and had no list error, so its run key is kept
}

// applyWarRun saves what a run learned onto the war as it is saved now, as it may have been changed by SaveWar during the run
func (a *App) applyWarRun(id int, run warRunUpdate) {
	a.warMu.Lock()
	defer a.warMu.Unlock()
	war, found := a.cache.WarByID(id)
	if !found {
		return
	}
	war.LastRun = time.Now().UTC()
	if run.newest.After(war.NewestKillmail) {
		war.NewestKillmail = run.newest
	}
	if run.complete {
		war.RunKey = run.runKey
	}
	if run.full {
		war.Reach = run.reach
	} else {
		for _, entry := range run.reach {
			if entry.Complete {
				continue
			}
			if index := slices.IndexFunc(war.Reach, func(old retrieval.WarReach) bool { return old.Group == entry.Group }); index >= 0 {
				war.Reach[index] = entry
			} else {
				war.Reach = append(war.Reach, entry)
			}
		}
	}
	a.cache.SaveWar(war)
}

/*
	CollectWar lists both sides' killmails, keeps what counts with what the war held, and builds its report. It takes a.mu, reports progress, and stops as CollectIntel does.
	A war whose RunKey is CurrentRunKey lists only from its NewestKillmail less warUpdateOverlap, 5 days. Any other reads from its start. It ignores the search limits button.

Parameters:
  - id: the war

Returns:
  - WarReport: the report, from what was held when the run stopped or met an error part way
  - error: errNoWar, a side no longer existing named, or a user readable message if nothing could be collected and it holds nothing
*/
func (a *App) CollectWar(id int) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		a.finishDebug(func(search *debugSearch) {
			search.Killmails, search.Stopped, search.RateLimited = len(report.Killmails), report.Stopped, report.RateLimited
			if err != nil {
				search.Error = err.Error()
			}
		})
	}()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, found := a.cache.WarByID(id)
	if !found {
		return report, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return report, err
	}
	// a run starts unstopped, as a search does: an Update does not go through SearchTarget, so an earlier Stop would otherwise end it at once
	a.stopRequested.Store(false)
	a.esiGuard.Resume()
	defer a.esiGuard.Resume()
	lookupsBefore := a.cache.Counts()

	a.beginWarReport(war)
	runKey := war.RunKeyFor(sides.A, sides.B)
	full := war.RunKey != runKey
	newerThan := war.Start()
	if !full && !war.NewestKillmail.IsZero() {
		if since := war.NewestKillmail.Add(-warUpdateOverlap); since.After(newerThan) {
			newerThan = since
		}
	}
	if full {
		comm.ReportProgress("Collecting %s in full, as its sides or start changed since its last full run", war.Name)
	} else {
		comm.ReportProgress("Updating %s from %s", war.Name, api.FormatShortDate(newerThan))
	}

	target := a.warStandIn(war, sides)
	a.noteDebugSearch(func(search *debugSearch) {
		search.TargetID, search.TargetType, search.TargetName = war.ID, warType, war.Name
		for _, holding := range target.Holdings {
			search.Holdings = append(search.Holdings, holding.Name)
		}
	})

	var reachMu sync.Mutex
	gathered := []retrieval.WarReach{}
	limits := retrieval.KillmailLimits{Max: retrieval.AllKillmails, Pages: retrieval.AllPages, IncludeCached: true, NewerThan: newerThan,
		OnListed: func(group string, oldest time.Time, complete bool) {
			reachMu.Lock()
			defer reachMu.Unlock()
			gathered = append(gathered, retrieval.WarReach{Group: group, Oldest: oldest, Complete: complete})
		}}
	listed, collectErr := retrieval.CollectKillmails(target, limits)
	rateLimited := errors.Is(collectErr, api.ErrRateLimited)
	held := a.cache.WarKillmails(war.ID)
	if collectErr != nil {
		comm.Logger.WithError(collectErr).Warnf("Continuing with %d collected killmails", len(*listed))
		if len(*listed) == 0 && len(held) == 0 {
			if a.stopRequested.Load() {
				a.saveCache()
				return report, errors.New("Search stopped before any killmails were collected")
			}
			if rateLimited {
				return report, errors.New("The ESI has limited your requests before any killmails were collected. You will need to wait 15 minutes before continuing.")
			}
			return report, fmt.Errorf("Could not retrieve killmails for this war: %s", api.FailureReason(collectErr))
		}
	}

	// what is listed wins over what is held, each killmail once
	newest := time.Time{}
	merged := []retrieval.RawKillmail{}
	position := map[int]int{}
	put := func(raw retrieval.RawKillmail) {
		if raw.ESIKillmail == nil || raw.Zkillmail == nil || !war.Within(raw.ESIKillmail.KillmailTime) {
			return
		}
		raw.Via = nil
		if index, seen := position[raw.Zkillmail.KillmailID]; seen {
			merged[index] = raw
			return
		}
		position[raw.Zkillmail.KillmailID] = len(merged)
		merged = append(merged, raw)
	}
	for _, raw := range held {
		put(raw)
	}
	for _, raw := range *listed {
		// only what the war keeps moves its newest: past its end date it would leave a gap once the end date is extended
		if raw.ESIKillmail != nil && war.Within(raw.ESIKillmail.KillmailTime) && raw.ESIKillmail.KillmailTime.After(newest) {
			newest = raw.ESIKillmail.KillmailTime
		}
		put(raw)
	}
	comm.ReportProgress("Sorting %d killmails into the war's battles", len(merged))
	kept := aggregate.ClassifyRawWar(merged, sides)
	if _, still := a.cache.WarByID(war.ID); still {
		a.cache.AddWarKillmails(war.ID, kept)
	}

	stopped := a.stopRequested.Load()
	a.applyWarRun(war.ID, warRunUpdate{full: full, runKey: runKey, newest: newest, reach: gathered, complete: !stopped && collectErr == nil})

	current, found := a.cache.WarByID(war.ID)
	if !found {
		current = war
	}
	report = a.buildWarReport(current, sides)
	comm.ReportProgress("Saving cache")
	a.saveCache()

	report.Stopped = stopped
	report.RateLimited = rateLimited
	sources := []string{"zKillboard live"}
	if report.Stopped {
		sources = append(sources, "stopped early")
	} else if rateLimited {
		sources = append(sources, "stopped early, the ESI rate limited killmails")
	}
	if source := a.cache.Counts().Since(lookupsBefore).Source(); source != "" {
		sources = append([]string{source}, sources...)
	}
	report.Sources = strings.Join(sources, "; ")
	return report, nil
}

/*
	AddWarGroup puts a group under Others onto a side, and builds the report again from what the war holds. Its own list comes in at the next run, a full one. It takes a.mu.

Parameters:
  - id: the war
  - side: a or b
  - group: its Type, ID, and Name, and FromDate: empty for from the beginning of the war, else YYYY-MM-DD, its killmails before then staying under Others

Returns:
  - WarReport: the report after the change
  - error: a user readable message: an unknown side, a group of a wrong type, one already on either side or added, or an unreadable date. Nothing changes.
*/
func (a *App) AddWarGroup(id int, side string, group WarAddedCard) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, sides, err := a.changeWarGroups(id, func(war *retrieval.War, sides aggregate.WarSides) error {
		side = strings.ToLower(strings.TrimSpace(side))
		if side != "a" && side != "b" {
			return fmt.Errorf("Unknown side %q, it is a or b", side)
		}
		if group.Type != types.CORPORATION.String() && group.Type != types.ALLIANCE.String() || group.ID <= 0 {
			return errors.New("Only a corporation or an alliance can be put on a side")
		}
		from, _, err := parseWarDate(group.FromDate, "From")
		if err != nil {
			return err
		}
		if err := warGroupFree(war, sides, group); err != nil {
			return err
		}
		entry := retrieval.WarAddedGroup{Type: group.Type, ID: group.ID, Name: warGroupName(group), FromDate: from}
		war.Ignored = slices.DeleteFunc(war.Ignored, func(ignored retrieval.WarAddedGroup) bool {
			return ignored.Type == group.Type && ignored.ID == group.ID
		})
		if side == "a" {
			war.AddedA = append(war.AddedA, entry)
		} else {
			war.AddedB = append(war.AddedB, entry)
		}
		return nil
	})
	if err != nil {
		return report, err
	}
	a.lastReport, a.lastTarget = nil, nil
	report = a.buildWarReport(war, sides)
	return report, nil
}

/*
	IgnoreWarGroup sets a group under Others aside: it is on no side and its losses count nowhere, and it leaves the Others list. RemoveWarGroup takes it back.
	It builds the report again from what the war holds, and takes a.mu.

Parameters:
  - id: the war
  - group: its Type, ID, and Name

Returns:
  - WarReport: the report after the change
  - error: a user readable message if the war is gone or the group is on a side
*/
func (a *App) IgnoreWarGroup(id int, group WarAddedCard) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, sides, err := a.changeWarGroups(id, func(war *retrieval.War, sides aggregate.WarSides) error {
		if group.Type != types.CORPORATION.String() && group.Type != types.ALLIANCE.String() || group.ID <= 0 {
			return errors.New("Only a corporation or an alliance can be ignored")
		}
		if err := warGroupFree(war, sides, group); err != nil {
			return err
		}
		if !slices.ContainsFunc(war.Ignored, func(ignored retrieval.WarAddedGroup) bool {
			return ignored.Type == group.Type && ignored.ID == group.ID
		}) {
			war.Ignored = append(war.Ignored, retrieval.WarAddedGroup{Type: group.Type, ID: group.ID, Name: warGroupName(group)})
		}
		return nil
	})
	if err != nil {
		return report, err
	}
	a.lastReport, a.lastTarget = nil, nil
	return a.buildWarReport(war, sides), nil
}

// warGroupFree refuses a group already on one of a war's sides, listed or added
func warGroupFree(war *retrieval.War, sides aggregate.WarSides, group WarAddedCard) error {
	sameGroup := func(type_ string, groupID int) bool { return type_ == group.Type && groupID == group.ID }
	for _, listed := range slices.Concat(sides.A, sides.B) {
		if sameGroup(listed.Type.String(), listed.ID) {
			return fmt.Errorf("%s is already on a side", group.Name)
		}
	}
	for _, added := range slices.Concat(war.AddedA, war.AddedB) {
		if sameGroup(added.Type, added.ID) {
			return fmt.Errorf("%s was already added to a side", group.Name)
		}
	}
	return nil
}

// warGroupName is a group's name as sent, or one made from its id
func warGroupName(group WarAddedCard) string {
	if name := strings.TrimSpace(group.Name); name != "" {
		return name
	}
	return fmt.Sprintf("%s %d", strings.ToLower(group.Type), group.ID)
}

/*
	RemoveWarGroup takes a group added by AddWarGroup off its side, or one ignored by IgnoreWarGroup back into the report, and builds the report again from what the war holds. It takes a.mu.

Parameters:
  - id: the war
  - groupType: the group's Type
  - groupID: the group's ID

Returns:
  - WarReport: the report after the change
  - error: a user readable message if the war is gone
*/
func (a *App) RemoveWarGroup(id int, groupType string, groupID int) (report WarReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	if a.closing.Load() {
		return report, errAppClosing
	}
	war, sides, err := a.changeWarGroups(id, func(war *retrieval.War, sides aggregate.WarSides) error {
		match := func(added retrieval.WarAddedGroup) bool { return added.Type == groupType && added.ID == groupID }
		war.AddedA = slices.DeleteFunc(war.AddedA, match)
		war.AddedB = slices.DeleteFunc(war.AddedB, match)
		war.Ignored = slices.DeleteFunc(war.Ignored, match)
		return nil
	})
	if err != nil {
		return report, err
	}
	a.lastReport, a.lastTarget = nil, nil
	return a.buildWarReport(war, sides), nil
}

// changeWarGroups applies a change to a saved war's added groups, one at a time as it reads then saves, and returns it with its sides as they are now. Nothing is saved if change refuses.
func (a *App) changeWarGroups(id int, change func(*retrieval.War, aggregate.WarSides) error) (retrieval.War, aggregate.WarSides, error) {
	a.warMu.Lock()
	defer a.warMu.Unlock()

	war, found := a.cache.WarByID(id)
	if !found {
		return war, aggregate.WarSides{}, errNoWar
	}
	sides, err := a.warSides(war)
	if err != nil {
		return war, sides, err
	}
	if err := change(&war, sides); err != nil {
		return war, sides, err
	}
	war = a.cache.SaveWar(war)
	sides.AddedA, sides.AddedB, sides.Ignored = war.AddedA, war.AddedB, war.Ignored
	a.saveCacheQuietly()
	return war, sides, nil
}

// warMeasures converts a side's measures, with its efficiency from the ISK lost on both sides
func warMeasures(measures aggregate.WarMeasures, efficiency float64) WarMeasures {
	converted := WarMeasures{Killmails: measures.Killmails, ISKLost: measures.ISKLost, ISKFights: measures.ISKFights, ISKTrash: measures.ISKTrash,
		ISKStructures: measures.ISKStructures, ShipsLost: measures.ShipsLost, ShipsFights: measures.ShipsFights, ShipsTrash: measures.ShipsTrash,
		ISKTrashDefinite: measures.ISKTrashDefinite, ShipsTrashDefinite: measures.ShipsTrashDefinite,
		Citadels: []WarStructureCount{}, POS: measures.POS, StationRefs: measures.StationRefs, RefAttempts: measures.RefAttempts, RefAttemptsUnsure: measures.RefAttemptsUnsure, Efficiency: efficiency}
	for _, citadel := range measures.Citadels {
		count := WarStructureCount{Count: citadel.Count}
		if citadel.Type != nil {
			count.Name, count.ImageURL = citadel.Type.Name, api.TypeImageURL(citadel.Type.ItemID)
		}
		converted.Citadels = append(converted.Citadels, count)
		converted.CitadelCount += citadel.Count
	}
	return converted
}

// warGroupRef is a group's EntityRef, nil for none
func warGroupRef(group *aggregate.WarGroup) *EntityRef {
	if group == nil {
		return nil
	}
	return entityRef(searchTypes[group.Type], group.ID, group.Name)
}

func warBattleSide(side aggregate.WarBattleSide) WarBattleSide {
	converted := WarBattleSide{Group: warGroupRef(side.Group), Pilots: side.Pilots, ISKLost: side.ISKLost, ShipsLost: side.ShipsLost, Structures: side.StructureLost, StructureKinds: []WarStructureKind{},
		TrashKillmails: side.TrashKillmails, TrashISK: side.TrashISK, TrashShips: side.TrashShips, TrashDefiniteISK: side.TrashDefiniteISK, TrashDefiniteShips: side.TrashDefiniteShips}
	for _, kind := range side.StructureKinds {
		converted.StructureKinds = append(converted.StructureKinds, WarStructureKind{Name: kind.Name, Count: kind.Count})
	}
	return converted
}

func warSystemSide(side aggregate.WarSystemSide) WarSystemSide {
	return WarSystemSide{Killmails: side.Killmails, ISKLost: side.ISKLost}
}

// warSideName is a killmail's side as the frontend names it
func warSideName(side aggregate.WarSideKey) string {
	switch side {
	case aggregate.WarSideA:
		return "a"
	case aggregate.WarSideB:
		return "b"
	}
	return "others"
}

/*
	buildWarReport builds a war's report from the killmails it holds within its dates, expanding each through the cache. The caller holds a.mu.

Parameters:
  - war: the war as saved
  - sides: its sides as they are now
*/
func (a *App) buildWarReport(war retrieval.War, sides aggregate.WarSides) WarReport {
	held := []retrieval.RawKillmail{}
	for _, raw := range a.cache.WarKillmails(war.ID) {
		if raw.ESIKillmail != nil && raw.Zkillmail != nil && war.Within(raw.ESIKillmail.KillmailTime) {
			held = append(held, raw)
		}
	}
	a.cache.PrefetchKillmailEntities(held)
	expanded := make(map[int]*retrieval.Killmail, len(held))
	for index := range held {
		if index%200 == 0 {
			comm.ReportProgress("Reading killmail %d/%d", index+1, len(held))
		}
		if km := held[index].Expand(&a.clients.ESIClient, a.cache); km != nil {
			expanded[km.KillmailID] = km
		}
	}
	comm.ReportProgress("Working out the war's figures")
	data := aggregate.BuildWar(held, expanded, sides, battleMinPilots, a.cache.WarEditsOf(war.ID))
	a.lastWar = &builtWar{id: war.ID, data: data}

	efficiency := func(other, own float64) float64 {
		if total := other + own; total > 0 {
			return other / total * 100
		}
		return 0
	}
	report := WarReport{
		War:            a.warCard(war),
		A:              warMeasures(data.A, efficiency(data.B.ISKLost, data.A.ISKLost)),
		B:              warMeasures(data.B, efficiency(data.A.ISKLost, data.B.ISKLost)),
		Others:         warMeasures(data.Others, 0),
		Battles:        []WarBattle{},
		BattleCount:    data.BattleCount,
		Skirmishes:     data.Skirmishes,
		IgnoredBattles: data.IgnoredBattles,
		Classes:        []WarClassCount{},
		Systems:        []WarSystem{},
		Killmails:      []WarKillmail{},
		OtherGroups:    []WarOtherGroup{},
		Days:           []WarDay{},
		Reach:          []WarReachLine{},
		FullRunDue:     war.RunKey != war.RunKeyFor(sides.A, sides.B),
	}
	for _, reach := range war.Reach {
		if !reach.Complete {
			report.Reach = append(report.Reach, WarReachLine{Group: reach.Group, Oldest: reach.Oldest})
		}
	}

	for _, day := range data.Days {
		report.Days = append(report.Days, WarDay{Date: day.Date,
			A: warMeasures(day.A, efficiency(day.B.ISKLost, day.A.ISKLost)),
			B: warMeasures(day.B, efficiency(day.A.ISKLost, day.B.ISKLost))})
	}

	for _, battle := range data.Battles {
		row := WarBattle{ID: battle.ID, Start: battle.Start, End: battle.End, System: structureSystem(battle.System), OtherSystems: append([]string{}, battle.OtherSystems...),
			OtherSystemReports: warOtherSystemReports(battle),
			Class:              warClass(battle.System), Pilots: battle.Pilots, Killmails: len(battle.Killmails), Skirmish: battle.Skirmish, Ignored: battle.Ignored,
			A: warBattleSide(battle.A), B: warBattleSide(battle.B), Others: warBattleSide(battle.Others), StationRefs: []WarStationRef{}, Edits: []WarEditMark{}}
		for _, edit := range battle.Edits {
			row.Edits = append(row.Edits, WarEditMark(edit))
		}
		for _, ref := range battle.StationRefs {
			mark := WarStationRef{Side: warSideName(ref.Side), Kind: ref.Kind, Destroyed: ref.Destroyed, Attempt: ref.Attempt, Unsure: ref.Unsure, Structure: ref.Structure, StructureAt: ref.DiedAt, Notes: []WarRefNote{}}
			for _, note := range ref.Notes {
				mark.Notes = append(mark.Notes, WarRefNote(note))
			}
			row.StationRefs = append(row.StationRefs, mark)
		}
		if battle.System != nil {
			first, last := warSystemSpans(battle)
			start, end := battle.Start, battle.End
			if at, seen := first[battle.System.SystemID]; seen {
				start, end = at, last[battle.System.SystemID]
			}
			middle := start.Add(end.Sub(start) / 2)
			row.ZkillURL = api.ZkillRelatedURL(battle.System.SystemID, middle)
			row.EveToolsURL = api.EveToolsRelatedURL(battle.System.SystemID, middle)
			row.WarbeaconURL = api.WarbeaconRelatedURL(battle.System.SystemID, middle)
		}
		report.Battles = append(report.Battles, row)
	}

	classes := map[string]int{}
	for _, system := range data.Systems {
		if system.System == nil {
			continue
		}
		class := warClass(system.System)
		classes[class]++
		report.Systems = append(report.Systems, WarSystem{System: structureSystem(system.System), Class: class, Battles: system.Battles,
			A: warSystemSide(system.A), B: warSystemSide(system.B), Others: warSystemSide(system.Others)})
	}
	report.SystemCount = len(report.Systems)
	for class, count := range classes {
		report.Classes = append(report.Classes, WarClassCount{Class: class, Count: count})
	}
	slices.SortFunc(report.Classes, func(x, y WarClassCount) int {
		if x.Count != y.Count {
			return y.Count - x.Count
		}
		return strings.Compare(x.Class, y.Class)
	})

	for _, km := range data.Killmails {
		report.Killmails = append(report.Killmails, WarKillmail{ID: km.Killmail.KillmailID, Side: warSideName(km.VictimSide), BattleID: km.BattleID})
	}
	report.KillmailCount = len(report.Killmails)

	report.NPCPilots = WarNPCPilots{Killmails: data.NPCPilots.Killmails, ISKLost: data.NPCPilots.ISKLost, ShipsLost: data.NPCPilots.ShipsLost}
	for _, other := range data.OtherGroups {
		group := other.Group
		report.OtherGroups = append(report.OtherGroups, WarOtherGroup{Group: *warGroupRef(&group), Killmails: other.Measures.Killmails,
			ISKLost: other.Measures.ISKLost, ShipsLost: other.Measures.ShipsLost})
	}
	return report
}

/*
	warResult is a war search's result, made from the cache with no ESI call: a war is not an EVE entity. The caller must hold a.mu.
	The report itself comes from OpenWar or CollectWar.

Parameters:
  - war: the war searched
*/
func (a *App) warResult(war retrieval.War) TargetResult {
	card := a.warCard(war)
	comm.ReportProgress("Found war %s", war.Name)
	a.cache.AddRecentSearch(retrieval.RecentSearch{Name: war.Name, Type: warType, ID: war.ID}, a.currentSettings().MaxRecentSearches)

	result := TargetResult{ID: war.ID, Type: warType, Display: api.TargetDisplay{Name: war.Name}, War: &card}
	a.noteDebugSearch(func(search *debugSearch) {
		search.TargetID, search.TargetType, search.TargetName = result.ID, result.Type, war.Name
	})
	return result
}

// searchWar finds a war by its id, else its name, for SearchTarget
func (a *App) searchWar(name string, id int) (TargetResult, error) {
	var war retrieval.War
	var found bool
	if id != 0 {
		war, found = a.cache.WarByID(id)
	} else {
		war, found = a.cache.WarNamed(name)
	}
	if !found {
		return TargetResult{}, errNoWar
	}
	return a.warResult(war), nil
}

// warSystemSpans is the first and last killmail of a battle in each of its systems, by system id
func warSystemSpans(battle *aggregate.WarBattle) (first, last map[int]time.Time) {
	first, last = map[int]time.Time{}, map[int]time.Time{}
	for _, km := range battle.Killmails {
		if km.System == nil {
			continue
		}
		id := km.System.SystemID
		if at, seen := first[id]; !seen || km.KillmailTime.Before(at) {
			first[id] = km.KillmailTime
		}
		if at, seen := last[id]; !seen || km.KillmailTime.After(at) {
			last[id] = km.KillmailTime
		}
	}
	return first, last
}

// warOtherSystemReports is the battle report links of each of a battle's other systems, each at the hour of the middle of its own killmails there, as a long battle can leave a system hours before it ends
func warOtherSystemReports(battle *aggregate.WarBattle) []WarSystemReport {
	first, last := warSystemSpans(battle)
	out := make([]WarSystemReport, 0, len(battle.OtherDetails))
	for _, system := range battle.OtherDetails {
		start, end := battle.Start, battle.End
		if at, seen := first[system.SystemID]; seen {
			start, end = at, last[system.SystemID]
		}
		middle := start.Add(end.Sub(start) / 2)
		out = append(out, WarSystemReport{Name: system.Name, ZkillURL: api.ZkillRelatedURL(system.SystemID, middle),
			EveToolsURL: api.EveToolsRelatedURL(system.SystemID, middle), WarbeaconURL: api.WarbeaconRelatedURL(system.SystemID, middle)})
	}
	return out
}
