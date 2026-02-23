package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ────────────────────────────────────────────────────────────────────────────
// Public API
// ────────────────────────────────────────────────────────────────────────────

// ArchiveList lists one level of a top-level archive on disk.
func (a *App) ArchiveList(fsPath string, innerPath string) (ArchiveListing, error) {
	entries, err := a.walkFile(fsPath, innerPath, nil)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		Entries:     entries,
	}, nil
}

// ArchiveListNested lists one level inside a nested archive chain without
// writing any temporary files.  nestedChain is the ordered list of archive
// entry names that leads to the archive you want to browse, e.g.:
//
//	["inner.zip"]                  – one level deep
//	["outer.tar.gz", "inner.zip"]  – two levels deep
func (a *App) ArchiveListNested(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	archiveData, err := a.extractNestedArchive(fsPath, nestedChain)
	if err != nil {
		return ArchiveListing{}, err
	}
	entries, err := a.walkBytes(archiveData, innerPath, nestedChain)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: nestedChain,
		Entries:     entries,
	}, nil
}

// ArchiveReadFile returns the raw bytes (base64-encoded) of a single file
// inside an archive (optionally nested).  entryPath is the full inner path
// inside the innermost archive, e.g. "src/main.go".
func (a *App) ArchiveReadFile(fsPath string, nestedChain []string, entryPath string) (*FileContent, error) {
	entryPath = normalize(entryPath)

	var data []byte
	var err error

	if len(nestedChain) == 0 {
		// Read directly from the on-disk archive.
		data, err = a.extractFromFile(fsPath, entryPath)
	} else {
		// Resolve the innermost archive into memory, then extract the entry.
		archiveData, nerr := a.extractNestedArchive(fsPath, nestedChain)
		if nerr != nil {
			return nil, nerr
		}
		data, err = a.extractFromBytes(archiveData, entryPath, nestedChain[len(nestedChain)-1])
	}
	if err != nil {
		return nil, err
	}

	return &FileContent{
		Path:     entryPath,
		Content:  string(data),
		Encoding: "utf8",
		Size:     int64(len(data)),
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Walk helpers – on-disk archive
// ────────────────────────────────────────────────────────────────────────────

func (a *App) walkFile(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	err := archiver.Walk(fsPath, func(f archiver.File) error {
		ep := normalize(archiveEntryPath(f))
		return a.processEntryInto(ep, f.IsDir(), f.Size(), f.ModTime().UnixMilli(), innerPath, nestedChain, seen)
	})
	if err != nil {
		return nil, err
	}
	return a.sortFileInfo(seen), nil
}

// ────────────────────────────────────────────────────────────────────────────
// Walk helpers – in-memory archive bytes
// ────────────────────────────────────────────────────────────────────────────

func (a *App) walkBytes(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	if isZipMagic(data) {
		return a.walkZip(data, innerPath, nestedChain)
	}
	if len(nestedChain) > 0 && isRarArchiveName(nestedChain[len(nestedChain)-1]) {
		return a.walkArchiveReader(data, innerPath, nestedChain, nestedChain[len(nestedChain)-1])
	}
	return a.walkTar(data, innerPath, nestedChain)
}

func (a *App) walkZip(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for _, f := range r.File {
		ep := normalize(f.Name)
		isDir := f.FileInfo().IsDir()
		if err := a.processEntryInto(ep, isDir, int64(f.UncompressedSize64), f.Modified.UnixMilli(), innerPath, nestedChain, seen); err != nil {
			return nil, err
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkTar(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(h.Name)
		isDir := h.FileInfo().IsDir()
		if err := a.processEntryInto(ep, isDir, h.Size, h.ModTime.UnixMilli(), innerPath, nestedChain, seen); err != nil {
			return nil, err
		}
	}
	return a.sortFileInfo(seen), nil
}

// ────────────────────────────────────────────────────────────────────────────
// Core entry processing (shared between ZIP / TAR / on-disk)
// ────────────────────────────────────────────────────────────────────────────

// processEntryInto resolves a single archive entry into a FileInfo for the
// requested innerPath level and writes it into seen (deduplicating dirs).
func (a *App) processEntryInto(
	entryPath string,
	isDir bool,
	size int64,
	modifiedMs int64,
	innerPath string,
	nestedChain []string,
	seen map[string]FileInfo,
) error {
	rel := getRelativePath(entryPath, innerPath)
	if rel == "" {
		return nil
	}

	// Only the immediate child name at this level.
	parts := strings.SplitN(rel, "/", 2)
	childName := parts[0]

	// If there is a deeper path, this child is a directory regardless of the
	// entry's own isDir flag (implicit directory from a nested path).
	implicitDir := len(parts) > 1
	childIsDir := isDir || implicitDir
	childIsArchive := !childIsDir && isArchiveFile(childName)

	if _, exists := seen[childName]; exists {
		return nil // already recorded
	}

	fi := FileInfo{
		Name:     childName,
		Path:     joinPath(innerPath, childName),
		Type:     FileTypeFile,
		Size:     size,
		Modified: modifiedMs,
	}
	switch {
	case childIsDir:
		fi.Type = FileTypeDirectory
		fi.Size = 0 // size is meaningless for virtual dirs
	case childIsArchive:
		fi.Type = FileTypeArchive
		fi.IsNested = true
		// Build the chain that reaches this nested archive.
		fi.NestedChain = append(append([]string{}, nestedChain...), childName)
	}

	seen[childName] = fi
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Nested archive extraction (all in-memory)
// ────────────────────────────────────────────────────────────────────────────

// extractNestedArchive resolves a chain of nested archives into the raw bytes
// of the innermost archive, entirely in memory.
func (a *App) extractNestedArchive(fsPath string, nestedChain []string) ([]byte, error) {
	if len(nestedChain) == 0 {
		return nil, fmt.Errorf("empty nested chain")
	}

	// Step 1: pull first entry from the on-disk archive.
	current, err := a.extractFromFile(fsPath, nestedChain[0])
	if err != nil {
		return nil, fmt.Errorf("extracting %q from %q: %w", nestedChain[0], fsPath, err)
	}

	// Step 2: recurse through the remaining layers in memory.
	for i := 1; i < len(nestedChain); i++ {
		next, err := a.extractFromBytes(current, nestedChain[i], nestedChain[i])
		if err != nil {
			return nil, fmt.Errorf("extracting %q (chain depth %d): %w", nestedChain[i], i, err)
		}
		current = next
	}
	return current, nil
}

// extractFromFile pulls one named entry out of an on-disk archive.
func (a *App) extractFromFile(fsPath, entryPath string) ([]byte, error) {
	want := normalize(entryPath)
	var buf []byte
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if normalize(archiveEntryPath(f)) == want {
			var rerr error
			buf, rerr = io.ReadAll(f)
			if rerr != nil {
				return rerr
			}
			return archiver.ErrStopWalk
		}
		return nil
	})
	if err != nil && err != archiver.ErrStopWalk {
		return nil, err
	}
	if buf == nil {
		return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
	}
	return buf, nil
}

// extractFromBytes pulls one named entry out of an in-memory archive.
func (a *App) extractFromBytes(data []byte, entryPath string, archiveName string) ([]byte, error) {
	want := normalize(entryPath)

	// Try ZIP first (magic bytes PK).
	if isZipMagic(data) {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			for _, f := range r.File {
				if normalize(f.Name) == want {
					rc, err := f.Open()
					if err != nil {
						return nil, err
					}
					defer rc.Close()
					return io.ReadAll(rc)
				}
			}
			return nil, fmt.Errorf("entry %q not found in ZIP", entryPath)
		}
	}

	if archiveName != "" && isRarArchiveName(archiveName) {
		return a.extractFromArchiveReader(data, archiveName, entryPath)
	}

	// Fall back to TAR (covers .tar, .tar.gz, .tgz, .tar.bz2, .tar.xz).
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(h.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry %q not found in archive", entryPath)
}

// ────────────────────────────────────────────────────────────────────────────
// Sorting & helpers
// ────────────────────────────────────────────────────────────────────────────

func (a *App) sortFileInfo(seen map[string]FileInfo) []FileInfo {
	entries := make([]FileInfo, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, e)
	}
	order := map[FileType]int{FileTypeDirectory: 0, FileTypeArchive: 1, FileTypeFile: 2, FileTypeSymlink: 3}
	sort.Slice(entries, func(i, j int) bool {
		oi, oj := order[entries[i].Type], order[entries[j].Type]
		if oi != oj {
			return oi < oj
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

// isZipMagic returns true when data starts with the ZIP local file header
// signature "PK\x03\x04" or the empty-archive signature "PK\x05\x06".
func isZipMagic(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' && data[1] == 'K' &&
		((data[2] == 0x03 && data[3] == 0x04) ||
			(data[2] == 0x05 && data[3] == 0x06))
}

func isRarArchiveName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".rar")
}

func (a *App) walkArchiveReader(data []byte, innerPath string, nestedChain []string, archiveName string) ([]FileInfo, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(archiveEntryPath(f))
		isDir := f.IsDir()
		if err := a.processEntryInto(ep, isDir, f.Size(), f.ModTime().UnixMilli(), innerPath, nestedChain, seen); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return a.sortFileInfo(seen), nil
}

func (a *App) extractFromArchiveReader(data []byte, archiveName, entryPath string) ([]byte, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	want := normalize(entryPath)

	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(archiveEntryPath(f)) == want {
			buf, readErr := io.ReadAll(f)
			f.Close()
			if readErr != nil {
				return nil, readErr
			}
			return buf, nil
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("entry %q not found in %s", entryPath, archiveName)
}

func openArchiveReader(archiveName string, data []byte) (archiver.Reader, error) {
	if archiveName == "" {
		return nil, fmt.Errorf("archive name required to read from bytes")
	}
	aIface, err := archiver.ByExtension(archiveName)
	if err != nil {
		return nil, err
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return nil, fmt.Errorf("archive format %q cannot be read from bytes", archiveName)
	}
	if err := reader.Open(bytes.NewReader(data), int64(len(data))); err != nil {
		reader.Close()
		return nil, err
	}
	return reader, nil
}
