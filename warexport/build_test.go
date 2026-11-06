package warexport_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/warexport"
	"github.com/stretchr/testify/assert"
)

const hostile = `Evil</script><script>alert(1)</script><a href="javascript:alert(2)">x</a><img src=x onerror=alert(3)><url=showinfo:1>y</url>`

// warInput has every kind of name set to something hostile
func warInput() warexport.Input {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	battle := func(id int, system, holder string, at time.Time) warexport.Battle {
		return warexport.Battle{Start: at, End: at.Add(time.Hour), SystemID: id, System: system, Nickname: hostile, OtherSystems: []string{hostile}, Class: "C5", Pilots: 40, Killmails: 12,
			Marks: []warexport.Mark{{Side: "a", Title: "Citadel", Word: "citadel", State: "attacked", Notes: []string{hostile}}},
			Notes: []string{hostile, "HP < 50% then > 70%"}, A: warexport.BattleSide{Group: hostile, ISKLost: 5e9}, B: warexport.BattleSide{Group: "Beta", ISKLost: 2e9, Structures: 1},
			Links: []warexport.Link{{Label: "zKillboard", URL: "https://zkillboard.com/related/1/2/"}, {Label: "Bad", URL: "javascript:alert(4)"}, {Label: "Plain", URL: "http://example.com"}}}
	}
	return warexport.Input{
		Name: hostile, FromDate: "2026-10-01", SideA: hostile, SideB: "Beta", Exported: start, Version: "1.4.0",
		A:       warexport.Measures{ISKLost: 5e9, Citadels: []warexport.StructureKind{{Name: hostile, Count: 1}}},
		B:       warexport.Measures{ISKLost: 2e9},
		Classes: []warexport.ClassCount{{Class: "C5", Count: 2}},
		Reach:   []warexport.ReachLine{{Group: hostile, Oldest: "2026-10-02"}},
		GroupsA: []warexport.Group{{Name: hostile, Pilots: 3}},
		Days:    []warexport.Day{{Date: "2026-10-01", A: warexport.DaySide{ISK: 5e9, Ships: 3}, B: warexport.DaySide{ISK: 2e9, Structures: 1}}},
		Systems: []warexport.System{
			{ID: 1, Name: "J100001", Nickname: hostile, Class: "C5", Statics: []string{"C5", "Highsec"}, Battles: 1, SeenA: true, LostA: start},
			{ID: 2, Name: "J100002", Class: "C5", Battles: 1, SeenA: true, SeenB: true},
			{ID: 3, Name: "J100003", Class: "C5", Battles: 1},
		},
		Battles:      []warexport.Battle{battle(3, "J100003", "", start.Add(2*time.Hour)), battle(2, "J100002", "", start.Add(time.Hour)), battle(1, "J100001", "a", start), battle(0, "(Unknown system)", "", start)},
		FlownA:       []warexport.Hull{{Name: "Zealot", Seen: 3, Lost: 1, Icon: "https://images.evetech.net/types/1/render"}, {Name: "Abaddon", Seen: 2, Icon: "https://evil.example/x.png"}},
		AppearancesA: []warexport.GroupAppearance{{Name: "Alpha", Battles: 3, Corps: []warexport.CorpAppearance{{Name: hostile, Battles: 2}}}, {Name: "Lone", Battles: 1}},
	}
}

func pageData(t *testing.T, html string) map[string]any {
	t.Helper()
	found := regexp.MustCompile(`(?s)<script type="application/json" id="war-data">(.*?)</script>`).FindStringSubmatch(html)
	if !assert.NotNil(t, found) {
		t.FailNow()
	}
	out := map[string]any{}
	assert.NoError(t, json.Unmarshal([]byte(found[1]), &out))
	return out
}

