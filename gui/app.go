package gui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// progressEvent is the Wails event name the frontend listens on for comm.ReportProgress status updates
const progressEvent = "progress"

// App holds the backend state shared by every call from the frontend. Its exported methods are bound to the frontend by Wails, as window.go.gui.App.<Method>.
type App struct {
	ctx          context.Context
	mu           sync.Mutex // Wails runs each frontend call in its own goroutine, one search runs at a time
	clients      retrieval.CollectionClientPool
	cache        *retrieval.CachedEveIDs
	sde          api.SDEClient
	sdeCheckedAt time.Time

	promptAnswers chan bool // AnswerPrompt hands the frontend's answer to the search waiting in ask

	// StopSearch sets stopRequested and halts esiGuard, which every ESI request goes through, so the running search sends nothing more
	esiGuard      *api.RateGuard
	stopRequested atomic.Bool

	// settings have their own lock, so changing the theme never waits behind a running search
	settingsMu   sync.Mutex
	settings     Settings
	settingsPath string

	// the Members tab's list for the last corp or alliance searched, with its own lock so paging through it never waits behind a search
	rosterMu sync.Mutex
	roster   *roster

	// the last pilot, corp, or alliance search, so SaveIgnoredAllies can rank its allies again. Guarded by mu.
	lastAllies *alliesSource
}

// sdeCheckInterval is the least time between checks for a new SDE release, which is what expires cached ESI data
const sdeCheckInterval = time.Hour

// TargetResult is the frontend representation of a searched target
type TargetResult struct {
	ID       int               `json:"id"`
	Type     string            `json:"type"`
	Display  api.TargetDisplay `json:"display"`
	Holdings *Holdings         `json:"holdings"` // a corp or alliance target's holding links, nil for pilots and systems
	// the holding that was searched for, when the search went to its main instead, empty otherwise
	SearchedHolding string `json:"searchedHolding,omitempty"`
}

// IntelReport holds everything built from a target's killmails. New killmail based sections are added here so a single pull feeds them all.
type IntelReport struct {
	Killmails    []KillmailCard       `json:"killmails"`
	Stopped      bool                 `json:"stopped"`     // the user stopped the search, so the report is built from what was collected by then
	RateLimited  bool                 `json:"rateLimited"` // the ESI rate limited the search, so the report is built from the killmails collected before it
	LabelNames   map[string]string    `json:"labelNames"`  // names for the killmails' zKillboard labels that need a lookup, ie cat:6 is Ship. Other labels the frontend names itself.
	Fleetmates   []Fleetmate          `json:"fleetmates"`
	Members      []Fleetmate          `json:"members"`      // a corp or alliance target's most active pilots, shown the same way as fleetmates
	Pilots       []Fleetmate          `json:"pilots"`       // a system target's most active pilots
	Corporations []CorporationSummary `json:"corporations"` // a system target's most active corporations
	Allies       []AllySummary        `json:"allies"`       // a pilot, corp, or alliance target's most common alliances and corps on its kills, outside its own alliance
	// the allies the user chose to leave out of this target's Allies, see SaveIgnoredAllies
	IgnoredAllies []IgnoredAllySummary `json:"ignoredAllies"`
	// how many allies the target also killed, which the Hide killed allies preference leaves out of Allies
	KilledAllies int `json:"killedAllies"`
	// a corp or alliance target's holdings, with the kills each shared with it, for the line at the top of Allies
	Holdings   []HoldingActivity `json:"holdings"`
	Systems    []SystemSummary   `json:"systems"`
	Capitals   []CapitalShip     `json:"capitals"`   // a corp or alliance target's capital ships and the members who flew them
	PIPilots   []PIPilot         `json:"piPilots"`   // a system target's pilots who lost haulers near its planets, or ships with PI aboard
	Ratters    []Ratter          `json:"ratters"`    // a system target's pilots who lost ships with NPCs on the killmail
	MemberList bool              `json:"memberList"` // a corp or alliance target has a Members tab, its rows come from GetMembers
	// a corp or alliance target's Structures tab: the structures seen on its kills, and those it took part in destroying
	Structures *StructuresReport `json:"structures"`
	// a j-space system target's newest upwell structure or control tower lost, however long ago, shown above its Activity. nil for other targets, or when it has none.
	LastStructureLoss *StructureLoss `json:"lastStructureLoss"`
	JSpace            bool           `json:"jspace"` // a j-space system target, which has the last structure lost box, even empty
	// the structures lost in a j-space system target besides its last, minor ones too, newest first, for the collapsed list in its box
	PreviousStructureLosses []StructureLoss `json:"previousStructureLosses"`
	Sources                 string          `json:"sources"` // where the search's data came from, ie "9748 hits from cache, 1763 from the ESI; zKillboard live", for the frontend's summary line
}

