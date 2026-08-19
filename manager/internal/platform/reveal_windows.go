//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RevealInFileManager(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("reveal: empty path")
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return exec.Command("explorer.exe", path).Start()
	}

	// Explorer's /select syntax requires the comma as part of the same argument.
	return exec.Command(
		"explorer.exe",
		"/select,"+filepath.Clean(path),
	).Start()
}