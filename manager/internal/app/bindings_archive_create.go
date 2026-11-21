package app

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	internalarchive "wails-app/internal/archive"
	"wails-app/internal/contracts"
)

// CreateZipArchive creates a ZIP archive from public file-manager paths.
// Selected roots are stored by basename so absolute host paths never leak.
func (a *App) CreateZipArchive(paths []string, destination string) (*contracts.OperationResult, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("create zip: no source paths")
	}
	if strings.TrimSpace(destination) == "" {
		return nil, fmt.Errorf("create zip: empty destination")
	}

	resolvedDestination, err := a.State.Filesystem.RealPath(destination)
	if err != nil {
		return nil, fmt.Errorf("create zip: resolve destination: %w", err)
	}
	if _, err := os.Lstat(resolvedDestination); err == nil {
		return nil, fmt.Errorf("create zip: destination already exists: %w", os.ErrExist)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("create zip: inspect destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(resolvedDestination), 0755); err != nil {
		return nil, fmt.Errorf("create zip: prepare destination: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(resolvedDestination), ".file-manager-archive-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create zip: temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	writer := zip.NewWriter(tempFile)
	seen := make(map[string]struct{})
	destinationAbs, _ := filepath.Abs(resolvedDestination)

	for _, publicSource := range paths {
		resolvedSource, err := a.State.Filesystem.RealPath(publicSource)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create zip: resolve %q: %w", publicSource, err)
		}

		sourceAbs, err := filepath.Abs(resolvedSource)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("create zip: source path %q: %w", publicSource, err)
		}
		if sameFilesystemPath(sourceAbs, destinationAbs) {
			_ = writer.Close()
			return nil, fmt.Errorf("create zip: destination cannot also be a source")
		}

		rootName := filepath.Base(filepath.Clean(sourceAbs))
		if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
			rootName = "root"
		}

		if err := addZipTree(writer, sourceAbs, rootName, destinationAbs, seen); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("create zip: finalize: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("create zip: close temp file: %w", err)
	}

	if err := internalarchive.PublishNewArchive(tempPath, resolvedDestination); err != nil {
		return nil, fmt.Errorf("create zip: commit: %w", err)
	}

	return &contracts.OperationResult{
		Success:       true,
		Message:       "archive created",
		AffectedPaths: []string{destination},
	}, nil
}

func addZipTree(writer *zip.Writer, sourcePath, archiveRoot, destinationAbs string, seen map[string]struct{}) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("create zip: stat %q: %w", sourcePath, err)
	}
	if !info.IsDir() {
		return addZipFile(writer, sourcePath, archiveRoot, info, seen)
	}

	return filepath.Walk(sourcePath, func(path string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		absolute, err := filepath.Abs(path)
		if err == nil && sameFilesystemPath(absolute, destinationAbs) {
			if walkInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		name := archiveRoot
		if rel != "." {
			name = filepath.Join(archiveRoot, rel)
		}
		name = filepath.ToSlash(name)

		if walkInfo.IsDir() {
			if !strings.HasSuffix(name, "/") {
				name += "/"
			}
			return addZipDirectory(writer, name, walkInfo, seen)
		}
		return addZipFile(writer, path, name, walkInfo, seen)
	})
}

func addZipDirectory(writer *zip.Writer, name string, info os.FileInfo, seen map[string]struct{}) error {
	if _, exists := seen[name]; exists {
		return nil
	}
	seen[name] = struct{}{}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Store
	_, err = writer.CreateHeader(header)
	return err
}

func addZipFile(writer *zip.Writer, sourcePath, name string, info os.FileInfo, seen map[string]struct{}) error {
	name = filepath.ToSlash(strings.TrimPrefix(name, "/"))
	if name == "" {
		return nil
	}
	if _, exists := seen[name]; exists {
		return fmt.Errorf("create zip: duplicate archive path %q", name)
	}
	seen[name] = struct{}{}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(sourcePath)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Store
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.WriteString(entry, target)
		return err
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func sameFilesystemPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	return left == right || strings.EqualFold(left, right)
}

