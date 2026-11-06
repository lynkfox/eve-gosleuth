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
		data := aggregate.BuildWar(alone, expand(alone...), sides, 10, retrieval.WarEdits{})
		assert.Empty(t, data.Battles)
		assert.Zero(t, data.A.Killmails)
	})

	t.Run("a same-side killmail does not link battles", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(3), rifter, pA1(1)),
			raw(2, 10, sysHome, pA1(1), rifter, pA2(1)), // between the two, which it would bridge
			raw(3, 24, sysHome, pB(4), rifter, pA1(5)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 1, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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

	t.Run("a battle's structures go by kind, most lost first, towers as one kind", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, astrahus, pB(2)),
			raw(2, 1, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
			raw(3, 2, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 3, data.Battles[0].A.StructureLost)
			assert.Equal(t, []aggregate.WarStructureKind{{Name: "POS", Count: 2}, {Name: "Astrahus", Count: 1}}, data.Battles[0].A.StructureKinds)
			assert.Empty(t, data.Battles[0].B.StructureKinds)
		}
	})

	t.Run("towers and customs offices are POS, and every structure's ISK is structure ISK", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
			raw(2, 1, sysHome, wp{corp: corpA1, ally: allyA}, customs, pB(2)),
			raw(3, 2, sysHome, pA1(5), rifter, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
		assert.Equal(t, 2, data.A.POS)
		assert.Empty(t, data.A.Citadels)
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 2, data.Battles[0].A.StructureLost)
			assert.ElementsMatch(t, []aggregate.WarStructureKind{{Name: "POS", Count: 1}, {Name: "Customs Office", Count: 1}}, data.Battles[0].A.StructureKinds)
		}
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), withAdded, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), dated, 10, retrieval.WarEdits{})
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
		data = aggregate.BuildWar(later, expand(later...), dated, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), dated, 10, retrieval.WarEdits{})
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

	data := aggregate.BuildWar(kms, expand(kms...), ignoring, 10, retrieval.WarEdits{})

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
		data := aggregate.BuildWar(kms, expand(kms...), ignoring, 5, retrieval.WarEdits{})
		assert.Equal(t, 1, data.Skirmishes, "5 pilots with C's, 3 without")
		assert.Zero(t, data.BattleCount)
	})

	t.Run("a member of a side can not be ignored", func(t *testing.T) {
		onSide := baseSides()
		onSide.Ignored = []retrieval.WarAddedGroup{{Type: "ALLIANCE", ID: onSide.B[0].ID}}
		data := aggregate.BuildWar(kms, expand(kms...), onSide, 10, retrieval.WarEdits{})
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
	data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})

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
		if assert.Len(t, battle.OtherDetails, 1) {
			assert.Equal(t, "Other", battle.OtherDetails[0].Name)
		}
		assert.InDelta(t, data.A.ISKLost, battle.A.ISKLost, 0.001)
		assert.Equal(t, 1, battle.A.StructureLost)
		assert.Equal(t, 2, battle.B.StructureLost)
		assert.Equal(t, 3, battle.A.ShipsLost+battle.B.ShipsLost)
	}
	assert.Equal(t, 8, len(data.Killmails))
	assert.Equal(t, 8, data.Killmails[0].Killmail.KillmailID, "newest first")
	if assert.Len(t, data.Systems, 2) {
		assert.Equal(t, "Home", data.Systems[0].System.Name)
		assert.Equal(t, 1, data.Systems[0].Battles)
		assert.Equal(t, 1, data.Systems[1].Battles, "a battle that moved counts in each of its systems")
		assert.Equal(t, 7, data.Systems[0].A.Killmails+data.Systems[0].B.Killmails)
	}
	assert.Equal(t, 0, data.BattleCount)
	assert.Equal(t, 1, data.Skirmishes)

	again := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
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
		data := aggregate.BuildWar(kms, expand(kms...), withAdded, 10, retrieval.WarEdits{})
		assert.Equal(t, "Charlie", data.Battles[0].A.Group.Name)
		assert.Equal(t, 3, data.Battles[0].A.Pilots)
		assert.Equal(t, 2, data.Battles[0].B.Pilots)
		assert.Equal(t, aggregate.WarGroup{Type: "ALLIANCE", ID: allyC, Name: "Charlie"}, *data.Battles[0].A.Group)
	})

	t.Run("a battle of minPilots pilots is not a skirmish", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(2), rifter, pA1(1), pA1(3))}
		assert.False(t, aggregate.BuildWar(kms, expand(kms...), baseSides(), 3, retrieval.WarEdits{}).Battles[0].Skirmish)
		assert.True(t, aggregate.BuildWar(kms, expand(kms...), baseSides(), 4, retrieval.WarEdits{}).Battles[0].Skirmish)
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
		data := aggregate.BuildWar(kms, expanded, baseSides(), 10, retrieval.WarEdits{})
		assert.NotNil(t, data)
		assert.Len(t, data.Killmails, 2)
		assert.NotPanics(t, func() { aggregate.BuildWar(nil, nil, baseSides(), 10, retrieval.WarEdits{}) })
		assert.NotPanics(t, func() { aggregate.BuildWar([]retrieval.RawKillmail{{}}, nil, baseSides(), 10, retrieval.WarEdits{}) })
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
	assert.Len(t, aggregate.BuildWar(kms, expanded, baseSides(), 1, retrieval.WarEdits{}).Battles, 3)
}

// standup fighters and structures for the trash tests: a Raitaru is a medium, a Fortizar a large
const (
	standupTemplar, carrierTemplar = 47035, 23055
	raitaru, fortizar              = 35825, 35833
)

func trashIDs(data *aggregate.WarData) []int {
	out := []int{}
	for _, km := range data.Killmails {
		if km.Trash {
			out = append(out, km.Killmail.KillmailID)
		}
	}
	return out
}

// onKill makes an attacker's ship a given type, ie a structure on a killmail
func onKill(km retrieval.RawKillmail, ship int) retrieval.RawKillmail {
	km.ESIKillmail.Attackers[0].ShipTypeID = ship
	return km
}

