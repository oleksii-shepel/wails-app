//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func MoveToTrash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("trash: cannot resolve path %q: %w", path, err)
	}
	if _, err := os.Lstat(abs); err != nil {
		return fmt.Errorf("trash: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return moveToTrashDarwin(abs)
	default:
		return moveToTrashFreedesktop(abs)
	}
}

func moveToTrashDarwin(abs string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: cannot determine home directory: %w", err)
	}
	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		return fmt.Errorf("trash: cannot create ~/.Trash: %w", err)
	}

	dst := uniqueTrashPath(trashDir, filepath.Base(abs))
	if err := renameCrossDevice(abs, dst); err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	return nil
}

func moveToTrashFreedesktop(abs string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: cannot determine home directory: %w", err)
	}

	trashFilesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	trashInfoDir := filepath.Join(home, ".local", "share", "Trash", "info")

	for _, dir := range []string{trashFilesDir, trashInfoDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("trash: cannot create trash directory %s: %w", dir, err)
		}
	}

	baseName := uniqueTrashBaseName(trashFilesDir, trashInfoDir, filepath.Base(abs))
	dst := filepath.Join(trashFilesDir, baseName)
	infoPath := filepath.Join(trashInfoDir, baseName+".trashinfo")

	infoContent := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		abs,
		time.Now().Format("2006-01-02T15:04:05"),
	)
	if err := os.WriteFile(infoPath, []byte(infoContent), 0600); err != nil {
		return fmt.Errorf("trash: cannot write .trashinfo: %w", err)
	}

	if err := renameCrossDevice(abs, dst); err != nil {
		_ = os.Remove(infoPath)
		return fmt.Errorf("trash: %w", err)
	}
	return nil
}

func renameCrossDevice(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	linkErr := &os.LinkError{}
	if errors.As(err, &linkErr) && errors.Is(linkErr.Err, syscall.EXDEV) {
		return crossDeviceMoveToTrash(src, dst)
	}
	return err
}

func crossDeviceMoveToTrash(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := copyDirAll(src, dst); err != nil {
			return fmt.Errorf("cross-device copy failed: %w", err)
		}
	} else if err := copyFileData(src, dst, info.Mode()); err != nil {
		return fmt.Errorf("cross-device copy failed: %w", err)
	}
	return os.RemoveAll(src)
}

func copyDirAll(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFileData(path, target, info.Mode())
	})
}

func copyFileData(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

func uniqueTrashPath(dir, baseName string) string {
	candidate := filepath.Join(dir, baseName)
	if _, err := os.Lstat(candidate); os.IsNotExist(err) {
		return candidate
	}
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	for i := 2; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func uniqueTrashBaseName(filesDir, infoDir, baseName string) string {
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)

	candidate := baseName
	for i := 2; ; i++ {
		_, errFiles := os.Lstat(filepath.Join(filesDir, candidate))
		_, errInfo := os.Lstat(filepath.Join(infoDir, candidate+".trashinfo"))
		if os.IsNotExist(errFiles) && os.IsNotExist(errInfo) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
}
