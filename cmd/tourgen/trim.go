package main

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/lynkfox/go_eve_sleuth/gui"
)

const (
	// killmailBudget is about how many killmails a demo report keeps
	killmailBudget = 60
	// battleBudget is the most killmails the kept battles may take, so the rest of the budget goes to variety
	battleBudget = 30
	// maxBattles is the most battles kept
	maxBattles = 4
	// maxFitGroups is the most distinct fits kept
	maxFitGroups = 14
	// maxStructureRows is the most rows kept of each structure list, which can hold hundreds
	maxStructureRows = 8
	// maxRatters and maxPIPilots are the most rows kept of those system tabs, each with all its losses
	maxRatters  = 10
	maxPIPilots = 8
	// maxMemberRows is the most member rows kept
	maxMemberRows = 150
)

// killLink finds the killmail id in a zKillboard kill url
var killLink = regexp.MustCompile(`/kill/(\d+)`)

// trimmer cuts a report to its kept killmails and keeps the rest of the report in step with them
type trimmer struct {
	report *gui.IntelReport
	keep   map[int]bool
	ratio  float64 // kept killmails over all of them, the factor the counts the backend made from the whole list are cut by
	days   map[string]bool
}

// killmailsIn lists the killmails a battle was made of: the ones in its time span in its systems
func killmailsIn(battle gui.Battle, killmails []gui.KillmailCard) []int {
	var ids []int
	for _, km := range killmails {
		if km.Time.Before(battle.Start) || km.Time.After(battle.End) {
			continue
		}
		if km.SystemID == battle.System.ID || slices.Contains(battle.OtherSystems, km.SystemName) {
			ids = append(ids, km.ID)
		}
	}
	return ids
}

/*
	trimReport cuts a report to about killmailBudget killmails, keeping every tab filled as far as the source allows.

The killmails kept are those of a few whole battles, the ones the fits, capitals, structure kills, PI pilots, and ratters point at, then the newest of the rest,
alternating kills and losses. Lists built from the killmails are cut to match: a fit loses its losses that were cut, systems are counted again from what is left,
and the counts the backend made from killmails that have no data here (a pilot's killmails shared, an ally's kills) are cut by the same ratio, never below 1.

Parameters:
  - report: the report, changed in place

Returns:
  - *trimmer: what cut it, to cut the other lists the demo has by the same ratio
*/
func trimReport(report *gui.IntelReport) *trimmer {
	t := &trimmer{report: report, keep: map[int]bool{}}
	if len(report.Ratters) > maxRatters {
		report.Ratters = report.Ratters[:maxRatters]
	}
	if len(report.PIPilots) > maxPIPilots {
		report.PIPilots = report.PIPilots[:maxPIPilots]
	}
	all := report.Killmails
	byID := map[int]gui.KillmailCard{}
	for _, km := range all {
		byID[km.ID] = km
	}
	add := func(id int) {
		if _, ok := byID[id]; ok {
			t.keep[id] = true
		}
	}

	// whole battles first: their counts are the backend's and can only stay true if every killmail stays
	battles := slices.Clone(report.Battles)
	slices.SortFunc(battles, func(a, b gui.Battle) int { return b.Pilots - a.Pilots })
	var keptBattles []gui.Battle
	taken := 0
	for _, battle := range battles {
		ids := killmailsIn(battle, all)
		if len(ids) != battle.Killmails || taken+len(ids) > battleBudget || len(keptBattles) == maxBattles {
			continue
		}
		taken += len(ids)
		keptBattles = append(keptBattles, battle)
		for _, id := range ids {
			add(id)
		}
	}
	// a battle shares killmails with another kept one only when they overlap, so the kept battles' counts are checked at the end

	// what the other tabs point at
	for _, group := range pickFitGroups(report.Fits) {
		add(group.Losses[0].KillmailID)
	}
	for _, capital := range report.Capitals {
		for _, pilot := range capital.Pilots {
			for _, km := range all {
				if km.Time.Equal(pilot.LastSeen) && km.SystemName == pilot.LastSystem {
					add(km.ID)
				}
			}
		}
	}
	for _, pilot := range report.PIPilots {
		for _, loss := range pilot.Losses {
			add(loss.ID)
		}
	}
	for _, ratter := range report.Ratters {
		for _, loss := range ratter.Losses {
			add(loss.ID)
		}
	}
	if report.Structures != nil {
		for _, used := range report.Structures.Used {
			if match := killLink.FindStringSubmatch(used.ZkillURL); match != nil {
				id, _ := strconv.Atoi(match[1])
				add(id)
			}
		}
	}

	// then the newest of the rest, a kill after a loss
	var kills, losses []int
	for _, km := range all {
		if t.keep[km.ID] {
			continue
		}
		if km.Outcome == "kill" {
			kills = append(kills, km.ID)
		} else {
			losses = append(losses, km.ID)
		}
	}
	for len(t.keep) < killmailBudget && (len(kills) > 0 || len(losses) > 0) {
		if len(kills) > 0 {
			add(kills[0])
			kills = kills[1:]
		}
		if len(t.keep) < killmailBudget && len(losses) > 0 {
			add(losses[0])
			losses = losses[1:]
		}
	}

	var kept []gui.KillmailCard
	t.days = map[string]bool{}
	for _, km := range all {
		if t.keep[km.ID] {
			kept = append(kept, km)
			t.days[km.Time.UTC().Format(time.DateOnly)] = true
		}
	}
	t.ratio = float64(len(kept)) / float64(len(all))
	report.Killmails = kept

	t.battles(keptBattles)
	t.fits()
	t.systems(all)
	t.counts()
	t.structures()
	return t
}

