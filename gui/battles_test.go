package gui

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestBuildBattles(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)

	jita := &retrieval.System{SystemID: 30000142, Name: "Jita"}
	perimeter := &retrieval.System{SystemID: 30000144, Name: "Perimeter"}
	rifter := &retrieval.Item{ItemID: 587, Name: "Rifter"}
	slasher := &retrieval.Item{ItemID: 585, Name: "Slasher"}
	pilot := func(id int) *retrieval.Pilot { return &retrieval.Pilot{CharacterID: id} }
	start := time.Date(2026, 10, 30, 11, 50, 0, 0, time.UTC)
	killmail := func(id int, system *retrieval.System, at time.Duration, victim int, attackers ...*retrieval.KillmailAttacker) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailID: id, System: system, KillmailTime: start.Add(at),
			Victim: &retrieval.KillmailVictim{Pilot: pilot(victim), Ship: rifter}, Attackers: attackers}
	}
	attacker := func(id int, ship *retrieval.Item) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: pilot(id), Ship: ship}
	}
	npc := &retrieval.KillmailAttacker{Pilot: &retrieval.Pilot{}, Ship: slasher}

	// a fight of 10 pilots through a gate, most of it in Perimeter, pilot 2 swapping hulls, then an older fight in Jita of too few pilots
	gang := []*retrieval.KillmailAttacker{attacker(2, rifter), npc}
	for id := 10; id < 16; id++ {
		gang = append(gang, attacker(id, rifter))
	}
	killmails := []*retrieval.Killmail{
		killmail(1, jita, 0, 1, gang...),
		killmail(2, perimeter, 10*time.Minute, 3, attacker(2, slasher)),
		killmail(3, perimeter, 20*time.Minute, 4, attacker(2, slasher)),
		killmail(4, jita, -48*time.Hour, 5, attacker(6, rifter)),
		killmail(5, jita, -48*time.Hour+time.Minute, 6, attacker(5, rifter)),
		killmail(6, jita, 72*time.Hour, 7, attacker(8, rifter)), // on its own, not a battle
	}
	collected := aggregate.CollectedData{BattlesFought: map[string]aggregate.BattleFought{}}
	collected.FindBattles(killmails)

	battles := buildBattles(collected.BattlesFought, killmails)
	if !assert.Len(t, battles, 1, "the Jita fight of 2 pilots is left out") {
		return
	}

	newest := battles[0]
	assert.Equal(t, start, newest.Start)
	assert.Equal(t, start.Add(20*time.Minute), newest.End)
	assert.Equal(t, "Perimeter", newest.System.Name, "the system with the most killmails")
	assert.Equal(t, []string{"Jita"}, newest.OtherSystems)
	assert.Equal(t, 3, newest.Killmails)
	assert.Equal(t, 10, newest.Pilots, "the NPC is no pilot")
	assert.Equal(t, 11, newest.Ships, "pilot 2's Rifter and Slasher each count")
	assert.Equal(t, "https://zkillboard.com/related/30000144/202610301200/", newest.ZkillURL, "the hour of its middle")
	assert.Equal(t, "https://br.evetools.org/related/30000144/202610301200", newest.EveToolsURL)
	assert.Equal(t, "https://warbeacon.net/br/related/30000144/202610301200/", newest.WarbeaconURL)

	assert.Empty(t, buildBattles(map[string]aggregate.BattleFought{}, killmails))
}
