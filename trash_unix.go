//go:build !windows

package main

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

// moveToTrash moves path to the platform trash / recycle location without
// permanently deleting it.
//
//   - macOS  : moves to ~/.Trash/
//   - Linux  : follows the FreeDesktop.org Trash specification
//     (https://specifications.freedesktop.org/trash-spec/trashspec-latest.html)
//     Files land in ~/.local/share/Trash/files/ and a corresponding
//     .trashinfo metadata file is written to
//     ~/.local/share/Trash/info/.
//
// Both implementations handle name collisions by appending a counter
// (e.g. "report (2).pdf").
func moveToTrash(path string) error {
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

// ---- macOS ------------------------------------------------------------------

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

// ---- Linux / FreeDesktop ----------------------------------------------------

func moveToTrashFreedesktop(abs string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: cannot determine home directory: %w", err)
	}

	trashFilesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	trashInfoDir := filepath.Join(home, ".local", "share", "Trash", "info")

	for _, d := range []string{trashFilesDir, trashInfoDir} {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("trash: cannot create trash directory %s: %w", d, err)
		}
	}

	// Pick a unique base name (without collision in either files/ or info/).
	baseName := uniqueTrashBaseName(trashFilesDir, trashInfoDir, filepath.Base(abs))
	dst := filepath.Join(trashFilesDir, baseName)
	infoPath := filepath.Join(trashInfoDir, baseName+".trashinfo")

	// Write .trashinfo first so the entry is always consistent.
	infoContent := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		abs,
		time.Now().Format("2006-01-02T15:04:05"),
	)
	if err := os.WriteFile(infoPath, []byte(infoContent), 0600); err != nil {
		return fmt.Errorf("trash: cannot write .trashinfo: %w", err)
	}

	if err := renameCrossDevice(abs, dst); err != nil {
		// Clean up the orphaned .trashinfo on failure.
		os.Remove(infoPath)
		return fmt.Errorf("trash: %w", err)
	}
	return nil
}

// ---- helpers ----------------------------------------------------------------

// renameCrossDevice tries os.Rename first; if it fails with EXDEV (cross-device
// link), it falls back to a recursive copy followed by removal of the source.
func renameCrossDevice(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Unwrap to check for EXDEV (cross-device link).
	linkErr := &os.LinkError{}
	if errors.As(err, &linkErr) {
		if errors.Is(linkErr.Err, syscall.EXDEV) {
			return crossDeviceMoveToTrash(src, dst)
		}
	}
	return err
}

// crossDeviceMoveToTrash copies src to dst across devices, then removes src.
func crossDeviceMoveToTrash(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := copyDirAll(src, dst); err != nil {
			return fmt.Errorf("cross-device copy failed: %w", err)
		}
	} else {
		if err := copyFileData(src, dst, info.Mode()); err != nil {
			return fmt.Errorf("cross-device copy failed: %w", err)
		}
	}
	// Remove the original after a successful copy.
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
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// uniqueTrashPath returns a path inside dir that does not yet exist, based on
// baseName. Collisions are resolved by appending " (N)" before the extension.
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

// uniqueTrashBaseName returns a base name that is free in both files/ and
// info/ directories to keep the two directories in sync.
func uniqueTrashBaseName(filesDir, infoDir, baseName string) string {
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)

	candidate := baseName
	for i := 2; ; i++ {
		_, errF := os.Lstat(filepath.Join(filesDir, candidate))
		_, errI := os.Lstat(filepath.Join(infoDir, candidate+".trashinfo"))
		if os.IsNotExist(errF) && os.IsNotExist(errI) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
	}
}