func TestBuild(t *testing.T) {
	files, err := warexport.Build(warInput())
	if !assert.NoError(t, err) || !assert.Len(t, files, 2) {
		return
	}
	text := string(files[0].Data)
	body := pageData(t, text)["body"].(map[string]any)
	section := func(name string) map[string]any { return body[name].(map[string]any) }

	t.Run("it is one page and a JSON file, named for the war", func(t *testing.T) {
		assert.Regexp(t, `\.html$`, files[0].Name)
		assert.Regexp(t, `\.json$`, files[1].Name)
		assert.NotContains(t, files[0].Name, "/")
		assert.Equal(t, 1, strings.Count(text, "plotly.js (basic - minified)"), "Plotly once")
		meta := pageData(t, text)["meta"].(map[string]any)
		assert.Equal(t, files[1].Name, meta["jsonFile"])
		assert.Len(t, meta["tabs"], 5)
	})

	t.Run("no name or note can run or link", func(t *testing.T) {
		for _, file := range files {
			for _, banned := range []string{"alert(1)</script>", "<script>alert", "javascript:alert", "onerror=alert(3)>", "<img", "<url=", "\\u003ca", "\\u003cimg", "\\u003cscript"} {
				assert.NotContains(t, string(file.Data), banned, file.Name)
			}
		}
	})

	t.Run("a lone angle bracket keeps the words after it", func(t *testing.T) {
		assert.Contains(t, text, "HP  50% then  70%")
	})

	t.Run("only https links are kept", func(t *testing.T) {
		assert.Contains(t, text, "https://zkillboard.com/related/1/2/")
		assert.NotContains(t, text, "http://example.com")
	})

	t.Run("only icons on the image server are kept", func(t *testing.T) {
		assert.Contains(t, text, "https://images.evetech.net/types/1/render")
		assert.NotContains(t, text, "evil.example")
	})

	t.Run("the policy has no unsafe-eval, and allows exactly its own scripts and the image server", func(t *testing.T) {
		policy := regexp.MustCompile(`http-equiv="Content-Security-Policy" content="([^"]*)"`).FindStringSubmatch(text)
		if !assert.NotNil(t, policy) {
			return
		}
		assert.NotContains(t, policy[1], "unsafe-eval")
		assert.Contains(t, policy[1], "default-src 'none'")
		assert.Contains(t, policy[1], "img-src data: https://images.evetech.net;")
		scripts := regexp.MustCompile(`(?s)<script>(.*?)</script>\n`).FindAllStringSubmatch(text, -1)
		assert.Len(t, scripts, 2)
		assert.Equal(t, 2, strings.Count(policy[1], "'sha256-"))
		for _, script := range scripts {
			sum := sha256.Sum256([]byte(script[1]))
			assert.Contains(t, policy[1], "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
	})

	battles := section("battles")["battles"].([]any)
	t.Run("a battle takes its color from its main system, and one with no known system has no bubble", func(t *testing.T) {
		timeline := section("timeline")
		order := timeline["order"].([]any)
		assert.Equal(t, []any{2.0, 1.0, 0.0}, order, "oldest first, the unknown system left out")
		assert.Equal(t, "a", battles[2].(map[string]any)["holder"], "side A alone seen")
		assert.Equal(t, "", battles[1].(map[string]any)["holder"], "both seen")
		assert.Equal(t, "", battles[0].(map[string]any)["holder"], "neither seen")
		assert.Equal(t, []any{"J100001", "J100002", "J100003"}, timeline["rows"], "by first fight")
	})

	t.Run("a battle's station marks are on it", func(t *testing.T) {
		marks := battles[0].(map[string]any)["marks"].([]any)
		if assert.Len(t, marks, 1) {
			assert.Equal(t, "attacked", marks[0].(map[string]any)["state"])
		}
	})

	t.Run("ships run A to Z", func(t *testing.T) {
		assert.Equal(t, "Abaddon", section("ships")["a"].([]any)[0].(map[string]any)["name"])
	})

	t.Run("a system's first loss is on its day", func(t *testing.T) {
		lost := section("totals")["lost"].([]any)
		if assert.Len(t, lost, 1) {
			assert.Equal(t, "2026-10-01", lost[0].(map[string]any)["date"])
			assert.Equal(t, "a", lost[0].(map[string]any)["side"])
		}
	})

	t.Run("the JSON has its three sections, corps under their alliance", func(t *testing.T) {
		out := map[string]any{}
		assert.NoError(t, json.Unmarshal(files[1].Data, &out))
		alliances := out["alliance_appearances"].(map[string]any)["side_a"].([]any)
		assert.Len(t, alliances, 2)
		corps := out["corporation_appearances"].(map[string]any)["side_a"].([]any)
		assert.Equal(t, "Alpha", corps[0].(map[string]any)["alliance"])
		assert.Equal(t, "", corps[1].(map[string]any)["alliance"], "the corp with no alliance")
		assert.Equal(t, "C5/Highsec", out["system_appearances"].([]any)[0].(map[string]any)["statics"])
	})

	t.Run("an empty war still builds", func(t *testing.T) {
		files, err := warexport.Build(warexport.Input{Name: "Quiet", SideA: "A", SideB: "B", Exported: time.Now()})
		assert.NoError(t, err)
		assert.Equal(t, "Quiet.html", files[0].Name)
		assert.Equal(t, "Quiet.json", files[1].Name)
	})
}

func TestFolderName(t *testing.T) {
	for name, want := range map[string]string{
		"Scope Creep":   "Scope Creep",
		`A/B: "C"?`:     `A_B_ _C__`,
		"Trailing.. ":   "Trailing",
		"..":            "War",
		"":              "War",
		"   ":           "War",
		"CON":           "War CON",
		"nul.war":       "War nul.war",
		"com1":          "War com1",
		`..\..\Windows`: `.._.._Windows`,
		"Console":       "Console",
	} {
		assert.Equal(t, want, warexport.FolderName(name), name)
	}
	assert.Len(t, []rune(warexport.FolderName(strings.Repeat("x", 300))), 80)
}

func TestWrite(t *testing.T) {
	parent := t.TempDir()
	first := []warexport.File{{Name: "My War.html", Data: []byte("one")}, {Name: "My War.json", Data: []byte("{}")}}
	folder, err := warexport.Write(parent, "My War", first)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(parent, "My War"), folder)
	assert.NoError(t, os.WriteFile(filepath.Join(folder, "mine.txt"), []byte("keep"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(folder, "06_notes.json"), []byte("keep"), 0o600))

	t.Run("exporting again replaces its files and leaves every other", func(t *testing.T) {
		_, err := warexport.Write(parent, "My War", []warexport.File{{Name: "My War.html", Data: []byte("two")}, {Name: "My War.json", Data: []byte("{}")}})
		assert.NoError(t, err)
		data, _ := os.ReadFile(filepath.Join(folder, "My War.html"))
		assert.Equal(t, "two", string(data))
		assert.FileExists(t, filepath.Join(folder, "mine.txt"))
		assert.FileExists(t, filepath.Join(folder, "06_notes.json"))
		assert.NoFileExists(t, filepath.Join(folder, "My War.html.tmp"))
	})

	t.Run("a war's name can not climb out of the parent", func(t *testing.T) {
		folder, err := warexport.Write(parent, `..\..\evil`, first)
		assert.NoError(t, err)
		assert.Equal(t, parent, filepath.Dir(folder))
	})
}
