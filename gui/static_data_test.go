package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportStaticData(t *testing.T) {
	t.Run("writes each file and the readme, and removes json a previous export left", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), staticExportFolderName)
		assert.NoError(t, os.MkdirAll(dir, 0o755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "old.json"), []byte("{}"), 0o644))

		assert.NoError(t, exportStaticData(dir, map[string][]byte{"ships.json": []byte(`{"capital_types": {}}`)}))

		ships, err := os.ReadFile(filepath.Join(dir, "ships.json"))
		assert.NoError(t, err)
		assert.Equal(t, `{"capital_types": {}}`, string(ships))
		assert.FileExists(t, filepath.Join(dir, staticExportReadmeName))
		assert.NoFileExists(t, filepath.Join(dir, "old.json"))
	})

	t.Run("a folder name with glob characters is used as written", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "bob[1]", staticExportFolderName)
		assert.NoError(t, os.MkdirAll(dir, 0o755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "old.json"), []byte("{}"), 0o644))

		assert.NoError(t, exportStaticData(dir, map[string][]byte{"items.json": []byte(`{}`)}))
		assert.FileExists(t, filepath.Join(dir, "items.json"))
		assert.NoFileExists(t, filepath.Join(dir, "old.json"))
	})

	t.Run("files that are not json are kept", func(t *testing.T) {
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644))

		assert.NoError(t, exportStaticData(dir, map[string][]byte{"items.json": []byte(`{}`)}))
		assert.FileExists(t, filepath.Join(dir, "notes.txt"))
	})

	t.Run("makes the folder when there is none", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", staticExportFolderName)

		assert.NoError(t, exportStaticData(dir, map[string][]byte{"items.json": []byte(`{}`)}))
		assert.FileExists(t, filepath.Join(dir, "items.json"))
	})
}
