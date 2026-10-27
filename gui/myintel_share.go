package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/comm/types"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	intelFileFormat  = "gosleuth-intel"
	intelFileVersion = 1
	maxIntelFileSize = 5 << 20 // an import larger than this is refused, as no real one comes close
	maxIntelEntries  = 20000   // the most holdings, or ignored allies, an import may hold
	intelLookups     = 10      // how many of an import's entities are checked with the ESI at once
)

// intelFileEntity is a pilot, corporation, or alliance in a My Intel file. Its name is there for people reading the file; an import looks it up again by id.
type intelFileEntity struct {
	Type string `json:"type"` // PILOT, CORPORATION, or ALLIANCE
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// intelFileHolding is one of a card's holdings in a My Intel file, with its note
type intelFileHolding struct {
	intelFileEntity
	Note string `json:"note,omitempty"`
}

// intelFileCard is a My Intel card in a file: the pilot, corporation, or alliance, the holdings it keeps, and its ignored allies
type intelFileCard struct {
	Entity        intelFileEntity    `json:"entity"`
	Holdings      []intelFileHolding `json:"holdings,omitempty"`
	IgnoredAllies []intelFileEntity  `json:"ignoredAllies,omitempty"`
}

// intelFile is a My Intel export, to give to other users: the cards exported, each as My Intel shows it
type intelFile struct {
	Format   string          `json:"format"`
	Version  int             `json:"version"`
	Exported time.Time       `json:"exported"`
	Cards    []intelFileCard `json:"cards"`
}

/*
	buildIntelFile makes the export of the chosen My Intel cards: the holdings each keeps and its ignored allies.
	A link where a card is another main's holding is that main's, so it goes only with the main.

Parameters:
  - cards: the chosen cards' keys, ie ALLIANCE:99013330
*/
func (a *App) buildIntelFile(cards []string) intelFile {
	chosen := map[string]bool{}
	for _, card := range cards {
		chosen[card] = true
	}
	file := intelFile{Format: intelFileFormat, Version: intelFileVersion, Exported: time.Now().UTC(), Cards: []intelFileCard{}}

	groups, _ := a.GetMyIntel()
	for _, group := range groups {
		if !chosen[intelGroupKey(group.Entity.Type, group.Entity.ID)] || len(group.Holdings)+len(group.IgnoredAllies) == 0 {
			continue
		}
		card := intelFileCard{Entity: intelFileEntity{Type: group.Entity.Type, ID: group.Entity.ID, Name: group.Entity.Name}}
		for _, holding := range group.Holdings {
			card.Holdings = append(card.Holdings, intelFileHolding{intelFileEntity: intelFileEntity{Type: holding.Type, ID: holding.ID, Name: holding.Name}, Note: holding.Note})
		}
		for _, ally := range group.IgnoredAllies {
			card.IgnoredAllies = append(card.IgnoredAllies, intelFileEntity{Type: ally.Type, ID: ally.ID, Name: ally.Name})
		}
		file.Cards = append(file.Cards, card)
	}
	return file
}

// importedHolding and importedIgnored are a file's cards as the links and lists the cache keeps, for checking one entry at a time
type importedHolding struct {
	Main, Holding intelFileEntity
	Note          string
}

type importedIgnored struct {
	Target intelFileEntity
	Allies []intelFileEntity
}

// flatten turns a file's cards into the holding links and ignored ally lists they hold
func (file intelFile) flatten() ([]importedHolding, []importedIgnored) {
	holdings, ignored := []importedHolding{}, []importedIgnored{}
	for _, card := range file.Cards {
		for _, holding := range card.Holdings {
			holdings = append(holdings, importedHolding{Main: card.Entity, Holding: holding.intelFileEntity, Note: holding.Note})
		}
		if len(card.IgnoredAllies) > 0 {
			ignored = append(ignored, importedIgnored{Target: card.Entity, Allies: card.IgnoredAllies})
		}
	}
	return holdings, ignored
}

/*
	ExportMyIntel saves the chosen My Intel cards to a file the user picks, to give to other users. It does not wait for a running search.

Parameters:
  - cards: the chosen cards' keys, ie ALLIANCE:99013330

Returns:
  - string: the file's name, empty if the user closed the picker
  - error: a user readable message if nothing was chosen or the file could not be written
*/
func (a *App) ExportMyIntel(cards []string) (name string, err error) {
	defer recoverAsError(&err)
	file := a.buildIntelFile(cards)
	if len(file.Cards) == 0 {
		return "", errors.New("Choose at least one card with something on it to export")
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export My Intel",
		DefaultFilename: "gosleuth-intel-" + time.Now().Format("2006-01-02") + ".json",
		Filters:         []runtime.FileFilter{{DisplayName: "GoSleuth intel (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return "", fmt.Errorf("Could not open the file picker: %w", err)
	}
	if path == "" {
		return "", nil
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("Could not write %s", filepath.Base(path))
	}
	return filepath.Base(path), nil
}

// IntelNoteChange is a holding an import has with another note than the user's
type IntelNoteChange struct {
	Main    string `json:"main"`
	Holding string `json:"holding"`
	Mine    string `json:"mine"`
	Theirs  string `json:"theirs"`
}

// ImportPreview is what an import would change, for the user to agree to before it does
type ImportPreview struct {
	Cancelled       bool              `json:"cancelled,omitempty"` // the user closed the picker
	File            string            `json:"file"`
	NewHoldings     int               `json:"newHoldings"`
	KnownHoldings   int               `json:"knownHoldings"` // already saved, with the same note
	NoteChanges     []IntelNoteChange `json:"noteChanges"`
	NewIgnored      int               `json:"newIgnored"`
	KnownIgnored    int               `json:"knownIgnored"`
	Skipped         int               `json:"skipped"`         // entries left out, as they did not check out with the ESI or were not valid
	SkippedExamples []string          `json:"skippedExamples"` // why, for the first few
}

// importPlan is a checked import, kept between its preview and the user agreeing to it
type importPlan struct {
	newHoldings    []retrieval.HoldingLink
	changedNotes   []retrieval.HoldingLink // with their notes
	newIgnored     map[string][]retrieval.IgnoredAlly
	ignoredTargets map[string]intelFileEntity
}

/*
	PreviewMyIntelImport asks for a My Intel file, checks it, and says what importing it would change, keeping it for ApplyMyIntelImport.
	It does not wait for a running search.

Returns:
  - ImportPreview: what would change, or Cancelled if the user closed the picker
  - error: a user readable message if the file can not be read or is not a My Intel file
*/
func (a *App) PreviewMyIntelImport() (preview ImportPreview, err error) {
	defer recoverAsError(&err)
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import My Intel",
		Filters: []runtime.FileFilter{{DisplayName: "GoSleuth intel (*.json)", Pattern: "*.json"}},
	})
	if err != nil {
		return preview, fmt.Errorf("Could not open the file picker: %w", err)
	}
	if path == "" {
		return ImportPreview{Cancelled: true}, nil
	}

	data, err := readLimited(path, maxIntelFileSize)
	if err != nil {
		return preview, err
	}
	plan, preview, err := a.planImport(data)
	if err != nil {
		return preview, err
	}
	preview.File = filepath.Base(path)

	a.importMu.Lock()
	a.pendingImport = &plan
	a.importMu.Unlock()
	return preview, nil
}

/*
	ApplyMyIntelImport saves the import last previewed. It only adds: nothing the user has is removed. It does not wait for a running search.

Parameters:
  - useTheirNotes: replace the user's notes with the file's where they differ, otherwise the user's are kept

Returns:
  - error: a user readable message if there is no previewed import
*/
func (a *App) ApplyMyIntelImport(useTheirNotes bool) (err error) {
	defer recoverAsError(&err)
	a.importMu.Lock()
	plan := a.pendingImport
	a.pendingImport = nil
	a.importMu.Unlock()
	if plan == nil {
		return errors.New("Choose a file to import first")
	}
	a.applyImport(*plan, useTheirNotes)
	return nil
}

func (a *App) applyImport(plan importPlan, useTheirNotes bool) {
	for _, link := range plan.newHoldings {
		a.cache.SaveHoldingLink(link)
	}
	if useTheirNotes {
		for _, link := range plan.changedNotes {
			a.cache.SaveHoldingLink(link)
		}
	}
	for key, allies := range plan.newIgnored {
		target := plan.ignoredTargets[key]
		a.cache.SetIgnoredAllies(target.Type, target.ID, append(slices.Clone(a.cache.IgnoredAlliesFor(target.Type, target.ID)), allies...))
	}
	a.saveCacheQuietly()
}

// readLimited reads a file, refusing one larger than limit
func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Could not open %s", filepath.Base(path))
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("Could not read %s", filepath.Base(path))
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is too large to be a My Intel file", filepath.Base(path))
	}
	return data, nil
}