// CapitalShip is a capital ship type a corp or alliance target's pilots flew, and who flew it
type CapitalShip struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	ImageURL string         `json:"imageUrl"`
	Pilots   []CapitalPilot `json:"pilots"` // by name
}

// CapitalPilot is a pilot who flew a capital for the target, with their current corp and alliance
type CapitalPilot struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Corporation     string `json:"corporation"` // current, empty when not known
	Alliance        string `json:"alliance"`    // current, empty when in none
	CorporationIcon string `json:"corporationIcon"`
	AllianceIcon    string `json:"allianceIcon"`
	Left            bool   `json:"left"` // no longer in the target's alliance, see aggregate.HasLeftTarget
	ZkillURL        string `json:"zkillUrl"`
	EveWhoURL       string `json:"evewhoUrl"`
}

// SystemSummary is the frontend representation of a system the target's killmails happened in
type SystemSummary struct {
	ID             int                `json:"id"`
	Name           string             `json:"name"`
	SecurityStatus float64            `json:"securityStatus"`
	SecurityBand   string             `json:"securityBand"` // high, low, or null, see api.SecurityBand
	Region         string             `json:"region"`
	Class          string             `json:"class"`   // wormhole class for j-space systems, empty elsewhere
	Statics        []api.SystemStatic `json:"statics"` // known statics for j-space systems
	ZkillURL       string             `json:"zkillUrl"`
	DotlanURL      string             `json:"dotlanUrl"`
	Killmails      int                `json:"killmails"`
	Kills          int                `json:"kills"`
	Losses         int                `json:"losses"`
	UniqueDays     int                `json:"uniqueDays"`
	FirstSeen      time.Time          `json:"firstSeen"`
	LastSeen       time.Time          `json:"lastSeen"`
}

// KillmailCard is the frontend summary of one killmail, from the target's point of view
type KillmailCard struct {
	ID                int       `json:"id"`
	URL               string    `json:"url"`
	Time              time.Time `json:"time"`
	Outcome           string    `json:"outcome"` // kill, loss, or internal (the target's own killed it, see aggregate.IsInternal) from the target's side, empty for a system target
	Solo              bool      `json:"solo"`
	TotalValue        float64   `json:"totalValue"`
	DroppedValue      float64   `json:"droppedValue"` // value of what actually dropped from the victim
	Labels            []string  `json:"labels"`       // zKillboard's tags, ie tz:eu, loc:w-space, shown as chips
	SystemName        string    `json:"systemName"`
	SecurityStatus    float64   `json:"securityStatus"`
	SecurityBand      string    `json:"securityBand"` // high, low, or null, see api.SecurityBand
	Region            string    `json:"region"`
	Class             string    `json:"class"` // wormhole class for j-space systems, empty elsewhere
	ShipName          string    `json:"shipName"`
	ShipImageURL      string    `json:"shipImageUrl"`
	VictimName        string    `json:"victimName"`
	VictimCorp        string    `json:"victimCorp"`
	VictimAlliance    string    `json:"victimAlliance"`
	FinalBlowName     string    `json:"finalBlowName"`
	FinalBlowCorp     string    `json:"finalBlowCorp"`
	FinalBlowAlliance string    `json:"finalBlowAlliance"`
	// small logos for the corp and alliance names, empty when there is no corp or alliance
	VictimCorpIcon        string `json:"victimCorpIcon"`
	VictimAllianceIcon    string `json:"victimAllianceIcon"`
	FinalBlowCorpIcon     string `json:"finalBlowCorpIcon"`
	FinalBlowAllianceIcon string `json:"finalBlowAllianceIcon"`
	AttackerCount         int    `json:"attackerCount"`
	Via                   string `json:"via,omitempty"` // the holding whose killmail list it came from, empty when it is on the target's own
}

// CorporationSummary is the frontend representation of a corporation active on the target's killmails
type CorporationSummary struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	ImageURL     string    `json:"imageUrl"`
	Alliance     string    `json:"alliance"`     // the corp's alliance on its most recent killmail
	AllianceIcon string    `json:"allianceIcon"` // a small logo for the alliance, empty when there is none
	ZkillURL     string    `json:"zkillUrl"`
	EveWhoURL    string    `json:"evewhoUrl"`
	Killmails    int       `json:"killmails"`
	Kills        int       `json:"kills"`
	Losses       int       `json:"losses"`
	Pilots       int       `json:"pilots"`
	UniqueDays   int       `json:"uniqueDays"`
	FirstSeen    time.Time `json:"firstSeen"`
	LastSeen     time.Time `json:"lastSeen"`
}

