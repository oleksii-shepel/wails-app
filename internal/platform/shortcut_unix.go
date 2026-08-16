//go:build !windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CreateShortcut(target, shortcutPath string) error {
	target = strings.TrimSpace(target)
	shortcutPath = strings.TrimSpace(shortcutPath)
	if target == "" || shortcutPath == "" {
		return fmt.Errorf("shortcut: target and destination are required")
	}
	if _, err := os.Lstat(shortcutPath); err == nil {
		return fmt.Errorf("shortcut: destination already exists: %s", shortcutPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(shortcutPath), 0755); err != nil {
		return err
	}
	return os.Symlink(target, shortcutPath)
}