// pickFitGroups chooses the fits to keep: the ones with the most losses first, then the rest as long as their hull is not kept yet
func pickFitGroups(groups []gui.FitGroup) []gui.FitGroup {
	sorted := slices.Clone(groups)
	slices.SortStableFunc(sorted, func(a, b gui.FitGroup) int { return len(b.Losses) - len(a.Losses) })
	var picked []gui.FitGroup
	hulls := map[int]bool{}
	for _, group := range sorted {
		if len(group.Losses) == 0 || hulls[group.ShipID] || len(picked) == maxFitGroups {
			continue
		}
		hulls[group.ShipID] = true
		picked = append(picked, group)
	}
	return picked
}

// battles keeps the chosen battles, newest first as the backend has them, with their killmail counts checked against what is kept
func (t *trimmer) battles(chosen []gui.Battle) {
	var kept []gui.Battle
	for _, battle := range t.report.Battles {
		if !slices.ContainsFunc(chosen, func(c gui.Battle) bool { return c.ID == battle.ID }) {
			continue
		}
		ids := killmailsIn(battle, t.report.Killmails)
		if len(ids) != battle.Killmails {
			fmt.Printf("battle %s: %d killmails kept of %d\n", battle.ID, len(ids), battle.Killmails)
			battle.Killmails = len(ids)
		}
		kept = append(kept, battle)
	}
	t.report.Battles = kept
}

// fits keeps the fits with a loss on a kept killmail, and those losses only
func (t *trimmer) fits() {
	var kept []gui.FitGroup
	for _, group := range t.report.Fits {
		var losses []gui.FitLoss
		for _, loss := range group.Losses {
			if t.keep[loss.KillmailID] {
				losses = append(losses, loss)
			}
		}
		if len(losses) == 0 {
			continue
		}
		group.Losses = losses
		group.LastLost = losses[0].Time
		for _, loss := range losses {
			if loss.Time.After(group.LastLost) {
				group.LastLost = loss.Time
			}
		}
		kept = append(kept, group)
	}
	if t.report.Fits != nil {
		t.report.Fits = kept
	}
}

