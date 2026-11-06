package warexport

import (
	"cmp"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

//go:embed assets/plotly-basic-2.35.2.min.js
var plotlyBundle string

//go:embed assets/page.css
var pageCSS string

//go:embed assets/common.js
var commonJS string

//go:embed assets/summary.js
var summaryJS string

//go:embed assets/totals.js
var totalsJS string

//go:embed assets/ships.js
var shipsJS string

//go:embed assets/timeline.js
var timelineJS string

//go:embed assets/battles.js
var battlesJS string

// Caveat is on the page
const Caveat = "Every figure here comes from killmails, which show neither objectives nor survivors, so these are what was lost, not who won. Treat them as clues, not conclusions."

// imageHost is the only place the page loads anything from: CCP's image server, for logos and ship icons
const imageHost = "images.evetech.net"

// File is one file of the export
type File struct {
	Name string
	Data []byte
}

// tabLink is a tab of the page
type tabLink struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// pageTabs are the page's tabs in order. The first opens unless the address names another, ie #timeline.
var pageTabs = []tabLink{{"summary", "Summary"}, {"totals", "Totals"}, {"ships", "Ship counts"}, {"timeline", "Timeline"}, {"battles", "Battles"}}

// pageMeta is what the whole page knows of the war
type pageMeta struct {
	Name     string    `json:"name"`
	FromDate string    `json:"fromDate"`
	Until    string    `json:"until"`
	SideA    string    `json:"sideA"`
	SideB    string    `json:"sideB"`
	Exported time.Time `json:"exported"`
	Version  string    `json:"version"`
	Caveat   string    `json:"caveat"`
	Tabs     []tabLink `json:"tabs"`
	JSONFile string    `json:"jsonFile"`
}

type summaryBody struct {
	A       Measures     `json:"a"`
	B       Measures     `json:"b"`
	Others  Measures     `json:"others"`
	Counts  Counts       `json:"counts"`
	Classes []ClassCount `json:"classes"`
	Reach   []ReachLine  `json:"reach"`
	GroupsA []Group      `json:"groupsA"`
	GroupsB []Group      `json:"groupsB"`
}

// lostDay is a system a side first lost a citadel or POCO in
type lostDay struct {
	Date   string `json:"date"`
	Side   string `json:"side"`
	System string `json:"system"`
}

type totalsBody struct {
	Days []Day     `json:"days"`
	Lost []lostDay `json:"lost"`
}

type shipsBody struct {
	A []Hull `json:"a"`
	B []Hull `json:"b"`
}

// timelineBody points into the Battles tab's list, so a battle is written once
type timelineBody struct {
	Order []int    `json:"order"` // the battles with a known system, oldest first
	Rows  []string `json:"rows"`  // the systems, by first fight
}

type battlesBody struct {
	Battles []Battle     `json:"battles"`
	Classes []ClassCount `json:"classes"`
}

// pageBody is what each tab shows
type pageBody struct {
	Summary  summaryBody  `json:"summary"`
	Totals   totalsBody   `json:"totals"`
	Ships    shipsBody    `json:"ships"`
	Timeline timelineBody `json:"timeline"`
	Battles  battlesBody  `json:"battles"`
}

/*
	Build makes a war's export: one web page with a tab for each view, and a JSON file. Names, notes, and nicknames are cleaned first, so they are plain text wherever they land.

Parameters:
  - input: what the export shows. Its strings and links are cleaned in place.

Returns:
  - []File: the page, then the JSON
  - error: if either could not be encoded
*/
func Build(input Input) ([]File, error) {
	cleanStrings(reflect.ValueOf(&input))
	safeImages(&input)
	for index := range input.Battles {
		input.Battles[index].Links = safeLinks(input.Battles[index].Links)
		input.Battles[index].Notes = nonNil(input.Battles[index].Notes)
		input.Battles[index].Marks = nonNil(input.Battles[index].Marks)
		input.Battles[index].OtherSystems = nonNil(input.Battles[index].OtherSystems)
	}
	input.Battles = withHolders(input.Battles, input.Systems)

	stem := FolderName(input.Name)
	jsonName := stem + ".json"
	meta := pageMeta{Name: input.Name, FromDate: input.FromDate, Until: input.Until, SideA: input.SideA, SideB: input.SideB, Exported: input.Exported, Version: input.Version, Caveat: Caveat, Tabs: pageTabs, JSONFile: jsonName}
	body := pageBody{
		Summary:  summaryOf(input),
		Totals:   totalsOf(input),
		Ships:    shipsBody{A: hullsByName(input.FlownA), B: hullsByName(input.FlownB)},
		Timeline: timelineOf(input),
		Battles:  battlesBody{Battles: nonNil(input.Battles), Classes: nonNil(input.Classes)},
	}
	page, err := renderPage(meta, body)
	if err != nil {
		return nil, fmt.Errorf("could not make the page: %w", err)
	}
	data, err := appearancesJSON(input)
	if err != nil {
		return nil, fmt.Errorf("could not make %s: %w", jsonName, err)
	}
	return []File{{stem + ".html", page}, {jsonName, data}}, nil
}

// renderPage writes the page. Its scripts are inline and allowed by hash, its data in a block that never runs. json.Marshal escapes < and >, so nothing in the data can close the block.
func renderPage(meta pageMeta, body pageBody) ([]byte, error) {
	data, err := json.Marshal(struct {
		Meta pageMeta `json:"meta"`
		Body pageBody `json:"body"`
	}{meta, body})
	if err != nil {
		return nil, err
	}
	script := strings.Join([]string{commonJS, summaryJS, totalsJS, shipsJS, timelineJS, battlesJS, "startPage();"}, "\n")
	scripts := []string{plotlyBundle, script}
	hashes := make([]string, 0, len(scripts))
	for _, body := range scripts {
		sum := sha256.Sum256([]byte(body))
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="%s">
<meta name="referrer" content="no-referrer">
<title>%s</title>
<style>%s</style>
</head>
<body>
<div id="app"></div>
<script type="application/json" id="war-data">%s</script>
`, policy(hashes), html.EscapeString(meta.Name), pageCSS, data)
	for _, body := range scripts {
		out.WriteString("<script>" + body + "</script>\n")
	}
	out.WriteString("</body>\n</html>\n")
	return []byte(out.String()), nil
}

// policy is the page's Content-Security-Policy: its own scripts only, with no unsafe-eval, and images from the image server. Plotly sets inline styles, so they stay allowed.
func policy(hashes []string) string {
	return "default-src 'none'; script-src " + strings.Join(hashes, " ") + "; style-src 'unsafe-inline'; img-src data: https://" + imageHost + "; base-uri 'none'; form-action 'none'"
}

// tagPattern is a tag, including EVE's own link tags, and not a lone < as in "HP < 50%"
var tagPattern = regexp.MustCompile(`</?[a-zA-Z!][^>]*>`)

// clean makes a string plain text: no tags, including EVE's own link tags, and no angle bracket left to start one
func clean(text string) string {
	return strings.NewReplacer("<", "", ">", "").Replace(tagPattern.ReplaceAllString(text, ""))
}

// cleanStrings cleans every string it reaches, so a field added later can not be forgotten. Slices are changed in place.
func cleanStrings(value reflect.Value) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !value.IsNil() {
			cleanStrings(value.Elem())
		}
	case reflect.Struct:
		if _, isTime := value.Interface().(time.Time); isTime {
			return
		}
		for index := range value.NumField() {
			cleanStrings(value.Field(index))
		}
	case reflect.Slice:
		for index := range value.Len() {
			cleanStrings(value.Index(index))
		}
	case reflect.String:
		value.SetString(clean(value.String()))
	}
}

// safeLinks keeps only https links
func safeLinks(links []Link) []Link {
	kept := make([]Link, 0, len(links))
	for _, link := range links {
		if parsed, err := url.Parse(link.URL); err == nil && parsed.Scheme == "https" && parsed.Host != "" {
			kept = append(kept, link)
		}
	}
	return kept
}

// safeImage keeps an address only if it is on the image server
func safeImage(address string) string {
	if parsed, err := url.Parse(address); err == nil && parsed.Scheme == "https" && parsed.Host == imageHost {
		return address
	}
	return ""
}

// safeImages clears every icon that is not on the image server, as the policy would block it anyway
func safeImages(input *Input) {
	for _, measures := range []*Measures{&input.A, &input.B, &input.Others} {
		for index := range measures.Citadels {
			measures.Citadels[index].Icon = safeImage(measures.Citadels[index].Icon)
		}
	}
	for _, groups := range [][]Group{input.GroupsA, input.GroupsB} {
		for index := range groups {
			groups[index].Icon = safeImage(groups[index].Icon)
		}
	}
	for _, hulls := range [][]Hull{input.FlownA, input.FlownB} {
		for index := range hulls {
			hulls[index].Icon = safeImage(hulls[index].Icon)
		}
	}
	for index := range input.Battles {
		battle := &input.Battles[index]
		battle.A.GroupIcon, battle.B.GroupIcon, battle.Others.GroupIcon = safeImage(battle.A.GroupIcon), safeImage(battle.B.GroupIcon), safeImage(battle.Others.GroupIcon)
	}
}

// withHolders colors each battle by the side whose citadels or POCOs killmails show in its main system. Nothing, or both sides, leaves it empty.
func withHolders(battles []Battle, systems []System) []Battle {
	byID := make(map[int]System, len(systems))
	for _, system := range systems {
		byID[system.ID] = system
	}
	for index := range battles {
		system := byID[battles[index].SystemID]
		switch {
		case system.SeenA && !system.SeenB:
			battles[index].Holder = "a"
		case system.SeenB && !system.SeenA:
			battles[index].Holder = "b"
		}
	}
	return battles
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func summaryOf(input Input) summaryBody {
	return summaryBody{A: input.A, B: input.B, Others: input.Others, Counts: input.Counts, Classes: nonNil(input.Classes), Reach: nonNil(input.Reach), GroupsA: nonNil(input.GroupsA), GroupsB: nonNil(input.GroupsB)}
}

func totalsOf(input Input) totalsBody {
	body := totalsBody{Days: nonNil(input.Days), Lost: []lostDay{}}
	for _, system := range input.Systems {
		if !system.LostA.IsZero() {
			body.Lost = append(body.Lost, lostDay{system.LostA.UTC().Format("2006-01-02"), "a", system.Name})
		}
		if !system.LostB.IsZero() {
			body.Lost = append(body.Lost, lostDay{system.LostB.UTC().Format("2006-01-02"), "b", system.Name})
		}
	}
	slices.SortFunc(body.Lost, func(x, y lostDay) int {
		return cmp.Or(cmp.Compare(x.Date, y.Date), cmp.Compare(x.Side, y.Side), cmp.Compare(x.System, y.System))
	})
	return body
}

// hullsByName is a side's ships from A to Z
func hullsByName(hulls []Hull) []Hull {
	sorted := slices.Clone(nonNil(hulls))
	slices.SortFunc(sorted, func(x, y Hull) int { return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)) })
	return sorted
}

// timelineOf is the battles with a known system oldest first, as places in the list newest first, and each one's system in the order they were first fought in
func timelineOf(input Input) timelineBody {
	body := timelineBody{Order: []int{}, Rows: []string{}}
	for index, battle := range input.Battles {
		if battle.SystemID != 0 {
			body.Order = append(body.Order, index)
		}
	}
	slices.SortStableFunc(body.Order, func(x, y int) int { return input.Battles[x].Start.Compare(input.Battles[y].Start) })
	seen := map[string]bool{}
	for _, index := range body.Order {
		if system := input.Battles[index].System; !seen[system] {
			seen[system] = true
			body.Rows = append(body.Rows, system)
		}
	}
	return body
}

// FolderName makes a war's name safe as a Windows file or folder name: no characters it can not hold, no trailing dots or spaces, and not a name the system keeps
func FolderName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	clean = strings.TrimRight(strings.TrimSpace(clean), ". ")
	if runes := []rune(clean); len(runes) > 80 {
		clean = strings.TrimRight(string(runes[:80]), ". ")
	}
	if clean == "" {
		return "War"
	}
	stem, _, _ := strings.Cut(strings.ToUpper(clean), ".")
	if reservedName.MatchString(strings.TrimSpace(stem)) {
		return "War " + clean
	}
	return clean
}

var reservedName = regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[0-9¹²³]|LPT[0-9¹²³])$`)