func TestWarStationTrash(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	const hour = 60
	build := func(kms ...retrieval.RawKillmail) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	}

	t.Run("a repeat victim's losses are trash and the battle ends without them", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 10, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 11, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 24, sysHome, pA1(5), rifter, pB(2)),
		)
		assert.ElementsMatch(t, []int{2, 3, 4}, trashIDs(data))
		if assert.Len(t, data.Battles, 1) {
			battle := data.Battles[0]
			assert.Equal(t, battle.Start, battle.End, "the trash doesn't make the battle longer")
			assert.Equal(t, 3, battle.A.TrashKillmails)
			assert.Equal(t, 3, battle.A.TrashShips)
			assert.InDelta(t, 900, battle.A.TrashISK, 0.001)
			assert.InDelta(t, 100, battle.A.ISKLost, 0.001, "trash is not in the battle's own figures")
		}
		assert.InDelta(t, 900, data.A.ISKTrash, 0.001)
		assert.Equal(t, 3, data.A.ShipsTrash)
		assert.Zero(t, data.A.ShipsFights)
		assert.InDelta(t, data.A.ISKLost, data.A.ISKFights+data.A.ISKTrash+data.A.ISKStructures, 0.001)
		assert.Equal(t, data.A.ShipsLost, data.A.ShipsFights+data.A.ShipsTrash)
	})

	t.Run("a capsule is never trash", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(6), capsule, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("a pilot's capsule dying soon after their ships says they were in them", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 61, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 64, sysHome, pA1(5), capsule, pB(2)),
			raw(5, 60, sysHome, pA1(6), rifter, pB(2)),
			raw(6, 64, sysHome, pA1(6), capsule, pB(2)),
		)
		assert.Empty(t, trashIDs(data), "the repeat victim and the one in the window alike")
	})

	t.Run("a capsule long after leaves the empty ships as trash", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 61, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 70, sysHome, pA1(5), capsule, pB(2)),
		)
		assert.ElementsMatch(t, []int{2, 3}, trashIDs(data))
	})

	t.Run("a pilot's earlier empty ships stay trash though they later die in one", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 61, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 90, sysHome, pA1(5), rifter, pB(2)),
			raw(5, 91, sysHome, pA1(5), capsule, pB(2)),
		)
		assert.ElementsMatch(t, []int{2, 3}, trashIDs(data))
	})

	t.Run("blue on blue after a station died is trash, on the victim's side and whichever side's station it was", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 2, sysHome, pB(5), rifter, pB(2)),
			raw(3, 3, sysHome, pB(5), rifter, pB(3)),
			raw(4, 60, sysHome, pB(6), rifter, pB(3)),
			raw(5, 5*hour+5, sysHome, pB(7), rifter, pB(3)),
		)
		assert.ElementsMatch(t, []int{2, 3, 4}, trashIDs(data))
		assert.Equal(t, 3, data.B.ShipsTrash)
		assert.Zero(t, data.B.ShipsFights)
		assert.Equal(t, 2, data.B.ShipsTrashDefinite, "the repeat victim, the third only in the window")
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 3, data.Battles[0].B.TrashKillmails)
		}
	})

	t.Run("a repeat victim is definite, one only in the window probable", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 61, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 90, sysHome, pA1(6), rifter, pB(2)),
		)
		assert.InDelta(t, 200+300, data.A.ISKTrashDefinite, 0.001)
		assert.InDelta(t, 200+300+400, data.A.ISKTrash, 0.001)
		assert.Equal(t, 2, data.A.ShipsTrashDefinite)
		assert.Equal(t, 3, data.A.ShipsTrash)
		if assert.Len(t, data.Battles, 1) {
			assert.InDelta(t, 200+300, data.Battles[0].A.TrashDefiniteISK, 0.001)
			assert.Equal(t, 2, data.Battles[0].A.TrashDefiniteShips)
		}
	})

	t.Run("a ship and its pod are not a repeat", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 1, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 2, sysHome, pA1(5), capsule, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("a repeat victim can be past the 4 hours, but not past the reach", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 30*hour, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 30*hour+1, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 8*24*hour, sysHome, pA1(6), rifter, pB(2)),
			raw(5, 8*24*hour+1, sysHome, pA1(6), rifter, pB(2)),
		)
		assert.ElementsMatch(t, []int{2, 3}, trashIDs(data))
	})

	t.Run("two losses 2 minutes apart or more are not a repeat, as a pilot can reship", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 5*hour, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 5*hour+2, sysHome, pA1(5), rifter, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("after the battle: 4 hours from its end, not from the structure", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 10, sysHome, pB(3), rifter, pA1(4)), // the battle runs on to here
			raw(3, 10+4*hour, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 11+4*hour, sysHome, pA1(6), rifter, pB(2)),
		)
		assert.Equal(t, []int{3}, trashIDs(data))
	})

	t.Run("another system's killmails are not trash", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysOther, pA1(5), rifter, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("the side that killed the station keeps its real losses out", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pB(3), rifter, pA1(4)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("a revenge fleet shooting back keeps its losses out", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pA1(4), rifter, pB(2)),
			raw(3, 62, sysHome, pB(3), rifter, pA1(5)),
		)
		assert.Empty(t, trashIDs(data))
		assert.Len(t, data.Battles, 2)
	})

	t.Run("a repeat victim in a later fight where the owner shot back is not trash", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 7*hour, sysHome, pA1(5), rifter, pB(2)),
			raw(3, 7*hour+1, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 7*hour+2, sysHome, pB(3), rifter, pA1(6)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("a killmail goes with the last station to die before it", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 28*hour, sysHome, owner, astrahus, pB(2)),
			raw(3, 28*hour+1, sysHome, pA1(5), rifter, pB(2)),
			raw(4, 28*hour+2, sysHome, pA1(5), rifter, pB(2)),
		)
		assert.ElementsMatch(t, []int{3, 4}, trashIDs(data))
		if assert.Len(t, data.Battles, 2) {
			assert.Equal(t, 2, data.Battles[0].A.TrashKillmails, "newest first, so the second station's")
			assert.Zero(t, data.Battles[1].A.TrashKillmails)
		}
	})

	t.Run("a neutral victim counts for the station's owner, and is held though no side fought", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, pC(7), rifter, pB(2)),
		}
		assert.Equal(t, []int{1, 2}, ids(aggregate.ClassifyRawWar(kms, sides)))
		data := build(kms...)
		assert.Equal(t, []int{2}, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsTrash)
		assert.Zero(t, data.Others.Killmails)
	})

	t.Run("no trash after a third party's station", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, wp{corp: corpC1, ally: allyC}, astrahus, pB(2)),
			raw(2, 1, sysHome, pA1(9), rifter, pB(2)),
			raw(3, 60, sysHome, pC(7), rifter, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
	})

	t.Run("another structure dying later stays a structure", func(t *testing.T) {
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 60, sysHome, owner, customs, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
		assert.Equal(t, 1, data.A.POS)
	})
}

func TestWarEscapeFleet(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	const hour = 60
	build := func(kms ...retrieval.RawKillmail) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	}
	fitted := func(km retrieval.RawKillmail) retrieval.RawKillmail {
		km.ESIKillmail.Victim.Items = []api.ESIKillmailItem{{ItemTypeID: 2873, Flag: 27}}
		return km
	}
	station := raw(1, 0, sysHome, owner, astrahus, pB(2))

	t.Run("a fitted ship with more than one attacker in the 4 hours is not trash", func(t *testing.T) {
		data := build(station, fitted(raw(2, hour, sysHome, pA1(5), rifter, pB(2), pB(3))))
		assert.Empty(t, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsFights)
	})

	t.Run("one attacker, or nothing fitted, is still trash, and uncertain", func(t *testing.T) {
		for _, km := range []retrieval.RawKillmail{fitted(raw(2, hour, sysHome, pA1(5), rifter, pB(2))), raw(2, hour, sysHome, pA1(5), rifter, pB(2), pB(3))} {
			data := build(station, km)
			assert.Equal(t, []int{2}, trashIDs(data))
			assert.Zero(t, data.A.ShipsTrashDefinite)
		}
	})

	t.Run("a pilot's rapid losses are trash even fitted with several attackers", func(t *testing.T) {
		data := build(station, fitted(raw(2, hour, sysHome, pA1(5), rifter, pB(2), pB(3))), fitted(raw(3, hour+1, sysHome, pA1(5), rifter, pB(2), pB(3))))
		assert.ElementsMatch(t, []int{2, 3}, trashIDs(data))
		assert.Equal(t, 2, data.A.ShipsTrashDefinite)
	})
}

