package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// entityURL matches a pilot, corporation, or alliance's portrait, logo, or site page, and gives its kind and id
var entityURL = regexp.MustCompile(`(?:images\.evetech\.net/(characters|corporations|alliances)/(\d+)|(?:zkillboard\.com|evewho\.com)/(character|corporation|alliance)/(\d+))`)

// pageURL matches the links that open a real killmail, battle report, or search, which name the real players on the page they open
var pageURL = regexp.MustCompile(`zkillboard\.com/(kill|related|asearch)|br\.evetools\.org|warbeacon\.net|evewho\.com/|zkillboard\.com/(character|corporation|alliance)/`)

// urlKinds are the kind each URL path word means
var urlKinds = map[string]kind{"characters": pilotKind, "character": pilotKind, "corporations": corpKind, "corporation": corpKind, "alliances": allianceKind, "alliance": allianceKind}

// idKeys are the keys an entity's id is under
var idKeys = []string{"id", "ID", "CorporationID"}

// nameKeys are the keys an entity's name is under
var nameKeys = []string{"name", "Name"}

// tickerKeys are the keys an entity's ticker is under
var tickerKeys = []string{"ticker", "Ticker"}

// scrubber renames the real players in a demo file's JSON tree
type scrubber struct {
	names *renamer
	words *regexp.Regexp // the real names and tickers, to find in any text
	found map[string]int // where a real name was replaced in free text, by key, for a look at what was caught
}

func newScrubber(names *renamer) *scrubber {
	return &scrubber{names: names, found: map[string]int{}}
}

// toTree turns a demo into the generic JSON tree its file holds, numbers kept exact
func toTree(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var tree any
	err = decoder.Decode(&tree)
	return tree, err
}

