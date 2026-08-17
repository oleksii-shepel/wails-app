package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"wails-app/internal/pathref"
)

// SplitFile splits a local file into sequential .partNNN files next to it.
// The operation writes each part through a temporary file before rename so a
// failed split never exposes a partially written part as complete.
func (a *App) SplitFile(path string, partSize int64) ([]string, error) {
	if partSize <= 0 {
		return nil, fmt.Errorf("split file: part size must be positive")
	}

	source, err := a.State.Filesystem.RealPath(pathref.Normalize(path))
	if err != nil {
		return nil, fmt.Errorf("split file: resolve source: %w", err)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("split file: stat source: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("split file: source is a directory")
	}

	input, err := os.Open(source)
	if err != nil {
		return nil, fmt.Errorf("split file: open source: %w", err)
	}
	defer input.Close()

	var created []string
	cleanup := func() {
		for _, public := range created {
			if native, resolveErr := a.State.Filesystem.RealPath(public); resolveErr == nil {
				_ = os.Remove(native)
			}
		}
	}

	for index := 1; ; index++ {
		partPath := fmt.Sprintf("%s.part%03d", source, index)
		temp, err := os.CreateTemp(filepath.Dir(source), ".reassembly-part-*")
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("split file: create part %d: %w", index, err)
		}
		tempPath := temp.Name()

		written, copyErr := io.CopyN(temp, input, partSize)
		closeErr := temp.Close()

		if copyErr != nil && copyErr != io.EOF {
			_ = os.Remove(tempPath)
			cleanup()
			return nil, fmt.Errorf("split file: write part %d: %w", index, copyErr)
		}
		if closeErr != nil {
			_ = os.Remove(tempPath)
			cleanup()
			return nil, fmt.Errorf("split file: close part %d: %w", index, closeErr)
		}
		if written == 0 {
			_ = os.Remove(tempPath)
			break
		}

		_ = os.Remove(partPath)
		if err := os.Rename(tempPath, partPath); err != nil {
			_ = os.Remove(tempPath)
			cleanup()
			return nil, fmt.Errorf("split file: commit part %d: %w", index, err)
		}

		created = append(created, pathref.FromNativePath(partPath))
		if copyErr == io.EOF {
			break
		}
	}

	return created, nil
}

// JoinFiles joins ordered local part files into destination. Destination is
// committed atomically from a temporary file in the same directory.
func (a *App) JoinFiles(parts []string, destination string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("join files: no parts")
	}

	dest, err := a.State.Filesystem.RealPath(pathref.Normalize(destination))
	if err != nil {
		return "", fmt.Errorf("join files: resolve destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("join files: prepare destination: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(dest), ".reassembly-join-*")
	if err != nil {
		return "", fmt.Errorf("join files: create temporary destination: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	for _, publicPart := range parts {
		part, err := a.State.Filesystem.RealPath(pathref.Normalize(publicPart))
		if err != nil {
			_ = temp.Close()
			return "", fmt.Errorf("join files: resolve part: %w", err)
		}

		input, err := os.Open(part)
		if err != nil {
			_ = temp.Close()
			return "", fmt.Errorf("join files: open %q: %w", publicPart, err)
		}

		_, copyErr := io.Copy(temp, input)
		closeErr := input.Close()
		if copyErr != nil {
			_ = temp.Close()
			return "", fmt.Errorf("join files: copy %q: %w", publicPart, copyErr)
		}
		if closeErr != nil {
			_ = temp.Close()
			return "", fmt.Errorf("join files: close %q: %w", publicPart, closeErr)
		}
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("join files: sync destination: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("join files: close destination: %w", err)
	}

	_ = os.Remove(dest)
	if err := os.Rename(tempPath, dest); err != nil {
		return "", fmt.Errorf("join files: commit destination: %w", err)
	}

	return pathref.FromNativePath(dest), nil
}
