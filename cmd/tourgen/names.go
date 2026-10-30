package main

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// The made-up players are built from these word lists. Ids and names come from a hash of the real id, so a regeneration names the same entity the same.

const (
	// fakeIDBase is where the made-up ids start. Real ids stop near 2.2 billion, so nothing the frontend or the ESI knows sits above it, and the NPC (1M to 4M),
	// system (30M), station (60M) and type id ranges are far below it.
	fakeIDBase = 5_000_000_000
	fakeIDSpan = 100_000_000 // each kind's own span above the base: pilots, then corporations, then alliances

	// placeholderImage is the one neutral picture every made-up pilot, corporation, and alliance shows, beside the tour's data files
	placeholderImage = "tour/placeholder.svg"

	// firstPlayerID is the lowest id a player's pilot, corp, or alliance can have: below it are the factions, NPC corps, and NPC agents, which are game data and stay as they are
	firstPlayerID = 10_000_000
)

var pilotFirst = strings.Fields(`Aldric Brenna Calder Dorian Elara Fenwick Garrick Hesper Isolde Jorin Kestrel Lysander Mirela Nolan Orla Perrin Quillon Rowan Sable Tamsin
	Ulric Vesna Wystan Xanthe Yorick Zephyr Alder Briar Corin Dara Emeric Fallon Gideon Hallie Ione Jasper Kaia Lorcan Maren Nessa Oswin Pia Quinn Rhea Soren Thale Una Vance
	Wren Yara Zane Anselm Bryn Cassian Delphine Eamon Fiora Gwyn Hollis Ivor Juno Kellan`)

var pilotLast = strings.Fields(`Ashgrove Brightwater Cinderfell Dunmere Eastwick Fairbanks Greywick Hollowell Ironwood Juniper Kettleby Larkspur Marlowe Northcott Oakhurst Pennywhistle
	Quillfeather Ravenscroft Stonebridge Thistledown Underhill Valewood Whitlock Yarrow Zephyrine Amberly Birchfield Copperfield Driftwood Emberly Foxglove Glenmoor Hawthorn
	Inglenook Kingsley Lowmoor Mistral Nightingale Oldcastle Pinecroft Redmayne Silverleaf Tumbleton Westcombe Applegate Bellweather Crowhurst Daffodil Elmstead Fernsby`)

var corpFirst = strings.Fields(`Amber Beacon Cobalt Drift Ember Fable Granite Harbor Indigo Juniper Keystone Lantern Meridian Nimbus Orchard Pioneer Quartz Ripple Summit Tidewater
	Umber Vantage Willow Zenith Ashen Brass Cedar Dawn Echo Flint Gilded Hollow Iron Jade Kindle Lark`)

var corpSecond = strings.Fields(`Anchor Bridge Compass Dock Engine Forge Gate Hammer Island Junction Kiln Lodge Mill Network Outpost Pier Quarry Reach Spire Tower Union Vault Wharf Yard
	Arch Basin Crossing Depot Field Grove Haven Ledge Mesa Narrows Orbit Pass Ridge`)

var corpSuffix = []string{"Logistics", "Industries", "Salvage", "Freight", "Mining Co", "Trading", "Survey", "Cooperative", "Expeditions", "Works", "Holdings", "Collective"}

var allianceFirst = strings.Fields(`Silver Northern Crimson Quiet Distant Golden Hidden Midnight Rising Scarlet Verdant Wandering Azure Burning Calm Dusky Eternal Fading Gentle Hollow Lonely Mellow Noble Open Patient`)

var allianceSecond = []string{"Tide", "Lanterns", "Wolves", "Harvest", "Meridian", "Horizon", "Foxes", "Orchard", "Ravens", "Compass", "Cairn", "Sparrows", "Embers", "Willows", "Anchors", "Comets", "Kestrels", "Beacons", "Thistles", "Moors"}

var allianceSuffix = []string{"Pact", "Accord", "League", "Alliance", "Confederacy", "Compact", "Union", "Coalition"}

// kind of a made-up entity, which sets its id range and the shape of its name
type kind int

const (
	pilotKind kind = iota
	corpKind
	allianceKind
)

// kindOfType reads the PILOT, CORPORATION, or ALLIANCE the app's JSON names an entity type by
func kindOfType(entityType string) (kind, bool) {
	switch entityType {
	case "PILOT":
		return pilotKind, true
	case "CORPORATION":
		return corpKind, true
	case "ALLIANCE":
		return allianceKind, true
	}
	return 0, false
}

func (k kind) String() string {
	return [...]string{"PILOT", "CORPORATION", "ALLIANCE"}[k]
}

// entityRealNames is a real entity as found in the reports, before it is replaced
type entityReal struct {
	kind   kind
	name   string
	ticker string
}

// fakeEntity is what a real entity is shown as
type fakeEntity struct {
	ID     int
	Name   string
	Ticker string
}

// fixedNames are the entities with a name picked by hand rather than from the word lists, by real id
var fixedNames = map[int]fakeEntity{}