// Fleetmate is the frontend representation of a pilot frequently seen on killmails with the target
type Fleetmate struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	ImageURL        string `json:"imageUrl"`
	KillmailsShared int    `json:"killmailsShared"`
	UniqueDays      int    `json:"uniqueDays"`
	Corporation     string `json:"corporation"`
	Alliance        string `json:"alliance"`
	CorporationIcon string `json:"corporationIcon"` // small logos for the corp and alliance names, empty when there is none
	AllianceIcon    string `json:"allianceIcon"`
	ZkillURL        string `json:"zkillUrl"`
	EveWhoURL       string `json:"evewhoUrl"`
	// every collected killmail involves the target, so the pilot's oldest and latest killmails are the oldest and latest shared ones
	FirstTogether time.Time `json:"firstTogether"`
	LastTogether  time.Time `json:"lastTogether"`
}

// searchTypes are the entity types the frontend search dropdown can ask for, keyed by their EntityType String
var searchTypes = map[string]types.EntityType{
	types.PILOT.String():       types.PILOT,
	types.CORPORATION.String(): types.CORPORATION,
	types.ALLIANCE.String():    types.ALLIANCE,
	types.SYSTEM.String():      types.SYSTEM,
}

// fleetmateLimit is how many fleetmates the intel report ranks
const fleetmateLimit = 10

// memberLimit is how many members of a corp or alliance target the intel report ranks
const memberLimit = 10

// pilotLimit and corporationLimit are how many pilots and corporations a system target's intel report ranks
const (
	pilotLimit       = 10
	corporationLimit = 10
)

// systemLimit is how many systems the intel report ranks
const systemLimit = 10

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

/*
Lifecycle returns the app's startup and shutdown hooks, for the Wails options.
They are unexported methods handed over this way, because Wails offers every exported method of a bound struct to the frontend.
*/
func Lifecycle(a *App) (startup func(context.Context), shutdown func(context.Context)) {
	return a.startup, a.shutdown
}

// startup is called when the app starts. It loads static data and the ESI cache before the frontend can make calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	comm.InitLogger(logrus.InfoLevel)
	comm.SetProgressReporter(func(message string) {
		runtime.EventsEmit(a.ctx, progressEvent, message)
	})
	comm.SetRateLimitReporter(a.reportRateLimited)
	a.promptAnswers = make(chan bool, 1)

	if err := comm.InitStaticData(staticdata.Files, staticdata.Overrides()...); err != nil {
		comm.Logger.WithError(err).Error("Failed to load static data")
	}

	a.settingsPath = settingsPath()
	a.settings = loadSettings(a.settingsPath)

	esi := api.NewESIClient()
	a.esiGuard = esi.Guard()
	cache, err := retrieval.LoadCache(esi, cachePathFor(a.settings))
	if err != nil {
		comm.Logger.WithError(err).Warn("Starting with an empty cache")
	}
	a.cache = cache

	// every ESI call goes through the cache, which only reaches the ESI for data it does not have
	a.clients = retrieval.CollectionClientPool{
		ZkillClient:  api.NewZkillClient(),
		ESIClient:    cache,
		EveWhoClient: api.NewEveWhoClient(),
	}

	a.sde = api.NewSDEClient()
	a.checkSDERelease()
}

// checkSDERelease drops cached ESI data from before a newer SDE release, at most once per sdeCheckInterval. If the check fails the cache is kept as is.
func (a *App) checkSDERelease() {
	if time.Since(a.sdeCheckedAt) < sdeCheckInterval {
		return
	}
	a.sdeCheckedAt = time.Now()

	release, err := a.sde.GetLatestSDERelease()
	if err != nil {
		comm.Logger.WithError(err).Warn("Unable to check for a new SDE release, keeping cached ESI data")
		return
	}

	if dropped := a.cache.UpdateSDERelease(release); dropped > 0 {
		comm.ReportProgress("New EVE static data released %s, refreshing %d cached ESI entries as they are used", api.FormatShortDate(release), dropped)
		a.saveCache()
	}
}

// shutdown is called when the app closes, saving anything newly cached and closing the cache database
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.cache.Close(); err != nil {
		comm.Logger.WithError(err).Error("Failed to save and close the cache")
	}
}