func TestWarAssetDenial(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	const day = 24 * 60
	const naglfar = 19722
	build := func(kms ...retrieval.RawKillmail) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	}
	// fitted gives a killmail's victim a module in a high slot
	fitted := func(km retrieval.RawKillmail) retrieval.RawKillmail {
		km.ESIKillmail.Victim.Items = []api.ESIKillmailItem{{ItemTypeID: 2873, Flag: 27}}
		return km
	}
	blue := func(id, minutes int) retrieval.RawKillmail {
		return fitted(raw(id, minutes, sysHome, pA1(5), naglfar, pA2(6)))
	}

	// stationDies is the side's station dying a day after the killmails it follows
	stationDies := func() retrieval.RawKillmail { return raw(90, day, sysHome, owner, astrahus, pB(2)) }

	t.Run("a side's capital with nothing fitted is definite, whoever killed it, before its station dies there", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, pA1(5), naglfar, pB(2)), stationDies())
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsTrash)
		assert.Equal(t, 1, data.A.ShipsTrashDefinite)
		assert.Zero(t, data.A.ShipsFights)
		if assert.Len(t, data.Battles, 1, "it never was a fight, only the station's death is") {
			assert.Equal(t, 1, data.Battles[0].A.TrashKillmails)
		}
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pA1(5), naglfar, pC(2)), stationDies()}
		assert.Equal(t, []int{1, 90}, ids(aggregate.ClassifyRawWar(kms, sides)), "held even when no side killed it")
	})

	t.Run("with no station lost after it, it is still denial, but only the weaker grade, as the station may never die", func(t *testing.T) {
		uncertain := func(kms ...retrieval.RawKillmail) {
			t.Helper()
			data := build(kms...)
			assert.Equal(t, []int{1}, trashIDs(data))
			assert.Equal(t, 1, data.A.ShipsTrash)
			assert.Zero(t, data.A.ShipsTrashDefinite)
			assert.Zero(t, data.A.ShipsFights)
		}
		uncertain(raw(1, 0, sysHome, pA1(5), naglfar, pB(2)))
		lower := onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner, pB(2))), astrahus)
		lower.ESIKillmail.Attackers[1].DamageDone = 9
		uncertain(lower)
		uncertain(raw(1, 0, sysHome, pA1(5), naglfar, pB(2)), raw(2, 5*day, sysHome, owner, astrahus, pB(2)))
		uncertain(raw(1, 0, sysHome, pA1(5), naglfar, pB(2)), raw(2, 60, sysOther, owner, astrahus, pB(2)))

		assert.Equal(t, []int{1}, ids(aggregate.ClassifyRawWar([]retrieval.RawKillmail{raw(1, 0, sysHome, pA1(5), naglfar, pC(2))}, sides)), "held, so its grade can rise when the station dies")
	})

	t.Run("its own station alone on the kill, or at top damage on a capital, is always the stronger grade, with no station lost", func(t *testing.T) {
		alone := build(onKill(raw(1, 0, sysHome, pA1(5), rifter, owner), astrahus))
		assert.Equal(t, []int{1}, trashIDs(alone))
		assert.Equal(t, 1, alone.A.ShipsTrashDefinite)

		aloneCapital := build(onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner)), astrahus))
		assert.Equal(t, 1, aloneCapital.A.ShipsTrashDefinite)

		top := onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner, pB(2))), astrahus)
		top.ESIKillmail.Attackers[0].DamageDone = 9
		assert.Equal(t, 1, build(top).A.ShipsTrashDefinite)
	})

	t.Run("a killmail its own side made keeps its grade when a station is lost after", func(t *testing.T) {
		alone := build(onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner)), astrahus), stationDies())
		assert.Equal(t, []int{1}, trashIDs(alone))
		assert.Equal(t, 1, alone.A.ShipsTrashDefinite, "its own station alone")

		unfitted := build(raw(1, 0, sysHome, pA1(5), naglfar, pA2(6)), stationDies())
		assert.Equal(t, []int{1}, trashIDs(unfitted))
		assert.Equal(t, 1, unfitted.A.ShipsTrashDefinite, "nothing fitted")

		repeats := build(raw(10, 0, sysHome, owner, astrahus, pB(2)), blue(1, day), blue(2, day+1), raw(11, 2*day, sysHome, owner, astrahus, pB(2)))
		assert.ElementsMatch(t, []int{1, 2}, trashIDs(repeats))
		assert.Equal(t, 2, repeats.A.ShipsTrashDefinite, "a repeat victim's capitals stay trash")
	})

	t.Run("a capital killed by its own side's pilots still needs its station lost after", func(t *testing.T) {
		assert.Empty(t, trashIDs(build(blue(1, 0))))
		data := build(blue(1, 0), stationDies())
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Zero(t, data.A.ShipsTrashDefinite)
	})

	t.Run("after its side lost a station there it is not denial, unless another is lost soon after", func(t *testing.T) {
		first := raw(2, 0, sysHome, owner, astrahus, pB(2))
		data := build(first, raw(1, day, sysHome, pA1(5), naglfar, pB(2)))
		assert.Empty(t, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsFights)

		assert.Empty(t, trashIDs(build(first, onKill(raw(1, day, sysHome, pA1(5), rifter, owner), astrahus))))

		data = build(first, raw(1, day, sysHome, pA1(5), naglfar, pB(2)), raw(3, 2*day, sysHome, owner, astrahus, pB(2)))
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsTrashDefinite)

		assert.Equal(t, []int{1}, trashIDs(build(raw(2, 0, sysHome, wp{corp: corpB1, ally: allyB}, astrahus, pA1(2)), raw(1, day, sysHome, pA1(5), naglfar, pB(2)))), "another side's station is no matter")
	})

	t.Run("rigs and holds do not make it fitted", func(t *testing.T) {
		km := raw(1, 0, sysHome, pA1(5), naglfar, pB(2))
		km.ESIKillmail.Victim.Items = []api.ESIKillmailItem{{ItemTypeID: 30993, Flag: 92}, {ItemTypeID: 2873, Flag: 5}, {ItemTypeID: 2873, Flag: 155}}
		assert.Equal(t, []int{1}, trashIDs(build(km, stationDies())))
	})

	t.Run("a fitted capital killed by the other side is a fight", func(t *testing.T) {
		data := build(fitted(raw(1, 0, sysHome, pA1(5), naglfar, pB(2))))
		assert.Empty(t, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsFights)
	})

	t.Run("an unfitted subcapital is not denial, nor a third party's capital", func(t *testing.T) {
		assert.Empty(t, trashIDs(build(raw(1, 0, sysHome, pA1(5), rifter, pB(2)), stationDies())))
		assert.Empty(t, trashIDs(build(raw(1, 0, sysHome, pC(5), naglfar, pB(2)), stationDies())))
	})

	t.Run("a subcapital is denial only with nothing but its own station on the kill, and definite", func(t *testing.T) {
		data := build(onKill(raw(1, 0, sysHome, pA1(5), rifter, owner), astrahus), stationDies())
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsTrashDefinite)

		assert.Empty(t, trashIDs(build(onKill(raw(1, 0, sysHome, pA1(5), rifter, owner, pA2(6)), astrahus), stationDies())), "with a pilot on it too")
		assert.Empty(t, trashIDs(build(raw(1, 0, sysHome, pA1(5), rifter, pA2(6)), raw(2, 60, sysHome, owner, astrahus, pB(2)))), "blue on blue before a station's loss is capitals only")
		assert.Empty(t, trashIDs(build(onKill(fitted(raw(1, 0, sysHome, wp{corp: corpB1, ally: allyB, char: 5}, rifter, owner)), astrahus))), "another side's station")
	})

	t.Run("a fitted capital with its side's station or fighters on the kill is denial", func(t *testing.T) {
		assert.Equal(t, []int{1}, trashIDs(build(onKill(blue(1, 0), astrahus), stationDies())))
		assert.Equal(t, []int{1}, trashIDs(build(onKill(blue(1, 0), standupTemplar), stationDies())))
	})

	t.Run("its own station among the other side's fleet is still denial, out of the battles", func(t *testing.T) {
		data := build(onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner, pB(2), pB(3))), astrahus), stationDies())
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsTrash)
		assert.Zero(t, data.A.ShipsFights)
	})

	t.Run("graded definite with nothing fitted or its station at top damage, else probable", func(t *testing.T) {
		grade := func(kms ...retrieval.RawKillmail) bool {
			for _, km := range build(kms...).Killmails {
				if km.Killmail.KillmailID == 1 {
					assert.True(t, km.Trash)
					return km.Definite
				}
			}
			assert.Fail(t, "killmail 1 is not counted")
			return false
		}
		assert.True(t, grade(raw(1, 0, sysHome, pA1(5), naglfar, pB(2)), stationDies()), "nothing fitted")

		top := onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner, pB(2))), astrahus)
		top.ESIKillmail.Attackers[0].DamageDone = 9
		assert.True(t, grade(top, stationDies()), "its station did the most")

		lower := onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, owner, pB(2))), astrahus)
		lower.ESIKillmail.Attackers[1].DamageDone = 9
		assert.False(t, grade(lower, stationDies()), "its station did less")

		assert.False(t, grade(blue(1, 0), raw(2, day, sysHome, owner, astrahus, pB(2))), "only blue on blue before the station died")
	})

	t.Run("a capsule only its own station shot has nothing to do with the war", func(t *testing.T) {
		pod := onKill(raw(1, 0, sysHome, pA1(5), capsule, owner), astrahus)
		assert.Empty(t, aggregate.ClassifyRawWar([]retrieval.RawKillmail{pod}, sides))
		data := build(pod)
		assert.Empty(t, data.Killmails)
		assert.Zero(t, data.A.Killmails)

		pilots := onKill(raw(1, 0, sysHome, pA1(5), capsule, owner, pA2(6)), astrahus)
		assert.Equal(t, []int{1}, ids(aggregate.ClassifyRawWar([]retrieval.RawKillmail{pilots}, sides)), "with a pilot on it, it is an ordinary same-side killmail")
	})

	t.Run("the other side's station on the kill is a fight", func(t *testing.T) {
		data := build(onKill(fitted(raw(1, 0, sysHome, pA1(5), naglfar, pB(2))), astrahus), stationDies())
		assert.Empty(t, trashIDs(data))
		assert.Equal(t, 1, data.A.ShipsFights)
	})

	t.Run("a fitted capital killed blue on blue with no station and no loss is left out", func(t *testing.T) {
		data := build(blue(1, 0))
		assert.Empty(t, trashIDs(data))
		assert.Zero(t, data.A.Killmails)
	})

	t.Run("before the side's station dies in its system, and attached to that battle", func(t *testing.T) {
		data := build(blue(1, 0), raw(2, day, sysHome, owner, astrahus, pB(2)))
		assert.Equal(t, []int{1}, trashIDs(data))
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 1, data.Battles[0].A.TrashKillmails)
		}
	})

	t.Run("after the station died, in another system, or by another side's loss is left out", func(t *testing.T) {
		assert.Empty(t, trashIDs(build(raw(1, 0, sysHome, owner, astrahus, pB(2)), blue(2, 5*60))), "past the 4 hours in which it would be trash")
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 60, sysOther, owner, astrahus, pB(2)))))
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 60, sysHome, wp{corp: corpB1, ally: allyB}, astrahus, pA1(2)))))
	})

	t.Run("only as far before as its timers can reach, by its size", func(t *testing.T) {
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 112*60+31, sysHome, owner, raitaru, pB(2)))))
		assert.Equal(t, []int{1}, trashIDs(build(blue(1, 0), raw(2, 112*60+30, sysHome, owner, raitaru, pB(2)))))
		assert.Equal(t, []int{1}, trashIDs(build(blue(1, 0), raw(2, 141*60, sysHome, owner, fortizar, pB(2)))))
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 141*60+1, sysHome, owner, fortizar, pB(2)))))
	})

	t.Run("a structure lost to a third party is held and still anchors it", func(t *testing.T) {
		kms := []retrieval.RawKillmail{blue(1, 0), raw(2, 60, sysHome, owner, astrahus, pC(2))}
		assert.Equal(t, []int{1, 2}, ids(aggregate.ClassifyRawWar(kms, sides)))
		data := build(kms...)
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.Empty(t, data.A.Citadels, "the structure itself is outside any counted battle")
	})
}

