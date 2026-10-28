package gui

import (
	"encoding/json"
)

/*
	ProjectVersion reads the app's version from wails.json, where Wails also takes it for the exe's file properties, so it is bumped in one place

Parameters:
  - wailsJSON: the contents of wails.json, embedded by main

Returns:
  - its info.productVersion, or "" if the file has none or can't be read
*/
func ProjectVersion(wailsJSON []byte) string {
	var project struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsJSON, &project); err != nil {
		return ""
	}
	return project.Info.ProductVersion
}

// Version returns the app's version from wails.json, for the top bar and the About window, "" if it has none
func (a *App) Version() string {
	return a.version
}
