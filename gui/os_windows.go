//go:build windows

package gui

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// osVersion names the Windows version, ie Windows 10.0 build 26100, for debug bundles
func osVersion() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d build %d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}

// openFolderSelecting opens Explorer at a file's folder with the file selected. Explorer wants the path quoted after /select, which exec would quote as a whole, so the command line is given as is.
func openFolderSelecting(path string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`explorer.exe /select,"%s"`, path)}
	return cmd.Start()
}
