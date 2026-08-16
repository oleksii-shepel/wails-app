package app

import (
	"fmt"

	"wails-app/internal/platform"
)

func (a *App) CreateShortcut(target, shortcutPath string) error {
	resolvedTarget, err := a.State.Filesystem.RealPath(target)
	if err != nil {
		return fmt.Errorf("shortcut: resolve target: %w", err)
	}
	resolvedShortcut, err := a.State.Filesystem.RealPath(shortcutPath)
	if err != nil {
		return fmt.Errorf("shortcut: resolve destination: %w", err)
	}
	return platform.CreateShortcut(resolvedTarget, resolvedShortcut)
}