func TestWarAnchorables(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	const bubble = 12200
	build := func(kms ...retrieval.RawKillmail) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	}

	t.Run("shot by the other side with no station anywhere, it is definite trash, makes no battle, and is not a ship", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, bubble, pB(2)))
		assert.Equal(t, []int{1}, trashIDs(data))
		assert.InDelta(t, 100, data.A.ISKTrashDefinite, 0.001)
		assert.Zero(t, data.A.ShipsLost)
		assert.Empty(t, data.Battles)
	})

	t.Run("after a station, it goes with that station's battle, past the 4 hours too", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, astrahus, pB(2)), raw(2, 5*60, sysHome, owner, bubble, pB(2)))
		assert.Equal(t, []int{2}, trashIDs(data))
		if assert.Len(t, data.Battles, 1) {
			assert.Equal(t, 1, data.Battles[0].A.TrashKillmails)
			assert.InDelta(t, 200, data.Battles[0].A.TrashDefiniteISK, 0.001)
		}
	})

	t.Run("a mobile tractor unit is never trash, in the window or with a repeat, and stays in the fights", func(t *testing.T) {
		const tractor = 33475
		assert.Empty(t, trashIDs(build(raw(1, 0, sysHome, owner, tractor, pB(2)))))
		data := build(
			raw(1, 0, sysHome, owner, astrahus, pB(2)),
			raw(2, 5, sysHome, pA1(5), tractor, pB(2)),
			raw(3, 6, sysHome, pA1(5), tractor, pB(2)),
		)
		assert.Empty(t, trashIDs(data))
		assert.InDelta(t, 500, data.A.ISKFights, 0.001)
		assert.Zero(t, data.A.ShipsLost, "nor is it a ship")
	})

	t.Run("one only a third party shot is not the war's", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, owner, bubble, pC(2))}
		assert.Empty(t, trashIDs(build(kms...)))
		assert.Empty(t, aggregate.ClassifyRawWar(kms, sides))
	})

	t.Run("a neutral's in the window is definite too", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, astrahus, pB(2)), raw(2, 60, sysHome, wp{corp: corpC1, ally: allyC}, bubble, pB(2)))
		assert.Equal(t, []int{2}, trashIDs(data))
		assert.InDelta(t, 200, data.A.ISKTrashDefinite, 0.001)
	})
}

