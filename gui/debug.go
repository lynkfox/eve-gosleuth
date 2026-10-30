package gui

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/lynkfox/go_eve_sleuth/retrieval"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"github.com/sirupsen/logrus"
)

const (
	debugFolderName    = "debug"     // beside the settings, holding the debug bundles
	crashLogName       = "crash.log" // beside the settings, every panic, whether or not debug mode is on
	debugBundlePrefix  = "gosleuth-debug-"
	debugLogPrefix     = "search-"
	debugBundlesKept   = 10
	crashLogInBundle   = 256 << 10             // the most of the crash log a bundle holds, its newest entries
	debugTimeLayout    = "20060102-150405.000" // to the millisecond, so two searches never share a bundle name, and sorting the names sorts them by time
	frontendErrorLimit = 4000                  // the most characters of a frontend error kept, so a runaway one can not fill the log
)

// debugSearch is what a debug bundle says about its search, beyond the log
type debugSearch struct {
	Typed       string   `json:"typed"` // the name searched for, as typed
	TargetID    int      `json:"targetId,omitempty"`
	TargetType  string   `json:"targetType,omitempty"`
	TargetName  string   `json:"targetName,omitempty"`
	Holding     string   `json:"searchedHolding,omitempty"` // the holding searched, when the search went to its main
	Holdings    []string `json:"holdings,omitempty"`        // the holdings searched with the target
	Killmails   int      `json:"killmails"`
	Stopped     bool     `json:"stopped,omitempty"`
	RateLimited bool     `json:"rateLimited,omitempty"`
	Error       string   `json:"error,omitempty"` // how the search failed, empty if it did not
	Took        string   `json:"took"`
}

/*
debugRecorder writes every log entry to a file while a search runs in debug mode, then bundles it into a zip to send with a problem report.
It is a logrus hook, added once at startup, that writes nothing between searches.
*/
type debugRecorder struct {
	mu       sync.Mutex
	dir      string
	file     *os.File // the running search's log, nil between searches
	started  time.Time
	search   debugSearch
	frontend string // the frontend's user agent, which names the WebView2 version
}

var debugFormatter = &logrus.JSONFormatter{}

func (r *debugRecorder) Levels() []logrus.Level { return logrus.AllLevels }

// Fire writes a log entry to the running search's log, with the user's home folder taken out of it
func (r *debugRecorder) Fire(entry *logrus.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return nil
	}
	line, err := debugFormatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = r.file.WriteString(comm.RedactPaths(string(line)))
	return err
}

// setupDebug adds the debug recorder to the log and points panics at the crash log. It is called once the settings, which say where both go, are loaded.
func (a *App) setupDebug() {
	settingsDir := filepath.Dir(a.settingsPath)
	a.debug = &debugRecorder{dir: filepath.Join(settingsDir, debugFolderName)}
	comm.Logger.AddHook(a.debug)
	comm.SetCrashLog(filepath.Join(settingsDir, crashLogName))
	a.applyLogLevel()
	a.salvageDebugLogs()
}

// applyLogLevel logs everything in debug mode, and only from Info up otherwise
func (a *App) applyLogLevel() {
	if a.currentSettings().DebugMode {
		comm.Logger.SetLevel(logrus.DebugLevel)
	} else {
		comm.Logger.SetLevel(logrus.InfoLevel)
	}
}

// startDebug starts a search's debug log, in debug mode. A search already logging, ie one asked again after choosing between matches, carries on in the same log.
func (a *App) startDebug(typed string) {
	if a.debug == nil || !a.currentSettings().DebugMode {
		return
	}
	a.debug.mu.Lock()
	defer a.debug.mu.Unlock()
	if a.debug.file != nil {
		return
	}
	if err := os.MkdirAll(a.debug.dir, 0o700); err != nil {
		return
	}
	file, err := os.Create(filepath.Join(a.debug.dir, debugLogPrefix+time.Now().Format(debugTimeLayout)+".log"))
	if err != nil {
		return
	}
	a.debug.file, a.debug.started, a.debug.search = file, time.Now(), debugSearch{Typed: typed}
}

// noteDebugSearch adds to what the running search's bundle says about it, if one is logging
func (a *App) noteDebugSearch(note func(*debugSearch)) {
	if a.debug == nil {
		return
	}
	a.debug.mu.Lock()
	defer a.debug.mu.Unlock()
	if a.debug.file != nil {
		note(&a.debug.search)
	}
}

// finishDebugOnError bundles the running search's log when a search ends in an error, deferred by SearchTarget before recoverAsError so it sees a panic's error too
func (a *App) finishDebugOnError(err *error) {
	if *err != nil {
		a.finishDebug(func(search *debugSearch) { search.Error = (*err).Error() })
	}
}

