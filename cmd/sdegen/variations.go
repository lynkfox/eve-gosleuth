package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"

	"github.com/lynkfox/go_eve_sleuth/comm"
)

// variationCategories are the categories whose types get a base type: modules and rigs (both category 7), subsystems, and structure modules and rigs. Ships, drones, charges, and implants have none, as fits merge only by their slotted modules.
var variationCategories = []int{7, subsystemCategory, 66}

// sdeVariationType is the part of a types.jsonl line the variations extract reads
type sdeVariationType struct {
	Key    int `json:"_key"`
	Parent int `json:"variationParentTypeID"`
}

/*
	ExtractVariations reads each module's base type from a JSONL SDE: the variationParentTypeID of its T2, meta, faction, deadspace, and officer variants.
	Mutated (abyssal) modules have none and get none, so a fit with one never merges with one without (user's call 2026-10-30): their rolls are not on the killmail, and would throw off a merged fit's ranges.

Parameters:
  - sde: the SDE's jsonl files at its root, ie an extracted folder or the zip
  - extract: the fit types extract of the same SDE, whose module, rig, and subsystem types are the ones that get an entry

Returns:
  - *comm.FitVariationsFile: each type to its base type, those that are their own base type left out
  - error: a file that could not be read
*/
func ExtractVariations(sde fs.FS, extract *comm.FitTypesFile) (*comm.FitVariationsFile, error) {
	variations := &comm.FitVariationsFile{SDEBuild: extract.SDEBuild, Parents: map[int]*int{}}

	parentOf := map[int]int{}
	err := eachLine(sde, "types.jsonl", func(item *sdeVariationType) {
		if item.Parent != 0 && item.Parent != item.Key {
			parentOf[item.Key] = item.Parent
		}
	})
	if err != nil {
		return nil, err
	}
	for typeID, parent := range parentOf {
		if fitType, kept := extract.Types[typeID]; kept && slices.Contains(variationCategories, fitType.CategoryID) {
			base := parent
			variations.Parents[typeID] = &base
		}
	}

	return variations, nil
}

/*
	marshalVariations writes the variations as json with one entry to a line in id order, so a new SDE changes only the lines that differ

Parameters:
  - variations: the extract

Returns:
  - []byte: the file's text
  - error: the text is not valid json
*/
func marshalVariations(variations *comm.FitVariationsFile) ([]byte, error) {
	ids := make([]int, 0, len(variations.Parents))
	for id := range variations.Parents {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var out bytes.Buffer
	fmt.Fprintf(&out, "{\"sde_build\":%d,\n\"parents\":{", variations.SDEBuild)
	for index, id := range ids {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString("\n" + strconv.Quote(strconv.Itoa(id)) + ":" + strconv.Itoa(*variations.Parents[id]))
	}
	out.WriteString("\n}}\n")

	if !json.Valid(out.Bytes()) {
		return nil, errors.New("the variations are not valid json")
	}
	return out.Bytes(), nil
}

// missingParents counts the base types that are not in the fit types extract
func missingParents(variations *comm.FitVariationsFile, extract *comm.FitTypesFile) int {
	missing := map[int]struct{}{}
	for _, parent := range variations.Parents {
		if _, found := extract.Types[*parent]; !found {
			missing[*parent] = struct{}{}
		}
	}
	return len(missing)
}
