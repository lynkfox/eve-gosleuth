// Command tagcheck prints the fit and tags of killmails, for tuning static_data/fit_tag_rules.json against real losses.
//
//	go run ./cmd/tagcheck 139498664 https://zkillboard.com/kill/139506855/ saved.json
//
// Each argument is a killmail id, a zKillboard killmail link, or an ESI killmail saved as json. Ids and links are looked up on zKillboard for their hash, then read from the ESI.
// It reads the static data beside the repo's exe as the app does, so an edited copy in build/bin/static_data is tried too.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lynkfox/go_eve_sleuth/aggregate"
	"github.com/lynkfox/go_eve_sleuth/api"
	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
)

// killmailLink finds the id in a zKillboard killmail link
var killmailLink = regexp.MustCompile(`/kill/(\d+)`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/tagcheck <killmail id, zKillboard link, or ESI killmail json> ...")
		os.Exit(2)
	}
	comm.InitLogger(logrus.WarnLevel)
	if err := comm.InitStaticData(staticdata.Files, os.DirFS(filepath.Join("build", "bin", staticdata.OverrideFolderName))); err != nil {
		fmt.Fprintln(os.Stderr, "static data:", err)
	}
	esi := api.NewESIClient()
	cache := retrieval.NewCache(esi, filepath.Join(os.TempDir(), "eve-gosleuth-tagcheck.db"))

	for _, arg := range os.Args[1:] {
		killmail, err := readKillmail(esi, arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", arg, err)
			continue
		}
		npcs := aggregate.PvENPCs(attackers(killmail), cache)
		printFit(arg, cache.BuildFit(killmail.Victim.ShipTypeID, killmail.Victim.Items), npcs)
	}
}

// attackers are a killmail's attackers with only the ids aggregate.IsPvENPC reads, so no names are looked up
func attackers(killmail *api.ESIKillmail) []*retrieval.KillmailAttacker {
	listed := []*retrieval.KillmailAttacker{}
	for _, attacker := range killmail.Attackers {
		each := &retrieval.KillmailAttacker{}
		if attacker.CharacterID != nil {
			each.Pilot = &retrieval.Pilot{CharacterID: *attacker.CharacterID}
		}
		if attacker.CorporationID != nil {
			each.CorpAtKill = &retrieval.Corporation{CorporationID: *attacker.CorporationID}
		}
		if attacker.ShipTypeID != 0 {
			each.Ship = &retrieval.Item{ItemID: attacker.ShipTypeID}
		}
		listed = append(listed, each)
	}
	return listed
}

// readKillmail reads a saved ESI killmail, or looks up an id or link's hash on zKillboard and reads it from the ESI
func readKillmail(esi *api.ESIConnection, arg string) (*api.ESIKillmail, error) {
	if strings.HasSuffix(arg, ".json") {
		data, err := os.ReadFile(arg)
		if err != nil {
			return nil, err
		}
		killmail := &api.ESIKillmail{}
		return killmail, json.Unmarshal(data, killmail)
	}

	if match := killmailLink.FindStringSubmatch(arg); match != nil {
		arg = match[1]
	}
	id, err := strconv.Atoi(arg)
	if err != nil {
		return nil, fmt.Errorf("not a killmail id, link, or json file")
	}
	hash, err := zkillHash(id)
	if err != nil {
		return nil, err
	}
	return esi.GetKillmail(api.ZkbResponse{KillmailID: id, Zkb: api.Zkb{Hash: hash}})
}

// zkillHash looks up a killmail's hash on zKillboard, which the ESI needs to read it
func zkillHash(id int) (string, error) {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://zkillboard.com/api/killID/%d/", id), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "eve-gosleuth tagcheck (+https://github.com/lynkfox/go_eve_sleuth)")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("zKillboard answered %s, try again shortly", response.Status)
	}
	entries := []api.ZkbResponse{}
	if err := json.NewDecoder(response.Body).Decode(&entries); err != nil {
		return "", fmt.Errorf("reading zKillboard: %w", err)
	}
	if len(entries) == 0 || entries[0].Zkb.Hash == "" {
		return "", fmt.Errorf("zKillboard has no killmail %d", id)
	}
	return entries[0].Zkb.Hash, nil
}

// printFit prints the fit's racks and bays, then its tags in the order the app shows them, marking the one that names the row.
// npcs are the rats on its killmail, which count toward Ratting. Fleet Doctrine needs the battle, so is never shown.
func printFit(label string, fit *retrieval.Fit, npcs int) {
	fmt.Printf("== %s  %s  (%d rats on the killmail)\n", label, fit.Hull.Name, npcs)
	for _, rack := range []struct {
		name  string
		slots []*retrieval.FittedModule
	}{{"high", fit.Highs}, {"mid", fit.Mids}, {"low", fit.Lows}, {"rig", fit.Rigs}, {"sub", fit.Subsystems}} {
		names := []string{}
		for _, slot := range rack.slots {
			if slot == nil {
				names = append(names, "-")
			} else {
				names = append(names, slot.Module.Name)
			}
		}
		if len(names) > 0 {
			fmt.Printf("   %-5s %s\n", rack.name, strings.Join(names, " | "))
		}
	}
	for _, bay := range [][]*retrieval.FitCargo{fit.Drones, fit.Fighters} {
		for _, stack := range bay {
			fmt.Printf("   bay   %s x%d\n", stack.Item.Name, stack.Quantity)
		}
	}

	named := false
	for _, tag := range fit.TagsWith(retrieval.FitTagContext{NPCs: npcs}) {
		mark := "      "
		switch {
		case tag.Hidden:
			mark = "hidden"
		case tag.Role && !named:
			mark, named = "NAME  ", true
		}
		reasons := []string{}
		for _, reason := range tag.Reasons {
			count := ""
			if reason.Count > 1 {
				count = fmt.Sprintf(" ×%d", reason.Count)
			}
			bonus := ""
			if reason.Bonus != 0 {
				bonus = fmt.Sprintf(" (hull bonus %+g)", math.Round(reason.Bonus*100)/100)
			}
			reasons = append(reasons, fmt.Sprintf("%s%s %+g%s", reason.Name, count, math.Round(reason.Weight*100)/100, bonus))
		}
		fmt.Printf("   %s %-12s priority %2d  weight %5.2f of %g  share %4.2f  <- %s\n", mark, tag.Name, tag.Priority, tag.Weight, tag.Threshold, tag.Share, strings.Join(reasons, ", "))
	}
}
