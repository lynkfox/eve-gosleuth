package main

import (
	"testing"
	"testing/fstest"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/stretchr/testify/assert"
)

// variationsSDE has a base module, its T2 and a faction variant, a module that is its own parent, a charge with a parent, and a mutated module, which has no parent in the SDE
func variationsSDE() fstest.MapFS {
	return fstest.MapFS{
		"types.jsonl": jsonl(
			`{"_key":10}`,
			`{"_key":11,"variationParentTypeID":10}`,
			`{"_key":12,"variationParentTypeID":10}`,
			`{"_key":13,"variationParentTypeID":13}`,
			`{"_key":20,"variationParentTypeID":10}`,
			`{"_key":30}`,
			`{"_key":31,"variationParentTypeID":30}`,
			`{"_key":40}`,
			`{"_key":41,"variationParentTypeID":40}`,
			`{"_key":50,"metaGroupID":15}`),
	}
}

func variationsFit() *comm.FitTypesFile {
	module := func(id int) *comm.FitType { return &comm.FitType{TypeID: id, CategoryID: 7} }
	return &comm.FitTypesFile{SDEBuild: 7, Types: map[int]*comm.FitType{
		10: module(10), 11: module(11), 12: module(12), 13: module(13),
		20: {TypeID: 20, CategoryID: 8}, // a charge
		30: module(30), 31: module(31), 40: module(40), 41: module(41),
		50: {TypeID: 50, CategoryID: 7, Abyssal: true},
	}}
}

func TestExtractVariations(t *testing.T) {
	variations, err := ExtractVariations(variationsSDE(), variationsFit())
	assert.NoError(t, err)

	parent := func(id int) int {
		if p, found := variations.Parents[id]; found {
			return *p
		}
		return 0
	}
	assert.Equal(t, 7, variations.SDEBuild)
	assert.Equal(t, 10, parent(11), "a variant to its parent")
	assert.Equal(t, 10, parent(12))
	assert.Equal(t, 30, parent(31))
	assert.NotContains(t, variations.Parents, 10, "a base type has no entry")
	assert.NotContains(t, variations.Parents, 13, "its own parent is left out")
	assert.NotContains(t, variations.Parents, 20, "a charge is left out")
	assert.Equal(t, 40, parent(41))
	assert.NotContains(t, variations.Parents, 50, "a mutated module is its own base type, so a fit with one never merges with one without")
}

func TestMarshalVariations(t *testing.T) {
	variations, err := ExtractVariations(variationsSDE(), variationsFit())
	assert.NoError(t, err)

	text, err := marshalVariations(variations)
	assert.NoError(t, err)
	assert.Contains(t, string(text), "\"11\":10")

	again, err := marshalVariations(variations)
	assert.NoError(t, err)
	assert.Equal(t, text, again, "the same extract gives the same text")
	assert.Equal(t, 0, missingParents(variations, variationsFit()))
}
