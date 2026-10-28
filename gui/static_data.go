package gui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lynkfox/go_eve_sleuth/comm"
	staticdata "github.com/lynkfox/go_eve_sleuth/static_data"
)

const (
	staticExportFolderName = "static_data_export" // beside the settings, where Export game data writes the static data files
	staticExportReadmeName = "README.txt"
)

// staticExportReadme is written beside the exported files, as editing them changes nothing
var staticExportReadme = `These are the game data files GoSleuth is using: its built in data, with any changes from the ` + staticdata.OverrideFolderName + ` folder beside eve-gosleuth.exe merged in.

Editing the files here changes nothing. To change GoSleuth's data, make a ` + staticdata.OverrideFolderName + ` folder beside eve-gosleuth.exe and put a file of the same name in it, holding only your changes. Copy what you need from here.

How to write those files: https://github.com/lynkfox/eve-gosleuth/blob/main/docs/static-data-overrides.md

Each export replaces the files here. Export again after changing your files or updating GoSleuth.
`

/*
	ExportStaticData writes the static data files in use, the built in ones merged with any edited copies beside the exe, to a folder beside the settings,
	and opens it in the file manager. It does not wait for a running search.

Returns:
  - string: the folder written to
  - error: the files could not be written. Not opening the folder is only logged.
*/
func (a *App) ExportStaticData() (dir string, err error) {
	defer recoverAsError(&err)
	dir = filepath.Join(filepath.Dir(a.settingsPath), staticExportFolderName)
	if err := exportStaticData(dir, comm.StaticDataFiles()); err != nil {
		comm.Logger.WithError(err).Warn("Could not export the static data")
		return "", fmt.Errorf("Could not export the game data to %s", dir)
	}
	if err := openFolderSelecting(filepath.Join(dir, staticExportReadmeName)); err != nil {
		comm.Logger.WithError(err).Warn("Could not open the game data export folder")
	}
	return dir, nil
}

// exportStaticData writes the static data files and the readme to a folder, removing any json file a previous export left there
func exportStaticData(dir string, files map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	previous, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range previous {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(dir, staticExportReadmeName), []byte(staticExportReadme), 0o644)
}
