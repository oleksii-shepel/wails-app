package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
)

// readArchiveEntry reads a single archive entry through the shared VFS-backed
// archive cache so browsing and preview use the same handler implementation.
func (a *App) readArchiveEntry(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	entryPath = normalize(entryPath)

	ctx := context.Background()
	arch, err := a.archCache.Get(ctx, fsPath, nestedChain, "")
	if err != nil {
		return nil, fmt.Errorf("archive read: open %q: %w", fsPath, err)
	}

	f, err := arch.Open(ctx, entryPath)
	if err != nil {
		return nil, fmt.Errorf("archive read: open entry %q: %w", entryPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if encoding == "" {
		encoding = "utf8"
	}
	var content string
	if encoding == "base64" {
		content = base64.StdEncoding.EncodeToString(data)
	} else {
		content = string(data)
		encoding = "utf8"
	}

	return &FileContent{
		Path:     entryPath,
		Content:  content,
		Encoding: encoding,
		Size:     int64(len(data)),
	}, nil
}

func (a *App) extractNestedArchive(fsPath string, nestedChain []string) ([]byte, error) {
	if len(nestedChain) == 0 {
		return nil, fmt.Errorf("empty nested chain")
	}

	current, err := a.extractFromFile(fsPath, nestedChain[0])
	if err != nil {
		return nil, fmt.Errorf("extracting %q from %q: %w", nestedChain[0], fsPath, err)
	}

	for index := 1; index < len(nestedChain); index += 1 {
		next, err := a.extractFromBytes(current, nestedChain[index], nestedChain[index])
		if err != nil {
			return nil, fmt.Errorf("extracting %q (chain depth %d): %w", nestedChain[index], index, err)
		}
		current = next
	}
	return current, nil
}