func TestWarStationRef(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	const day = 24 * 60
	build := func(kms ...retrieval.RawKillmail) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	}
	// a station's reinforcement timers outside a wormhole, in minutes, from sysHome being neither
	const (
		mediumSoonest = 82*60 + 30
		mediumLatest  = 112*60 + 30
		largeSoonest  = 21 * 60
		largeLatest   = 141 * 60
	)
	// marks is the newest battle's marks that are, or are not, Station Destroyed
	marks := func(data *aggregate.WarData, destroyed bool) []aggregate.WarStationRef {
		for _, battle := range data.Battles {
			found := []aggregate.WarStationRef{}
			for _, mark := range battle.StationRefs {
				if mark.Destroyed == destroyed {
					found = append(found, mark)
				}
			}
			if len(found) > 0 {
				return found
			}
		}
		return nil
	}
	ref := func(data *aggregate.WarData) []aggregate.WarStationRef { return marks(data, false) }

	// noNotes leaves a mark's side, grade, and structure to compare
	noNotes := func(marks []aggregate.WarStationRef) []aggregate.WarStationRef {
		for index := range marks {
			marks[index].Notes = nil
			marks[index].DiedAt = time.Time{}
			marks[index].Kind = ""
		}
		return marks
	}

	t.Run("standup fighters with no structure dying are an attempt, in ISK under structures but not ships", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)))
		assert.Equal(t, aggregate.WarSideA, ref(data)[0].Side)
		assert.True(t, ref(data)[0].Attempt)
		assert.False(t, ref(data)[0].Unsure)
		assert.Equal(t, 1, data.A.RefAttempts)
		assert.Zero(t, data.A.StationRefs)
		assert.Zero(t, data.A.ShipsLost)
		assert.InDelta(t, 100, data.A.ISKStructures, 0.001)
		assert.Empty(t, data.A.Citadels)
		assert.Zero(t, data.Battles[0].A.StructureLost)
	})

	t.Run("a carrier's fighter marks nothing", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, carrierTemplar, pB(2)))
		assert.Nil(t, ref(data))
	})

	// firing is a fight where the attacker on the other side's loss flies ship, alongside a pilot
	firing := func(ship int) retrieval.RawKillmail {
		return onKill(raw(1, 0, sysHome, pB(2), rifter, owner, pA1(5)), ship)
	}

	t.Run("standup fighters used, or a side's station firing, mark that side, with no fighter lost", func(t *testing.T) {
		fighters := ref(build(firing(standupTemplar)))
		assert.Equal(t, aggregate.WarSideA, fighters[0].Side)
		assert.True(t, fighters[0].Attempt)
		assert.False(t, fighters[0].Unsure)
		data := build(firing(astrahus))
		assert.Equal(t, aggregate.WarSideA, ref(data)[0].Side)
		assert.True(t, ref(data)[0].Attempt)
		assert.Equal(t, 1, data.A.RefAttemptsUnsure)
		assert.Zero(t, data.B.RefAttemptsUnsure)
	})

	t.Run("a station alone among few pilots is unsure, with five pilots beside it it is not, and each says what fired where", func(t *testing.T) {
		data := build(firing(astrahus))
		marks := ref(data)
		assert.True(t, marks[0].Unsure)
		assert.Equal(t, 1, data.A.RefAttemptsUnsure)
		assert.Zero(t, data.A.RefAttempts)
		if assert.Len(t, marks[0].Notes, 1) {
			assert.False(t, marks[0].Notes[0].Lost)
			assert.Equal(t, 1, marks[0].Notes[0].Killmails)
			assert.NotEmpty(t, marks[0].Notes[0].Type)
		}

		data = build(onKill(raw(1, 0, sysHome, pB(2), rifter, owner, pA1(5), pA1(6), pA1(7), pA1(8), pA1(9)), astrahus))
		assert.False(t, ref(data)[0].Unsure)
		assert.Equal(t, 1, data.A.RefAttempts)
		assert.Zero(t, data.A.RefAttemptsUnsure)
	})

	t.Run("a station that goes on to die is a station ref, never unsure", func(t *testing.T) {
		first := ref(build(firing(raitaru), raw(2, 4*day, sysHome, owner, raitaru, pB(2))))
		assert.False(t, first[0].Attempt)
		assert.False(t, first[0].Unsure)
		later := ref(build(firing(raitaru), raw(2, 4*day+60, sysHome, owner, raitaru, pB(2))))
		assert.Equal(t, 60*time.Minute, later[0].DiedAt.Sub(first[0].DiedAt), "it says when the station died")
		assert.True(t, ref(build(firing(astrahus)))[0].DiedAt.IsZero())
	})

	t.Run("a carrier's fighter or a pilot firing marks nothing", func(t *testing.T) {
		assert.Nil(t, ref(build(firing(carrierTemplar))))
		assert.Nil(t, ref(build(raw(1, 0, sysHome, pB(2), rifter, pA1(5)))))
	})

	t.Run("a station that dies in the battle is destroyed, with no other mark for what fired there", func(t *testing.T) {
		dies := raw(2, 5, sysHome, owner, astrahus, pB(2))
		for _, data := range []*aggregate.WarData{build(firing(astrahus), dies), build(firing(standupTemplar), dies), build(dies)} {
			assert.Nil(t, ref(data))
			mark := marks(data, true)
			if assert.Len(t, mark, 1) {
				assert.Equal(t, aggregate.WarSideA, mark[0].Side)
				assert.Equal(t, "Astrahus", mark[0].Structure)
				if assert.Len(t, mark[0].Notes, 1) {
					assert.True(t, mark[0].Notes[0].Lost)
				}
			}
			assert.Zero(t, data.A.StationRefs)
			assert.Zero(t, data.A.RefAttempts)
		}
	})

	t.Run("another station firing where one dies can still be that fight's ref, and fighters there are taken for the dying one's", func(t *testing.T) {
		dies := raw(2, 5, sysHome, owner, astrahus, pB(2))
		data := build(firing(fortizar), dies, raw(3, 26*60, sysHome, owner, fortizar, pB(2)))
		var fight *aggregate.WarBattle
		for _, battle := range data.Battles {
			if battle.Start.Equal(data.Battles[len(data.Battles)-1].Start) {
				fight = battle
			}
		}
		destroyed, reffed := 0, 0
		for _, mark := range fight.StationRefs {
			if mark.Destroyed {
				destroyed++
			} else if !mark.Attempt {
				reffed++
				assert.Equal(t, "Fortizar", mark.Structure)
			}
		}
		assert.Equal(t, 1, destroyed)
		assert.Equal(t, 1, reffed)
		assert.Equal(t, 1, data.A.StationRefs)
	})

	t.Run("a station dying sooner after a fight than its timers allow is not that fight's ref", func(t *testing.T) {
		data := build(firing(astrahus), raw(2, mediumSoonest-1, sysHome, owner, astrahus, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
		assert.Len(t, marks(data, true), 1)

		data = build(firing(astrahus), raw(2, mediumSoonest, sysHome, owner, astrahus, pB(2)))
		assert.False(t, ref(data)[0].Attempt)
		assert.Equal(t, 1, data.A.StationRefs)
		assert.Len(t, marks(data, true), 1)
	})

	t.Run("a station firing is a station ref if it dies later, within its timer", func(t *testing.T) {
		data := build(firing(raitaru), raw(2, mediumLatest, sysHome, owner, raitaru, pB(2)))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Structure: "Raitaru"}}, noNotes(ref(data)))
		assert.Equal(t, 1, data.A.StationRefs)
		assert.True(t, ref(build(firing(raitaru), raw(2, mediumLatest+1, sysHome, owner, raitaru, pB(2))))[0].Attempt)
	})

	t.Run("a medium dying on its armor timer makes it a station ref, either side of that an attempt", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, mediumSoonest, sysHome, owner, raitaru, pB(2)))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Structure: "Raitaru"}}, noNotes(ref(data)))
		assert.Equal(t, 1, data.A.StationRefs)
		assert.Zero(t, data.A.RefAttempts)

		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, mediumLatest+1, sysHome, owner, raitaru, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, mediumSoonest-1, sysHome, owner, raitaru, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
	})

	t.Run("a large can die from a day after its first fight, low on power, to its two timers' latest", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, largeSoonest, sysHome, owner, fortizar, pB(2)))
		assert.False(t, ref(data)[0].Attempt)
		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, largeSoonest-1, sysHome, owner, fortizar, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, largeLatest, sysHome, owner, fortizar, pB(2)))
		assert.False(t, ref(data)[0].Attempt)
		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, largeLatest+1, sysHome, owner, fortizar, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
	})

	t.Run("a wormhole's timers are shorter", func(t *testing.T) {
		const hole = 31000001
		dies := func(at, ship int) *aggregate.WarData {
			return build(raw(1, 0, hole, owner, standupTemplar, pB(2)), raw(2, at, hole, owner, ship, pB(2)))
		}
		assert.False(t, ref(dies(58*60+30, raitaru))[0].Attempt)
		assert.True(t, ref(dies(58*60+29, raitaru))[0].Attempt)
		assert.False(t, ref(dies(88*60+30, raitaru))[0].Attempt)
		assert.True(t, ref(dies(88*60+31, raitaru))[0].Attempt)
		assert.False(t, ref(dies(117*60, fortizar))[0].Attempt)
		assert.True(t, ref(dies(117*60+1, fortizar))[0].Attempt)
	})

	t.Run("a tower firing is its kind's mark and never Ref'd, a customs office dying is its own kind", func(t *testing.T) {
		attacked := ref(build(firing(tower), raw(2, 4*day, sysHome, owner, tower, pB(2))))
		assert.Equal(t, aggregate.WarKindPOS, attacked[0].Kind)
		assert.True(t, attacked[0].Attempt)

		for _, test := range []struct {
			ship int
			kind string
		}{{astrahus, aggregate.WarKindCitadel}, {tower, aggregate.WarKindPOS}, {customs, aggregate.WarKindPOCO}} {
			died := marks(build(raw(1, 0, sysHome, owner, test.ship, pB(2))), true)
			if assert.Len(t, died, 1) {
				assert.Equal(t, test.kind, died[0].Kind)
			}
		}
	})

	t.Run("the structure must be that side's, there, and not dead before", func(t *testing.T) {
		enemy := wp{corp: corpB1, ally: allyB}
		assert.True(t, ref(build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 60, sysHome, enemy, raitaru, pA1(2))))[0].Attempt)
		assert.True(t, ref(build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 60, sysOther, owner, raitaru, pB(2))))[0].Attempt)
		assert.True(t, ref(build(raw(1, 0, sysHome, owner, raitaru, pB(2)), raw(2, 60, sysHome, owner, standupTemplar, pB(2))))[0].Attempt)
		assert.True(t, ref(build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 60, sysHome, owner, customs, pB(2))))[0].Attempt)
	})

	t.Run("a third party beside a side keeps its ref on what the war holds, its station dying outside any counted fight", func(t *testing.T) {
		third := wp{corp: corpC1, ally: allyC}
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pA1(5), rifter, pB(2)),
			raw(2, 1, sysHome, third, standupTemplar, pB(2)),
			raw(3, 4*day, sysHome, third, raitaru, pD(9)),
		}
		held := aggregate.ClassifyRawWar(kms, sides)
		assert.Equal(t, []int{1, 2, 3}, ids(held))
		data := aggregate.BuildWar(held, expand(held...), sides, 10, retrieval.WarEdits{})
		if refs := ref(data); assert.Len(t, refs, 1) {
			assert.False(t, refs[0].Attempt)
		}
	})

}

