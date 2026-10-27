package comm

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	crashLogMax  = 1 << 20   // past this size, the crash log is cut down to its newest crashLogKept bytes
	crashLogKept = 512 << 10 // starting at a whole entry
	crashEntry   = "=== "    // what each entry starts with
)

var (
	crashMu      sync.Mutex
	crashLogPath string // where panics are written, always, whether or not debug mode is on. Empty writes them only to the log.
)

// SetCrashLog sets the file panics are added to, ie crash.log beside the settings
func SetCrashLog(path string) {
	crashMu.Lock()
	defer crashMu.Unlock()
	crashLogPath = path
}

/*
	RecordPanic writes a recovered panic, with the stack of the goroutine it happened in, to the log and the crash log

Parameters:
  - where: what was running, ie "a frontend call" or "fetching a killmail"
  - value: what recover returned
*/
func RecordPanic(where string, value any) {
	stack := RedactPaths(string(debug.Stack()))
	Logger.WithField("stack", stack).Errorf("Panic in %s: %v", where, value)

	crashMu.Lock()
	defer crashMu.Unlock()
	if crashLogPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(crashLogPath), 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(crashLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	fmt.Fprintf(file, "%s%s panic in %s: %s\n%s\n", crashEntry, time.Now().UTC().Format(time.RFC3339), where, RedactPaths(fmt.Sprint(value)), stack)
	file.Close()
	trimCrashLog(crashLogPath)
}

// trimCrashLog cuts a crash log past crashLogMax down to its newest entries, so it can not grow without end. The caller must hold crashMu.
func trimCrashLog(path string) {
	if stat, err := os.Stat(path); err != nil || stat.Size() <= crashLogMax {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	os.WriteFile(path, []byte(NewestEntries(string(data), crashLogKept)), 0o600)
}

/*
	NewestEntries is the end of a crash log, at most limit bytes, starting at a whole entry

Parameters:
  - log: the crash log's text
  - limit: the most bytes to keep

Returns:
  - string: the newest entries that fit, or the last limit bytes if not even one does
*/
func NewestEntries(log string, limit int) string {
	if len(log) <= limit {
		return log
	}
	tail := log[len(log)-limit:]
	if start := strings.Index(tail, "\n"+crashEntry); start >= 0 {
		return tail[start+1:]
	}
	return tail
}

/*
	RecoverGoroutine is deferred at the top of a goroutine, so a panic in it is recorded, see RecordPanic, rather than closing the app.
	A goroutine whose caller waits on a result it would have sent passes then, to send one in its place.

Parameters:
  - where: what the goroutine does, for the log
  - then: called with the panic's value after it is recorded, nil for nothing
*/
func RecoverGoroutine(where string, then func(value any)) {
	if r := recover(); r != nil {
		RecordPanic(where, r)
		if then != nil {
			then(r)
		}
	}
}

// RedactPaths replaces the user's home folder in text with %USERPROFILE%, as it holds their Windows user name, so logs can be sent on
func RedactPaths(text string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return text
	}
	for _, form := range []string{home, filepath.ToSlash(home), strings.ReplaceAll(home, `\`, `\\`)} {
		text = strings.ReplaceAll(text, form, "%USERPROFILE%")
	}
	return text
}
