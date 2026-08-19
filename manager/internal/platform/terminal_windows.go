//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

func OpenTerminalAt(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("terminal: empty path")
	}

	// Prefer Windows Terminal when installed. It understands -d as the startup directory.
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		return exec.Command(wt, "-d", path).Start()
	}

	// cmd.exe opens its own console window from a Wails GUI process.
	return exec.Command("cmd.exe", "/K", "cd", "/d", path).Start()
}