package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// ids for the war tests: A is alliance 1000 with corps 101 and 102, B is alliance 2000 with corp 201, the rest are third parties
const (
	allyA, allyB, allyC, allyD = 1000, 2000, 3000, 4000
	corpA1, corpA2, corpB1     = 101, 102, 201
	corpC1, corpD1, corpLone   = 301, 401, 501 // C is in allyC, D in allyD, Lone has no alliance
	npcCorp                    = 1000125
	rifter, capsule            = 587, 670
	astrahus, tower, customs   = 35832, 12235, 2233
)

var warStart = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// wp is a party on a killmail: a corp, an alliance, a character
type wp struct{ corp, ally, char int }

func (p wp) victim(ship int) api.ESIVictim {
	v := api.ESIVictim{ShipTypeID: ship}
	if p.corp != 0 {
		v.CorporationID = &p.corp
	}
	if p.ally != 0 {
		v.AllianceID = &p.ally
	}
	if p.char != 0 {
		v.CharacterID = &p.char
	}
	return v
}

func (p wp) attacker() api.ESIAttacker {
	a := api.ESIAttacker{ShipTypeID: rifter, DamageDone: 1}
	if p.corp != 0 {
		a.CorporationID = &p.corp
	}
	if p.ally != 0 {
		a.AllianceID = &p.ally
	}
	if p.char != 0 {
		a.CharacterID = &p.char
	}
	return a
}

var (
	pA1  = func(char int) wp { return wp{corpA1, allyA, char} }
	pA2  = func(char int) wp { return wp{corpA2, allyA, char} }
	pB   = func(char int) wp { return wp{corpB1, allyB, char} }
	pC   = func(char int) wp { return wp{corpC1, allyC, char} }
	pD   = func(char int) wp { return wp{corpD1, allyD, char} }
	pNPC = wp{corp: npcCorp}
)

const sysHome, sysOther = 30000001, 30000002

// raw builds a raw killmail the given minutes after warStart
func raw(id, minutes, system int, victim wp, ship int, attackers ...wp) retrieval.RawKillmail {
	esi := &api.ESIKillmail{KillmailID: id, KillmailTime: warStart.Add(time.Duration(minutes) * time.Minute), SolarSystemID: system, Victim: victim.victim(ship)}
	for _, a := range attackers {
		esi.Attackers = append(esi.Attackers, a.attacker())
	}
	return retrieval.RawKillmail{ESIKillmail: esi, Zkillmail: &api.ZkbResponse{KillmailID: id, KillmailTime: esi.KillmailTime, Zkb: api.Zkb{TotalValue: float64(id) * 100}}}
}

// expand builds the expanded form of raw killmails, with names for the groups
func expand(raws ...retrieval.RawKillmail) map[int]*retrieval.Killmail {
	names := map[int]string{allyA: "Alpha", allyB: "Bravo", allyC: "Charlie", allyD: "Delta", corpLone: "Lone Corp"}
	systems := map[int]*retrieval.System{sysHome: {SystemID: sysHome, Name: "Home"}, sysOther: {SystemID: sysOther, Name: "Other"}}
	party := func(corp, ally int) (*retrieval.Corporation, *retrieval.Alliance) {
		var c *retrieval.Corporation
		var a *retrieval.Alliance
		if corp != 0 {
			c = &retrieval.Corporation{CorporationID: corp, Name: "Corp"}
			if names[corp] != "" {
				c.Name = names[corp]
			}
		}
		if ally != 0 {
			a = &retrieval.Alliance{AllianceID: ally, Name: names[ally]}
		}
		return c, a
	}
	out := map[int]*retrieval.Killmail{}
	for _, r := range raws {
		esi := r.ESIKillmail
		km := &retrieval.Killmail{KillmailID: esi.KillmailID, KillmailTime: esi.KillmailTime, System: systems[esi.SolarSystemID], TotalValue: r.Zkillmail.Zkb.TotalValue}
		km.Victim = &retrieval.KillmailVictim{Ship: &retrieval.Item{ItemID: esi.Victim.ShipTypeID, Name: map[int]string{astrahus: "Astrahus", tower: "Tower", customs: "Customs Office", rifter: "Rifter"}[esi.Victim.ShipTypeID]}}
		if esi.Victim.CharacterID != nil {
			km.Victim.Pilot = &retrieval.Pilot{CharacterID: *esi.Victim.CharacterID}
		}
		km.Victim.CorpAtKill, km.Victim.AllianceAtKill = party(intp(esi.Victim.CorporationID), intp(esi.Victim.AllianceID))
		for _, a := range esi.Attackers {
			attacker := &retrieval.KillmailAttacker{}
			if a.CharacterID != nil {
				attacker.Pilot = &retrieval.Pilot{CharacterID: *a.CharacterID}
			}
			attacker.CorpAtKill, attacker.AllianceAtKill = party(intp(a.CorporationID), intp(a.AllianceID))
			km.Attackers = append(km.Attackers, attacker)
		}
		out[km.KillmailID] = km
	}
	return out
}

