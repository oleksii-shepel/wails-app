//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
)

func OpenFileWithChooser(path string) error {
	ret, callErr, err := shellExecute("openas", path)
	if err != nil {
		return err
	}
	if ret > 32 {
		return nil
	}
	if callErr != nil {
		return fmt.Errorf("Open With failed: %w", callErr)
	}
	return fmt.Errorf("Open With failed with code %d", ret)
}

func OpenFileWithApplication(path, application string) error {
	application = strings.TrimSpace(application)
	if application == "" {
		return fmt.Errorf("open with: application is required")
	}

	cmd := exec.Command(application, path)
	return cmd.Start()
}