// fromTree reads a tree back into a demo
func fromTree(tree any, value any) error {
	data, err := json.Marshal(tree)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

// entityOf says which entity an object is, if it is one: its kind, real id, and whether the id came from its own id key
func entityOf(m map[string]any) (kind, int, bool) {
	k, id, ok := rawEntityOf(m)
	return k, id, ok && id >= firstPlayerID
}

// rawEntityOf is entityOf before the game's own ids (factions, NPC corps and agents) are put aside
func rawEntityOf(m map[string]any) (kind, int, bool) {
	var id int
	haveID := false
	for _, key := range idKeys {
		if number, ok := m[key].(json.Number); ok {
			if value, err := number.Int64(); err == nil {
				id, haveID = int(value), true
				break
			}
		}
	}
	for _, key := range []string{"type", "TypeName"} {
		if text, ok := m[key].(string); ok && haveID {
			if k, ok := kindOfType(text); ok {
				return k, id, true
			}
		}
	}
	if _, ok := m["CorporationID"]; ok && haveID {
		return corpKind, id, true
	}
	for _, value := range m {
		text, ok := value.(string)
		if !ok {
			continue
		}
		if match := entityURL.FindStringSubmatch(text); match != nil {
			word, digits := match[1], match[2]
			if word == "" {
				word, digits = match[3], match[4]
			}
			urlID, _ := strconv.Atoi(digits)
			return urlKinds[word], urlID, true
		}
	}
	return 0, 0, false
}

// textOf reads the first of keys that holds a string
func textOf(m map[string]any, keys []string) string {
	for _, key := range keys {
		if text, ok := m[key].(string); ok {
			return text
		}
	}
	return ""
}

// collect registers every entity in the tree, so every name is known before any text is rewritten
func (s *scrubber) collect(tree any) {
	switch node := tree.(type) {
	case map[string]any:
		if k, id, ok := entityOf(node); ok {
			s.names.register(id, entityReal{kind: k, name: textOf(node, nameKeys), ticker: textOf(node, tickerKeys)})
		}
		for _, child := range node {
			s.collect(child)
		}
	case []any:
		for _, child := range node {
			s.collect(child)
		}
	}
}

// prepare builds the finder of the real names (a ticker is only ever in a ticker field, which is rewritten as one), from everything registered so far
func (s *scrubber) prepare() {
	seen := map[string]bool{}
	var words []string
	add := func(word string) {
		word = strings.TrimSpace(word)
		if len(word) >= 4 && !seen[strings.ToLower(word)] {
			seen[strings.ToLower(word)] = true
			words = append(words, word)
		}
	}
	for _, real := range s.names.reals {
		add(real.name)
	}
	sort.Slice(words, func(i, j int) bool { return len(words[i]) > len(words[j]) })
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = regexp.QuoteMeta(word)
	}
	s.words = nil
	if len(quoted) > 0 {
		s.words = regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])(` + strings.Join(quoted, "|") + `)([^\p{L}\p{N}]|$)`)
	}
}

// rewrite replaces every real entity in the tree: its ids, names, and tickers, its pictures, its links, and then its name wherever else it is written
func (s *scrubber) rewrite(tree any, key string) any {
	switch node := tree.(type) {
	case map[string]any:
		_, id, isEntity := entityOf(node)
		if isEntity {
			k, _, _ := entityOf(node)
			fake := s.names.register(id, entityReal{kind: k})
			for _, idKey := range idKeys {
				if _, has := node[idKey]; has {
					node[idKey] = json.Number(strconv.Itoa(fake.ID))
				}
			}
			for _, nameKey := range nameKeys {
				if _, has := node[nameKey]; has {
					node[nameKey] = fake.Name
				}
			}
			for _, tickerKey := range tickerKeys {
				if text, has := node[tickerKey].(string); has && text != "" {
					node[tickerKey] = fake.Ticker
				}
			}
		}
		for childKey, child := range node {
			if isEntity && (slices.Contains(nameKeys, childKey) || slices.Contains(tickerKeys, childKey) || slices.Contains(idKeys, childKey)) {
				continue
			}
			node[childKey] = s.rewrite(child, childKey)
		}
		return node
	case []any:
		for i, child := range node {
			node[i] = s.rewrite(child, key)
		}
		return node
	case string:
		return s.rewriteText(node, key)
	}
	return tree
}

// rewriteText replaces a string's links and the real names in it
func (s *scrubber) rewriteText(text string, key string) string {
	if match := entityURL.FindString(text); match != "" {
		if strings.Contains(match, "images.evetech.net") {
			return placeholderImage
		}
		return ""
	}
	if pageURL.MatchString(text) {
		return ""
	}
	return s.replaceNames(text, key)
}

// replaceNames swaps each real name in text, as a whole word and whatever its case, for the made-up one
func (s *scrubber) replaceNames(text string, key string) string {
	if s.words == nil || len(text) < 4 {
		return text
	}
	for pass := 0; pass < 4; pass++ {
		changed := false
		text = s.words.ReplaceAllStringFunc(text, func(match string) string {
			parts := s.words.FindStringSubmatch(match)
			changed = true
			s.found[key]++
			return parts[1] + s.names.replacement(parts[2]) + parts[3]
		})
		if !changed {
			break
		}
	}
	return text
}

// leaks finds the real names and ids still in a tree, as a path and what was found for each
func (s *scrubber) leaks(tree any) []string {
	var found []string
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch value := node.(type) {
		case map[string]any:
			for key, child := range value {
				if text, ok := child.(string); ok && slices.Contains(tickerKeys, key) {
					if _, real := s.names.tickers[strings.ToLower(text)]; real && text != "" {
						found = append(found, fmt.Sprintf("%s.%s: real ticker %q", path, key, text))
					}
				}
				walk(child, path+"."+key)
			}
		case []any:
			for i, child := range value {
				walk(child, fmt.Sprintf("%s[%d]", path, i))
			}
		case string:
			if s.words != nil {
				if match := s.words.FindStringSubmatch(value); match != nil {
					found = append(found, fmt.Sprintf("%s: %q", path, match[2]))
				}
			}
		case json.Number:
			if id, err := value.Int64(); err == nil {
				if _, real := s.names.reals[int(id)]; real && int(id) < fakeIDBase {
					found = append(found, fmt.Sprintf("%s: real id %d", path, id))
				}
			}
		}
	}
	walk(tree, "")
	sort.Strings(found)
	return found
}
