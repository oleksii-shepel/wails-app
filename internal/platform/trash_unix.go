//go:build !windows

package platform

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	contracts "wails-app/internal/contracts"
)

type darwinTrashMetadata struct {
	OriginalPath string `json:"originalPath"`
	TrashedPath  string `json:"trashedPath"`
	DeletedAt    string `json:"deletedAt"`
}

func MoveToTrash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("trash: cannot resolve path %q: %w", path, err)
	}
	if _, err := os.Lstat(abs); err != nil {
		return fmt.Errorf("trash: %w", err)
	}
	if runtime.GOOS == "darwin" {
		return moveToTrashDarwin(abs)
	}
	return moveToTrashFreedesktop(abs)
}

func ListTrashEntries() ([]contracts.TrashEntry, error) {
	if runtime.GOOS == "darwin" {
		return listTrashDarwin()
	}
	return listTrashFreedesktop()
}

func RestoreTrashEntry(id string) (string, error) {
	if runtime.GOOS == "darwin" {
		return restoreTrashDarwin(id)
	}
	return restoreTrashFreedesktop(id)
}

func DeleteTrashEntry(id string) error {
	if runtime.GOOS == "darwin" {
		return deleteTrashDarwin(id)
	}
	return deleteTrashFreedesktop(id)
}

func EmptyTrash() error {
	entries, err := ListTrashEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := DeleteTrashEntry(entry.ID); err != nil {
			return err
		}
	}
	return nil
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

	meta := darwinTrashMetadata{
		OriginalPath: abs,
		TrashedPath:  dst,
		DeletedAt:    time.Now().Format(time.RFC3339),
	}
	_ = writeDarwinTrashMetadata(filepath.Base(dst), meta)
	return nil
}

func moveToTrashFreedesktop(abs string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("trash: cannot determine home directory: %w", err)
	}

	filesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	infoDir := filepath.Join(home, ".local", "share", "Trash", "info")
	for _, dir := range []string{filesDir, infoDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("trash: cannot create trash directory %s: %w", dir, err)
		}
	}

	baseName := uniqueTrashBaseName(filesDir, infoDir, filepath.Base(abs))
	dst := filepath.Join(filesDir, baseName)
	infoPath := filepath.Join(infoDir, baseName+".trashinfo")
	infoContent := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		url.PathEscape(abs),
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

func listTrashFreedesktop() ([]contracts.TrashEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	filesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	infoDir := filepath.Join(home, ".local", "share", "Trash", "info")
	dirEntries, err := os.ReadDir(filesDir)
	if os.IsNotExist(err) {
		return []contracts.TrashEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]contracts.TrashEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		name := entry.Name()
		trashedPath := filepath.Join(filesDir, name)
		info, statErr := os.Lstat(trashedPath)
		originalPath, deletedAt := readTrashInfo(filepath.Join(infoDir, name+".trashinfo"))
		item := contracts.TrashEntry{
			ID: name, Name: name, OriginalPath: originalPath,
			TrashedPath: trashedPath, DeletedAt: deletedAt,
			CanRestore: originalPath != "",
		}
		if statErr == nil {
			item.Size = info.Size()
			item.IsDir = info.IsDir()
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DeletedAt > result[j].DeletedAt })
	return result, nil
}

