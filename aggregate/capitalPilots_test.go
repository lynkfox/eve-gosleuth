package aggregate_test

import (
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGetCapitalPilots(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	naglfar := &retrieval.Item{ItemID: 19722, Name: "Naglfar"}
	revelation := &retrieval.Item{ItemID: 19720, Name: "Revelation"}
	rifter := &retrieval.Item{ItemID: 587, Name: "Rifter"}

	ursaPrimus := &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS"}
	ursaMinor := &retrieval.Corporation{CorporationID: 98000002, Name: "Ursa Minor"}
	outsiders := &retrieval.Corporation{CorporationID: 98000003, Name: "Outsiders"}
	ursa := &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."}

	pilot := func(id int, name string) *retrieval.Pilot { return &retrieval.Pilot{CharacterID: id, Name: name} }
	attacker := func(who *retrieval.Pilot, ship *retrieval.Item, corp *retrieval.Corporation, alliance *retrieval.Alliance) *retrieval.KillmailAttacker {
		return &retrieval.KillmailAttacker{Pilot: who, Ship: ship, CorpAtKill: corp, AllianceAtKill: alliance}
	}

	zed, amy, bob, outsider := pilot(1, "zed"), pilot(2, "Amy"), pilot(3, "Bob"), pilot(4, "Outsider")
	killmails := []*retrieval.Killmail{
		{
			KillmailID: 1,
			Victim:     &retrieval.KillmailVictim{Pilot: bob, Ship: revelation, CorpAtKill: ursaMinor, AllianceAtKill: ursa},
			Attackers: []*retrieval.KillmailAttacker{
				attacker(zed, naglfar, ursaPrimus, ursa),
				attacker(amy, naglfar, ursaPrimus, ursa),
				attacker(outsider, naglfar, outsiders, nil),
				attacker(pilot(5, "Subcap"), rifter, ursaPrimus, ursa),
				{Pilot: &retrieval.Pilot{CharacterID: 19722, NPC: true}, Ship: naglfar},
			},
		},
		{
			KillmailID: 2,
			Victim:     &retrieval.KillmailVictim{Ship: rifter},
			Attackers:  []*retrieval.KillmailAttacker{attacker(zed, naglfar, ursaPrimus, ursa)},
		},
	}

	names := func(capitals []aggregate.CapitalShipPilots) map[string][]string {
		found := map[string][]string{}
		for _, capital := range capitals {
			for _, flown := range capital.Pilots {
				found[capital.Ship.Name] = append(found[capital.Ship.Name], flown.Name)
			}
		}
		return found
	}

	t.Run("an alliance's capital pilots, by ship then name, each once, victims included", func(t *testing.T) {
		capitals := aggregate.GetCapitalPilots(killmails, &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE})

		assert.Equal(t, []string{"Naglfar", "Revelation"}, []string{capitals[0].Ship.Name, capitals[1].Ship.Name})
		assert.Equal(t, map[string][]string{"Naglfar": {"Amy", "zed"}, "Revelation": {"Bob"}}, names(capitals),
			"no outsiders, sub capitals, or NPCs, and zed once despite two killmails")
	})

	t.Run("a corp only counts pilots who flew for that corp", func(t *testing.T) {
		capitals := aggregate.GetCapitalPilots(killmails, &retrieval.IntelTarget{IDNumber: 98744491, TypeOfEntity: types.CORPORATION})

		assert.Equal(t, map[string][]string{"Naglfar": {"Amy", "zed"}}, names(capitals), "Bob flew for Ursa Minor, a different corp in the alliance")
	})

	t.Run("no capitals is an empty list", func(t *testing.T) {
		assert.Empty(t, aggregate.GetCapitalPilots(killmails, &retrieval.IntelTarget{IDNumber: 1, TypeOfEntity: types.CORPORATION}))
		assert.NotNil(t, aggregate.GetCapitalPilots(nil, &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE}))
	})
}

func TestGetCapitalPilotsSeen(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)
	assert.NoError(t, comm.InitStaticData(staticdata.Files))

	naglfar := &retrieval.Item{ItemID: 19722, Name: "Naglfar"}
	corp := &retrieval.Corporation{CorporationID: 98744491, Name: "URSA PRIMUS"}
	ursa := &retrieval.Alliance{AllianceID: 99013330, Name: "URSA."}
	zed := &retrieval.Pilot{CharacterID: 1, Name: "zed"}
	jita, amarr := &retrieval.System{SystemID: 1, Name: "Jita"}, &retrieval.System{SystemID: 2, Name: "Amarr"}
	early, late := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	flown := func(at time.Time, system *retrieval.System) *retrieval.Killmail {
		return &retrieval.Killmail{KillmailTime: at, System: system, Victim: &retrieval.KillmailVictim{},
			Attackers: []*retrieval.KillmailAttacker{{Pilot: zed, Ship: naglfar, CorpAtKill: corp, AllianceAtKill: ursa}}}
	}

	// the newest is in the middle of the list, so order does not decide it
	capitals := aggregate.GetCapitalPilots([]*retrieval.Killmail{flown(early, jita), flown(late, amarr), flown(early.Add(time.Hour), jita)},
		&retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE})

	assert.Len(t, capitals, 1)
	assert.Equal(t, aggregate.Sighting{Time: late, System: amarr}, capitals[0].Seen[1])
	assert.Len(t, capitals[0].Seen, 1)
}

func TestHasLeftTarget(t *testing.T) {
	ursa := &retrieval.Alliance{AllianceID: 99013330}
	other := &retrieval.Alliance{AllianceID: 99000001}
	inCorp := func(corpID int, alliance *retrieval.Alliance) *retrieval.Pilot {
		return &retrieval.Pilot{Corporation: &retrieval.Corporation{CorporationID: corpID, Alliance: alliance}}
	}

	allianceTarget := &retrieval.IntelTarget{IDNumber: 99013330, TypeOfEntity: types.ALLIANCE}
	corpInAlliance := &retrieval.IntelTarget{IDNumber: 98744491, TypeOfEntity: types.CORPORATION, Expanded: &retrieval.Corporation{CorporationID: 98744491, Alliance: ursa}}
	corpAlone := &retrieval.IntelTarget{IDNumber: 98744491, TypeOfEntity: types.CORPORATION, Expanded: &retrieval.Corporation{CorporationID: 98744491}}

	cases := []struct {
		name   string
		pilot  *retrieval.Pilot
		target *retrieval.IntelTarget
		left   bool
	}{
		{"still in the alliance, even in another of its corps", inCorp(98000002, ursa), allianceTarget, false},
		{"moved to another alliance", inCorp(98000002, other), allianceTarget, true},
		{"in a corp with no alliance", inCorp(98000002, nil), allianceTarget, true},
		{"a corp target's member who changed corp within the alliance has not left", inCorp(98000002, ursa), corpInAlliance, false},
		{"a corp target's member who left its alliance", inCorp(98000002, other), corpInAlliance, true},
		{"a corp with no alliance: still in the corp", inCorp(98744491, nil), corpAlone, false},
		{"a corp with no alliance: changed corp", inCorp(98000002, nil), corpAlone, true},
		{"unknown current corp, ie a deleted character, is not marked", &retrieval.Pilot{}, allianceTarget, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.left, aggregate.HasLeftTarget(c.pilot, c.target))
		})
	}
}
