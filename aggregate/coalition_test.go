package aggregate_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCoalitionTarget(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	alpha := &retrieval.Alliance{AllianceID: 99000001, Name: "Alpha"}
	alphaCorp := &retrieval.Corporation{CorporationID: 98000001, Name: "Alpha Corp"}
	beta := &retrieval.Corporation{CorporationID: 98000002, Name: "Beta Corp"}
	affiliate := &retrieval.Corporation{CorporationID: 98000009, Name: "Alpha Holdings"}
	outsider := &retrieval.Corporation{CorporationID: 98000777, Name: "Outsider"}
	friends := &retrieval.Alliance{AllianceID: 99000003, Name: "Friends"}
	friendCorp := &retrieval.Corporation{CorporationID: 98000003, Name: "Friend Corp"}

	// no Expanded, as a coalition has none
	target := &retrieval.IntelTarget{IDNumber: -1700000000000, TypeOfEntity: types.COALITION, Holdings: []retrieval.TargetGroup{
		{Type: types.ALLIANCE, ID: alpha.AllianceID, Member: true},
		{Type: types.CORPORATION, ID: beta.CorporationID, Member: true},
		{Type: types.CORPORATION, ID: affiliate.CorporationID},
	}}

	attacker := func(id int, corp *retrieval.Corporation, alliance *retrieval.Alliance) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{CharacterID: id}, CorpAtKill: corp, AllianceAtKill: alliance}
	}
	kill := func(victimCorp *retrieval.Corporation, victimAlliance *retrieval.Alliance, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{Victim: &retrieval.KillmailVictim{Pilot: &retrieval.Pilot{CharacterID: 50}, CorpAtKill: victimCorp, AllianceAtKill: victimAlliance}, Attackers: attackers}
	}

	t.Run("a member or affiliate losing is the target's loss", func(t *testing.T) {
		assert.True(t, aggregate.IsTargetLoss(target, kill(alphaCorp, alpha, attacker(1, outsider, nil))))
		assert.True(t, aggregate.IsTargetLoss(target, kill(beta, nil, attacker(1, outsider, nil))))
		assert.True(t, aggregate.IsTargetLoss(target, kill(affiliate, nil, attacker(1, outsider, nil))))
		assert.False(t, aggregate.IsTargetLoss(target, kill(outsider, nil, attacker(1, beta, nil))))
	})

	t.Run("one member killing another is internal, a loss and never a kill", func(t *testing.T) {
		internal := kill(beta, nil, attacker(1, alphaCorp, alpha), attacker(2, beta, nil))
		assert.True(t, aggregate.IsInternal(target, internal))
		assert.True(t, aggregate.IsTargetLoss(target, internal))

		assert.False(t, aggregate.IsInternal(target, kill(beta, nil, attacker(1, alphaCorp, alpha), attacker(2, outsider, nil))), "an outside attacker makes it a normal loss")
		assert.False(t, aggregate.IsInternal(target, kill(outsider, nil, attacker(1, beta, nil))), "a kill is not internal")

		allies := aggregate.GetTopAllies(10, []*retrieval.Killmail{internal}, target, nil, false)
		assert.Empty(t, allies.Top, "a loss counts for no ally")
	})

	t.Run("allies leave out every member and affiliate", func(t *testing.T) {
		killmails := []*retrieval.Killmail{kill(outsider, nil,
			attacker(1, alphaCorp, alpha), attacker(2, beta, nil), attacker(3, affiliate, nil), attacker(4, friendCorp, friends), attacker(5, outsider, nil))}
		ranking := aggregate.GetTopAllies(10, killmails, target, nil, false)

		found := []int{}
		for _, ally := range ranking.Top {
			found = append(found, ally.ID())
		}
		assert.ElementsMatch(t, []int{friends.AllianceID, outsider.CorporationID}, found)
	})

	t.Run("holdings kills together are not counted for a coalition", func(t *testing.T) {
		killmails := []*retrieval.Killmail{kill(outsider, nil, attacker(1, alphaCorp, alpha), attacker(2, affiliate, nil))}
		assert.Empty(t, aggregate.HoldingKillsTogether(killmails, target, target.Holdings))
	})

	t.Run("the group helpers match members with no expanded target to assert on", func(t *testing.T) {
		assert.True(t, aggregate.IsSameCorp(target, beta.CorporationID))
		assert.False(t, aggregate.IsSameCorp(target, outsider.CorporationID))
		assert.False(t, aggregate.IsSameCorp(target, target.IDNumber))
		assert.True(t, aggregate.IsSameGroupAsTarget(target, retrieval.Pilot{Corporation: &retrieval.Corporation{CorporationID: 98000555, Alliance: alpha}}))
		assert.True(t, aggregate.IsSameGroupAsTarget(target, *affiliate))
		assert.False(t, aggregate.IsSameGroupAsTarget(target, *outsider))

		inMember := &retrieval.Pilot{Corporation: &retrieval.Corporation{CorporationID: beta.CorporationID}}
		gone := &retrieval.Pilot{Corporation: outsider}
		assert.False(t, aggregate.HasLeftTarget(inMember, target))
		assert.True(t, aggregate.HasLeftTarget(gone, target))
		assert.False(t, aggregate.HasLeftTarget(&retrieval.Pilot{}, target), "a pilot whose corp is not known has not left")

		member := aggregate.PilotSeen{CorporationsAtKill: map[int]struct{}{beta.CorporationID: {}}}
		assert.True(t, member.FlewFor(target))
		assert.False(t, aggregate.PilotSeen{CorporationsAtKill: map[int]struct{}{outsider.CorporationID: {}}}.FlewFor(target))
	})
}