// saveCache writes the cache to disk, logging rather than failing since the in memory cache is still valid
func (a *App) saveCache() {
	if err := retrieval.SaveCache(a.cache); err != nil {
		comm.Logger.WithError(err).Error("Failed to save cache")
	}
}

// lookupTarget finds a target by name and type, returning the user readable reason when it can not
func (a *App) lookupTarget(name string, entityType types.EntityType) (retrieval.IntelTarget, api.TargetEntity, error) {
	comm.ReportProgress("Looking up %s %s", strings.ToLower(entityType.String()), name)

	target := retrieval.IntelTarget{Name: name, Clients: a.clients}
	target.GetTargetInfoOfType(entityType)

	if target.Error != nil {
		comm.Logger.WithError(target.Error).Warnf("Search for %s failed", name)
		return target, nil, errors.New(target.DisplayMessage)
	}

	entity, ok := target.Data.(api.TargetEntity)
	if !ok {
		return target, nil, fmt.Errorf("No details available for %s", name)
	}
	return target, entity, nil
}

/*
holdingMain finds the main a corp or alliance target is a holding of, while the Include holdings preference is on.
With more than one, the first by name is used. A main that is itself a holding is not followed further.
*/
func (a *App) holdingMain(target *retrieval.IntelTarget) (retrieval.HoldingLink, bool) {
	if !a.GetSettings().IncludeHoldings || !canHold(target.TypeOfEntity.String()) {
		return retrieval.HoldingLink{}, false
	}
	_, heldFor := a.cache.HoldingLinks(target.TypeOfEntity.String(), target.IDNumber)
	if len(heldFor) == 0 {
		return retrieval.HoldingLink{}, false
	}
	if len(heldFor) > 1 {
		comm.ReportProgress("%s is a holding of %d mains, using %s", heldFor[0].HoldingName, len(heldFor), heldFor[0].MainName)
	}
	return heldFor[0], true
}

/*
	SearchTarget looks up a pilot, corporation, alliance, or system by name and builds its display values

Parameters:
  - name: the exact in game name of the target
  - searchType: PILOT, CORPORATION, ALLIANCE, or SYSTEM. Only a match of this type is accepted.

Returns:
  - TargetResult: the target's id, type, and display values
  - error: a user readable message if the target could not be found or loaded
*/
func (a *App) SearchTarget(name string, searchType string) (result TargetResult, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	// a new search starts unstopped, and the ESI is let through again once this part of it is done even if it was stopped
	a.stopRequested.Store(false)
	a.esiGuard.Resume()
	defer a.esiGuard.Resume()

	name = strings.TrimSpace(name)
	if name == "" {
		return result, errors.New("Enter a name to search for")
	}

	a.checkSDERelease()

	entityType, ok := searchTypes[searchType]
	if !ok {
		return result, fmt.Errorf("Unsupported search type %q", searchType)
	}

	lookupsBefore := a.cache.Counts()
	target, entity, err := a.lookupTarget(name, entityType)
	if err != nil {
		return result, err
	}

	// a holding is searched as its main, which collects the holding's killmails with its own
	searchedHolding := ""
	if main, found := a.holdingMain(&target); found {
		comm.ReportProgress("%s is a holding of %s, searching %s with its holdings", main.HoldingName, main.MainName, main.MainName)
		if mainTarget, mainEntity, mainErr := a.lookupTarget(main.MainName, searchTypes[main.MainType]); mainErr == nil {
			target, entity, searchedHolding = mainTarget, mainEntity, main.HoldingName
			searchType = main.MainType
		} else {
			comm.ReportProgress("Could not look up %s (%s), searching %s itself", main.MainName, mainErr, main.HoldingName)
		}
	}

	comm.ReportProgress("Loading %s details", strings.ToLower(target.TypeOfEntity.String()))

	display := entity.BuildDisplay(a.clients.ESIClient, a.clients.EveWhoClient)
	if target.TypeOfEntity != types.SYSTEM {
		display.TargetImageUrl = api.EntityImageURL(target.TypeOfEntity, target.IDNumber)
	} // a system's image is its star, set by BuildDisplay
	display.ZkillUrl = api.ZkillEntityURL(target.TypeOfEntity, target.IDNumber)
	if target.TypeOfEntity == types.SYSTEM {
		display.DotlanUrl = api.DotlanSystemURL(display.Name)
	} else {
		display.EveWhoUrl = api.EveWhoURL(target.TypeOfEntity, target.IDNumber)
	}

	sources := []string{}
	if source := a.cache.Counts().Since(lookupsBefore).Source(); source != "" {
		sources = append(sources, source)
	}
	if target.TypeOfEntity != types.SYSTEM {
		sources = append(sources, "EveWho live")
	}
	comm.ReportProgress("Found %s (%s)", display.Name, strings.Join(sources, "; "))
	foundName := display.Name
	if foundName == "" {
		foundName = name
	}
	a.cache.AddRecentSearch(retrieval.RecentSearch{Name: foundName, Type: searchType}, a.GetSettings().MaxRecentSearches)

	result = TargetResult{
		ID:              target.IDNumber,
		Type:            target.TypeOfEntity.String(),
		Display:         display,
		SearchedHolding: searchedHolding,
	}
	if canHold(result.Type) {
		holdings := a.holdingsFor(result.Type, result.ID)
		result.Holdings = &holdings
	}
	return result, nil
}