func intp(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func baseSides() aggregate.WarSides {
	return aggregate.WarSides{
		A: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyA, Name: "Alpha", Member: true}},
		B: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyB, Name: "Bravo", Member: true}},
	}
}

func ids(raws []retrieval.RawKillmail) []int {
	out := []int{}
	for _, r := range raws {
		out = append(out, r.ESIKillmail.KillmailID)
	}
	return out
}

func warSetup(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
}

func TestWarSideOf(t *testing.T) {
	warSetup(t)
	at := warStart

	t.Run("a corp listed on one side beats its alliance listed on the other, either way round", func(t *testing.T) {
		sides := baseSides()
		sides.B = append(sides.B, retrieval.TargetGroup{Type: types.CORPORATION, ID: corpA2, Name: "Defector", Member: true})
		assert.Equal(t, aggregate.WarSideB, sides.SideOf(corpA2, allyA, at))
		assert.Equal(t, aggregate.WarSideA, sides.SideOf(corpA1, allyA, at))
		swapped := aggregate.WarSides{A: sides.B, B: sides.A}
		assert.Equal(t, aggregate.WarSideA, swapped.SideOf(corpA2, allyA, at))
	})

	t.Run("a corp moved between alliances follows the alliance it was in at the kill", func(t *testing.T) {
		sides := baseSides()
		assert.Equal(t, aggregate.WarSideA, sides.SideOf(corpC1, allyA, at))
		assert.Equal(t, aggregate.WarSideB, sides.SideOf(corpC1, allyB, at))
		assert.Equal(t, aggregate.WarSideNone, sides.SideOf(corpC1, allyC, at))
	})

	t.Run("a member beats an affiliate, an added group loses to both, and then side A wins", func(t *testing.T) {
		both := aggregate.WarSides{
			A: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyC}},
			B: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyC, Member: true}},
		}
		assert.Equal(t, aggregate.WarSideB, both.SideOf(0, allyC, at), "member over affiliate")
		tie := aggregate.WarSides{A: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyC}}, B: []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyC}}}
		assert.Equal(t, aggregate.WarSideA, tie.SideOf(0, allyC, at))
		added := aggregate.WarSides{
			A:      []retrieval.TargetGroup{{Type: types.ALLIANCE, ID: allyC}},
			AddedB: []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC}},
		}
		assert.Equal(t, aggregate.WarSideA, added.SideOf(0, allyC, at))
		onlyAdded := aggregate.WarSides{AddedB: []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC}}}
		assert.Equal(t, aggregate.WarSideB, onlyAdded.SideOf(0, allyC, at))
	})

	t.Run("an added group counts from midnight UTC of its date", func(t *testing.T) {
		sides := aggregate.WarSides{AddedA: []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, FromDate: "2026-10-02"}}}
		assert.Equal(t, aggregate.WarSideNone, sides.SideOf(0, allyC, time.Date(2026, 10, 1, 23, 59, 0, 0, time.UTC)))
		assert.Equal(t, aggregate.WarSideA, sides.SideOf(0, allyC, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)))
	})

	t.Run("nothing listed is on no side", func(t *testing.T) {
		assert.Equal(t, aggregate.WarSideNone, baseSides().SideOf(0, 0, at))
		assert.Equal(t, aggregate.WarSideNone, baseSides().SideOf(npcCorp, 0, at))
	})
}

