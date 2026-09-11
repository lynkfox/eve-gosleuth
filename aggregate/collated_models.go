package aggregate

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

type AggregateResults interface {
	CalculateDaysSeenOverTime() int
}

type CollectedData struct {
	PilotsSeen       map[int]PilotSeen
	SystemsVisted    map[int]SystemVisted
	ShipsFlown       map[int]ShipFlown
	BattlesFought    map[string]BattleFought
	CapitalPilots    map[int]CapitalPilot
	AlliancesSeen    map[int]AllianceSeen
	CorporationsSeen map[int]CorporationSeen
}

type AllianceSeen struct {
	Alliance       *retrieval.Alliance
	Pilots         map[int]*PilotSeen
	SystemsVisited map[int]*SystemVisted
	Activity       []time.Time
	TotalKillmails int
	KillmailIDs    map[int]struct{}

	BattlesFoughtIDs map[string]struct{}
}

type CorporationSeen struct {
	Corporation    *retrieval.Corporation
	ALliance       *retrieval.Alliance
	Pilots         map[int]*PilotSeen
	SystemsVisited map[int]*SystemVisted
	Activity       []time.Time
	TotalKillmails int
	KillmailIDs    map[int]struct{}

	BattlesFoughtIDs map[string]struct{}
}

type SystemVisted struct {
	System           *retrieval.System
	TotalKillmails   int
	KillmailIDs      []int
	Activity         []time.Time
	BattlesFoughtIDs map[string]struct{}
	StructuresSeen   map[string]*Structure // key is structureTypeID-corporationID
	StructuresBashed map[time.Time]Structure
}

type Structure struct {
	Structure        *retrieval.Item
	Fits             map[string][]*retrieval.FitItem
	TotalKillmails   int
	KillmailIDs      []int
	Activity         []time.Time
	BattlesFoughtIDs map[string]struct{}
	Destroyed        int
	Owner            string
}

type ShipFlown struct {
	Ship                *retrieval.Item
	PilotID             int
	Fits                map[string][]*retrieval.FitItem //key of KillmailID-shipID
	TotalKillmails      int
	Activity            []time.Time
	BattlesFoughtIDs    map[string]struct{}
	Destroyed           int
	AttackerKillmailIDs []int
	VictimKillmailIDs   []int
}

type BattleFought struct {
	BattleIDReference string // also used as the id in the CollectedData map. systemID/YYYYMMDDHH
	TotalKillmails    int
	SystemID          int
	UniquePilots      map[int]struct{}
	KillmailIDs       []int
}

type CapitalPilot struct {
	Pilot          string
	Corporation    string
	Alliance       string
	TotalKillmails int
	KillmailIDs    []int
	Activity       []time.Time
	Ships          map[int]*ShipFlown
}