/*
	CollectIntel pulls a target's killmails and builds every killmail based section of the intel report from them. The frontend calls it right after a successful SearchTarget.

Parameters:
  - targetID: the eve id of the target
  - targetType: PILOT, CORPORATION, ALLIANCE, or SYSTEM, as returned in TargetResult.Type

Returns:
  - IntelReport: the killmail count and each report section. Sections that do not apply to the target type are left nil.
  - error: a user readable message if no killmails could be collected
*/
func (a *App) CollectIntel(targetID int, targetType string) (report IntelReport, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer recoverAsError(&err)

	// stopped between finding the target and collecting its intel
	if a.stopRequested.Load() {
		return report, errors.New("Search stopped")
	}
	defer a.esiGuard.Resume()

	lookupsBefore := a.cache.Counts()

	entityType, ok := searchTypes[targetType]
	if !ok {
		return report, fmt.Errorf("Unsupported target type %q", targetType)
	}

	target := retrieval.IntelTarget{
		IDNumber:     targetID,
		TypeOfEntity: entityType,
		Clients:      a.clients,
	}

	switch entityType {
	case types.PILOT:
		target.Expanded = a.cache.ExpandPilot(targetID, true)
	case types.CORPORATION:
		target.Expanded = a.cache.ExpandCorporation(targetID, true)
	case types.ALLIANCE:
		target.Expanded = a.cache.ExpandAlliance(targetID)
	case types.SYSTEM:
		target.Expanded = a.cache.ExpandSystem(targetID)
	}

	settings := a.GetSettings()
	if settings.IncludeHoldings {
		target.Holdings = a.targetHoldings(&target)
	}
	limits := settings.KillmailLimits(time.Now().UTC())
	limits.ConfirmIgnoreDate = a.confirmIgnoreDate(limits.NewerThan)
	killmails, collectErr := retrieval.CollectKillmails(target, limits)
	// the frontend has already shown the rate limit warning, the report is built from the killmails collected without the ESI
	rateLimited := errors.Is(collectErr, api.ErrRateLimited)
	if collectErr != nil {
		comm.Logger.WithError(collectErr).Warnf("Continuing with %d collected killmails", len(*killmails))
		if len(*killmails) == 0 {
			if a.stopRequested.Load() {
				a.saveCache()
				return report, errors.New("Search stopped before any killmails were collected")
			}
			if rateLimited {
				return report, errors.New("The ESI has limited your requests before any killmails were collected. You will need to wait 15 minutes before continuing.")
			}
			return report, fmt.Errorf("Could not retrieve killmails for this target: %s", api.FailureReason(collectErr))
		}
	}
	// the structure lists are pulled with the killmails, and everything on them looked up together
	structureRaw, expandStructures := a.collectStructureKillmails(target, limits)
	a.cache.PrefetchKillmailEntities(append(append([]retrieval.RawKillmail{}, *killmails...), structureRaw...))
	structures := expandStructures()

	collected := aggregate.CollateAllKillmailData(killmails, &a.clients.ESIClient, a.cache)
	report.Killmails = buildKillmailCards(collected.Killmails, &target)
	report.LabelNames = labelNames(report.Killmails, a.clients.ESIClient)

	// each target type gets its own sections: who flies with a pilot, who flies for a corp or alliance, and who is active in a system
	switch entityType {
	case types.PILOT:
		comm.ReportProgress("Ranking fleetmates")
		report.Fleetmates = buildFleetmates(aggregate.GetMostFrequentFleetmates(fleetmateLimit, collected.PilotsSeen, &target))
	case types.CORPORATION, types.ALLIANCE:
		comm.ReportProgress("Ranking members")
		report.Members = buildFleetmates(aggregate.GetTopMembers(memberLimit, collected.PilotsSeen, &target))
		comm.ReportProgress("Finding capital pilots")
		report.Capitals = buildCapitalShips(aggregate.GetCapitalPilots(collected.Killmails, &target), &target)
		memberCount := 0
		if entityType == types.CORPORATION {
			if corp, err := a.clients.ESIClient.GetCorporationInfo(targetID); err == nil && corp != nil {
				memberCount = corp.MemberCount
			}
		}
		a.setRoster(newRoster(targetID, entityType, collected.PilotsSeen, memberCount))
		report.MemberList = true
		comm.ReportProgress("Finding structures")
		report.Structures = buildStructures(structures, collected.Killmails, &target, a.clients.ESIClient)
	case types.SYSTEM:
		comm.ReportProgress("Ranking pilots and corporations")
		report.Pilots = buildFleetmates(aggregate.GetTopPilots(pilotLimit, collected.PilotsSeen))
		report.Corporations = buildCorporationSummaries(aggregate.GetTopCorporations(corporationLimit, collected.Killmails))
		comm.ReportProgress("Finding PI pilots")
		report.PIPilots = a.buildPIPilots(targetID, collected.Killmails)
		comm.ReportProgress("Finding ratters")
		report.Ratters = a.buildRatters(collected.Killmails)
		if system, _ := target.Expanded.(*retrieval.System); system != nil {
			report.JSpace = comm.WormholeClass(system.Name, system.Region) != ""
		}
		if structures.last != nil {
			loss := structureLoss(structures.last)
			report.LastStructureLoss = &loss
		}
		report.PreviousStructureLosses = []StructureLoss{}
		for _, km := range structures.previous {
			report.PreviousStructureLosses = append(report.PreviousStructureLosses, structureLoss(km))
		}
	}

	// a system target is the only system on its killmails, so it has no top systems, and no side to have allies on
	if entityType != types.SYSTEM {
		comm.ReportProgress("Ranking allies")
		a.lastAllies = &alliesSource{target: target, killmails: collected.Killmails}
		report.Allies, report.IgnoredAllies, report.KilledAllies = a.rankAllies(a.lastAllies)
		report.Holdings = a.holdingActivity(a.lastAllies)
		comm.ReportProgress("Ranking systems")
		report.Systems = buildSystemSummaries(aggregate.GetMostVisitedSystems(systemLimit, collected.SystemsVisted), collected.Killmails, &target)
	}

	// a killmail pull fills the cache with many new entries, save them now rather than only on shutdown
	comm.ReportProgress("Saving cache")
	a.saveCache()

	// stopped at any point, the ESI was refused from then on, so the report may be missing killmails and details not cached
	report.Stopped = a.stopRequested.Load()
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
	// the frontend adds this to its own summary line, with the search's total time
	report.Sources = strings.Join(sources, "; ")

	return report, nil
}

