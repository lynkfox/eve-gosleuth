package api

import (
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
)

type TargetDisplay struct {
	Name              string
	Type              types.EntityType
	TargetImageUrl    string
	ZkillUrl          string
	EveWhoUrl         string             // empty for entities EveWho does not cover
	DotlanUrl         string             // a system's Dotlan map page, empty for other targets
	Related           []RelatedEntity    // shown beside the target, ie a pilot's corp and alliance
	CorpHistory       []CorpHistoryEntry // a pilot's corps newest first, empty for other targets or when EveWho is unavailable
	AllianceHistory   []HistoryEntry     // a corp's alliances newest first, empty for other targets, a corp never in one, or when the ESI is unavailable
	Statics           []SystemStatic     // a j-space system's statics, empty for other targets
	Stargates         []SystemLink       // the systems a k-space system's stargates lead to, empty for other targets
	MemberCorps       []MemberCorp       // an alliance's member corporations, largest first, empty for other targets
	DetailRows        []DetailRow        // the left details as label and value columns, used instead of LeftDetailsValue when set
	SpaceLabel        string             // a system's kind of space shown beside its name: a wormhole class such as C5, or High Sec, Low Sec, Null Sec, or Triglavian
	LeftDetailsTitle  string
	LeftDetailsValue  string
	RightDetailsTitle string
	RightDetailsValue string
}

// RelatedEntity is a pilot, corporation, or alliance shown beside the target, ie a pilot's corp, with its portrait and site links
type RelatedEntity struct {
	Title     string // what it is to the target, ie Corporation or Executor CEO
	Name      string
	ID        int
	Type      types.EntityType
	TypeName  string // Type as a string, ie CORPORATION, matching the frontend search types
	ImageUrl  string
	ZkillUrl  string
	EveWhoUrl string
}

// NewRelatedEntity builds a RelatedEntity with its portrait or logo, zKillboard, and EveWho urls
func NewRelatedEntity(title string, name string, idNumber int, typeOfEntity types.EntityType) RelatedEntity {
	return RelatedEntity{
		Title:     title,
		Name:      name,
		ID:        idNumber,
		Type:      typeOfEntity,
		TypeName:  typeOfEntity.String(),
		ImageUrl:  EntityImageURL(typeOfEntity, idNumber),
		ZkillUrl:  ZkillEntityURL(typeOfEntity, idNumber),
		EveWhoUrl: EveWhoURL(typeOfEntity, idNumber),
	}
}

// CorpHistoryEntry is one stint of a pilot in a corp
type CorpHistoryEntry struct {
	CorporationID int
	Name          string
	IconUrl       string
	Start         string // ie Jul 9th, 2026
	End           string // ie Jul 9th, 2026, or "present" for the pilot's current corp
}

// HistoryEntry is one stint of a corp in an alliance, shown the same way as a CorpHistoryEntry
type HistoryEntry struct {
	ID      int
	Name    string
	IconUrl string
	Start   string // ie Jul 9th, 2026
	End     string // ie Jul 9th, 2026, or "present" for the current one
}

// SystemStatic is one static wormhole of a j-space system
type SystemStatic struct {
	Code        string `json:"code"`        // ie H296
	Destination string `json:"destination"` // Highsec, Lowsec, Nullsec, or C1 - C6
}

// SystemStaticsFor returns the known statics of a j-space system, or an empty list for any other system
func SystemStaticsFor(systemName string) []SystemStatic {
	statics := []SystemStatic{}
	for _, static := range comm.JSpaceStatics[systemName] {
		statics = append(statics, SystemStatic{Code: static.Identification, Destination: static.Destination})
	}
	return statics
}

// SystemLink is a system a stargate leads to
// DetailRow is one labelled line of the target details, ie Region: The Forge
type DetailRow struct {
	Label    string
	Value    string
	Security *float64 // set when the value is a security status, so it can be colored
}

// MemberCorp is one of an alliance's member corporations
type MemberCorp struct {
	CorporationID int
	Name          string
	IconUrl       string
	MemberCount   int
}

type SystemLink struct {
	SystemID       int
	Name           string
	SecurityStatus float64
}