func TestClassifyRawWar(t *testing.T) {
	warSetup(t)
	sides := baseSides()

	t.Run("both sides in a battle with no killmail between them is forgotten", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 3, sysHome, pC(11), rifter, pB(2)),
		}
		assert.Empty(t, aggregate.ClassifyRawWar(kms, sides))
	})

	t.Run("one killmail between the sides counts the whole battle", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 3, sysHome, pC(11), rifter, pB(2)),
			raw(3, 4, sysHome, pB(3), rifter, pA1(1)),
		}
		assert.Equal(t, []int{1, 2, 3}, ids(aggregate.ClassifyRawWar(kms, sides)))
	})

	t.Run("both sides shooting the same third party is not a fight between them", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pC(10), rifter, pA1(1), pB(2))}
		assert.Empty(t, aggregate.ClassifyRawWar(kms, sides))
	})

	t.Run("a battle with one side only is forgotten, whoever else is in it", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 3, sysHome, pD(11), rifter, pC(12)),
			raw(3, 4, sysHome, pA2(3), rifter, pD(13)),
		}
		assert.Empty(t, aggregate.ClassifyRawWar(kms, sides))
	})

	t.Run("a lone killmail between the sides is a battle of one", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(2), rifter, pA1(1))}
		assert.Equal(t, []int{1}, ids(aggregate.ClassifyRawWar(kms, sides)))
	})

	t.Run("two fights apart in time are two battles and only the one with both sides stays", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 120, sysOther, pB(2), rifter, pA1(1)),
		}
		assert.Equal(t, []int{2}, ids(aggregate.ClassifyRawWar(kms, sides)))
	})

	t.Run("a same-side killmail is held, apart from battles", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pA1(1), rifter, pA2(2)),       // A kills A
			raw(2, 1, sysHome, pA1(1), rifter, pA2(2), pNPC), // an NPC among the attackers: not same-side
			raw(3, 2, sysHome, pB(3), rifter, pA1(1)),
		}
		out := ids(aggregate.ClassifyRawWar(kms, sides))
		assert.Equal(t, []int{1, 2, 3}, out)

		// the same-side one does not join the battle: alone in the system it is not counted by its own
		alone := []retrieval.RawKillmail{kms[0]}
		assert.Equal(t, []int{1}, ids(aggregate.ClassifyRawWar(alone, sides)), "still held")
		data := aggregate.BuildWar(alone, expand(alone...), sides, 10)
		assert.Empty(t, data.Battles)
		assert.Zero(t, data.A.Killmails)
	})

	t.Run("a same-side killmail does not link battles", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(3), rifter, pA1(1)),
			raw(2, 10, sysHome, pA1(1), rifter, pA2(1)), // between the two, which it would bridge
			raw(3, 24, sysHome, pB(4), rifter, pA1(5)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 1)
		assert.Equal(t, 2, data.BattleCount)
		assert.Equal(t, 2, data.B.Killmails)
		assert.Zero(t, data.A.Killmails, "the same-side loss counts for nothing")
	})

	t.Run("NPCs alone make no side", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pNPC),
			raw(2, 1, sysHome, wp{}, rifter, pNPC),
		}
		assert.Empty(t, aggregate.ClassifyRawWar(kms, sides))
		withA := append(kms, raw(3, 2, sysHome, pNPC, rifter, pA1(1)))
		assert.Empty(t, aggregate.ClassifyRawWar(withA, sides), "an NPC corp's loss is no one's")
	})

	t.Run("a structure with no pilot counts as its side", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, astrahus, pB(2))}
		assert.Equal(t, []int{1}, ids(aggregate.ClassifyRawWar(kms, sides)))
		attacker := []retrieval.RawKillmail{raw(2, 0, sysHome, pB(2), rifter, wp{corp: corpA1, ally: allyA})}
		assert.Equal(t, []int{2}, ids(aggregate.ClassifyRawWar(attacker, sides)))
	})

	t.Run("an empty or nil input is fine", func(t *testing.T) {
		assert.Empty(t, aggregate.ClassifyRawWar(nil, sides))
		assert.Empty(t, aggregate.ClassifyRawWar([]retrieval.RawKillmail{{}}, sides))
	})
}

