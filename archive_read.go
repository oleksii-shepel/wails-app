package main

import (
	"encoding/base64"
	"fmt"
)

// ArchiveReadFile extracts a single entry from an archive into memory.
func (a *App) archiveReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	entryPath = normalize(entryPath)

	var data []byte
	var err error

	if len(nestedChain) == 0 {
		data, err = a.extractFromFile(fsPath, entryPath)
	} else {
		archiveData, nestedErr := a.extractNestedArchive(fsPath, nestedChain)
		if nestedErr != nil {
			return nil, nestedErr
		}
		data, err = a.extractFromBytes(archiveData, entryPath, nestedChain[len(nestedChain)-1])
	}
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