// systems counts the systems again from the kept killmails, the same way the backend does, most killmails first
func (t *trimmer) systems(all []gui.KillmailCard) {
	if t.report.Systems == nil {
		return
	}
	type tally struct{ killmails, kills, losses int }
	counts := map[int]*tally{}
	days := map[int]map[string]bool{}
	first, last := map[int]time.Time{}, map[int]time.Time{}
	for _, km := range t.report.Killmails {
		c := counts[km.SystemID]
		if c == nil {
			c, days[km.SystemID] = &tally{}, map[string]bool{}
			counts[km.SystemID] = c
		}
		c.killmails++
		if km.Outcome == "kill" {
			c.kills++
		} else {
			c.losses++
		}
		days[km.SystemID][km.Time.UTC().Format(time.DateOnly)] = true
		if first[km.SystemID].IsZero() || km.Time.Before(first[km.SystemID]) {
			first[km.SystemID] = km.Time
		}
		if km.Time.After(last[km.SystemID]) {
			last[km.SystemID] = km.Time
		}
	}
	var kept []gui.SystemSummary
	for _, system := range t.report.Systems {
		c := counts[system.ID]
		if c == nil {
			continue
		}
		system.Killmails, system.Kills, system.Losses, system.UniqueDays = c.killmails, c.kills, c.losses, len(days[system.ID])
		system.FirstSeen, system.LastSeen = first[system.ID], last[system.ID]
		kept = append(kept, system)
	}
	slices.SortStableFunc(kept, func(a, b gui.SystemSummary) int { return b.Killmails - a.Killmails })
	t.report.Systems = kept
}

// scale cuts a count the backend made from all the killmails to the kept ones, never to 0 unless it was, and never above the kept killmails
func (t *trimmer) scale(count int) int {
	if count == 0 {
		return 0
	}
	return min(max(1, int(math.Round(float64(count)*t.ratio))), len(t.report.Killmails))
}

// scaleDays cuts a count of days to what the kept killmails cover
func (t *trimmer) scaleDays(count int) int {
	return min(count, len(t.days))
}

// counts cuts the counts of the pilot, corp, and ally lists, which the backend made from the attackers of every killmail
func (t *trimmer) counts() {
	r := t.report
	for _, list := range [][]gui.Fleetmate{r.Fleetmates, r.Members, r.Pilots} {
		for i := range list {
			list[i].KillmailsShared = t.scale(list[i].KillmailsShared)
			list[i].UniqueDays = t.scaleDays(list[i].UniqueDays)
		}
	}
	for i := range r.Corporations {
		c := &r.Corporations[i]
		c.Killmails, c.Kills, c.Losses, c.Pilots, c.UniqueDays = t.scale(c.Killmails), t.scale(c.Kills), t.scale(c.Losses), t.scale(c.Pilots), t.scaleDays(c.UniqueDays)
	}
	t.allies(r.Allies)
	for i := range r.Holdings {
		r.Holdings[i].Killmails = t.scale(r.Holdings[i].Killmails)
	}
}

// allies cuts an ally list's counts
func (t *trimmer) allies(list []gui.AllySummary) {
	for i := range list {
		a := &list[i]
		a.Killmails, a.Killed, a.Pilots, a.UniqueDays = t.scale(a.Killmails), t.scale(a.Killed), t.scale(a.Pilots), t.scaleDays(a.UniqueDays)
	}
}

// structures keeps the first rows of each structure list, which are the busiest or newest, with the counts cut
func (t *trimmer) structures() {
	s := t.report.Structures
	if s == nil {
		return
	}
	if len(s.Used) > maxStructureRows {
		s.Used = s.Used[:maxStructureRows]
	}
	for i := range s.Used {
		s.Used[i].Kills = t.scale(s.Used[i].Kills)
	}
	if len(s.Destroyed) > maxStructureRows {
		s.Destroyed = s.Destroyed[:maxStructureRows]
	}
	if len(s.Lost) > maxStructureRows {
		s.Lost = s.Lost[:maxStructureRows]
	}
	if len(t.report.PreviousStructureLosses) > maxStructureRows {
		t.report.PreviousStructureLosses = t.report.PreviousStructureLosses[:maxStructureRows]
	}
}