func TestBuildWarStructures(t *testing.T) {
	warSetup(t)
	sides := baseSides()

	t.Run("a lone structure kill is a skirmish naming its owner when its side has no pilot", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, astrahus, pB(2))}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		if assert.Len(t, data.Battles, 1) {
			battle := data.Battles[0]
			assert.True(t, battle.Skirmish)
			assert.Equal(t, 1, battle.Pilots)
			assert.Zero(t, battle.A.Pilots)
			if assert.NotNil(t, battle.A.Group) {
				assert.Equal(t, "Alpha", battle.A.Group.Name)
				assert.Equal(t, "ALLIANCE", battle.A.Group.Type)
			}
			assert.Equal(t, 1, battle.A.StructureLost)
			assert.Equal(t, "Bravo", battle.B.Group.Name)
			assert.Nil(t, battle.Others.Group)
		}
		assert.Equal(t, 1, data.Skirmishes)
		assert.Zero(t, data.BattleCount)
		if assert.Len(t, data.A.Citadels, 1) {
			assert.Equal(t, 1, data.A.Citadels[0].Count)
			assert.Equal(t, "Astrahus", data.A.Citadels[0].Type.Name)
		}
		assert.Zero(t, data.A.ShipsLost, "a structure is not a ship")
	})

	t.Run("towers and customs offices are POS, and every structure's ISK is structure ISK", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
			raw(2, 1, sysHome, wp{corp: corpA1, ally: allyA}, customs, pB(2)),
			raw(3, 2, sysHome, pA1(5), rifter, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		assert.Equal(t, 2, data.A.POS)
		assert.Empty(t, data.A.Citadels)
		assert.InDelta(t, 300, data.A.ISKStructures, 0.001)
		assert.InDelta(t, 300, data.A.ISKFights, 0.001)
	})
}