/*
	planImport checks a My Intel file and works out what importing it would change. The file is untrusted: each entry must be well formed,
	and each pilot, corporation, and alliance in it must be found with the ESI as the type the file says, by id, its name taken from the ESI rather than the file.
	Entries that fail are skipped and counted.

Returns:
  - importPlan: what to save
  - ImportPreview: what would change
  - error: a user readable message if it is not a My Intel file this version can read
*/
func (a *App) planImport(data []byte) (importPlan, ImportPreview, error) {
	plan := importPlan{newIgnored: map[string][]retrieval.IgnoredAlly{}, ignoredTargets: map[string]intelFileEntity{}}
	preview := ImportPreview{NoteChanges: []IntelNoteChange{}, SkippedExamples: []string{}}

	var file intelFile
	if err := json.Unmarshal(data, &file); err != nil || file.Format != intelFileFormat {
		return plan, preview, errors.New("This is not a My Intel file")
	}
	if file.Version != intelFileVersion {
		return plan, preview, fmt.Errorf("This My Intel file is version %d, which this version of GoSleuth can not read", file.Version)
	}
	holdings, ignoredLists := file.flatten()
	allyCount := 0
	for _, ignored := range ignoredLists {
		allyCount += len(ignored.Allies)
	}
	if len(holdings) > maxIntelEntries || allyCount > maxIntelEntries {
		return plan, preview, errors.New("This My Intel file has more entries than GoSleuth imports")
	}

	skip := func(reason string) {
		preview.Skipped++
		if len(preview.SkippedExamples) < 5 {
			preview.SkippedExamples = append(preview.SkippedExamples, reason)
		}
	}

	// every entity named in the file, checked with the ESI once each
	entities := []intelFileEntity{}
	for _, holding := range holdings {
		entities = append(entities, holding.Main, holding.Holding)
	}
	for _, ignored := range ignoredLists {
		entities = append(entities, ignored.Target)
		entities = append(entities, ignored.Allies...)
	}
	names := a.checkIntelEntities(entities)
	checked := func(entity intelFileEntity, kinds ...string) (string, bool) {
		if !slices.Contains(kinds, entity.Type) {
			skip(fmt.Sprintf("%q is not a %s", entity.Name, strings.ToLower(strings.Join(kinds, " or "))))
			return "", false
		}
		name, found := names[intelGroupKey(entity.Type, entity.ID)]
		if !found {
			skip(fmt.Sprintf("%s %d (%q) was not found with the ESI", strings.ToLower(entity.Type), entity.ID, entity.Name))
		}
		return name, found
	}
	groupKinds := []string{types.CORPORATION.String(), types.ALLIANCE.String()}
	targetKinds := append([]string{types.PILOT.String()}, groupKinds...)

	existingLinks := map[string]retrieval.HoldingLink{}
	for _, link := range a.cache.AllHoldingLinks() {
		existingLinks[intelGroupKey(link.MainType, link.MainID)+">"+intelGroupKey(link.HoldingType, link.HoldingID)] = link
	}
	planned := map[string]bool{}
	for _, holding := range holdings {
		mainName, mainOK := checked(holding.Main, groupKinds...)
		holdingName, holdingOK := checked(holding.Holding, groupKinds...)
		if !mainOK || !holdingOK {
			continue
		}
		if holding.Main.Type == holding.Holding.Type && holding.Main.ID == holding.Holding.ID {
			skip(fmt.Sprintf("%s can not be its own holding", mainName))
			continue
		}
		key := intelGroupKey(holding.Main.Type, holding.Main.ID) + ">" + intelGroupKey(holding.Holding.Type, holding.Holding.ID)
		if planned[key] {
			continue
		}
		planned[key] = true
		note := strings.TrimSpace(holding.Note)
		if len([]rune(note)) > maxHoldingNote {
			note = string([]rune(note)[:maxHoldingNote])
		}
		link := retrieval.HoldingLink{MainType: holding.Main.Type, MainID: holding.Main.ID, MainName: mainName,
			HoldingType: holding.Holding.Type, HoldingID: holding.Holding.ID, HoldingName: holdingName, Note: note}
		existing, known := existingLinks[key]
		switch {
		case !known:
			plan.newHoldings = append(plan.newHoldings, link)
		case existing.Note == note || note == "":
			preview.KnownHoldings++
		default:
			plan.changedNotes = append(plan.changedNotes, link)
			preview.NoteChanges = append(preview.NoteChanges, IntelNoteChange{Main: mainName, Holding: holdingName, Mine: existing.Note, Theirs: note})
		}
	}
	preview.NewHoldings = len(plan.newHoldings)

	for _, ignored := range ignoredLists {
		if _, ok := checked(ignored.Target, targetKinds...); !ok {
			continue
		}
		key := intelGroupKey(ignored.Target.Type, ignored.Target.ID)
		have := a.cache.IgnoredAlliesFor(ignored.Target.Type, ignored.Target.ID)
		for _, ally := range ignored.Allies {
			allyName, ok := checked(ally, groupKinds...)
			if !ok {
				continue
			}
			same := func(other retrieval.IgnoredAlly) bool { return other.Type == ally.Type && other.ID == ally.ID }
			switch {
			case ally.Type == ignored.Target.Type && ally.ID == ignored.Target.ID:
				skip(fmt.Sprintf("%s can not be its own ally", allyName))
			case slices.ContainsFunc(have, same):
				preview.KnownIgnored++
			case slices.ContainsFunc(plan.newIgnored[key], same):
				// listed twice in the file
			default:
				plan.newIgnored[key] = append(plan.newIgnored[key], retrieval.IgnoredAlly{Type: ally.Type, ID: ally.ID, Name: allyName})
				plan.ignoredTargets[key] = ignored.Target
				preview.NewIgnored++
			}
		}
	}
	return plan, preview, nil
}

