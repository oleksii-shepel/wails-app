package main

// archive.go — ArchiveList and ArchiveListNested methods exposed via Wails.
//
// Dependency:  github.com/mholt/archiver/v3
//   go get github.com/mholt/archiver/v3

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ─── Protocol types ───────────────────────────────────────────────────────────

type ArchiveEntry struct {
	Name      string `json:"name"`
	InnerPath string `json:"innerPath"`
	Type      string `json:"type"` // "FILE" | "DIRECTORY"
	Size      int64  `json:"size"`
	Modified  int64  `json:"modified"` // unix milliseconds
}

type ArchiveListing struct {
	ArchivePath string         `json:"archivePath"`
	InnerPath   string         `json:"innerPath"`
	Entries     []ArchiveEntry `json:"entries"`
}

// ─── Public App methods ───────────────────────────────────────────────────────

// ArchiveList lists one directory level inside a root (filesystem) archive.
//
//	fsPath    – absolute path to the archive file, e.g. "C:/stuff/outer.zip"
//	innerPath – folder to list within the archive; "" = root
func (a *App) ArchiveList(fsPath string, innerPath string) (ArchiveListing, error) {
	entries, err := listArchiveEntries(fsPath, innerPath)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		Entries:     entries,
	}, nil
}

// ArchiveListNested lists one directory level inside a nested archive —
// an archive file that lives inside another archive.
//
//	fsPath          – outermost archive on disk
//	nestedEntryPath – path of the nested archive inside fsPath, e.g. "libs/inner.tar.gz"
//	innerPath       – folder to list within the nested archive; "" = root
func (a *App) ArchiveListNested(fsPath string, nestedEntryPath string, innerPath string) (ArchiveListing, error) {
	// Extract the nested archive bytes into a temp file
	// (archiver/v3 requires a seekable reader, so we can't use an in-memory buffer)
	data, err := extractEntryBytes(fsPath, nestedEntryPath)
	if err != nil {
		return ArchiveListing{}, err
	}

	ext := filepath.Ext(nestedEntryPath)
	// Handle compound extensions like .tar.gz
	if strings.HasSuffix(strings.ToLower(nestedEntryPath), ".tar.gz") {
		ext = ".tar.gz"
	}

	tmpFile, err := os.CreateTemp("", "nested-archive-*"+ext)
	if err != nil {
		return ArchiveListing{}, err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return ArchiveListing{}, err
	}
	tmpFile.Close()

	entries, err := listArchiveEntries(tmpName, innerPath)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath, // always report the outermost FS path
		InnerPath:   innerPath,
		Entries:     entries,
	}, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// listArchiveEntries walks an archive and returns only the immediate children
// of innerPath (non-recursive, like a single directory listing).
func listArchiveEntries(fsPath string, innerPath string) ([]ArchiveEntry, error) {
	innerPath = strings.Trim(strings.ReplaceAll(innerPath, "\\", "/"), "/")

	// Use a map to deduplicate: archives often store "dir/", "dir/file.txt" as
	// separate entries, but we only want to emit "dir" once as a DIRECTORY.
	seen := map[string]ArchiveEntry{}

	err := archiver.Walk(fsPath, func(f archiver.File) error {
		ep := archiveEntryPath(f)
		ep = strings.Trim(strings.ReplaceAll(ep, "\\", "/"), "/")
		if ep == "" {
			return nil
		}

		// Determine the path relative to innerPath
		var rel string
		if innerPath == "" {
			rel = ep
		} else {
			prefix := innerPath + "/"
			if !strings.HasPrefix(ep, prefix) {
				return nil
			}
			rel = ep[len(prefix):]
			if rel == "" {
				return nil
			}
		}

		// Only keep the first segment (immediate child)
		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		isDir := f.IsDir() || len(parts) > 1

		childInnerPath := childName
		if innerPath != "" {
			childInnerPath = innerPath + "/" + childName
		}

		if _, exists := seen[childName]; !exists {
			entryType := "FILE"
			if isDir {
				entryType = "DIRECTORY"
			}
			seen[childName] = ArchiveEntry{
				Name:      childName,
				InnerPath: childInnerPath,
				Type:      entryType,
				Size:      f.Size(),
				Modified:  f.ModTime().UnixMilli(),
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort: directories first, then alphabetically within each group
	entries := make([]ArchiveEntry, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "DIRECTORY"
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// extractEntryBytes extracts a single named entry from an archive into memory.
func extractEntryBytes(fsPath string, entryPath string) ([]byte, error) {
	entryPath = strings.Trim(strings.ReplaceAll(entryPath, "\\", "/"), "/")

	var buf []byte
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if archiveEntryPath(f) != entryPath {
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
		return nil, fmt.Errorf("entry %q not found in archive %s", entryPath, fsPath)
	}
	return buf, nil
}

// archiveEntryPath returns the full inner path from an archiver.File header.
// Only zip and tar expose typed headers with full paths in archiver/v3.
func archiveEntryPath(f archiver.File) string {
	switch hdr := f.Header.(type) {
	case *zip.FileHeader:
		return strings.TrimSuffix(hdr.Name, "/")
	case *tar.Header:
		return strings.TrimSuffix(hdr.Name, "/")
	}
	// For other formats (rar, 7z) archiver/v3 only exposes the basename via f.Name().
	// Full path support for those formats requires archiver/v4 or a dedicated library.
	return f.Name()
}