func TestBuildWarThirdParties(t *testing.T) {
	warSetup(t)
	sides := baseSides()

	// every fight has a direct A and B killmail so it counts, with C (and sometimes D) around it
	t.Run("a group that shot at B fought beside A and its losses are A's", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(2), rifter, pC(10), pA1(1)),
			raw(2, 1, sysHome, pC(10), rifter, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		if assert.Len(t, data.Battles, 1) {
			battle := data.Battles[0]
			assert.Equal(t, 2, battle.A.Pilots, "A's pilot and C's")
			assert.Equal(t, 1, battle.B.Pilots)
			assert.Zero(t, battle.Others.Pilots)
			assert.InDelta(t, 200, battle.A.ISKLost, 0.001, "C's loss")
			assert.InDelta(t, 100, battle.B.ISKLost, 0.001)
			assert.Equal(t, "Alpha", battle.A.Group.Name, "A's own pilot first on the tie by name")
		}
		assert.Equal(t, 1, data.A.Killmails)
		assert.Zero(t, data.Others.Killmails)
		assert.Empty(t, data.OtherGroups)
		for _, km := range data.Killmails {
			if km.Killmail.KillmailID == 2 {
				assert.Equal(t, aggregate.WarSideA, km.VictimSide)
			}
		}
	})

	t.Run("a group that was shot by A fought beside B", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 1, sysHome, pA1(1), rifter, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		assert.Equal(t, 1, data.B.Killmails, "C's loss counts for B")
		assert.Equal(t, 1, data.A.Killmails)
		assert.Zero(t, data.Others.Killmails)
	})

	t.Run("even, or no contact, leaves a group under Others with a row for it", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pA1(1)),
			raw(2, 1, sysHome, pC(10), rifter, pB(2)),
			raw(5, 2, sysHome, pD(20), rifter, pD(21)), // D shoots only D
			raw(4, 3, sysHome, wp{corp: npcCorp}, rifter, pB(2)),
			raw(6, 4, sysHome, pB(3), rifter, pA1(1)), // the sides fight, so the battle counts
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		assert.Equal(t, 4, data.Others.Killmails, "C twice, D, and the NPC corp")
		assert.Zero(t, data.A.Killmails)
		assert.Equal(t, 1, data.B.Killmails)
		names := []string{}
		for _, other := range data.OtherGroups {
			names = append(names, other.Group.Name)
		}
		assert.Equal(t, []string{"Delta", "Charlie"}, names, "most ISK lost first, and no row for the NPC corp")
		assert.Equal(t, 1, data.NPCPilots.Killmails, "its loss is on its own line")
		assert.Positive(t, data.NPCPilots.ISKLost)
		assert.InDelta(t, 300, data.OtherGroups[1].Measures.ISKLost, 0.001)
	})

	t.Run("a corp with no alliance is its own group", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, wp{corp: corpLone, char: 30}, rifter, pA1(1)),
			raw(2, 1, sysHome, pB(2), rifter, pA1(1)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
		assert.Equal(t, 2, data.B.Killmails, "the lone corp, shot by A, fought beside B")
		assert.Zero(t, data.Others.Killmails)
	})

	t.Run("an added group counts for its side from the beginning of the war", func(t *testing.T) {
		withAdded := sides
		withAdded.AddedA = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, Name: "Charlie"}}
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pD(20)),
			raw(2, 1, sysHome, pB(2), rifter, pA1(1)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), withAdded, 10)
		assert.Equal(t, 1, data.A.Killmails, "C's loss")
		assert.Equal(t, 1, data.B.Killmails)
		assert.Zero(t, data.Others.Killmails)
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 2, data.Battles[0].A.Pilots)
			assert.Equal(t, 2, data.Battles[0].B.Pilots, "D shot at A's C, so it fought beside B")
		}
	})

	t.Run("an added group from a date: before it, Others and counted toward no one, from it, its side's", func(t *testing.T) {
		dated := sides
		dated.AddedA = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, Name: "Charlie", FromDate: "2026-10-02"}}
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pC(10), rifter, pB(2)),  // C dies to B before the date
			raw(2, 1, sysHome, pD(20), rifter, pC(10)), // C kills D, which does not count toward D's side
			raw(3, 2, sysHome, pB(3), rifter, pA1(1)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), dated, 10)
		assert.Zero(t, data.A.Killmails)
		assert.Equal(t, 1, data.B.Killmails)
		assert.Equal(t, 2, data.Others.Killmails, "C's loss, and D's")

		later := make([]retrieval.RawKillmail, len(kms))
		for i, km := range kms {
			esi := *km.ESIKillmail
			esi.KillmailTime = esi.KillmailTime.AddDate(0, 0, 1)
			km.ESIKillmail = &esi
			later[i] = km
		}
		data = aggregate.BuildWar(later, expand(later...), dated, 10)
		assert.Equal(t, 1, data.A.Killmails, "C's loss, for A")
		assert.Equal(t, 2, data.B.Killmails, "its own, and D's, shot by A's C")
		assert.Zero(t, data.Others.Killmails)
	})

	t.Run("a third party's shot on a killmail with an added group before its date still counts toward the side it fought beside", func(t *testing.T) {
		dated := sides
		dated.AddedA = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, Name: "Charlie", FromDate: "2026-10-02"}}
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pA1(1), rifter, pC(10), pD(20)), // D shoots A beside C, whose date is later
			raw(2, 1, sysHome, pD(21), rifter, pC(10)),         // D's loss to C alone tells nothing of D
			raw(3, 2, sysHome, pB(3), rifter, pA1(1)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), dated, 10)
		assert.Equal(t, 1, data.A.Killmails)
		assert.Equal(t, 2, data.B.Killmails, "its own, and D's, as D fought beside B")
		assert.Zero(t, data.Others.Killmails)
	})
}

