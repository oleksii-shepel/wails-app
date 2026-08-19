package app

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	_ = os.Remove(resolvedDestination)
	if err := os.Rename(tempPath, resolvedDestination); err != nil {
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