// setRoster replaces the Members tab's list, nil for a target with none
func (a *App) setRoster(members *roster) {
	a.rosterMu.Lock()
	defer a.rosterMu.Unlock()
	a.roster = members
}

/*
	labelNames looks up the names of the item categories zKillboard's cat: labels give by id, ie cat:6 is Ship

Parameters:
  - cards: the killmail cards, whose labels are checked
  - esi: looks up each category, ie the cache

Returns:
  - map[string]string: each cat: label to its category's name. A category that can not be looked up is left out, and the frontend shows its id.
*/
func labelNames(cards []KillmailCard, esi api.ESIClient) map[string]string {
	names := map[string]string{}
	for _, card := range cards {
		for _, label := range card.Labels {
			if _, done := names[label]; done {
				continue
			}
			value, isCategory := strings.CutPrefix(label, "cat:")
			categoryID, err := strconv.Atoi(value)
			if !isCategory || err != nil {
				continue
			}
			category, err := esi.GetCategoryInfo(categoryID)
			if err != nil || category == nil || category.Name == "" {
				comm.Logger.WithError(err).Warnf("Unable to look up item category %d for its zKillboard label", categoryID)
				continue
			}
			names[label] = category.Name
		}
	}
	return names
}

// buildCapitalShips turns the capital ship types and their pilots into the frontend's Capital Pilots list, with each pilot's current corp and alliance
func buildCapitalShips(capitals []aggregate.CapitalShipPilots, target *retrieval.IntelTarget) []CapitalShip {
	ships := []CapitalShip{}
	for _, capital := range capitals {
		ship := CapitalShip{ID: capital.Ship.ItemID, Name: capital.Ship.Name, ImageURL: api.TypeImageURL(capital.Ship.ItemID), Pilots: []CapitalPilot{}}
		for _, pilot := range capital.Pilots {
			capitalPilot := CapitalPilot{ID: pilot.CharacterID, Name: pilot.Name, Left: aggregate.HasLeftTarget(pilot, target),
				ZkillURL: api.ZkillEntityURL(types.PILOT, pilot.CharacterID), EveWhoURL: api.EveWhoURL(types.PILOT, pilot.CharacterID)}
			if corp := pilot.Corporation; corp != nil {
				capitalPilot.Corporation, capitalPilot.Alliance = groupNames(corp, corp.Alliance)
				capitalPilot.CorporationIcon, capitalPilot.AllianceIcon = groupIcons(corp, corp.Alliance)
			}
			ship.Pilots = append(ship.Pilots, capitalPilot)
		}
		ships = append(ships, ship)
	}
	return ships
}