func TestBuildWarIgnored(t *testing.T) {
	warSetup(t)
	ignoring := baseSides()
	ignoring.Ignored = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, Name: "Charlie"}}
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pC(10), rifter, pB(2)), // beside A but for being ignored
		raw(2, 1, sysHome, pB(3), rifter, pA1(1), pC(11)),
	}

	data := aggregate.BuildWar(kms, expand(kms...), ignoring, 10)

	assert.Zero(t, data.A.Killmails, "its loss counts nowhere")
	assert.Equal(t, 1, data.B.Killmails, "its shots do not take the killmail from the side that lost it")
	assert.Zero(t, data.Others.Killmails)
	assert.Empty(t, data.OtherGroups)
	if assert.Len(t, data.Battles, 1) {
		assert.Equal(t, 1, data.Battles[0].A.Pilots, "its pilots are no one's")
		assert.Equal(t, 3, data.Battles[0].Pilots, "A's, and B's two, without C's")
		assert.Len(t, data.Battles[0].Killmails, 1, "without the killmail of its loss")
	}

	t.Run("it does not hold a fight above the battle size", func(t *testing.T) {
		data := aggregate.BuildWar(kms, expand(kms...), ignoring, 5)
		assert.Equal(t, 1, data.Skirmishes, "5 pilots with C's, 3 without")
		assert.Zero(t, data.BattleCount)
	})

	t.Run("a member of a side can not be ignored", func(t *testing.T) {
		onSide := baseSides()
		onSide.Ignored = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: onSide.B[0].ID}}
		data := aggregate.BuildWar(kms, expand(kms...), onSide, 10)
		assert.Equal(t, 1, data.B.Killmails)
	})
}

func TestBuildWarTotals(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	structure := func(corp, ally int) wp { return wp{corp: corp, ally: ally} }
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pA1(1), rifter, pB(2)),
		raw(2, 1, sysHome, pB(2), rifter, pA1(1)),
		raw(3, 2, sysHome, pA1(5), capsule, pB(2)),
		raw(4, 5, sysOther, structure(corpA1, allyA), astrahus, pB(2)),
		raw(5, 6, sysHome, structure(corpB1, allyB), tower, pA1(1)),
		raw(6, 7, sysHome, structure(corpB1, allyB), customs, pA1(1)),
		raw(7, 8, sysHome, pA1(6), 99999999, pB(2)),
		raw(8, 9, sysHome, structure(corpA2, allyA), 99999998, pB(2)),
	}
	data := aggregate.BuildWar(kms, expand(kms...), sides, 10)

	assert.Equal(t, 5, data.A.Killmails)
	assert.InDelta(t, 100+300+400+700+800, data.A.ISKLost, 0.001)
	assert.InDelta(t, 100+300+700+800, data.A.ISKFights, 0.001)
	assert.InDelta(t, 400, data.A.ISKStructures, 0.001)
	assert.Zero(t, data.A.ISKTrash)
	assert.Equal(t, 2, data.A.ShipsLost, "the rifter and the unknown type with a pilot, not the capsule or the pilotless unknown")
	assert.Equal(t, data.A.ShipsLost, data.A.ShipsFights)
	assert.Len(t, data.A.Citadels, 1)

	assert.Equal(t, 3, data.B.Killmails)
	assert.InDelta(t, 200+500+600, data.B.ISKLost, 0.001)
	assert.InDelta(t, 200, data.B.ISKFights, 0.001)
	assert.InDelta(t, 1100, data.B.ISKStructures, 0.001)
	assert.Equal(t, 1, data.B.ShipsLost)
	assert.Equal(t, 2, data.B.POS)

	for _, m := range []aggregate.WarMeasures{data.A, data.B, data.Others} {
		assert.InDelta(t, m.ISKFights+m.ISKTrash+m.ISKStructures, m.ISKLost, 0.001)
		assert.Equal(t, m.ShipsFights+m.ShipsTrash, m.ShipsLost)
	}
	assert.Zero(t, data.Others.Killmails)

	if assert.Len(t, data.Battles, 1) {
		battle := data.Battles[0]
		assert.Equal(t, 8, len(battle.Killmails))
		assert.Equal(t, 4, battle.Pilots, "characters 1, 2, 5, 6")
		assert.True(t, battle.Skirmish)
		assert.Equal(t, warStart, battle.Start)
		assert.Equal(t, warStart.Add(9*time.Minute), battle.End)
		assert.Equal(t, "Home", battle.System.Name)
		assert.Equal(t, []string{"Other"}, battle.OtherSystems)
		assert.InDelta(t, data.A.ISKLost, battle.A.ISKLost, 0.001)
		assert.Equal(t, 1, battle.A.StructureLost)
		assert.Equal(t, 2, battle.B.StructureLost)
		assert.Equal(t, 3, battle.A.ShipsLost+battle.B.ShipsLost)
	}
	assert.Equal(t, 8, len(data.Killmails))
	assert.Equal(t, 8, data.Killmails[0].Killmail.KillmailID, "newest first")
	if assert.Len(t, data.Systems, 2) {
		assert.Equal(t, "Home", data.Systems[0].System.Name)
		assert.Equal(t, 7, data.Systems[0].A.Killmails+data.Systems[0].B.Killmails)
	}
	assert.Equal(t, 0, data.BattleCount)
	assert.Equal(t, 1, data.Skirmishes)

	again := aggregate.BuildWar(kms, expand(kms...), sides, 10)
	assert.Equal(t, data, again, "deterministic")
}