// finishDebug ends the running search's log, if one is logging, and bundles it with the session's details into a zip in the debug folder, keeping the newest debugBundlesKept
func (a *App) finishDebug(note func(*debugSearch)) {
	if a.debug == nil {
		return
	}
	a.debug.mu.Lock()
	file, search, started, frontend := a.debug.file, a.debug.search, a.debug.started, a.debug.frontend
	a.debug.file = nil
	a.debug.mu.Unlock()
	if file == nil {
		return
	}
	if note != nil {
		note(&search)
	}
	search.Took = time.Since(started).Round(time.Millisecond).String()

	logPath := file.Name()
	file.Close()
	bundle, err := a.writeDebugBundle(logPath, started, a.debugSession(search, frontend))
	if err != nil {
		comm.Logger.WithError(err).Warn("Could not save the debug log")
		return
	}
	os.Remove(logPath)
	pruneDebugBundles(a.debug.dir)
	comm.ReportProgress("Debug log saved as %s", filepath.Base(bundle))
}

// salvageDebugLogs bundles the logs of searches that never finished, as when the app closed or crashed during one, which are the ones most worth sending
func (a *App) salvageDebugLogs() {
	logs, _ := filepath.Glob(filepath.Join(a.debug.dir, debugLogPrefix+"*.log"))
	for _, logPath := range logs {
		started, err := time.ParseInLocation(debugTimeLayout, strings.TrimSuffix(strings.TrimPrefix(filepath.Base(logPath), debugLogPrefix), ".log"), time.Local)
		if err != nil {
			started = time.Now()
		}
		search := debugSearch{Error: "The app closed before this search finished"}
		if _, err := a.writeDebugBundle(logPath, started, a.debugSession(search, "")); err == nil {
			os.Remove(logPath)
		}
	}
	pruneDebugBundles(a.debug.dir)
}

// debugSession are the details of the app and its machine a bundle holds besides the log, none of them personal
type debugSession struct {
	Created          string            `json:"created"`
	Build            map[string]string `json:"build"`
	OS               string            `json:"os"`
	Frontend         string            `json:"frontend,omitempty"` // the WebView's user agent, which names its version
	Preferences      Preferences       `json:"preferences"`
	Theme            string            `json:"theme"`
	CacheMoved       bool              `json:"cacheMoved"`
	SearchLimits     SearchLimits      `json:"searchLimits"` // what the search used
	LimitsOverridden bool              `json:"limitsOverridden"`
	Search           debugSearch       `json:"search"`
	Cache            debugCache        `json:"cache"`
	StaticOverrides  map[string]string `json:"staticOverrides"` // the static data files used from beside the exe, by name, with each one's sha256
	Memory           debugMemory       `json:"memory"`
}

type debugCache struct {
	Entries   int                   `json:"entries"`
	SizeBytes int64                 `json:"sizeBytes"`
	Lookups   retrieval.CacheCounts `json:"lookups"` // this session's cache hits and ESI fetches
}

type debugMemory struct {
	AllocMB    uint64 `json:"allocMB"`
	SysMB      uint64 `json:"sysMB"`
	NumGC      uint32 `json:"numGC"`
	Goroutines int    `json:"goroutines"`
}

func (a *App) debugSession(search debugSearch, frontend string) debugSession {
	settings := a.currentSettings()
	a.settingsMu.Lock()
	overridden := a.searchLimits != nil
	a.settingsMu.Unlock()

	session := debugSession{
		Created:          time.Now().UTC().Format(time.RFC3339),
		Build:            buildDetails(),
		OS:               osVersion() + " " + runtime.GOARCH,
		Frontend:         frontend,
		Preferences:      settings.Preferences,
		Theme:            settings.Theme,
		CacheMoved:       settings.CacheDir != "",
		SearchLimits:     a.searchPreferences().searchLimits(),
		LimitsOverridden: overridden,
		Search:           search,
		StaticOverrides:  staticOverrides(),
	}
	if a.cache != nil {
		info := a.GetCacheInfo()
		session.Cache = debugCache{Entries: info.Entries, SizeBytes: info.SizeBytes, Lookups: a.cache.Counts()}
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	session.Memory = debugMemory{AllocMB: memory.Alloc >> 20, SysMB: memory.Sys >> 20, NumGC: memory.NumGC, Goroutines: runtime.NumGoroutine()}
	return session
}

// buildDetails are the app's build: its commit and when, whether it had uncommitted changes, and the Go version
func buildDetails() map[string]string {
	details := map[string]string{"go": runtime.Version()}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return details
	}
	details["version"] = info.Main.Version
	for _, setting := range info.Settings {
		if strings.HasPrefix(setting.Key, "vcs.") {
			details[setting.Key] = setting.Value
		}
	}
	return details
}

// overrideDir is the static data folder beside the exe, a variable so tests can point it elsewhere
var overrideDir = staticdata.OverrideDir

// overrideFiles are the json files in the static data folder beside the exe, by full path, none if there is no folder
func overrideFiles() []string {
	dir := overrideDir()
	if dir == "" {
		return nil
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	return files
}

// staticOverrides are the static data files in use from the folder beside the exe, with a hash of each, as edited game data can explain odd results
func staticOverrides() map[string]string {
	overrides := map[string]string{}
	for _, path := range overrideFiles() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		overrides[filepath.Base(path)] = hex.EncodeToString(sum[:])
	}
	return overrides
}