func TestWarDays(t *testing.T) {
	warSetup(t)
	sides := baseSides()
	owner := wp{corp: corpA1, ally: allyA}
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pA1(5), rifter, pB(2)),
		raw(2, 24*60, sysHome, owner, astrahus, pB(2)),
		raw(3, 24*60+60, sysHome, pC(7), rifter, pB(2)),
		raw(4, 48*60, sysOther, pB(3), rifter, pC(8)), // no side fought: forgotten
	}
	data := aggregate.BuildWar(kms, expand(kms...), sides, 10, retrieval.WarEdits{})
	if assert.Len(t, data.Days, 2) {
		assert.Equal(t, "2026-10-01", data.Days[0].Date)
		assert.Equal(t, 1, data.Days[0].A.ShipsFights)
		assert.Equal(t, "2026-10-02", data.Days[1].Date)
		assert.Equal(t, 1, data.Days[1].A.ShipsTrash)
		assert.InDelta(t, 200, data.Days[1].A.ISKStructures, 0.001)
		assert.InDelta(t, 300, data.Days[1].A.ISKTrash, 0.001)
		assert.Len(t, data.Days[1].A.Citadels, 1)
	}
	sum := data.Days[0].A.ISKLost + data.Days[1].A.ISKLost
	assert.InDelta(t, data.A.ISKLost, sum, 0.001, "the days add up to the war")
}

// warFiguresOf is what an edit that only regroups rows must leave alone
func warFiguresOf(data *aggregate.WarData) map[string]any {
	systems := map[string]aggregate.WarSystem{}
	for _, system := range data.Systems {
		system.Battles = 0 // a row's count, which regrouping does change
		systems[system.System.Name] = system
	}
	strip := func(m aggregate.WarMeasures) aggregate.WarMeasures {
		m.StationRefs, m.RefAttempts, m.RefAttemptsUnsure = 0, 0, 0 // marks are read per row
		return m
	}
	days := []aggregate.WarDay{}
	for _, day := range data.Days {
		day.A, day.B = strip(day.A), strip(day.B)
		days = append(days, day)
	}
	return map[string]any{"a": strip(data.A), "b": strip(data.B), "others": strip(data.Others), "days": days, "systems": systems, "groups": data.OtherGroups, "npc": data.NPCPilots}
}

func TestWarBattleEdits(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	sides := baseSides()
	// two fights an hour apart at home, then one elsewhere. C is beside B in the first, as A kills it, and beside A in the third, shooting B twice: merged and counted afresh, it would be beside A in both, moving its loss.
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pA1(1), rifter, pB(2)),
		raw(2, 5, sysHome, pB(2), rifter, pA1(3)),
		raw(7, 2, sysHome, pC(9), rifter, pA1(3)),
		raw(3, 60, sysHome, pA1(4), rifter, pB(5)),
		raw(4, 65, sysHome, pC(9), rifter, pA1(4)),
		raw(5, 120, sysOther, pB(6), rifter, pA1(7), pC(8)),
		raw(8, 121, sysOther, pB(10), rifter, pA1(7), pC(8)),
	}
	expanded := expand(kms...)
	build := func(edits ...retrieval.WarBattleEdit) *aggregate.WarData {
		return aggregate.BuildWar(kms, expanded, sides, 1, retrieval.WarEdits{Edits: edits})
	}
	plain := build()
	assert.Len(t, plain.Battles, 3)

	merge := retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditMerge, Groups: [][]int{{1, 2, 7}, {5, 8}}}

	t.Run("merging moves no total, and keeps each third party on the side it had", func(t *testing.T) {
		data := build(merge)
		assert.Len(t, data.Battles, 2)
		assert.Equal(t, warFiguresOf(plain), warFiguresOf(data))
		var merged *aggregate.WarBattle
		for _, battle := range data.Battles {
			if len(battle.KillmailIDs) == 5 {
				merged = battle
			}
		}
		if assert.NotNil(t, merged) {
			assert.Equal(t, []int{1, 7, 2, 5, 8}, merged.KillmailIDs)
			assert.Equal(t, []aggregate.WarEditMark{{ID: 1, Kind: "merge"}}, merged.Edits)
			assert.Equal(t, "Home", merged.System.Name)
			assert.Equal(t, []string{"Other"}, merged.OtherSystems)
		}
	})

	t.Run("splitting moves no total, even a piece with no kill between the sides", func(t *testing.T) {
		// 4 alone is A killing a third party, which would not count by itself
		data := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditSplit, Groups: [][]int{{3}, {4}}})
		assert.Len(t, data.Battles, 4)
		assert.Equal(t, warFiguresOf(plain), warFiguresOf(data))
		seen := map[string]bool{}
		for _, battle := range data.Battles {
			assert.False(t, seen[battle.ID], "ids stay unique")
			seen[battle.ID] = true
		}
	})

	t.Run("two pieces starting the same minute in a system get their own ids", func(t *testing.T) {
		same := []retrieval.RawKillmail{raw(1, 0, sysHome, pA1(1), rifter, pB(2)), raw(2, 0, sysHome, pB(3), rifter, pA1(4))}
		data := aggregate.BuildWar(same, expand(same...), sides, 1, retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{{ID: 1, Kind: retrieval.WarEditSplit, Groups: [][]int{{1}, {2}}}}})
		if assert.Len(t, data.Battles, 2) {
			assert.NotEqual(t, data.Battles[0].ID, data.Battles[1].ID)
		}
	})

	t.Run("an edit follows the battle holding most of its killmails, whose first killmail changed", func(t *testing.T) {
		later := append([]retrieval.RawKillmail{raw(6, -2, sysHome, pA1(1), rifter, pB(2))}, kms...)
		data := aggregate.BuildWar(later, expand(later...), sides, 1, retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{merge}})
		assert.Len(t, data.Battles, 2)
		ids := [][]int{}
		for _, battle := range data.Battles {
			ids = append(ids, battle.KillmailIDs)
		}
		assert.Contains(t, ids, []int{6, 1, 7, 2, 5, 8})
	})

	t.Run("a killmail new since a split goes where the nearest one in its system went", func(t *testing.T) {
		split := retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditSplit, Groups: [][]int{{1}, {2}}}
		later := append([]retrieval.RawKillmail{raw(6, 4, sysHome, pA1(1), rifter, pB(2))}, kms...)
		data := aggregate.BuildWar(later, expand(later...), sides, 1, retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{split}})
		ids := [][]int{}
		for _, battle := range data.Battles {
			ids = append(ids, battle.KillmailIDs)
		}
		assert.Contains(t, ids, []int{1, 7})
		assert.Contains(t, ids, []int{6, 2})
	})

	t.Run("an edit that finds nothing changes nothing, and comes back with its killmails", func(t *testing.T) {
		held := kms[3:] // the war's dates no longer reach the first fight
		data := aggregate.BuildWar(held, expand(held...), sides, 1, retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{merge}})
		assert.Len(t, data.Battles, 2)
		for _, battle := range data.Battles {
			assert.Empty(t, battle.Edits)
		}
		assert.Len(t, build(merge).Battles, 2)
	})

	t.Run("merging a split's pieces back marks the split once", func(t *testing.T) {
		data := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditSplit, Groups: [][]int{{3}, {4}}}, retrieval.WarBattleEdit{ID: 2, Kind: retrieval.WarEditMerge, Groups: [][]int{{3}, {4}}})
		for _, battle := range data.Battles {
			if len(battle.Edits) > 0 {
				assert.ElementsMatch(t, []aggregate.WarEditMark{{ID: 1, Kind: "split"}, {ID: 2, Kind: "merge"}}, battle.Edits)
			}
		}
	})

	t.Run("undoing an earlier edit keeps the later one", func(t *testing.T) {
		split := retrieval.WarBattleEdit{ID: 2, Kind: retrieval.WarEditSplit, Groups: [][]int{{3}, {4}}}
		both := build(merge, split)
		assert.Len(t, both.Battles, 3)
		assert.Len(t, build(split).Battles, 4)
	})
}

