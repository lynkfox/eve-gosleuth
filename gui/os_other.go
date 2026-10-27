//go:build !windows

package gui

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// osVersion names the operating system, for debug bundles. The app is built for Windows, this is for development elsewhere.
func osVersion() string {
	return runtime.GOOS
}

// openFolderSelecting opens a file's folder in the file manager, which can not select the file here
func openFolderSelecting(path string) error {
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