/*
	writeDebugBundle zips a search's log with its session details, the newest of the crash log, a dump of every goroutine, and a copy of each static data file the user edited (under static_data/), each a readable text file, so the user can see what they send

Returns:
  - string: the zip's path
  - error: the zip could not be written
*/
func (a *App) writeDebugBundle(logPath string, started time.Time, session debugSession) (string, error) {
	bundlePath := filepath.Join(a.debug.dir, debugBundlePrefix+started.Format(debugTimeLayout)+".zip")
	temp := bundlePath + ".tmp"
	out, err := os.Create(temp)
	if err != nil {
		return "", err
	}
	archive := zip.NewWriter(out)

	writeErr := func() error {
		logFile, err := os.Open(logPath)
		if err != nil {
			return err
		}
		defer logFile.Close()
		entry, err := archive.Create("search.log")
		if err != nil {
			return err
		}
		if _, err := io.Copy(entry, logFile); err != nil {
			return err
		}

		sessionJSON, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return err
		}
		entry, err = archive.Create("session.json")
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(comm.RedactPaths(string(sessionJSON)))); err != nil {
			return err
		}

		// earlier crashes too, already without the user's home folder
		if crashLog, err := os.ReadFile(filepath.Join(filepath.Dir(a.settingsPath), crashLogName)); err == nil {
			entry, err = archive.Create(crashLogName)
			if err != nil {
				return err
			}
			if _, err := entry.Write([]byte(comm.NewestEntries(string(crashLog), crashLogInBundle))); err != nil {
				return err
			}
		}

		// the user's edited game data, which can explain odd results, as it was when the search ran
		for _, path := range overrideFiles() {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			entry, err = archive.Create("static_data/" + filepath.Base(path))
			if err != nil {
				return err
			}
			if _, err := entry.Write([]byte(comm.RedactPaths(string(data)))); err != nil {
				return err
			}
		}

		var goroutines strings.Builder
		pprof.Lookup("goroutine").WriteTo(&goroutines, 2)
		entry, err = archive.Create("goroutines.txt")
		if err != nil {
			return err
		}
		_, err = entry.Write([]byte(comm.RedactPaths(goroutines.String())))
		return err
	}()

	closeErr := errors.Join(archive.Close(), out.Close())
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(temp)
		return "", err
	}
	if err := os.Rename(temp, bundlePath); err != nil {
		os.Remove(temp)
		return "", err
	}
	return bundlePath, nil
}

// debugBundles are the bundles in the debug folder, oldest first, as their names hold when they were made
func debugBundles(dir string) []string {
	bundles, _ := filepath.Glob(filepath.Join(dir, debugBundlePrefix+"*.zip"))
	slices.Sort(bundles)
	return bundles
}

// pruneDebugBundles removes all but the newest debugBundlesKept bundles
func pruneDebugBundles(dir string) {
	bundles := debugBundles(dir)
	for len(bundles) > debugBundlesKept {
		os.Remove(bundles[0])
		bundles = bundles[1:]
	}
}

// DebugInfo is the newest debug bundle, for the Preferences window's Open debug folder button
type DebugInfo struct {
	Latest string `json:"latest"` // its file name, empty when there is none
	Count  int    `json:"count"`
}

// GetDebugInfo returns the newest debug bundle. It does not wait for a running search.
func (a *App) GetDebugInfo() DebugInfo {
	if a.debug == nil {
		return DebugInfo{}
	}
	bundles := debugBundles(a.debug.dir)
	if len(bundles) == 0 {
		return DebugInfo{}
	}
	return DebugInfo{Latest: filepath.Base(bundles[len(bundles)-1]), Count: len(bundles)}
}

/*
	OpenDebugFolder opens the debug folder in the file manager with the newest bundle selected, ready to send. It does not wait for a running search.

Returns:
  - error: a user readable message if there is no bundle or the folder could not be opened
*/
func (a *App) OpenDebugFolder() error {
	if a.debug == nil {
		return errors.New("There is no debug log yet")
	}
	bundles := debugBundles(a.debug.dir)
	if len(bundles) == 0 {
		return errors.New("There is no debug log yet, search with Debug mode on to make one")
	}
	if err := openFolderSelecting(bundles[len(bundles)-1]); err != nil {
		comm.Logger.WithError(err).Warn("Could not open the debug folder")
		return fmt.Errorf("Could not open the debug folder %s", a.debug.dir)
	}
	return nil
}

// LogFrontendError logs an error from the frontend's JavaScript, which is otherwise invisible, with its stack. It does not wait for a running search.
func (a *App) LogFrontendError(message string) {
	if len(message) > frontendErrorLimit {
		message = message[:frontendErrorLimit] + "…"
	}
	comm.Logger.WithField("source", "frontend").Error(comm.RedactPaths(message))
}

// SetFrontendInfo keeps the frontend's user agent, which names the WebView2 version, for debug bundles. It does not wait for a running search.
func (a *App) SetFrontendInfo(userAgent string) {
	a.waitReady()
	if a.debug == nil {
		return
	}
	a.debug.mu.Lock()
	defer a.debug.mu.Unlock()
	a.debug.frontend = userAgent
}
