//go:build !windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func RevealInFileManager(path string) error {
	if runtime.GOOS == "darwin" {
		return startDetached(exec.Command("open", "-R", path))
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	target := path
	if !info.IsDir() {
		target = filepath.Dir(path)
	}

	if _, err := exec.LookPath("xdg-open"); err != nil {
		return fmt.Errorf("reveal: xdg-open is unavailable: %w", err)
	}
	return startDetached(exec.Command("xdg-open", target))
}
