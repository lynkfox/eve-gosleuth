package warexport

import (
	"encoding/json"
	"strings"
)

// jsonCorp, jsonAlliance, and jsonCorpGroup are rows of the JSON's appearance lists
type jsonAlliance struct {
	Alliance string `json:"alliance"` // a corp with no alliance stands here too
	Battles  int    `json:"battles"`
}

type jsonCorp struct {
	Corporation string `json:"corporation"`
	Battles     int    `json:"battles"`
}

type jsonCorpGroup struct {
	Alliance     string     `json:"alliance"` // empty for the corps with none
	Corporations []jsonCorp `json:"corporations"`
}

// jsonSystem is a row of system_appearances, as in the 2024 archive with a nickname added
type jsonSystem struct {
	System   string `json:"system"`
	Nickname string `json:"nickname,omitempty"`
	Class    string `json:"class"`
	Statics  string `json:"statics"` // ie C5/Highsec
	Battles  int    `json:"battles"`
}

type jsonSides[T any] struct {
	Description string `json:"description"`
	SideAName   string `json:"side_a_name"`
	SideBName   string `json:"side_b_name"`
	SideA       []T    `json:"side_a"`
	SideB       []T    `json:"side_b"`
	Others      []T    `json:"others"`
}

func allianceRows(groups []GroupAppearance) []jsonAlliance {
	rows := make([]jsonAlliance, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, jsonAlliance{group.Name, group.Battles})
	}
	return rows
}

// corpRows puts corps under their alliance, then those with no alliance together, most battles first as they came
func corpRows(groups []GroupAppearance) []jsonCorpGroup {
	rows := make([]jsonCorpGroup, 0, len(groups))
	lone := []jsonCorp{}
	for _, group := range groups {
		if len(group.Corps) == 0 {
			lone = append(lone, jsonCorp{group.Name, group.Battles})
			continue
		}
		row := jsonCorpGroup{Alliance: group.Name, Corporations: make([]jsonCorp, 0, len(group.Corps))}
		for _, corp := range group.Corps {
			row.Corporations = append(row.Corporations, jsonCorp{corp.Name, corp.Battles})
		}
		rows = append(rows, row)
	}
	if len(lone) > 0 {
		rows = append(rows, jsonCorpGroup{Corporations: lone})
	}
	return rows
}

// appearancesJSON is the export's JSON file
func appearancesJSON(input Input) ([]byte, error) {
	systems := make([]jsonSystem, 0, len(input.Systems))
	for _, system := range input.Systems {
		systems = append(systems, jsonSystem{System: system.Name, Nickname: system.Nickname, Class: system.Class, Statics: strings.Join(system.Statics, "/"), Battles: system.Battles})
	}
	return json.MarshalIndent(struct {
		War          string                   `json:"war"`
		Compiled     string                   `json:"last_compiled"`
		Version      string                   `json:"gosleuth_version"`
		Alliances    jsonSides[jsonAlliance]  `json:"alliance_appearances"`
		Corporations jsonSides[jsonCorpGroup] `json:"corporation_appearances"`
		Systems      []jsonSystem             `json:"system_appearances"`
	}{
		War:      input.Name,
		Compiled: input.Exported.UTC().Format("2006-01-02"),
		Version:  input.Version,
		Alliances: jsonSides[jsonAlliance]{"Battles each alliance, or corp with no alliance, was seen in", input.SideA, input.SideB,
			allianceRows(input.AppearancesA), allianceRows(input.AppearancesB), allianceRows(input.AppearancesOthers)},
		Corporations: jsonSides[jsonCorpGroup]{"Battles each corp was seen in, under its alliance", input.SideA, input.SideB,
			corpRows(input.AppearancesA), corpRows(input.AppearancesB), corpRows(input.AppearancesOthers)},
		Systems: systems,
	}, "", "    ")
}
