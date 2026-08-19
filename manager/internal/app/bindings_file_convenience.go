package app

import (
	"fmt"
	"os"
	"path/filepath"

	"wails-app/internal/platform"
)

func (a *App) CreateSymbolicLink(target, linkPath string) error {
	resolvedTarget, err := a.State.Filesystem.RealPath(target)
	if err != nil {
		return fmt.Errorf("symlink: resolve target: %w", err)
	}
	resolvedLink, err := a.State.Filesystem.RealPath(linkPath)
	if err != nil {
		return fmt.Errorf("symlink: resolve link path: %w", err)
	}

	if _, err := os.Lstat(resolvedLink); err == nil {
		return fmt.Errorf("symlink: destination already exists: %s", linkPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedLink), 0755); err != nil {
		return err
	}

	if err := os.Symlink(resolvedTarget, resolvedLink); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}
	return nil
}

func (a *App) CreateHardLink(target, linkPath string) error {
	resolvedTarget, err := a.State.Filesystem.RealPath(target)
	if err != nil {
		return fmt.Errorf("hard link: resolve target: %w", err)
	}
	resolvedLink, err := a.State.Filesystem.RealPath(linkPath)
	if err != nil {
		return fmt.Errorf("hard link: resolve link path: %w", err)
	}

	info, err := os.Stat(resolvedTarget)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("hard link: directories are not supported")
	}

	if _, err := os.Lstat(resolvedLink); err == nil {
		return fmt.Errorf("hard link: destination already exists: %s", linkPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedLink), 0755); err != nil {
		return err
	}
	if err := os.Link(resolvedTarget, resolvedLink); err != nil {
		return fmt.Errorf("hard link: %w", err)
	}
	return nil
}

func (a *App) OpenTerminalAt(path string) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("terminal: resolve path: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		resolved = filepath.Dir(resolved)
	}

	return platform.OpenTerminalAt(resolved)
}