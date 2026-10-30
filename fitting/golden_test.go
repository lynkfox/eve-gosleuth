package fitting_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/fitting"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// goldenFit is one file of testdata/golden, see FORMAT.md there
type goldenFit struct {
	Name             string          `json:"name"`
	Pyfa             string          `json:"pyfa"`
	Input            fitting.Input   `json:"input"`
	Expected         json.RawMessage `json:"expected"`
	KnownDifferences []struct {
		Field  string `json:"field"`
		Reason string `json:"reason"`
	} `json:"known_differences"`
}

// goldenTolerance is how far a number may be from Pyfa's, relative to it, for rounding and Pyfa's float handling
const goldenTolerance = 0.001

// TestGoldenFits checks fitting.Compute against the numbers Pyfa gives for the same fits
func TestGoldenFits(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	if err := comm.InitStaticData(staticdata.Files); err != nil {
		t.Fatal(err)
	}

	files, _ := filepath.Glob(filepath.Join("testdata", "golden", "*.json"))
	if len(files) == 0 {
		t.Skip("no golden fits yet")
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".json"), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			golden := goldenFit{}
			if err := json.Unmarshal(data, &golden); err != nil {
				t.Fatal(err)
			}
			stats, err := fitting.Compute(golden.Input)
			if err != nil {
				t.Fatal(err)
			}
			actual, _ := json.Marshal(stats)

			skipped := map[string]bool{}
			for _, known := range golden.KnownDifferences {
				skipped[known.Field] = true
			}
			for _, diff := range compareNumbers("", decode(golden.Expected), decode(actual), skipped) {
				t.Error(diff)
			}
		})
	}
}

// decode reads json into maps, slices, and float64s for compareNumbers
func decode(data []byte) any {
	var value any
	_ = json.Unmarshal(data, &value)
	return value
}

/*
	compareNumbers walks Pyfa's expected stats and reports every number ours differs from beyond goldenTolerance. Only fields in expected are checked, so assumptions and unknown, which Pyfa doesn't give, are not.

Parameters:
  - path: the field's dotted path so far, ie shield.resonance
  - expected: Pyfa's value
  - actual: ours
  - skipped: the paths left out as known differences; a path also skips everything beneath it

Returns:
  - []string: one line per difference
*/
func compareNumbers(path string, expected any, actual any, skipped map[string]bool) []string {
	if skipped[path] {
		return nil
	}
	switch want := expected.(type) {
	case map[string]any:
		got, _ := actual.(map[string]any)
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		diffs := []string{}
		for _, key := range keys {
			diffs = append(diffs, compareNumbers(strings.TrimPrefix(path+"."+key, "."), want[key], got[key], skipped)...)
		}
		return diffs
	case []any:
		got, _ := actual.([]any)
		diffs := []string{}
		for index := range want {
			var each any
			if index < len(got) {
				each = got[index]
			}
			diffs = append(diffs, compareNumbers(fmt.Sprintf("%s[%d]", path, index), want[index], each, skipped)...)
		}
		return diffs
	case float64:
		got, ok := actual.(float64)
		if !ok {
			got = 0 // omitted as zero
		}
		if math.Abs(got-want) > goldenTolerance*math.Max(math.Abs(want), 1e-9) && math.Abs(got-want) > 1e-6 {
			return []string{fmt.Sprintf("%s: Pyfa %g, ours %g", path, want, got)}
		}
		return nil
	default:
		if !assert.ObjectsAreEqual(expected, actual) {
			return []string{fmt.Sprintf("%s: Pyfa %v, ours %v", path, expected, actual)}
		}
		return nil
	}
}

// TestAssaultDamageControlOff checks that an Assault Damage Control is left online unless its state says active: its burst isn't counted, its steady bonus is
func TestAssaultDamageControlOff(t *testing.T) {
	comm.InitLogger(logrus.ErrorLevel)
	if err := comm.InitStaticData(staticdata.Files); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "golden", "assault_dc_zealot.json"))
	if err != nil {
		t.Fatal(err)
	}
	golden := goldenFit{}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	burst, err := fitting.Compute(golden.Input)
	if err != nil {
		t.Fatal(err)
	}

	off := golden.Input
	off.Modules = append([]fitting.Module{}, golden.Input.Modules...)
	for i := range off.Modules {
		off.Modules[i].State = ""
	}
	steady, err := fitting.Compute(off)
	if err != nil {
		t.Fatal(err)
	}

	without := golden.Input
	without.Modules = []fitting.Module{}
	for _, module := range golden.Input.Modules {
		if module.Type != 47257 {
			without.Modules = append(without.Modules, module)
		}
	}
	none, err := fitting.Compute(without)
	if err != nil {
		t.Fatal(err)
	}

	assert.Less(t, steady.EHP, burst.EHP/2, "the burst is not counted")
	assert.Greater(t, steady.EHP, none.EHP, "the steady Damage Control bonus is")
}