/*
	checkIntelEntities looks up an import's pilots, corporations, and alliances with the ESI, through the cache, a few at a time

Returns:
  - map[string]string: the name of each found as the type the file says, by intelGroupKey. Those not found, or of another type, are left out.
*/
func (a *App) checkIntelEntities(entities []intelFileEntity) map[string]string {
	unique := map[string]intelFileEntity{}
	for _, entity := range entities {
		if entity.ID > 0 && slices.Contains([]string{"PILOT", "CORPORATION", "ALLIANCE"}, entity.Type) {
			unique[intelGroupKey(entity.Type, entity.ID)] = entity
		}
	}

	names := map[string]string{}
	var mu sync.Mutex
	var wait sync.WaitGroup
	slots := make(chan struct{}, intelLookups)
	for key, entity := range unique {
		wait.Add(1)
		go func() {
			defer comm.RecoverGoroutine("checking an imported entity", nil)
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			name := ""
			switch entity.Type {
			case types.ALLIANCE.String():
				if alliance, err := a.clients.ESIClient.GetAllianceInfo(entity.ID); err == nil && alliance != nil {
					name = alliance.Name
				}
			case types.CORPORATION.String():
				if corp, err := a.clients.ESIClient.GetCorporationInfo(entity.ID); err == nil && corp != nil {
					name = corp.Name
				}
			case types.PILOT.String():
				if pilot, err := a.clients.ESIClient.GetCharacterInfo(entity.ID); err == nil && pilot != nil {
					name = pilot.Name
				}
			}
			if name != "" {
				mu.Lock()
				names[key] = name
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	return names
}