// buildKillmailCards summarizes each expanded killmail for the frontend killmail list
func buildKillmailCards(killmails []*retrieval.Killmail, target *retrieval.IntelTarget) []KillmailCard {
	cards := []KillmailCard{}
	for _, km := range killmails {
		card := KillmailCard{
			ID:            km.KillmailID,
			URL:           api.ZkillKillmailURL(km.KillmailID),
			Time:          km.KillmailTime,
			Outcome:       killmailOutcome(target, km),
			Solo:          km.Solo,
			TotalValue:    km.TotalValue,
			DroppedValue:  km.DroppedValue,
			Labels:        km.Labels,
			AttackerCount: len(km.Attackers),
		}
		if km.Via != nil {
			card.Via = km.Via.Name
		}

		if km.System != nil {
			card.SystemName = km.System.Name
			card.SecurityStatus = km.System.SecurityStatus
			card.SecurityBand = api.SecurityBand(km.System.SecurityStatus)
			card.Region = km.System.Region
			card.Class = comm.WormholeClass(km.System.Name, km.System.Region)
		}

		victim := km.Victim
		if victim.Ship != nil {
			card.ShipName = victim.Ship.Name
			card.ShipImageURL = api.TypeImageURL(victim.Ship.ItemID)
		}
		card.VictimName = card.ShipName // structure and deployable losses have no pilot
		if victim.Pilot != nil {
			card.VictimName = victim.Pilot.Name
		}
		card.VictimCorp, card.VictimAlliance = groupNames(victim.CorpAtKill, victim.AllianceAtKill)
		card.VictimCorpIcon, card.VictimAllianceIcon = groupIcons(victim.CorpAtKill, victim.AllianceAtKill)

		for _, attacker := range km.Attackers {
			if !attacker.FinalBlow {
				continue
			}
			// npc and structure attackers have no pilot name, only their ship type
			if attacker.Pilot != nil && attacker.Pilot.Name != "" {
				card.FinalBlowName = attacker.Pilot.Name
			} else if attacker.Ship != nil {
				card.FinalBlowName = attacker.Ship.Name
			}
			card.FinalBlowCorp, card.FinalBlowAlliance = groupNames(attacker.CorpAtKill, attacker.AllianceAtKill)
			card.FinalBlowCorpIcon, card.FinalBlowAllianceIcon = groupIcons(attacker.CorpAtKill, attacker.AllianceAtKill)
		}

		cards = append(cards, card)
	}
	return cards
}

// killmailOutcome returns whether a killmail was a kill or a loss for the target, or an empty string for a system target, which has no side
func killmailOutcome(target *retrieval.IntelTarget, km *retrieval.Killmail) string {
	switch {
	case target.TypeOfEntity == types.SYSTEM:
		return ""
	case aggregate.IsInternal(target, km):
		return "internal"
	case aggregate.IsTargetLoss(target, km):
		return "loss"
	default:
		return "kill"
	}
}

// buildCorporationSummaries converts ranked corporations into their frontend representation
func buildCorporationSummaries(top []aggregate.CorporationActivity) []CorporationSummary {
	corporations := []CorporationSummary{}
	for _, activity := range top {
		summary := CorporationSummary{
			ID:         activity.Corporation.CorporationID,
			Name:       activity.Corporation.Name,
			ImageURL:   api.EntityImageURL(types.CORPORATION, activity.Corporation.CorporationID),
			ZkillURL:   api.ZkillEntityURL(types.CORPORATION, activity.Corporation.CorporationID),
			EveWhoURL:  api.EveWhoURL(types.CORPORATION, activity.Corporation.CorporationID),
			Killmails:  activity.Killmails,
			Kills:      activity.Kills,
			Losses:     activity.Losses,
			Pilots:     len(activity.PilotIDs),
			UniqueDays: activity.UniqueDays(),
			FirstSeen:  activity.FirstSeen(),
			LastSeen:   activity.LastSeen(),
		}
		if activity.Alliance != nil {
			summary.Alliance = activity.Alliance.Name
			_, summary.AllianceIcon = groupIcons(nil, activity.Alliance)
		}
		corporations = append(corporations, summary)
	}
	return corporations
}

