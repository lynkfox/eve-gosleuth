package comm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestRecoverGoroutine(t *testing.T) {
	comm.InitLogger(logrus.InfoLevel)
	crashLog := filepath.Join(t.TempDir(), "crash.log")
	comm.SetCrashLog(crashLog)
	t.Cleanup(func() { comm.SetCrashLog("") })

	t.Run("a panicking goroutine is recorded with its stack and handed on, rather than closing the app", func(t *testing.T) {
		handed := make(chan any, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer comm.RecoverGoroutine("fetching a killmail", func(value any) { handed <- value })
			panic("nil map")
		}()
		<-done

		assert.Equal(t, "nil map", <-handed)
		crash, err := os.ReadFile(crashLog)
		assert.NoError(t, err)
		assert.Contains(t, string(crash), "panic in fetching a killmail: nil map")
		assert.Contains(t, string(crash), "crash_test.go")
	})
}

func TestRedactPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder")
	}
	assert.Equal(t, `cache at %USERPROFILE%/eve-gosleuth`, comm.RedactPaths("cache at "+filepath.ToSlash(home)+"/eve-gosleuth"))
	assert.Equal(t, "no paths here", comm.RedactPaths("no paths here"))
}

func TestNewestEntries(t *testing.T) {
	log := "=== one panic\nstack one\n=== two panic\nstack two\n"

	assert.Equal(t, log, comm.NewestEntries(log, 1000), "a log that fits is kept whole")
	assert.Equal(t, "=== two panic\nstack two\n", comm.NewestEntries(log, 30), "cut at the start of a whole entry")
	assert.Equal(t, "two\n", comm.NewestEntries(log, 4), "with no whole entry in reach, the last bytes")
}
