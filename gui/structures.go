package gui

import (
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
)

// EntityRef is a pilot, corp, or alliance on a structure row, for its name, logo, and right click menu
type EntityRef struct {
	ID        int    `json:"id"`
	Type      string `json:"type"` // PILOT, CORPORATION, or ALLIANCE
	Name      string `json:"name"`
	IconURL   string `json:"iconUrl"`
	ZkillURL  string `json:"zkillUrl"`
	EveWhoURL string `json:"evewhoUrl"`
	Ticker    string `json:"ticker,omitempty"` // a corp's or alliance's ticker, where it is set, ie on My Intel's cards
}

// StructureDeath is a structure's loss: when, and who destroyed it
type StructureDeath struct {
	KillmailID     int        `json:"killmailId"`
	ZkillURL       string     `json:"zkillUrl"`
	Time           time.Time  `json:"time"`
	KillerCorp     *EntityRef `json:"killerCorp"`     // of the group with the most attackers, nil when no attacker had one
	KillerAlliance *EntityRef `json:"killerAlliance"` // nil when that group is a corp in no alliance
	Attackers      int        `json:"attackers"`
}

// StructureSummary is one of a corp or alliance target's structures seen on its kills, a row of the Structures tab's Used in Kills list
type StructureSummary struct {
	TypeID        int             `json:"typeId"` // 0 for a control tower seen only through its batteries
	Type          string          `json:"type"`
	ImageURL      string          `json:"imageUrl"`
	Battery       string          `json:"battery"` // a control tower's battery on its newest kill, empty for an upwell structure
	System        SystemSummary   `json:"system"`
	Owner         *EntityRef      `json:"owner"`
	OwnerAlliance *EntityRef      `json:"ownerAlliance"`
	Via           string          `json:"via"` // the holding owning it, empty for the target itself
	Kills         int             `json:"kills"`
	FirstSeen     time.Time       `json:"firstSeen"`
	LastSeen      time.Time       `json:"lastSeen"`
	ZkillURL      string          `json:"zkillUrl"`   // its newest kill
	KilledShip    string          `json:"killedShip"` // the ship it killed on its newest kill, empty when not known
	KilledShipURL string          `json:"killedShipImageUrl"`
	Destroyed     *StructureDeath `json:"destroyed"` // its loss since its newest kill, nil while it stands
}

// StructureLoss is a structure destroyed: a j-space system's last structure lost, or one in the Structures tab's Destroyed list
type StructureLoss struct {
	TypeID        int            `json:"typeId"`
	Type          string         `json:"type"`
	ImageURL      string         `json:"imageUrl"`
	RenderURL     string         `json:"renderUrl"` // 128px
	System        SystemSummary  `json:"system"`
	Owner         *EntityRef     `json:"owner"`
	OwnerAlliance *EntityRef     `json:"ownerAlliance"`
	Death         StructureDeath `json:"death"`
	Pilots        int            `json:"pilots"` // the target's pilots on it, for the Destroyed list
}

// StructuresReport is a corp or alliance target's Structures tab
type StructuresReport struct {
	Used      []StructureSummary `json:"used"`
	Destroyed []StructureLoss    `json:"destroyed"`
}

// structureKillmails are the expanded killmails the Structures tab and a j-space system's last structure lost are built from, collected with the target's killmails
type structureKillmails struct {
	used     []*retrieval.Killmail
	lost     []*retrieval.Killmail
	last     *retrieval.Killmail
	previous []*retrieval.Killmail // the structures lost before or besides last, minor ones too
}

/*
	collectStructureKillmails pulls what the structure sections need from zKillboard, before the killmails are expanded so they are prefetched together:
	for a corp or alliance its structures' kills and losses, for a j-space system its last structure lost. Other targets need none.

Returns:
  - []retrieval.RawKillmail: every structure killmail found, to prefetch with the target's
  - func() structureKillmails: expands them, once prefetched
*/
func (a *App) collectStructureKillmails(target retrieval.IntelTarget, limits retrieval.KillmailLimits) ([]retrieval.RawKillmail, func() structureKillmails) {
	none := func() structureKillmails { return structureKillmails{} }
	if a.stopRequested.Load() {
		return nil, none
	}
	expand := func(raw []retrieval.RawKillmail) []*retrieval.Killmail {
		expanded := []*retrieval.Killmail{}
		for index := range raw {
			if km := raw[index].Expand(&a.clients.ESIClient, a.cache); km != nil {
				expanded = append(expanded, km)
			}
		}
		return expanded
	}

	switch target.TypeOfEntity {
	case types.CORPORATION, types.ALLIANCE:
		found, err := retrieval.CollectStructureKillmails(target, limits)
		if err != nil {
			comm.Logger.WithError(err).Warn("Continuing with the structure killmails collected")
		}
		all := append(append([]retrieval.RawKillmail{}, found.Used...), found.Lost...)
		return all, func() structureKillmails {
			return structureKillmails{used: expand(found.Used), lost: expand(found.Lost)}
		}
	case types.SYSTEM:
		system, _ := target.Expanded.(*retrieval.System)
		if system == nil || comm.WormholeClass(system.Name, system.Region) == "" {
			return nil, none
		}
		last, previous, err := retrieval.StructureLosses(target)
		if err != nil {
			comm.Logger.WithError(err).Warn("Continuing with the structure losses found")
		}
		all := previous
		if last != nil {
			all = append([]retrieval.RawKillmail{*last}, previous...)
		}
		return all, func() structureKillmails {
			found := structureKillmails{previous: expand(previous)}
			if last != nil {
				if expanded := expand([]retrieval.RawKillmail{*last}); len(expanded) > 0 {
					found.last = expanded[0]
				}
			}
			return found
		}
	}
	return nil, none
}

