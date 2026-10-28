// the App's version is unexported, so this tests from inside package gui
package gui

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	t.Run("wails.json holds a version of three numbers, which Windows also needs for the exe's file version", func(t *testing.T) {
		wailsJSON, err := os.ReadFile("../wails.json")
		assert.NoError(t, err)
		assert.Regexp(t, regexp.MustCompile(`^\d+\.\d+\.\d+$`), ProjectVersion(wailsJSON))
	})

	t.Run("a wails.json without a version, or that can't be read, gives none", func(t *testing.T) {
		assert.Equal(t, "", ProjectVersion([]byte(`{"name": "eve-gosleuth"}`)))
		assert.Equal(t, "", ProjectVersion([]byte(`{"info": `)))
	})

	t.Run("the App hands the frontend the version it was made with", func(t *testing.T) {
		assert.Equal(t, "0.9.0", NewApp("0.9.0").Version())
	})
}