func TestBuildWarBattleSides(t *testing.T) {
	warSetup(t)

	t.Run("a side names its largest group by pilots, ties by name", func(t *testing.T) {
		withAdded := baseSides()
		withAdded.AddedA = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: allyC, Name: "Charlie"}}
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(2), rifter, pA1(1), pC(10), pC(11)),
			raw(2, 1, sysHome, pB(3), rifter, pA1(1)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), withAdded, 10)
		assert.Equal(t, "Charlie", data.Battles[0].A.Group.Name)
		assert.Equal(t, 3, data.Battles[0].A.Pilots)
		assert.Equal(t, 2, data.Battles[0].B.Pilots)
		assert.Equal(t, aggregate.WarGroup{Type: "ALLIANCE", ID: allyC, Name: "Charlie"}, *data.Battles[0].A.Group)
	})

	t.Run("a battle of minPilots pilots is not a skirmish", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(2), rifter, pA1(1), pA1(3))}
		assert.False(t, aggregate.BuildWar(kms, expand(kms...), baseSides(), 3).Battles[0].Skirmish)
		assert.True(t, aggregate.BuildWar(kms, expand(kms...), baseSides(), 4).Battles[0].Skirmish)
	})

	t.Run("missing expanded killmails and nil parts never panic", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(2), rifter, pA1(1)),
			raw(2, 1, sysHome, pB(2), rifter, pA1(1)),
			raw(3, 2, 0, pB(2), rifter, pA1(1)),
		}
		expanded := expand(kms...)
		delete(expanded, 1)
		expanded[2].System = nil
		expanded[2].Victim = nil
		expanded[3].Attackers = []*retrieval.KillmailAttacker{nil}
		data := aggregate.BuildWar(kms, expanded, baseSides(), 10)
		assert.NotNil(t, data)
		assert.Len(t, data.Killmails, 2)
		assert.NotPanics(t, func() { aggregate.BuildWar(nil, nil, baseSides(), 10) })
		assert.NotPanics(t, func() { aggregate.BuildWar([]retrieval.RawKillmail{{}}, nil, baseSides(), 10) })
	})
}

func TestWarBattlesMatchFindBattles(t *testing.T) {
	warSetup(t)
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pB(2), rifter, pA1(1)),
		raw(2, 10, sysOther, pB(3), rifter, pA1(1)),
		raw(3, 90, sysHome, pB(4), rifter, pA1(7)),
		raw(4, 200, sysHome, pB(5), rifter, pA1(8)),
		raw(5, 205, sysOther, pB(6), rifter, pA1(8)),
	}
	expanded := expand(kms...)
	list := []*retrieval.Killmail{}
	for _, km := range kms {
		list = append(list, expanded[km.ESIKillmail.KillmailID])
	}
	collected := &aggregate.CollectedData{BattlesFought: map[string]aggregate.BattleFought{}}
	fromExpanded := collected.FindBattles(list)

	inputs := []aggregate.BattleKillmail{}
	for index := range kms {
		inputs = append(inputs, aggregate.RawBattleKillmail(&kms[index]))
	}
	fromRaw := map[int]string{}
	for _, battle := range aggregate.FindBattlesOf(inputs) {
		for _, id := range battle.KillmailIDs {
			fromRaw[id] = battle.BattleIDReference
		}
	}
	assert.Equal(t, fromExpanded, fromRaw)
	assert.Len(t, aggregate.BuildWar(kms, expanded, baseSides(), 1).Battles, 3)
}
