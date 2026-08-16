package app

import (
	"fmt"
	"os"
	"time"

	"wails-app/internal/platform"
)

func (a *App) RevealInFileManager(path string) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("reveal: resolve path: %w", err)
	}
	return platform.RevealInFileManager(resolved)
}

// SetModifiedTime updates only the modification time and preserves the current
// access time when the platform exposes it through os.FileInfo.
func (a *App) SetModifiedTime(path string, unixMilliseconds int64) error {
	resolved, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return fmt.Errorf("timestamp: resolve path: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}

	modified := time.UnixMilli(unixMilliseconds)
	accessed := info.ModTime()
	return os.Chtimes(resolved, accessed, modified)
}
