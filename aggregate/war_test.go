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

	t.Run("a battle's structures go by kind, most lost first, towers as one kind", func(t *testing.T) {
		kms := []retrieval.RawKillmail{
			raw(1, 0, sysHome, wp{corp: corpA1, ally: allyA}, astrahus, pB(2)),
			raw(2, 1, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
			raw(3, 2, sysHome, wp{corp: corpA1, ally: allyA}, tower, pB(2)),
		}
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
		data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
		return aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
		return aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
		return aggregate.BuildWar(kms, expand(kms...), sides, 10)
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

	t.Run("only as far before as its timer: 3 days for a medium, 4 for a large", func(t *testing.T) {
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 3*day+1, sysHome, owner, raitaru, pB(2)))))
		assert.Equal(t, []int{1}, trashIDs(build(blue(1, 0), raw(2, 3*day, sysHome, owner, raitaru, pB(2)))))
		assert.Equal(t, []int{1}, trashIDs(build(blue(1, 0), raw(2, 4*day, sysHome, owner, fortizar, pB(2)))))
		assert.Empty(t, trashIDs(build(blue(1, 0), raw(2, 4*day+1, sysHome, owner, fortizar, pB(2)))))
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
		return aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
		return aggregate.BuildWar(kms, expand(kms...), sides, 10)
	}
	ref := func(data *aggregate.WarData) []aggregate.WarStationRef {
		for _, battle := range data.Battles {
			if len(battle.StationRefs) > 0 {
				return battle.StationRefs
			}
		}
		return nil
	}

	t.Run("standup fighters with no structure dying are an attempt, in ISK under structures but not ships", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Attempt: true}}, ref(data))
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
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Attempt: true}}, ref(build(firing(standupTemplar))))
		data := build(firing(astrahus))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Attempt: true}}, ref(data))
		assert.Equal(t, 1, data.A.RefAttempts)
		assert.Zero(t, data.B.RefAttempts)
	})

	t.Run("a carrier's fighter or a pilot firing marks nothing", func(t *testing.T) {
		assert.Nil(t, ref(build(firing(carrierTemplar))))
		assert.Nil(t, ref(build(raw(1, 0, sysHome, pB(2), rifter, pA1(5)))))
	})

	t.Run("a station firing that dies in the battle marks nothing, though fighters used still do", func(t *testing.T) {
		dies := raw(2, 5, sysHome, owner, astrahus, pB(2))
		assert.Nil(t, ref(build(firing(astrahus), dies)))

		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Structure: "Astrahus"}}, ref(build(firing(standupTemplar), dies)))
	})

	t.Run("a station firing is a station ref if it dies later, within its timer", func(t *testing.T) {
		data := build(firing(raitaru), raw(2, 3*day, sysHome, owner, raitaru, pB(2)))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Structure: "Raitaru"}}, ref(data))
		assert.Equal(t, 1, data.A.StationRefs)
		assert.True(t, ref(build(firing(raitaru), raw(2, 3*day+1, sysHome, owner, raitaru, pB(2))))[0].Attempt)
	})

	t.Run("a medium dying within 3 days makes it a station ref, past that an attempt", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 3*day, sysHome, owner, raitaru, pB(2)))
		assert.Equal(t, []aggregate.WarStationRef{{Side: aggregate.WarSideA, Structure: "Raitaru"}}, ref(data))
		assert.Equal(t, 1, data.A.StationRefs)
		assert.Zero(t, data.A.RefAttempts)

		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 3*day+1, sysHome, owner, raitaru, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
	})

	t.Run("a large gets 4 days", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 4*day, sysHome, owner, fortizar, pB(2)))
		assert.False(t, ref(data)[0].Attempt)
		data = build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 4*day+1, sysHome, owner, fortizar, pB(2)))
		assert.True(t, ref(data)[0].Attempt)
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
			raw(3, 3*60, sysHome, third, raitaru, pD(9)),
		}
		held := aggregate.ClassifyRawWar(kms, sides)
		assert.Equal(t, []int{1, 2, 3}, ids(held))
		data := aggregate.BuildWar(held, expand(held...), sides, 10)
		if refs := ref(data); assert.Len(t, refs, 1) {
			assert.False(t, refs[0].Attempt)
		}
	})

	t.Run("the fight the structure dies in counts", func(t *testing.T) {
		data := build(raw(1, 0, sysHome, owner, standupTemplar, pB(2)), raw(2, 5, sysHome, owner, raitaru, pB(2)))
		assert.False(t, ref(data)[0].Attempt)
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
	data := aggregate.BuildWar(kms, expand(kms...), sides, 10)
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