// buildSystemSummaries converts ranked systems into their frontend representation, counting the target's kills and losses in each
func buildSystemSummaries(top []aggregate.SystemVisted, killmails []*retrieval.Killmail, target *retrieval.IntelTarget) []SystemSummary {
	kills, losses := map[int]int{}, map[int]int{}
	for _, km := range killmails {
		if km.System == nil {
			continue
		}
		if aggregate.IsTargetLoss(target, km) {
			losses[km.System.SystemID]++
		} else {
			kills[km.System.SystemID]++
		}
	}

	systems := []SystemSummary{}
	for _, visited := range top {
		if visited.System == nil {
			continue
		}
		systems = append(systems, SystemSummary{
			ID:             visited.System.SystemID,
			Name:           visited.System.Name,
			SecurityStatus: visited.System.SecurityStatus,
			SecurityBand:   api.SecurityBand(visited.System.SecurityStatus),
			Region:         visited.System.Region,
			Class:          comm.WormholeClass(visited.System.Name, visited.System.Region),
			Statics:        api.SystemStaticsFor(visited.System.Name),
			ZkillURL:       api.ZkillEntityURL(types.SYSTEM, visited.System.SystemID),
			DotlanURL:      api.DotlanSystemURL(visited.System.Name),
			Killmails:      visited.TotalKillmails,
			Kills:          kills[visited.System.SystemID],
			Losses:         losses[visited.System.SystemID],
			UniqueDays:     visited.UniqueDays(),
			FirstSeen:      visited.FirstSeen(),
			LastSeen:       visited.LastSeen(),
		})
	}
	return systems
}

// groupNames returns the corp and alliance names, empty for either that is nil
func groupNames(corp *retrieval.Corporation, alliance *retrieval.Alliance) (corpName string, allianceName string) {
	if corp != nil {
		corpName = corp.Name
	}
	if alliance != nil {
		allianceName = alliance.Name
	}
	return corpName, allianceName
}

// groupIcons returns small logo urls for a corp and alliance, empty for either that is nil
func groupIcons(corp *retrieval.Corporation, alliance *retrieval.Alliance) (corpIcon string, allianceIcon string) {
	if corp != nil {
		corpIcon = api.EntityIconURL(types.CORPORATION, corp.CorporationID)
	}
	if alliance != nil {
		allianceIcon = api.EntityIconURL(types.ALLIANCE, alliance.AllianceID)
	}
	return corpIcon, allianceIcon
}

// buildFleetmates converts ranked PilotSeen values into their frontend representation
func buildFleetmates(top []aggregate.PilotSeen) []Fleetmate {
	fleetmates := []Fleetmate{}
	for _, pilotSeen := range top {
		fleetmate := Fleetmate{
			ID:              pilotSeen.Pilot.CharacterID,
			Name:            pilotSeen.Pilot.Name,
			ImageURL:        api.EntityImageURL(types.PILOT, pilotSeen.Pilot.CharacterID),
			ZkillURL:        api.ZkillEntityURL(types.PILOT, pilotSeen.Pilot.CharacterID),
			EveWhoURL:       api.EveWhoURL(types.PILOT, pilotSeen.Pilot.CharacterID),
			KillmailsShared: pilotSeen.TotalKillmails,
			UniqueDays:      pilotSeen.UniqueDays,
			FirstTogether:   pilotSeen.FirstSeen,
			LastTogether:    pilotSeen.LastSeen,
		}
		if corp := pilotSeen.Pilot.Corporation; corp != nil {
			fleetmate.Corporation, fleetmate.Alliance = groupNames(corp, corp.Alliance)
			fleetmate.CorporationIcon, fleetmate.AllianceIcon = groupIcons(corp, corp.Alliance)
		}
		fleetmates = append(fleetmates, fleetmate)
	}
	return fleetmates
}

// recoverAsError turns a panic in a bound method into an error for the frontend, rather than crashing the app
func recoverAsError(err *error) {
	if r := recover(); r != nil {
		comm.Logger.Errorf("Recovered from panic: %v", r)
		comm.ReportProgress("Stopped, something went wrong: %v", r)
		*err = fmt.Errorf("Something went wrong: %v", r)
	}
}
