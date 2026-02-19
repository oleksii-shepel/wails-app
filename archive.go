package main

// archive.go — Production-ready ArchiveList & ArchiveListNested for Wails.
// Uses github.com/mholt/archiver/v3

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ─── Protocol types ────────────────────────────────────────────────

type ArchiveEntry struct {
	Name        string   `json:"name"`
	InnerPath   string   `json:"innerPath"`
	Type        string   `json:"type"` // "FILE" | "DIRECTORY" | "ARCHIVE"
	Size        int64    `json:"size"`
	Modified    int64    `json:"modified"` // unix milliseconds
	IsNested    bool     `json:"isNested"`
	NestedChain []string `json:"nestedChain,omitempty"` // for nested archives
}

type ArchiveListing struct {
	ArchivePath string         `json:"archivePath"`
	InnerPath   string         `json:"innerPath"`
	NestedChain []string       `json:"nestedChain,omitempty"`
	Entries     []ArchiveEntry `json:"entries"`
}

// ─── Public App methods ────────────────────────────────────────────

// ArchiveList lists a single level of a filesystem archive.
func (a *App) ArchiveList(fsPath string, innerPath string) (ArchiveListing, error) {
	entries, err := listArchiveEntries(fsPath, innerPath, nil)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		Entries:     entries,
	}, nil
}

// ArchiveListNested lists a single level of a nested archive.
func (a *App) ArchiveListNested(fsPath string, nestedEntryPath, innerPath string) (ArchiveListing, error) {
	// Extract nested archive into temp file
	data, err := extractEntryBytes(fsPath, nestedEntryPath)
	if err != nil {
		return ArchiveListing{}, err
	}

	ext := detectExt(nestedEntryPath)
	tmpFile, err := os.CreateTemp("", "nested-archive-*"+ext)
	if err != nil {
		return ArchiveListing{}, err
	}
	// We don't defer remove here if we want to browse it,
	// but for a stateless "List" call, it's safer to cleanup or use a cache.
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return ArchiveListing{}, err
	}
	tmpFile.Close()

	entries, err := listArchiveEntries(tmpFile.Name(), innerPath, []string{nestedEntryPath})
	if err != nil {
		return ArchiveListing{}, err
	}

	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: []string{nestedEntryPath},
		Entries:     entries,
	}, nil
}

// ─── Internal helpers ─────────────────────────────────────────────

func listArchiveEntries(fsPath, innerPath string, nestedChain []string) ([]ArchiveEntry, error) {
	innerPath = normalize(innerPath)
	seen := map[string]ArchiveEntry{}

	err := archiver.Walk(fsPath, func(f archiver.File) error {
		ep := normalize(archiveEntryPath(f))
		if ep == "" {
			return nil
		}

		rel := getRelativePath(ep, innerPath)
		if rel == "" {
			return nil
		}

		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		isDir := f.IsDir() || len(parts) > 1
		isArchive := !isDir && isArchiveFile(childName)

		childInner := childName
		if innerPath != "" {
			childInner = innerPath + "/" + childName
		}

		if _, ok := seen[childName]; !ok {
			entry := ArchiveEntry{
				Name:      childName,
				InnerPath: childInner,
				Type:      "FILE",
				Size:      f.Size(),
				Modified:  f.ModTime().UnixMilli(),
				IsNested:  isArchive,
			}
			if isDir {
				entry.Type = "DIRECTORY"
			} else if isArchive {
				entry.Type = "ARCHIVE"
				entry.NestedChain = append([]string{}, nestedChain...)
				entry.NestedChain = append(entry.NestedChain, childName)
			}
			seen[childName] = entry
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	entries := make([]ArchiveEntry, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		order := map[string]int{"DIRECTORY": 0, "ARCHIVE": 1, "FILE": 2}
		if entries[i].Type != entries[j].Type {
			return order[entries[i].Type] < order[entries[j].Type]
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func extractEntryBytes(fsPath, entryPath string) ([]byte, error) {
	entryPath = normalize(entryPath)
	var buf []byte

	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if normalize(archiveEntryPath(f)) != entryPath {
			return nil
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		buf = data
		return archiver.ErrStopWalk
	})

	if err != nil && err != archiver.ErrStopWalk {
		return nil, err
	}
	if buf == nil {
		return nil, fmt.Errorf("entry %q not found in %s", entryPath, fsPath)
	}
	return buf, nil
}
