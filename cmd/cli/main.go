package main

import (
	"fmt"
	"strings"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
)

func main() {

	comm.InitLogger(logrus.DebugLevel)
	comm.InitStaticData(staticdata.Files, staticdata.Overrides()...)

	loaded_cache, err := retrieval.LoadCache(api.NewESIClient(), retrieval.DefaultCachePath())
	if err != nil {
		comm.Logger.WithError(err).Warn("Starting with an empty cache")
	}
	if release, err := api.NewSDEClient().GetLatestSDERelease(); err == nil {
		loaded_cache.UpdateSDERelease(release)
	}

	// every ESI call goes through the cache
	target := retrieval.IntelTarget{
		IDNumber:     2121139046,
		Name:         "Luxxianna Seraph",
		TypeOfEntity: types.PILOT,
		Clients: retrieval.CollectionClientPool{
			ZkillClient: api.NewZkillClient(),
			ESIClient:   loaded_cache,
		},
	}

	target.Expanded = loaded_cache.ExpandPilot(target.IDNumber, true)

	killmails, err := retrieval.CollectKillmails(target, retrieval.DefaultKillmailLimits())
	if err != nil {
		comm.Logger.WithError(err).Warnf("Continuing with %d collected killmails", len(*killmails))
	}

	collected_data := aggregate.CollateAllKillmailData(killmails, &target.Clients.ESIClient,
		loaded_cache)

	// each of the target's losses as EFT, to check the fits read from the killmails
	for _, killmail := range collected_data.Killmails {
		if aggregate.IsTargetLoss(&target, killmail) && killmail.Victim.Fit != nil {
			fmt.Printf("\n%s\n%s\n", strings.Repeat("-", 40), killmail.Victim.Fit.EFT(retrieval.EFTTitle(killmail)))
		}
	}

	fleetmates := aggregate.GetMostFrequentFleetmates(10, collected_data.PilotsSeen, &target)

	for i := range fleetmates {
		fmt.Printf("\nPilot Name: %s | KMs Shared: %d | Over Days: %d", fleetmates[i].Pilot.Name, fleetmates[i].TotalKillmails, fleetmates[i].UniqueDays)
	}

	if err := retrieval.SaveCache(loaded_cache); err != nil {
		comm.Logger.WithError(err).Error("Failed to save cache")
	}
}