// buildStructures builds a corp or alliance target's Structures tab, from its structures' kills and losses and the structures among its own killmails
func buildStructures(found structureKillmails, killmails []*retrieval.Killmail, target *retrieval.IntelTarget, esi api.ESIClient) *StructuresReport {
	report := &StructuresReport{Used: []StructureSummary{}, Destroyed: []StructureLoss{}}
	for _, use := range aggregate.StructuresUsed(found.used, found.lost, target, esi) {
		summary := StructureSummary{
			Type:      "Control Tower",
			System:    structureSystem(use.System),
			Kills:     use.Kills,
			FirstSeen: use.FirstSeen,
			LastSeen:  use.LastSeen,
			ZkillURL:  api.ZkillKillmailURL(use.LatestKill.KillmailID),
		}
		summary.Owner, summary.OwnerAlliance = groupRefs(use.Corporation, use.Alliance)
		if ship := use.LatestKill.Victim.Ship; ship != nil {
			summary.KilledShip, summary.KilledShipURL = ship.Name, api.TypeImageURL(ship.ItemID)
		}
		if use.Structure != nil {
			summary.TypeID, summary.Type, summary.ImageURL = use.Structure.ItemID, use.Structure.Name, api.TypeImageURL(use.Structure.ItemID)
		}
		if use.Battery != nil {
			summary.Battery = use.Battery.Name
			if summary.ImageURL == "" {
				summary.ImageURL = api.TypeImageURL(use.Battery.ItemID)
			}
		}
		if use.Via != nil {
			summary.Via = use.Via.Name
		}
		if use.Loss != nil {
			death := structureDeath(use.Loss)
			summary.Destroyed = &death
		}
		report.Used = append(report.Used, summary)
	}
	for _, kill := range aggregate.StructuresDestroyed(killmails, target) {
		loss := structureLoss(kill.Killmail)
		loss.Pilots = kill.Pilots
		report.Destroyed = append(report.Destroyed, loss)
	}
	return report
}

// structureLoss describes a structure killmail's victim and who destroyed it
func structureLoss(km *retrieval.Killmail) StructureLoss {
	loss := StructureLoss{System: structureSystem(km.System), Death: structureDeath(km)}
	if ship := km.Victim.Ship; ship != nil {
		loss.TypeID, loss.Type = ship.ItemID, ship.Name
		loss.ImageURL, loss.RenderURL = api.TypeImageURL(ship.ItemID), api.TypeRenderURL(ship.ItemID)
	}
	loss.Owner, loss.OwnerAlliance = groupRefs(km.Victim.CorpAtKill, km.Victim.AllianceAtKill)
	return loss
}

// structureDeath describes when a structure was destroyed and by whom
func structureDeath(km *retrieval.Killmail) StructureDeath {
	killer := aggregate.KillerOf(km)
	death := StructureDeath{KillmailID: km.KillmailID, ZkillURL: api.ZkillKillmailURL(km.KillmailID), Time: km.KillmailTime, Attackers: killer.Attackers}
	death.KillerCorp, death.KillerAlliance = groupRefs(killer.Corporation, killer.Alliance)
	return death
}

// structureSystem is the system a structure is in, as the Top Systems tab describes a system, without its counts
func structureSystem(system *retrieval.System) SystemSummary {
	if system == nil {
		return SystemSummary{Name: "(Unknown system)", Statics: []api.SystemStatic{}}
	}
	return SystemSummary{
		ID:             system.SystemID,
		Name:           system.Name,
		SecurityStatus: system.SecurityStatus,
		SecurityBand:   api.SecurityBand(system.SecurityStatus),
		Region:         system.Region,
		Class:          comm.WormholeClass(system.Name, system.Region),
		Statics:        api.SystemStaticsFor(system.Name),
		ZkillURL:       api.ZkillEntityURL(types.SYSTEM, system.SystemID),
		DotlanURL:      api.DotlanSystemURL(system.Name),
	}
}

// groupRefs are a corp's and alliance's EntityRefs, nil for either that is nil or has no id or name, as one not looked up yet
func groupRefs(corp *retrieval.Corporation, alliance *retrieval.Alliance) (*EntityRef, *EntityRef) {
	var corpRef, allianceRef *EntityRef
	if corp != nil && corp.CorporationID != 0 && corp.Name != "" {
		corpRef = entityRef(types.CORPORATION, corp.CorporationID, corp.Name)
	}
	if alliance != nil && alliance.AllianceID != 0 && alliance.Name != "" {
		allianceRef = entityRef(types.ALLIANCE, alliance.AllianceID, alliance.Name)
	}
	return corpRef, allianceRef
}

// pilotRef is a pilot's EntityRef, nil for a pilot with no id, ie an NPC, or no name, as one not looked up yet
func pilotRef(pilot *retrieval.Pilot) *EntityRef {
	if pilot == nil || pilot.CharacterID == 0 || pilot.Name == "" {
		return nil
	}
	return entityRef(types.PILOT, pilot.CharacterID, pilot.Name)
}

func entityRef(entityType types.EntityType, id int, name string) *EntityRef {
	return &EntityRef{ID: id, Type: entityType.String(), Name: name, IconURL: api.EntityIconURL(entityType, id),
		ZkillURL: api.ZkillEntityURL(entityType, id), EveWhoURL: api.EveWhoURL(entityType, id)}
}
