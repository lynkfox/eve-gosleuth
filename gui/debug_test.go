package gui

import (
	"archive/zip"
	"encoding/json"
	"errors"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// zipFiles reads every file in a zip by name
func zipFiles(t *testing.T, path string) map[string]string {
	archive, err := zip.OpenReader(path)
	assert.NoError(t, err)
	defer archive.Close()
	files := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		assert.NoError(t, err)
		data, _ := io.ReadAll(reader)
		reader.Close()
		files[file.Name] = string(data)
	}
	return files
}

func TestDebugMode(t *testing.T) {
	// debug app is an app in debug mode whose settings, debug folder, and crash log are in a temp folder
	debugApp := func(t *testing.T, on bool) *App {
		comm.InitLogger(logrus.InfoLevel)
		settings := defaultSettings()
		settings.DebugMode = on
		app := &App{settingsPath: filepath.Join(t.TempDir(), "settings.json"), settings: settings}
		app.setupDebug()
		t.Cleanup(func() { comm.SetCrashLog("") })
		return app
	}

	t.Run("debug mode logs everything, and only from Info up otherwise", func(t *testing.T) {
		debugApp(t, true)
		assert.Equal(t, logrus.DebugLevel, comm.Logger.GetLevel())
		debugApp(t, false)
		assert.Equal(t, logrus.InfoLevel, comm.Logger.GetLevel())
	})

	t.Run("a search's log is zipped with its session details and goroutines, and the loose log removed", func(t *testing.T) {
		app := debugApp(t, true)
		app.startDebug("URSA.")
		comm.Logger.Debug("Sending request")
		home, _ := os.UserHomeDir()
		comm.Logger.Infof("Cache at %s", filepath.Join(home, "eve-gosleuth", "cache.db"))
		app.noteDebugSearch(func(search *debugSearch) { search.TargetID, search.TargetType = 99013330, "ALLIANCE" })
		app.finishDebug(func(search *debugSearch) { search.Killmails = 20 })

		info := app.GetDebugInfo()
		assert.Equal(t, 1, info.Count)
		files := zipFiles(t, filepath.Join(app.debug.dir, info.Latest))
		assert.Contains(t, files["search.log"], "Sending request")
		assert.Contains(t, files["search.log"], "%USERPROFILE%")
		assert.NotContains(t, files["search.log"], home, "the user's home folder holds their user name")
		assert.Contains(t, files["goroutines.txt"], "goroutine")
		assert.NotContains(t, files, "static_data/ships.json", "no edited game data, so none copied")

		var session debugSession
		assert.NoError(t, json.Unmarshal([]byte(files["session.json"]), &session))
		assert.Equal(t, debugSearch{Typed: "URSA.", TargetID: 99013330, TargetType: "ALLIANCE", Killmails: 20, Took: session.Search.Took}, session.Search)
		assert.True(t, strings.HasPrefix(session.Build["go"], "go"), "the Go version, ie go1.24")
		assert.True(t, session.Preferences.DebugMode)

		logs, _ := filepath.Glob(filepath.Join(app.debug.dir, debugLogPrefix+"*.log"))
		assert.Empty(t, logs)
	})

	t.Run("nothing is logged to a file between searches, or with debug mode off", func(t *testing.T) {
		app := debugApp(t, false)
		app.startDebug("URSA.")
		comm.Logger.Info("Searching")
		app.finishDebug(nil)

		assert.Equal(t, DebugInfo{}, app.GetDebugInfo())
		assert.Error(t, app.OpenDebugFolder())
	})

	t.Run("the user's edited game data is copied into the bundle", func(t *testing.T) {
		edited := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(edited, "ships.json"), []byte(`{"capital_groups": {"513": null}}`), 0o644))
		assert.NoError(t, os.WriteFile(filepath.Join(edited, "notes.txt"), []byte("not game data"), 0o644))
		overrideDir = func() string { return edited }
		t.Cleanup(func() { overrideDir = staticdata.OverrideDir })

		app := debugApp(t, true)
		app.startDebug("URSA.")
		app.finishDebug(nil)

		files := zipFiles(t, filepath.Join(app.debug.dir, app.GetDebugInfo().Latest))
		assert.Equal(t, `{"capital_groups": {"513": null}}`, files["static_data/ships.json"])
		assert.NotContains(t, files, "static_data/notes.txt", "only json files are game data")
		assert.Contains(t, files["session.json"], "ships.json", "listed with its hash too")
	})

	t.Run("a search that fails is bundled with why", func(t *testing.T) {
		app := debugApp(t, true)
		app.startDebug("Nobody")
		err := errors.New("Nothing found by that name. Check spelling?")
		app.finishDebugOnError(&err)

		files := zipFiles(t, filepath.Join(app.debug.dir, app.GetDebugInfo().Latest))
		assert.Contains(t, files["session.json"], "Nothing found by that name")
	})

	t.Run("the log of a search the app closed during is bundled at the next start", func(t *testing.T) {
		app := debugApp(t, true)
		app.startDebug("URSA.")
		comm.Logger.Info("Collecting killmails")
		app.debug.mu.Lock()
		app.debug.file.Close()
		app.debug.file = nil // as if the app closed here
		app.debug.mu.Unlock()

		app.salvageDebugLogs()

		files := zipFiles(t, filepath.Join(app.debug.dir, app.GetDebugInfo().Latest))
		assert.Contains(t, files["search.log"], "Collecting killmails")
		assert.Contains(t, files["session.json"], "The app closed before this search finished")
	})

	t.Run("only the newest bundles are kept", func(t *testing.T) {
		app := debugApp(t, true)
		assert.NoError(t, os.MkdirAll(app.debug.dir, 0o700))
		start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.Local)
		for index := range debugBundlesKept + 3 {
			name := debugBundlePrefix + start.Add(time.Duration(index)*time.Minute).Format(debugTimeLayout) + ".zip"
			assert.NoError(t, os.WriteFile(filepath.Join(app.debug.dir, name), nil, 0o600))
		}

		pruneDebugBundles(app.debug.dir)

		bundles := debugBundles(app.debug.dir)
		assert.Len(t, bundles, debugBundlesKept)
		assert.Equal(t, debugBundlePrefix+start.Add(3*time.Minute).Format(debugTimeLayout)+".zip", filepath.Base(bundles[0]))
	})

	t.Run("a panic in a frontend call is written to the crash log with its stack, in debug mode or not", func(t *testing.T) {
		app := debugApp(t, false)
		err := func() (err error) {
			defer recoverAsError(&err)
			panic("index out of range")
		}()

		assert.EqualError(t, err, "Something went wrong: index out of range")
		crash, readErr := os.ReadFile(filepath.Join(filepath.Dir(app.settingsPath), crashLogName))
		assert.NoError(t, readErr)
		assert.Contains(t, string(crash), "panic in a frontend call: index out of range")
		assert.Contains(t, string(crash), "debug_test.go", "the stack shows where")
	})

	t.Run("a bundle holds the crash log, and none when there have been no crashes", func(t *testing.T) {
		app := debugApp(t, true)
		app.startDebug("URSA.")
		app.finishDebug(nil)
		files := zipFiles(t, filepath.Join(app.debug.dir, app.GetDebugInfo().Latest))
		_, hasCrashLog := files[crashLogName]
		assert.False(t, hasCrashLog)

		func() (err error) {
			defer recoverAsError(&err)
			panic("nil map")
		}()
		time.Sleep(2 * time.Millisecond) // a bundle is named for the millisecond its search started
		app.startDebug("URSA.")
		app.finishDebug(nil)

		files = zipFiles(t, filepath.Join(app.debug.dir, app.GetDebugInfo().Latest))
		assert.Contains(t, files[crashLogName], "panic in a frontend call: nil map")
	})
}
