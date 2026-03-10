package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ────────────────────────────────────────────────────────────────────────────
// Types
// ────────────────────────────────────────────────────────────────────────────

type ArchiveEntriesRequest struct {
	// FsPath is the absolute on-disk path of the archive.
	FsPath string `json:"fsPath"`
	// NestedChain is non-empty when operating on a nested archive.
	// Copy/Move support it; Delete requires it to be empty.
	NestedChain []string `json:"nestedChain"`
	// Entries are the inner paths to act on. Empty = all entries.
	Entries []string `json:"entries"`
	// StripPrefix is removed from destination paths when extracting.
	// Example: stripPrefix="a/b" and entry="a/b/c.txt" -> "c.txt".
	StripPrefix string `json:"stripPrefix,omitempty"`
	// DestinationDir is only used by Copy and Move.
	DestinationDir string `json:"destinationDir"`
	// FlattenPaths strips directory structure on extraction.
	FlattenPaths bool `json:"flattenPaths"`
}

type ArchiveEntriesResult struct {
	OperationID string `json:"operationId"`
	Total       int    `json:"total"`
	Succeeded   int    `json:"succeeded"`
	Failed      int    `json:"failed"`
	// SourceRemoved is false when the delete step was skipped (nested chain).
	SourceRemoved bool             `json:"sourceRemoved"`
	Errors        []BatchItemError `json:"errors,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// Public API - Archive Listing
// ────────────────────────────────────────────────────────────────────────────

// ArchiveList lists one level of a top-level archive on disk.
func (a *App) archiveList(fsPath string, innerPath string) (ArchiveListing, error) {
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
//
// The resolved archive is kept open in archCache so repeated navigation inside
// the same nested archive (e.g. entering subdirectories) costs only a ReadDir,
// not a fresh extraction.
func (a *App) archiveListNested(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	// Delegate to the cached VFS browse path, which resolveChain once and caches.
	return a.vfsBrowseArchive(fsPath, nestedChain, innerPath)
}

// ArchiveReadFile extracts a single entry from an archive into memory and
// returns its contents encoded as requested.
//
//   - encoding "base64" → base64 string  (use for binary: images, pdf, video…)
//   - encoding "utf8"/""  → raw UTF-8    (use for text files)
//
// Extraction is done entirely in-process — no temp files are written to disk
// for flat archives.  Nested archives do use a temp file only for the
// intermediate archive layer (not the final entry), which is unavoidable due
// to handler design, but the entry itself is always returned as []byte.
func (a *App) archiveReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	entryPath = normalize(entryPath)

	var data []byte
	var err error

	if len(nestedChain) == 0 {
		data, err = a.extractFromFile(fsPath, entryPath)
	} else {
		archiveData, nerr := a.extractNestedArchive(fsPath, nestedChain)
		if nerr != nil {
			return nil, nerr
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

// ────────────────────────────────────────────────────────────────────────────
// Walk helpers – on-disk archive
// ────────────────────────────────────────────────────────────────────────────

func (a *App) walkFile(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	lower := strings.ToLower(fsPath)

	// ZIP: use stdlib directly — archiver.Walk strips directory prefixes from hdr.Name
	if strings.HasSuffix(lower, ".zip") {
		r, err := zip.OpenReader(fsPath)
		if err != nil {
			return nil, err
		}
		defer r.Close()

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

	// 7z, RAR and other non-TAR formats: archiver.Walk mangles or drops entry
	// paths for these formats on some mholt/archiver/v3 builds. Use the Reader
	// API directly, which preserves full entry paths.
	if needsArchiveReader(fsPath) {
		return a.walkFileArchiveReader(fsPath, innerPath, nestedChain)
	}

	// TAR-based formats: archiver.Walk works correctly for these.
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

// walkFileArchiveReader lists entries of an on-disk 7z or RAR archive using
// archiver.Reader, which preserves full entry paths for these formats.
func (a *App) walkFileArchiveReader(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	f, err := os.Open(fsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	aIface, err := archiver.ByExtension(fsPath)
	if err != nil {
		return nil, fmt.Errorf("walkFileArchiveReader: unrecognised format %q: %w", fsPath, err)
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return nil, fmt.Errorf("walkFileArchiveReader: format %q is not readable", fsPath)
	}
	if err := reader.Open(f, fi.Size()); err != nil {
		return nil, err
	}
	defer reader.Close()

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(archiveEntryPath(entry))
		procErr := a.processEntryInto(ep, entry.IsDir(), entry.Size(), entry.ModTime().UnixMilli(), innerPath, nestedChain, seen)
		entry.Close()
		if procErr != nil {
			return nil, procErr
		}
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
	// Determine archive name from the chain so 7z/RAR bytes are routed to
	// walkArchiveReader regardless of chain depth (including depth 0).
	archiveName := ""
	if len(nestedChain) > 0 {
		archiveName = nestedChain[len(nestedChain)-1]
	}
	if archiveName != "" && needsArchiveReader(archiveName) {
		return a.walkArchiveReader(data, innerPath, nestedChain, archiveName)
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
	childIsArchive := !childIsDir && isArchive(childName)

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
		fi.NestedChain = append(append([]string{}, nestedChain...), fi.Path)
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
	lower := strings.ToLower(fsPath)

	// ZIP: use stdlib directly for the same reason as walkFile
	if strings.HasSuffix(lower, ".zip") {
		r, err := zip.OpenReader(fsPath)
		if err != nil {
			return nil, err
		}
		defer r.Close()
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
		return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
	}

	// 7z, RAR and other non-TAR formats: use the Reader API for correct paths.
	if needsArchiveReader(fsPath) {
		return a.extractFromFileArchiveReader(fsPath, entryPath)
	}

	// TAR-based formats
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

// extractFromFileArchiveReader pulls one named entry from an on-disk 7z or RAR
// archive using archiver.Reader, stopping as soon as the entry is found.
func (a *App) extractFromFileArchiveReader(fsPath, entryPath string) ([]byte, error) {
	want := normalize(entryPath)

	f, err := os.Open(fsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	aIface, err := archiver.ByExtension(fsPath)
	if err != nil {
		return nil, fmt.Errorf("extractFromFileArchiveReader: %w", err)
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return nil, fmt.Errorf("extractFromFileArchiveReader: format %q is not readable", fsPath)
	}
	if err := reader.Open(f, fi.Size()); err != nil {
		return nil, err
	}
	defer reader.Close()

	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(archiveEntryPath(entry)) == want {
			buf, readErr := io.ReadAll(entry)
			entry.Close()
			if readErr != nil {
				return nil, readErr
			}
			return buf, nil
		}
		entry.Close()
	}
	return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
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

	if archiveName != "" && needsArchiveReader(archiveName) {
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

// ────────────────────────────────────────────────────────────────────────────
// ZIP rewrite
// ────────────────────────────────────────────────────────────────────────────

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

// needsArchiveReader returns true for any format whose in-memory bytes must be
// decoded via archiver.Reader rather than the stdlib zip or tar packages.
// This covers RAR, 7z, and any other format supported by github.com/mholt/archiver
// that is NOT a ZIP (handled by magic bytes) and NOT a plain/compressed TAR.
func needsArchiveReader(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	// TAR family is handled directly by the tar package.
	for _, sfx := range []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".gz", ".bz2", ".xz"} {
		if strings.HasSuffix(lower, sfx) {
			return false
		}
	}
	// ZIP is handled by isZipMagic / stdlib zip; skip it here so the caller
	// can still attempt openArchiveReader on a mis-named file without error.
	if strings.HasSuffix(lower, ".zip") {
		return false
	}
	// Everything else (rar, 7z, …) goes through archiver.Reader.
	return true
}

// isRarArchiveName is kept for narrow RAR-only checks (e.g. in extractFromBytes
// where only RAR can appear as an in-memory nested archive name).
func isRarArchiveName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".rar")
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

// normalizePath strips leading slashes and normalises separators.
func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.TrimPrefix(p, "/")
}

// matchesEntries returns true when innerPath matches any of the listed entries.
// A directory entry matches the entry itself and all children.
// Used for archive rewrite (deletion) — exact matches are included so that
// explicit directory header entries are also removed from the archive.
func matchesEntries(innerPath string, entries []string) bool {
	cleanInner := normalize(innerPath)
	for _, e := range entries {
		e = normalize(e)
		if e == "" || cleanInner == e || strings.HasPrefix(cleanInner, e+"/") {
			return true
		}
	}
	return false
}

// matchesEntriesForExtract is like matchesEntries but skips an entry that
// exactly equals a selected path — i.e. the directory root header itself.
// When extracting "some/dir", we want its children ("some/dir/file.txt") but
// not the bare directory entry ("some/dir" or "some/dir/"), which would be
// written as a zero-byte file or spurious empty directory at the destination.
func matchesEntriesForExtract(innerPath string, entries []string) bool {
	cleanInner := normalize(innerPath)
	isDirHeader := strings.HasSuffix(strings.ReplaceAll(innerPath, "\\", "/"), "/")
	for _, e := range entries {
		e = normalize(e)
		if e == "" {
			return true
		}
		// Exact match for a directory header -> skip the root header itself.
		// Exact match for a file should still be included.
		if cleanInner == e {
			if isDirHeader {
				return false
			}
			return true
		}
		// Child of a selected directory → include it.
		if strings.HasPrefix(cleanInner, e+"/") {
			return true
		}
	}
	return false
}
