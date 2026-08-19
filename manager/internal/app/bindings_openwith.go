package app

import (
	"fmt"

	"wails-app/internal/platform"
)

func (a *App) OpenFileWithChooser(path string) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("open with: resolve path: %w", err)
	}
	return platform.OpenFileWithChooser(resolved)
}

func (a *App) OpenFileWithApplication(path, application string) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("open with: resolve path: %w", err)
	}
	return platform.OpenFileWithApplication(resolved, application)
}