func TestWarIgnoreBattle(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	sides := baseSides()
	ignore := func(ids ...int) retrieval.WarEdits {
		return retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{{ID: 1, Kind: retrieval.WarEditIgnore, Groups: [][]int{ids}}}}
	}

	t.Run("its killmails leave every figure as if never held, and its row stays with its own", func(t *testing.T) {
		kept := []retrieval.RawKillmail{raw(1, 0, sysHome, pA1(1), rifter, pB(2)), raw(2, 120, sysHome, pC(9), rifter, pA1(3)), raw(4, 122, sysHome, pB(4), rifter, pA1(3))}
		apart := raw(3, 300, sysOther, pB(5), rifter, pA1(6), pC(7))
		all := append(append([]retrieval.RawKillmail{}, kept...), apart)

		data := aggregate.BuildWar(all, expand(all...), sides, 1, ignore(3))
		never := aggregate.BuildWar(kept, expand(kept...), sides, 1, retrieval.WarEdits{})
		assert.Equal(t, warFiguresOf(never), warFiguresOf(data))
		assert.Equal(t, never.BattleCount, data.BattleCount)
		assert.Equal(t, 1, data.IgnoredBattles)
		assert.Len(t, data.Killmails, 3)
		if assert.Len(t, data.Battles, 3) {
			row := data.Battles[0]
			assert.True(t, row.Ignored)
			assert.Equal(t, []int{3}, row.KillmailIDs)
			assert.Equal(t, 300.0, row.B.ISKLost, "its row keeps its own figures")
			assert.Equal(t, []aggregate.WarEditMark{{ID: 1, Kind: "ignore"}}, row.Edits)
		}
	})

	t.Run("a killmail new since it was ignored goes with it", func(t *testing.T) {
		all := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(1), rifter, pA1(2)), raw(2, 3, sysHome, pB(3), rifter, pA1(2))}
		data := aggregate.BuildWar(all, expand(all...), sides, 1, ignore(1))
		assert.Zero(t, data.B.Killmails)
		assert.Equal(t, 1, data.IgnoredBattles)
		assert.Equal(t, 1, data.Battles[0].IgnoredJoined, "and the row says so")
	})

	t.Run("the trash of its station leaves with it, and shows on its row", func(t *testing.T) {
		all := []retrieval.RawKillmail{
			raw(1, 0, sysHome, pB(1), astrahus, pA1(2)),
			raw(2, 30, sysHome, pB(5), rifter, pA1(2)), // in the 4 hour window, one attacker
			raw(3, 500, sysOther, pB(6), rifter, pA1(7)),
		}
		plain := aggregate.BuildWar(all, expand(all...), sides, 1, retrieval.WarEdits{})
		assert.NotZero(t, plain.B.ISKTrash)

		data := aggregate.BuildWar(all, expand(all...), sides, 1, ignore(1))
		assert.Zero(t, data.B.ISKTrash)
		assert.Equal(t, 1, data.B.Killmails, "only the fight elsewhere is left")
		assert.Len(t, data.Killmails, 1)
		assert.Len(t, data.Systems, 1)
		var row *aggregate.WarBattle
		for _, battle := range data.Battles {
			if battle.Ignored {
				row = battle
			}
		}
		if assert.NotNil(t, row) {
			assert.Equal(t, 200.0, row.B.TrashISK)
		}
	})

	t.Run("a station in it no longer confirms a mark elsewhere", func(t *testing.T) {
		templar := standupTemplar
		fight := raw(1, 0, sysHome, pB(1), templar, pA1(2))
		fall := raw(2, 5000, sysHome, pB(3), raitaru, pA1(4)) // within a medium's timer
		all := []retrieval.RawKillmail{fight, fall}

		marked := func(data *aggregate.WarData) (int, int) { return data.B.StationRefs, data.B.RefAttempts }
		confirmed, attempts := marked(aggregate.BuildWar(all, expand(all...), sides, 1, retrieval.WarEdits{}))
		assert.Equal(t, []int{1, 0}, []int{confirmed, attempts})
		confirmed, attempts = marked(aggregate.BuildWar(all, expand(all...), sides, 1, ignore(2)))
		assert.Equal(t, []int{0, 1}, []int{confirmed, attempts})
	})

	t.Run("a split's piece can be ignored, leaving the other", func(t *testing.T) {
		all := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(1), rifter, pA1(2)), raw(2, 5, sysHome, pB(3), rifter, pA1(2))}
		data := aggregate.BuildWar(all, expand(all...), sides, 1, retrieval.WarEdits{Edits: []retrieval.WarBattleEdit{
			{ID: 1, Kind: retrieval.WarEditSplit, Groups: [][]int{{1}, {2}}},
			{ID: 2, Kind: retrieval.WarEditIgnore, Groups: [][]int{{2}}},
		}})
		assert.Equal(t, 1, data.B.Killmails)
		assert.Equal(t, 1, data.IgnoredBattles)
		assert.Len(t, data.Battles, 2)
		assert.NotEqual(t, data.Battles[0].ID, data.Battles[1].ID)
	})
}

