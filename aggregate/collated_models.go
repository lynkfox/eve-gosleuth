package aggregate

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

type AggregateResults interface {
	CalculateDaysSeenOverTime() int
}

type CollectedData struct {
	Killmails        []*retrieval.Killmail // every killmail that was expanded, in the order collected
	PilotsSeen       map[int]PilotSeen
	SystemsVisted    map[int]SystemVisted
	ShipsFlown       map[int]ShipFlown
	BattlesFought    map[string]BattleFought
	AlliancesSeen    map[int]AllianceSeen
	CorporationsSeen map[int]CorporationSeen
}

// AllianceSeen is an alliance whose pilots were on the killmails. Its pilots and systems are ids, looked up in CollectedData's PilotsSeen and SystemsVisted.
type AllianceSeen struct {
	Alliance       *retrieval.Alliance
	PilotIDs       map[int]struct{}
	SystemIDs      map[int]struct{}
	Activity       []time.Time // one per killmail
	TotalKillmails int
	KillmailIDs    map[int]struct{}

	BattlesFoughtIDs map[string]struct{}
}

// CorporationSeen is a corporation whose pilots were on the killmails. Its pilots and systems are ids, looked up in CollectedData's PilotsSeen and SystemsVisted.
type CorporationSeen struct {
	Corporation    *retrieval.Corporation
	PilotIDs       map[int]struct{}
	SystemIDs      map[int]struct{}
	Activity       []time.Time // one per killmail
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
	StructuresSeen   map[string]*Structure // key is structureTypeID-corporationID-allianceID, allianceID is 0 for a corp with no alliance
	StructuresBashed map[time.Time]Structure
}

// Structure is an upwell structure, control tower, or deployable seen on killmails in a system, owned by one corp and alliance combination
type Structure struct {
	Structure        *retrieval.Item
	Corporation      *retrieval.Corporation // owner at the time of the killmail
	Alliance         *retrieval.Alliance    // owner's alliance at the time of the killmail, nil if the corp had none
	Fits             map[string]*retrieval.Fit
	TotalKillmails   int
	KillmailIDs      []int
	Activity         []time.Time
	BattlesFoughtIDs map[string]struct{}
	Destroyed        int
}

type ShipFlown struct {
	Ship                *retrieval.Item
	PilotID             int
	Fits                map[string]*retrieval.Fit //key of KillmailID-shipID
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
