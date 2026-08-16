//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CreateShortcut(target, shortcutPath string) error {
	target = strings.TrimSpace(target)
	shortcutPath = strings.TrimSpace(shortcutPath)
	if target == "" || shortcutPath == "" {
		return fmt.Errorf("shortcut: target and destination are required")
	}
	if !strings.EqualFold(filepath.Ext(shortcutPath), ".lnk") {
		shortcutPath += ".lnk"
	}
	if _, err := os.Lstat(shortcutPath); err == nil {
		return fmt.Errorf("shortcut: destination already exists: %s", shortcutPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0755); err != nil {
		return err
	}

	const script = `
$ErrorActionPreference = 'Stop'
$target = $env:FM_SHORTCUT_TARGET
$path = $env:FM_SHORTCUT_PATH
$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($path)
$link.TargetPath = $target
if (Test-Path -LiteralPath $target -PathType Leaf) {
  $link.WorkingDirectory = Split-Path -Parent $target
} else {
  $link.WorkingDirectory = $target
}
$link.Save()
`
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		script,
	)
	cmd.Env = append(
		os.Environ(),
		"FM_SHORTCUT_TARGET="+target,
		"FM_SHORTCUT_PATH="+shortcutPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"shortcut: powershell: %w: %s",
			err,
			strings.TrimSpace(string(output)),
		)
	}
	return nil
}
