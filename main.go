package main

import (
	"fmt"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/sirupsen/logrus"
)

func main() {

	comm.InitLogger(logrus.DebugLevel)
	comm.InitStaticData("./static_data")

	target := retrieval.IntelTarget{
		IDNumber: 2121139046,
		Name:     "Luxxianna Seraph",
		Clients: retrieval.CollectionClientPool{
			ZkillClient: api.NewZkillClient(),
			ESIClient:   api.NewESIClient(),
		},
	}

	loaded_cache, _ := retrieval.LoadCache(target.Clients.ESIClient)

	target.Expanded = loaded_cache.ExpandPilot(target.IDNumber, true)

	killmails, err := retrieval.CollectKillmails(target)
	if err != nil {
		comm.Logger.WithError(err).Warnf("Continuing with %d collected killmails", len(*killmails))
	}

	collected_data := aggregate.CollateAllKillmailData(killmails, &target.Clients.ESIClient,
		loaded_cache)

	fleetmates := aggregate.GetMostFrequentFleetmates(10, collected_data.PilotsSeen, &target)

	for i := range fleetmates {
		fmt.Printf("\nPilot Name: %s | KMs Shared: %d | Over Days: %d", fleetmates[i].Pilot.Name, fleetmates[i].TotalKillmails, fleetmates[i].UniqueDays)
	}

	retrieval.SaveCache(*loaded_cache)
}