func TestWarBattleNotes(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(1), rifter, pA1(2)), raw(2, 3, sysHome, pB(3), rifter, pA1(2)), raw(3, 200, sysHome, pB(4), rifter, pA1(5))}
	notes := []retrieval.WarBattleNote{{ID: 1, Killmails: []int{1, 2}, Text: "first"}, {ID: 2, Killmails: []int{3}, Text: "second"}, {ID: 3, Killmails: []int{99}, Text: "gone"}}
	build := func(edits ...retrieval.WarBattleEdit) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), baseSides(), 1, retrieval.WarEdits{Edits: edits, Notes: notes})
	}
	texts := func(data *aggregate.WarData) [][]string {
		out := [][]string{}
		for _, battle := range data.Battles {
			row := []string{}
			for _, note := range battle.Notes {
				row = append(row, note.Text)
			}
			out = append(out, row)
		}
		return out
	}

	plain := build()
	assert.Equal(t, [][]string{{"second"}, {"first"}}, texts(plain), "each on its battle, and one whose battle has gone on none")
	merged := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditMerge, Groups: [][]int{{1, 2}, {3}}})
	assert.Equal(t, [][]string{{"first", "second"}}, texts(merged), "a merged battle keeps both")
	ignored := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditIgnore, Groups: [][]int{{3}}})
	assert.ElementsMatch(t, [][]string{{"first"}, {"second"}}, texts(ignored), "an ignored row keeps its note")
	assert.Equal(t, warFiguresOf(plain), warFiguresOf(build()), "notes change no figure")
}

func TestWarPeople(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))
	kms := []retrieval.RawKillmail{
		raw(1, 0, sysHome, pB(10), rifter, pA1(1), pA1(2)),
		raw(2, 3, sysHome, pB(10), capsule, pA1(1)),
		raw(3, 6, sysHome, pA2(1), rifter, pB(11)),
		// a later battle: pilot 1 is seen in a Rifter again, and is the same pilot once per battle per hull
		raw(4, 200, sysHome, pB(12), rifter, pA1(1)),
	}
	build := func(edits ...retrieval.WarBattleEdit) *aggregate.WarData {
		return aggregate.BuildWar(kms, expand(kms...), baseSides(), 1, retrieval.WarEdits{Edits: edits})
	}
	pilot := func(side aggregate.WarSidePeople, char int) aggregate.WarPilot {
		for _, row := range side.Pilots {
			if row.CharacterID == char {
				return row
			}
		}
		return aggregate.WarPilot{}
	}
	data := build()

	t.Run("a capsule counts in ISK and not as a kill, and everyone on a killmail has its full ISK", func(t *testing.T) {
		assert.Equal(t, float64(100+200+400), pilot(data.PeopleA, 1).ISKDestroyed)
		assert.Equal(t, 2, pilot(data.PeopleA, 1).Killmails)
		assert.Equal(t, float64(100), pilot(data.PeopleA, 2).ISKDestroyed)
		assert.Equal(t, 1, pilot(data.PeopleA, 2).Killmails)
		assert.Equal(t, 1, data.PeopleA.Pilots[0].CharacterID, "most ISK destroyed first")
	})

	t.Run("flown counts a pilot once per battle per hull, and no capsules", func(t *testing.T) {
		if assert.Len(t, data.PeopleA.Flown, 1) {
			assert.Equal(t, rifter, data.PeopleA.Flown[0].TypeID)
			// pilots 1 and 2 in the first battle, pilot 1 in the second
			assert.Equal(t, 3, data.PeopleA.Flown[0].Seen)
			assert.Equal(t, 1, data.PeopleA.Flown[0].Lost)
		}
		if assert.Len(t, data.PeopleB.Flown, 1) {
			// 10 and 11 in the first battle, 12 in the second
			assert.Equal(t, 3, data.PeopleB.Flown[0].Seen)
			assert.Equal(t, 2, data.PeopleB.Flown[0].Lost)
		}
	})

	t.Run("merged battles are one battle to flown", func(t *testing.T) {
		merged := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditMerge, Groups: [][]int{{1, 2, 3}, {4}}})
		assert.Equal(t, 2, merged.PeopleA.Flown[0].Seen)
	})

	t.Run("an ignored battle counts for no one", func(t *testing.T) {
		ignored := build(retrieval.WarBattleEdit{ID: 1, Kind: retrieval.WarEditIgnore, Groups: [][]int{{4}}})
		assert.Equal(t, 0, pilot(ignored.PeopleB, 12).CharacterID)
		assert.Equal(t, float64(100+200), pilot(ignored.PeopleA, 1).ISKDestroyed)
	})

	t.Run("a pilot who changed corps is under each, oldest stint first", func(t *testing.T) {
		if !assert.Len(t, data.PeopleA.Participants, 1) {
			return
		}
		group := data.PeopleA.Participants[0]
		assert.Equal(t, allyA, group.Group.ID)
		assert.Equal(t, 2, group.Count, "each pilot once")
		if !assert.Len(t, group.Corps, 2) {
			return
		}
		assert.Equal(t, corpA1, group.Corps[0].Corp.ID)
		for _, corp := range group.Corps {
			for _, row := range corp.Pilots {
				if row.CharacterID == 1 && assert.Len(t, row.Stints, 2) {
					assert.Equal(t, corpA1, row.Stints[0].CorporationID)
					assert.Equal(t, corpA2, row.Stints[1].CorporationID)
				}
				if row.CharacterID == 2 {
					assert.Nil(t, row.Stints)
				}
			}
		}
	})

	t.Run("a corp with no alliance sits at the top with its pilots", func(t *testing.T) {
		lone := wp{corp: corpLone, char: 30}
		sides := baseSides()
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(10), rifter, pA1(1), lone)}
		others := aggregate.BuildWar(kms, expand(kms...), sides, 1, retrieval.WarEdits{})
		found := false
		for _, side := range []aggregate.WarSidePeople{others.PeopleA, others.PeopleOthers} {
			for _, group := range side.Participants {
				if group.Group.ID == corpLone {
					found = true
					assert.Equal(t, "CORPORATION", group.Group.Type)
					assert.Len(t, group.Pilots, 1)
					assert.Empty(t, group.Corps)
				}
			}
		}
		assert.True(t, found)
	})

	t.Run("a pilot of a side has their NPC corp time left out, with no change noted", func(t *testing.T) {
		rookie := wp{corp: npcCorp, char: 40}
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(10), rifter, rookie, pA1(1)), raw(2, 200, sysHome, pB(11), rifter, pA1(40)), raw(3, 400, sysHome, pB(12), rifter, wp{corp: npcCorp, char: 41}, pA1(1))}
		data := aggregate.BuildWar(kms, expand(kms...), baseSides(), 1, retrieval.WarEdits{})
		assert.Equal(t, 0, pilot(data.PeopleOthers, 40).CharacterID, "not under Others")
		assert.Equal(t, float64(200), pilot(data.PeopleA, 40).ISKDestroyed, "only the killmail from their time on the side")
		assert.Equal(t, 41, pilot(data.PeopleOthers, 41).CharacterID, "an NPC corp pilot who is on no side stays under Others")
		assert.Equal(t, float64(100+300), pilot(data.PeopleA, 1).ISKDestroyed, "others on those killmails keep them")
		for _, group := range data.PeopleA.Participants {
			for _, corp := range group.Corps {
				for _, row := range corp.Pilots {
					assert.Nil(t, row.Stints)
				}
			}
		}
	})

	t.Run("a pilot on both sides shows the corp they had on each", func(t *testing.T) {
		kms := []retrieval.RawKillmail{raw(1, 0, sysHome, pB(10), rifter, pA1(50)), raw(2, 200, sysHome, pA2(11), rifter, pB(50))}
		data := aggregate.BuildWar(kms, expand(kms...), baseSides(), 1, retrieval.WarEdits{})
		assert.Equal(t, corpA1, pilot(data.PeopleA, 50).Corporation.ID)
		assert.Equal(t, corpB1, pilot(data.PeopleB, 50).Corporation.ID)
	})

	t.Run("people change no figure", func(t *testing.T) {
		assert.Equal(t, warFiguresOf(data), warFiguresOf(build()))
	})
}
