//go:build !windows

package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func OpenFileWithChooser(path string) error {
	return fmt.Errorf("open with chooser is not provided by this desktop; select an application explicitly")
}

func OpenFileWithApplication(path, application string) error {
	application = strings.TrimSpace(application)
	if application == "" {
		return fmt.Errorf("open with: application is required")
	}

	if runtime.GOOS == "darwin" {
		return startDetached(exec.Command("open", "-a", application, path))
	}

	// On Linux this intentionally accepts an executable name or absolute path.
	// We do not route through a shell, so user input cannot inject shell syntax.
	return startDetached(exec.Command(application, path))
}