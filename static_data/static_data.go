// Package staticdata embeds the static EVE data json files so they ship inside the binary
package staticdata

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed *.json
var Files embed.FS

// OverrideFolderName is the folder beside the app's exe whose json files are merged over the embedded ones of the same name, so game data can be updated without a rebuild
const OverrideFolderName = "static_data"

// OverrideDir returns the override folder beside the running exe, or an empty string if the exe can not be found
func OverrideDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), OverrideFolderName)
}

// Overrides returns the override folder as a filesystem for comm.InitStaticData to merge over Files. It may not exist, which leaves every file to Files.
func Overrides() []fs.FS {
	dir := OverrideDir()
	if dir == "" {
		return nil
	}
	return []fs.FS{os.DirFS(dir)}
}