// ArchiveImportEntries transactionally adds filesystem paths to a top-level ZIP.
// Existing entries with the same archive paths are replaced; unrelated entries
// are copied through without recompression.
func (a *App) ArchiveImportEntries(req contracts.ArchiveImportRequest) (*contracts.ArchiveEntriesResult, error) {
	if len(req.Sources) == 0 {
		return &contracts.ArchiveEntriesResult{}, nil
	}
	if len(req.NestedChain) != 0 {
		return nil, fmt.Errorf("archive import: nested archives are read-only")
	}

	resolvedArchive, err := a.State.Filesystem.RealPath(req.FsPath)
	if err != nil {
		return nil, fmt.Errorf("archive import: resolve archive: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(resolvedArchive), ".zip") {
		return nil, fmt.Errorf("archive import: only ZIP archives are writable")
	}

	archiveAbs, err := filepath.Abs(resolvedArchive)
	if err != nil {
		return nil, fmt.Errorf("archive import: archive path: %w", err)
	}

	type importRoot struct {
		source string
		root   string
	}
	roots := make([]importRoot, 0, len(req.Sources))
	newNames := make(map[string]struct{})
	destinationPrefix := normalizeZipImportPath(req.DestinationPath)

	for _, publicSource := range req.Sources {
		resolvedSource, err := a.State.Filesystem.RealPath(publicSource)
		if err != nil {
			return nil, fmt.Errorf("archive import: resolve %q: %w", publicSource, err)
		}
		sourceAbs, err := filepath.Abs(resolvedSource)
		if err != nil {
			return nil, fmt.Errorf("archive import: source path %q: %w", publicSource, err)
		}
		if sameFilesystemPath(sourceAbs, archiveAbs) {
			return nil, fmt.Errorf("archive import: archive cannot also be a source")
		}
		rootName := filepath.Base(filepath.Clean(sourceAbs))
		if rootName == "." || rootName == string(filepath.Separator) || rootName == "" {
			rootName = "root"
		}
		archiveRoot := joinZipImportPath(destinationPrefix, rootName)
		if err := collectZipTreeNames(sourceAbs, archiveRoot, newNames); err != nil {
			return nil, err
		}
		roots = append(roots, importRoot{source: sourceAbs, root: archiveRoot})
	}

	reader, err := zip.OpenReader(resolvedArchive)
	if err != nil {
		return nil, fmt.Errorf("archive import: open ZIP: %w", err)
	}
	defer reader.Close()

	tempFile, err := os.CreateTemp(filepath.Dir(resolvedArchive), ".archive-import-*.zip")
	if err != nil {
		return nil, fmt.Errorf("archive import: temp file: %w", err)
	}
	tmpPath := tempFile.Name()
	committed := false
	defer func() {
		_ = tempFile.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	writer := zip.NewWriter(tempFile)
	seen := make(map[string]struct{})
	for _, existing := range reader.File {
		name := normalizeZipImportPath(existing.Name)
		if name == "" {
			continue
		}
		lookup := name
		if existing.FileInfo().IsDir() && !strings.HasSuffix(lookup, "/") {
			lookup += "/"
		}
		if _, replaced := newNames[lookup]; replaced {
			continue
		}
		if _, duplicate := seen[lookup]; duplicate {
			continue
		}
		if err := writer.Copy(existing); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("archive import: preserve %q: %w", existing.Name, err)
		}
		seen[lookup] = struct{}{}
	}

	for _, root := range roots {
		if err := addZipTree(writer, root.source, root.root, archiveAbs, seen); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("archive import: add %q: %w", root.source, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("archive import: finalize ZIP: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("archive import: close temp ZIP: %w", err)
	}

	backupFile, err := os.CreateTemp(filepath.Dir(resolvedArchive), ".archive-import-backup-*.zip")
	if err != nil {
		return nil, fmt.Errorf("archive import: reserve backup: %w", err)
	}
	backup := backupFile.Name()
	_ = backupFile.Close()
	_ = os.Remove(backup)

	if err := os.Rename(resolvedArchive, backup); err != nil {
		return nil, fmt.Errorf("archive import: stage original: %w", err)
	}
	if err := os.Rename(tmpPath, resolvedArchive); err != nil {
		_ = os.Rename(backup, resolvedArchive)
		return nil, fmt.Errorf("archive import: commit: %w", err)
	}
	committed = true
	_ = os.Remove(backup)

	return &contracts.ArchiveEntriesResult{
		Total:     len(req.Sources),
		Succeeded: len(req.Sources),
	}, nil
}

func normalizeZipImportPath(value string) string {
	value = strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
	parts := make([]string, 0)
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func joinZipImportPath(base, leaf string) string {
	base = normalizeZipImportPath(base)
	leaf = normalizeZipImportPath(leaf)
	if base == "" {
		return leaf
	}
	if leaf == "" {
		return base
	}
	return base + "/" + leaf
}

func collectZipTreeNames(sourcePath, archiveRoot string, names map[string]struct{}) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("archive import: stat %q: %w", sourcePath, err)
	}
	if !info.IsDir() {
		names[normalizeZipImportPath(archiveRoot)] = struct{}{}
		return nil
	}

	return filepath.Walk(sourcePath, func(path string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}
		name := archiveRoot
		if rel != "." {
			name = filepath.ToSlash(filepath.Join(archiveRoot, rel))
		}
		name = normalizeZipImportPath(name)
		if walkInfo.IsDir() {
			name += "/"
		}
		names[name] = struct{}{}
		return nil
	})
}