func restoreTrashFreedesktop(id string) (string, error) {
	id = safeTrashID(id)
	if id == "" {
		return "", fmt.Errorf("trash restore: invalid id")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	filesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	infoDir := filepath.Join(home, ".local", "share", "Trash", "info")
	source := filepath.Join(filesDir, id)
	infoPath := filepath.Join(infoDir, id+".trashinfo")
	original, _ := readTrashInfo(infoPath)
	if original == "" {
		return "", fmt.Errorf("trash restore: original path is unavailable")
	}
	if _, err := os.Lstat(original); err == nil {
		return "", fmt.Errorf("trash restore: destination already exists: %s", original)
	}
	if err := os.MkdirAll(filepath.Dir(original), 0755); err != nil {
		return "", err
	}
	if err := renameCrossDevice(source, original); err != nil {
		return "", err
	}
	_ = os.Remove(infoPath)
	return original, nil
}

func deleteTrashFreedesktop(id string) error {
	id = safeTrashID(id)
	if id == "" {
		return fmt.Errorf("trash delete: invalid id")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	filesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	infoDir := filepath.Join(home, ".local", "share", "Trash", "info")
	if err := os.RemoveAll(filepath.Join(filesDir, id)); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(infoDir, id+".trashinfo"))
	return nil
}

func readTrashInfo(path string) (originalPath, deletedAt string) {
	file, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "Path="):
			value := strings.TrimPrefix(line, "Path=")
			if decoded, err := url.PathUnescape(value); err == nil {
				originalPath = decoded
			} else {
				originalPath = value
			}
		case strings.HasPrefix(line, "DeletionDate="):
			deletedAt = strings.TrimPrefix(line, "DeletionDate=")
		}
	}
	return
}

func listTrashDarwin() ([]contracts.TrashEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	trashDir := filepath.Join(home, ".Trash")
	dirEntries, err := os.ReadDir(trashDir)
	if os.IsNotExist(err) {
		return []contracts.TrashEntry{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := make([]contracts.TrashEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		name := entry.Name()
		path := filepath.Join(trashDir, name)
		info, _ := os.Lstat(path)
		meta, _ := readDarwinTrashMetadata(name)

		item := contracts.TrashEntry{
			ID: name, Name: name, TrashedPath: path,
			OriginalPath: meta.OriginalPath, DeletedAt: meta.DeletedAt,
			CanRestore: meta.OriginalPath != "",
		}
		if info != nil {
			item.Size = info.Size()
			item.IsDir = info.IsDir()
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DeletedAt > result[j].DeletedAt })
	return result, nil
}

func restoreTrashDarwin(id string) (string, error) {
	id = safeTrashID(id)
	if id == "" {
		return "", fmt.Errorf("trash restore: invalid id")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	meta, err := readDarwinTrashMetadata(id)
	if err != nil || meta.OriginalPath == "" {
		return "", fmt.Errorf("trash restore: original path is unavailable")
	}

	source := filepath.Join(home, ".Trash", id)
	if _, err := os.Lstat(meta.OriginalPath); err == nil {
		return "", fmt.Errorf("trash restore: destination already exists: %s", meta.OriginalPath)
	}
	if err := os.MkdirAll(filepath.Dir(meta.OriginalPath), 0755); err != nil {
		return "", err
	}
	if err := renameCrossDevice(source, meta.OriginalPath); err != nil {
		return "", err
	}
	_ = os.Remove(darwinTrashMetadataPath(id))
	return meta.OriginalPath, nil
}

func deleteTrashDarwin(id string) error {
	id = safeTrashID(id)
	if id == "" {
		return fmt.Errorf("trash delete: invalid id")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(home, ".Trash", id)); err != nil {
		return err
	}
	_ = os.Remove(darwinTrashMetadataPath(id))
	return nil
}

func darwinTrashIndexDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "FileManager", "trash-index")
}

func darwinTrashMetadataPath(id string) string {
	return filepath.Join(darwinTrashIndexDir(), safeTrashID(id)+".json")
}

func writeDarwinTrashMetadata(id string, meta darwinTrashMetadata) error {
	dir := darwinTrashIndexDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(darwinTrashMetadataPath(id), data, 0600)
}

func readDarwinTrashMetadata(id string) (darwinTrashMetadata, error) {
	var meta darwinTrashMetadata
	data, err := os.ReadFile(darwinTrashMetadataPath(id))
	if err != nil {
		return meta, err
	}
	err = json.Unmarshal(data, &meta)
	return meta, err
}

func safeTrashID(id string) string {
	id = strings.TrimSpace(filepath.Base(id))
	if id == "." || id == string(filepath.Separator) {
		return ""
	}
	return id
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
