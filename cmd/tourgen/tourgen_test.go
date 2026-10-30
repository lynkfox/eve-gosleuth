package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// demoFiles are the files the tour loads, where tourgen writes them
var demoFiles = []string{"alliance.json", "system.json"}

const demoFolder = "../../frontend/dist/tour"

// loadDemo reads a demo file as the generic tree the scrubber works on
func loadDemo(t *testing.T, name string) any {
	data, err := os.ReadFile(filepath.Join(demoFolder, name))
	if !assert.NoError(t, err, "run go run ./cmd/tourgen to make %s", name) {
		t.FailNow()
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var tree any
	assert.NoError(t, decoder.Decode(&tree))
	return tree
}

// walkTree calls visit on every value in a tree with its key and path
func walkTree(node any, key string, path string, visit func(key string, path string, value any)) {
	visit(key, path, node)
	switch value := node.(type) {
	case map[string]any:
		for childKey, child := range value {
			walkTree(child, childKey, path+"."+childKey, visit)
		}
	case []any:
		for _, child := range value {
			walkTree(child, key, path+"[]", visit)
		}
	}
}

var tickerShape = regexp.MustCompile(`^[A-Z0-9]{1,5}$`)

// The tour's files must hold no real player: every pilot, corp, and alliance id is a made-up one, and no picture or link leads to a real one.
func TestDemoFilesHoldNoRealPlayers(t *testing.T) {
	for _, name := range demoFiles {
		tree := loadDemo(t, name)
		walkTree(tree, "", "", func(key string, path string, value any) {
			switch node := value.(type) {
			case map[string]any:
				if k, id, ok := entityOf(node); ok {
					assert.GreaterOrEqual(t, id, fakeIDBase+int(k)*fakeIDSpan, "%s %s: %s id %d is not a made-up one", name, path, k, id)
					assert.Less(t, id, fakeIDBase+(int(k)+1)*fakeIDSpan, "%s %s: id %d is outside its kind's range", name, path, id)
				}
			case json.Number:
				// an id key holding a number in a player range, whatever object it is in
				if strings.EqualFold(key, "id") || strings.EqualFold(key, "CorporationID") {
					if id, err := node.Int64(); err == nil {
						assert.False(t, id >= 90_000_000 && id < 100_000_000 || id >= 2_100_000_000 && id < fakeIDBase, "%s %s: id %d is in a real player's range", name, path, id)
					}
				}
			case string:
				assert.False(t, entityURL.MatchString(node), "%s %s: %q leads to a real pilot, corp, or alliance", name, path, node)
				assert.False(t, pageURL.MatchString(node), "%s %s: %q leads to a real killmail, battle, or player page", name, path, node)
				if strings.EqualFold(key, "ticker") && node != "" {
					assert.Regexp(t, tickerShape, node, "%s %s", name, path)
				}
			}
		})
	}
}

// The one picture a made-up entity shows has to be beside the files
func TestPlaceholderImageExists(t *testing.T) {
	_, err := os.Stat(filepath.Join(demoFolder, filepath.Base(placeholderImage)))
	assert.NoError(t, err)
}

// The searched alliance is named by hand and a pilot has no ticker
func TestDemoAllianceName(t *testing.T) {
	tree := loadDemo(t, "alliance.json").(map[string]any)
	assert.Equal(t, demoAllianceName, tree["searchName"])
	display := tree["target"].(map[string]any)["display"].(map[string]any)
	assert.Equal(t, demoAllianceName, display["Name"])
	assert.Equal(t, demoAllianceTicker, display["Ticker"])
}

// An entity gets the same made-up id and name each time and in each file, and no two share one
func TestRenamerIsStable(t *testing.T) {
	first, second := newRenamer(), newRenamer()
	ids, names := map[int]bool{}, map[string]bool{}
	for id := 98_000_000; id < 98_000_400; id++ {
		a := first.register(id, entityReal{kind: corpKind, name: "Real Corp", ticker: "RC"})
		b := second.register(id, entityReal{kind: corpKind})
		assert.Equal(t, a.ID, b.ID)
		assert.Equal(t, a.Name, b.Name)
		assert.GreaterOrEqual(t, a.ID, fakeIDBase+fakeIDSpan)
		assert.Less(t, a.ID, fakeIDBase+2*fakeIDSpan)
		assert.False(t, ids[a.ID])
		assert.False(t, names[a.Name])
		assert.Regexp(t, tickerShape, a.Ticker)
		ids[a.ID], names[a.Name] = true, true
	}
}

// The scrubber renames entities, blanks links, and finds a real name left in free text
func TestScrubber(t *testing.T) {
	demo := map[string]any{
		"victim":     map[string]any{"id": json.Number("2112345678"), "type": "PILOT", "name": "Real Pilot", "iconUrl": "https://images.evetech.net/characters/2112345678/portrait?size=32", "zkillUrl": "https://zkillboard.com/character/2112345678/"},
		"victimName": "Real Pilot",
		"eft":        "[Rifter, GoSleuth - real pilot]\n\nSmall Armor Repairer I",
		"url":        "https://zkillboard.com/kill/139540011/",
		"npc":        map[string]any{"id": json.Number("1000125"), "type": "CORPORATION", "name": "Concord"},
		"ship":       "https://images.evetech.net/types/587/icon?size=64",
	}
	s := newScrubber(newRenamer())
	tree, err := toTree(demo)
	assert.NoError(t, err)
	s.collect(tree)
	s.prepare()
	assert.NotEmpty(t, s.leaks(tree), "the real name is found before it is replaced")
	tree = s.rewrite(tree, "")
	assert.Empty(t, s.leaks(tree))
	out := tree.(map[string]any)
	victim := out["victim"].(map[string]any)
	assert.NotEqual(t, "Real Pilot", victim["name"])
	assert.Equal(t, victim["name"], out["victimName"])
	assert.Contains(t, out["eft"], victim["name"])
	assert.Equal(t, placeholderImage, victim["iconUrl"])
	assert.Equal(t, "", victim["zkillUrl"])
	assert.Equal(t, "", out["url"])
	assert.Equal(t, "Concord", out["npc"].(map[string]any)["name"], "an NPC corp is game data")
	assert.Equal(t, "https://images.evetech.net/types/587/icon?size=64", out["ship"])
}