// renamer holds the one map from real entity to made-up entity, shared by both demo files
type renamer struct {
	fakes    map[int]fakeEntity // by real id
	reals    map[int]entityReal // by real id
	usedIDs  map[int]bool       // made-up ids taken
	usedName map[string]bool    // made-up names taken, lower case
	usedTick map[string]bool    // made-up tickers taken, by kind and ticker
	names    map[string]string  // real name, lower case, to its replacement, for the names in free text
	tickers  map[string]string  // real ticker, lower case, to its replacement
}

func newRenamer() *renamer {
	return &renamer{fakes: map[int]fakeEntity{}, reals: map[int]entityReal{}, usedIDs: map[int]bool{}, usedName: map[string]bool{}, usedTick: map[string]bool{}, names: map[string]string{}, tickers: map[string]string{}}
}

// hashOf is a stable number for a string
func hashOf(text string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(text))
	return h.Sum64()
}

/*
	register notes a real entity and gives it its made-up one, the same one each time it is seen.

Parameters:
  - id: the real id
  - found: its kind and, when this sighting has them, its real name and ticker

Returns:
  - fakeEntity: the entity it is shown as, its ticker filled in when it has been seen with one
*/
func (r *renamer) register(id int, found entityReal) fakeEntity {
	known, seen := r.reals[id]
	if seen {
		if known.name == "" {
			known.name = found.name
		}
		if known.ticker == "" {
			known.ticker = found.ticker
		}
		r.reals[id] = known
	} else {
		known = found
		r.reals[id] = known
	}
	fake, made := r.fakes[id]
	if !made {
		fake = r.make(id, known.kind)
	}
	if fake.Ticker == "" && known.ticker != "" && known.kind != pilotKind {
		fake.Ticker = r.ticker(id, known.kind, fake.Name)
	}
	r.fakes[id] = fake
	if known.name != "" {
		r.names[strings.ToLower(known.name)] = fake.Name
	}
	if known.ticker != "" && fake.Ticker != "" {
		r.tickers[strings.ToLower(known.ticker)] = fake.Ticker
	}
	return fake
}

// make picks the made-up id and name for a real id
func (r *renamer) make(id int, k kind) fakeEntity {
	if fixed, ok := fixedNames[id]; ok {
		r.usedIDs[fixed.ID] = true
		r.usedName[strings.ToLower(fixed.Name)] = true
		return fixed
	}
	seed := hashOf(fmt.Sprintf("%s:%d", k, id))
	fakeID := fakeIDBase + int(k)*fakeIDSpan + int(seed%uint64(fakeIDSpan))
	for r.usedIDs[fakeID] {
		fakeID++
	}
	r.usedIDs[fakeID] = true
	name := ""
	for attempt := uint64(0); ; attempt++ {
		name = nameFor(k, seed+attempt*0x9e3779b97f4a7c15)
		if !r.usedName[strings.ToLower(name)] {
			break
		}
	}
	r.usedName[strings.ToLower(name)] = true
	return fakeEntity{ID: fakeID, Name: name}
}

// nameFor builds a name of a kind from a number
func nameFor(k kind, n uint64) string {
	pick := func(words []string) string {
		w := words[n%uint64(len(words))]
		n /= uint64(len(words))
		return w
	}
	switch k {
	case pilotKind:
		return pick(pilotFirst) + " " + pick(pilotLast)
	case corpKind:
		return pick(corpFirst) + " " + pick(corpSecond) + " " + pick(corpSuffix)
	}
	return pick(allianceFirst) + " " + pick(allianceSecond) + " " + pick(allianceSuffix)
}

// ticker makes a corp's or alliance's made-up ticker, at most 5 characters of capital letters and digits, from its made-up name and not used before
func (r *renamer) ticker(id int, k kind, name string) string {
	if fixed, ok := fixedNames[id]; ok && fixed.Ticker != "" {
		r.usedTick[k.String()+fixed.Ticker] = true
		return fixed.Ticker
	}
	words := strings.Fields(strings.ToUpper(name))
	base := ""
	for i, w := range words {
		take := 2
		if i > 0 {
			take = 1
		}
		if len(w) < take {
			take = len(w)
		}
		base += w[:take]
	}
	if len(base) > 5 {
		base = base[:5]
	}
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	seed := hashOf(name)
	ticker := base
	for attempt := 0; r.usedTick[k.String()+ticker]; attempt++ {
		// keep the first letters and swap the last for another
		ticker = base[:len(base)-1] + string(letters[(seed+uint64(attempt))%26])
		if attempt > 26 {
			ticker = base[:len(base)-2] + string(letters[(seed+uint64(attempt))%26]) + string(letters[(seed>>8+uint64(attempt))%26])
		}
	}
	r.usedTick[k.String()+ticker] = true
	return ticker
}

// replacement is what the made-up version of a real name or ticker is, empty for one not registered
func (r *renamer) replacement(real string) string {
	lower := strings.ToLower(real)
	if name, ok := r.names[lower]; ok {
		return name
	}
	return r.tickers[lower]
}
