package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
// Public API - Archive Mutations
// ────────────────────────────────────────────────────────────────────────────

// ArchiveCopyEntries extracts the listed entries to DestinationDir
// synchronously, blocking until every file is fully written to disk.
//
// BatchSelectiveExtract is NOT used here because:
//  1. It is fire-and-forget (async), so callers cannot know when files are
//     actually on disk.
//  2. It uses matchesEntries which exact-matches directory root headers,
//     writing them as empty files at the destination instead of skipping them.
//
// extractEntriesSync uses matchesEntriesForExtract which correctly skips the
// directory root entry itself while still extracting all of its children.
func (a *App) ArchiveCopyEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}

	op := a.operations.StartOperation(OpCopy, "Copy from archive",
		req.FsPath, req.DestinationDir)

	succeeded, failed, errs, err := a.extractEntriesSync(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	op.Complete()
	return &ArchiveEntriesResult{
		OperationID: op.ID,
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}, nil
}

// ArchiveMoveEntries extracts entries to DestinationDir synchronously, then
// rewrites the archive without them only after all files are fully on disk.
//
// BatchSelectiveExtract is NOT used here because it is fire-and-forget (async):
// it starts an operation and returns before files are written to disk, so
// ArchiveDeleteEntries would rewrite the archive while extraction is still
// in progress, corrupting or losing files.
//
// extractEntriesSync blocks until every byte is written, then the archive
// rewrite via ArchiveDeleteEntries is safe to run.
func (a *App) ArchiveMoveEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}
	if len(req.Entries) == 0 {
		return &ArchiveEntriesResult{}, nil
	}

	op := a.operations.StartOperation(OpMove, "Move from archive",
		req.FsPath, req.DestinationDir)

	// Phase 1: synchronous extraction — blocks until all files are on disk.
	succeeded, failed, errs, err := a.extractEntriesSync(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	result := &ArchiveEntriesResult{
		OperationID: op.ID,
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}

	if failed > 0 {
		op.SetError(fmt.Errorf("%d file(s) failed to extract; archive not modified", failed))
		return result, nil
	}

	// Phase 2: atomic archive rewrite — safe because phase 1 has completed.
	if len(req.NestedChain) > 0 {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, BatchItemError{
			Source: req.FsPath,
			Error:  "entries copied but NOT removed: deleting from nested archives is not supported",
		})
		op.Complete()
		return result, nil
	}

	deleteReq := ArchiveEntriesRequest{FsPath: req.FsPath, Entries: req.Entries}
	delResult, delErr := a.ArchiveDeleteEntries(deleteReq)
	if delErr != nil {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, BatchItemError{
			Source: req.FsPath,
			Error:  "entries extracted but NOT removed from archive: " + delErr.Error(),
		})
		op.Complete()
		return result, nil
	}

	result.SourceRemoved = true
	if delResult.Failed > 0 {
		result.Errors = append(result.Errors, delResult.Errors...)
	}
	op.Complete()
	return result, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Synchronous extraction
// ────────────────────────────────────────────────────────────────────────────

// extractEntriesSync extracts all matching entries to req.DestinationDir and
// blocks until every file is fully written to disk.
// Directory root entries (e.g. the "some/dir/" header itself) are skipped via
// matchesEntriesForExtract — only the directory's children are extracted.
func (a *App) extractEntriesSync(req ArchiveEntriesRequest, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	if err := os.MkdirAll(req.DestinationDir, 0755); err != nil {
		return 0, 0, nil, fmt.Errorf("create destination: %w", err)
	}

	if len(req.NestedChain) > 0 {
		data, err := a.extractNestedArchive(req.FsPath, req.NestedChain)
		if err != nil {
			return 0, 0, nil, err
		}
		return extractFromBytesSync(data, req.NestedChain[len(req.NestedChain)-1], req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	}

	lower := strings.ToLower(req.FsPath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZipSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	default:
		return extractTarSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	}
}

func extractZipSync(fsPath string, entries []string, stripPrefix string, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	r, err := zip.OpenReader(fsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	defer r.Close()

	var matches []*zip.File
	for _, f := range r.File {
		if matchesEntriesForExtract(normalizePath(f.Name), entries) {
			matches = append(matches, f)
		}
	}
	op.SetTotal(int64(len(matches)), 0)

	var processed int64
	for _, f := range matches {
		if op.IsCancelled() {
			errs = append(errs, BatchItemError{Source: f.Name, Error: "cancelled"})
			failed++
			continue
		}
		if f.FileInfo().IsDir() {
			continue // create dirs implicitly via MkdirAll below
		}

		relPath, ok := rebaseExtractPath(f.Name, stripPrefix)
		if !ok {
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, BatchItemError{Source: f.Name, Error: err.Error()})
			failed++
			continue
		}

		rc, err := f.Open()
		if err != nil {
			errs = append(errs, BatchItemError{Source: f.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, rc)
		rc.Close()
		if writeErr != nil {
			errs = append(errs, BatchItemError{Source: f.Name, Error: writeErr.Error()})
			failed++
			continue
		}

		succeeded++
		processed++
		op.UpdateProgress(processed, n, f.Name, n)
	}
	return
}

func extractTarSync(fsPath string, entries []string, stripPrefix string, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	src, err := os.Open(fsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	defer src.Close()

	tr, closeExtra, err := wrapTarReader(src, fsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	if closeExtra != nil {
		defer closeExtra()
	}

	op.SetTotal(0, 0)
	var processed int64
	for {
		if op.IsCancelled() {
			break
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return succeeded, failed, errs, err
		}
		if hdr.FileInfo().IsDir() || !matchesEntriesForExtract(normalizePath(hdr.Name), entries) {
			continue
		}

		relPath, ok := rebaseExtractPath(hdr.Name, stripPrefix)
		if !ok {
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, BatchItemError{Source: hdr.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, BatchItemError{Source: hdr.Name, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}

func extractFromBytesSync(data []byte, archiveName string, entries []string, stripPrefix string, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	if isZipMagic(data) {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return 0, 0, nil, err
		}
		var matches []*zip.File
		for _, f := range r.File {
			if matchesEntriesForExtract(normalizePath(f.Name), entries) {
				matches = append(matches, f)
			}
		}
		op.SetTotal(int64(len(matches)), 0)
		var processed int64
		for _, f := range matches {
			if op.IsCancelled() {
				errs = append(errs, BatchItemError{Source: f.Name, Error: "cancelled"})
				failed++
				continue
			}
			if f.FileInfo().IsDir() {
				continue
			}
			relPath, ok := rebaseExtractPath(f.Name, stripPrefix)
			if !ok {
				continue
			}
			destPath := extractDestPath(relPath, destDir, flatten)
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				errs = append(errs, BatchItemError{Source: f.Name, Error: err.Error()})
				failed++
				continue
			}
			rc, err := f.Open()
			if err != nil {
				errs = append(errs, BatchItemError{Source: f.Name, Error: err.Error()})
				failed++
				continue
			}
			n, writeErr := extractWriteFile(destPath, rc)
			rc.Close()
			if writeErr != nil {
				errs = append(errs, BatchItemError{Source: f.Name, Error: writeErr.Error()})
				failed++
				continue
			}
			succeeded++
			processed++
			op.UpdateProgress(processed, n, f.Name, n)
		}
		return
	}

	tr := tar.NewReader(bytes.NewReader(data))
	op.SetTotal(0, 0)
	var processed int64
	for {
		if op.IsCancelled() {
			break
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return succeeded, failed, errs, err
		}
		if hdr.FileInfo().IsDir() || !matchesEntriesForExtract(normalizePath(hdr.Name), entries) {
			continue
		}
		relPath, ok := rebaseExtractPath(hdr.Name, stripPrefix)
		if !ok {
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, BatchItemError{Source: hdr.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, BatchItemError{Source: hdr.Name, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}

// wrapTarReader opens a tar.Reader over src, unwrapping gzip when the filename
// indicates it. Returns an optional close func for the gzip layer.
func wrapTarReader(src *os.File, fsPath string) (*tar.Reader, func(), error) {
	lower := strings.ToLower(fsPath)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tar.xz") {
		gr, err := gzip.NewReader(src)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gr), func() { gr.Close() }, nil
	}
	return tar.NewReader(src), nil, nil
}

// extractWriteFile creates destPath and copies all of r into it.
func extractWriteFile(destPath string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return n, copyErr
	}
	return n, closeErr
}

// extractDestPath resolves the output path for an archive entry.
func extractDestPath(entryName, destDir string, flatten bool) string {
	entryName = normalizePath(entryName)
	if flatten {
		return filepath.Join(destDir, filepath.Base(entryName))
	}
	return filepath.Join(destDir, filepath.FromSlash(entryName))
}

func rebaseExtractPath(entryName, stripPrefix string) (string, bool) {
	entry := normalize(entryName)
	if entry == "" {
		return "", false
	}

	prefix := normalize(stripPrefix)
	if prefix == "" {
		return entry, true
	}

	if entry == prefix {
		return "", false
	}

	prefixWithSep := prefix + "/"
	if strings.HasPrefix(entry, prefixWithSep) {
		rel := strings.TrimPrefix(entry, prefixWithSep)
		if rel == "" {
			return "", false
		}
		return rel, true
	}

	return entry, true
}

// ArchiveDeleteEntries rewrites the archive atomically (temp file → rename),
// omitting the listed entries. Emits operation:start / :progress / :complete.
// NestedChain must be empty — nested rewrite is not supported.
func (a *App) ArchiveDeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if len(req.NestedChain) > 0 {
		return nil, fmt.Errorf("deleting entries from nested archives is not supported")
	}
	if len(req.Entries) == 0 {
		return &ArchiveEntriesResult{}, nil
	}

	// Start operation so OperationService / toast stack picks it up.
	op := a.operations.StartOperation(OpDelete, "Delete from archive",
		req.FsPath, "")
	op.SetTotal(0, 0) // unknown total until we scan

	var (
		removed, kept int
		rewriteErr    error
	)

	lower := strings.ToLower(req.FsPath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		removed, kept, rewriteErr = rewriteZip(req.FsPath, req.Entries, op)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		removed, kept, rewriteErr = rewriteTar(req.FsPath, req.Entries, "gz", op)
	case strings.HasSuffix(lower, ".tar.bz2"):
		removed, kept, rewriteErr = rewriteTar(req.FsPath, req.Entries, "bz2", op)
	case strings.HasSuffix(lower, ".tar.xz"):
		removed, kept, rewriteErr = rewriteTar(req.FsPath, req.Entries, "xz", op)
	case strings.HasSuffix(lower, ".tar"):
		removed, kept, rewriteErr = rewriteTar(req.FsPath, req.Entries, "", op)
	default:
		op.SetError(fmt.Errorf("unsupported archive format"))
		return nil, fmt.Errorf("unsupported archive format: %q", req.FsPath)
	}

	result := &ArchiveEntriesResult{
		OperationID:   op.ID,
		Total:         removed + kept,
		Succeeded:     removed,
		SourceRemoved: rewriteErr == nil,
	}

	if rewriteErr != nil {
		op.SetError(rewriteErr)
		result.Failed = 1
		result.Errors = []BatchItemError{{Source: req.FsPath, Error: rewriteErr.Error()}}
		return result, rewriteErr
	}

	op.Complete()
	return result, nil
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

	// TAR-based formats: archiver.Walk works correctly for these
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

func rewriteZip(fsPath string, entries []string, op *ActiveOperation) (removed, kept int, err error) {
	r, err := zip.OpenReader(fsPath)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()

	op.SetTotal(int64(len(r.File)), 0)

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*.zip")
	if err != nil {
		return 0, 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(tmp)
	defer w.Close()

	var processed int64
	for _, f := range r.File {
		if op.IsCancelled() {
			return removed, kept, fmt.Errorf("cancelled")
		}

		innerPath := normalizePath(f.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			processed++
			op.UpdateProgress(processed, 0, f.Name, 0)
			continue
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return removed, kept, fmt.Errorf("open %q: %w", f.Name, openErr)
		}

		hdr, _ := zip.FileInfoHeader(f.FileInfo())
		hdr.Name = f.Name
		hdr.Method = f.Method
		fw, writeErr := w.CreateHeader(hdr)
		if writeErr != nil {
			rc.Close()
			return removed, kept, writeErr
		}

		n, copyErr := io.Copy(fw, rc)
		rc.Close()
		if copyErr != nil {
			return removed, kept, fmt.Errorf("copy %q: %w", f.Name, copyErr)
		}

		kept++
		processed++
		op.UpdateProgress(processed, n, f.Name, n)
	}

	if err = w.Close(); err != nil {
		return removed, kept, err
	}
	tmp.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}

// ────────────────────────────────────────────────────────────────────────────
// TAR rewrite (plain / gz / bz2 / xz)
// ────────────────────────────────────────────────────────────────────────────
//
// bz2 and xz cannot be written natively without extra deps.
// They are rewritten as plain .tar.gz — format mismatch is accepted in this
// prototype. For a production build, link in dsnet/compress.

func rewriteTar(fsPath string, entries []string, compression string, op *ActiveOperation) (removed, kept int, err error) {
	srcFile, err := os.Open(fsPath)
	if err != nil {
		return 0, 0, err
	}
	defer srcFile.Close()

	var tarReader *tar.Reader
	switch compression {
	case "gz":
		gr, gzErr := gzip.NewReader(srcFile)
		if gzErr != nil {
			return 0, 0, gzErr
		}
		defer gr.Close()
		tarReader = tar.NewReader(gr)
	default:
		tarReader = tar.NewReader(srcFile)
	}

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*")
	if err != nil {
		return 0, 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	var tarWriter *tar.Writer
	var gzWriter *gzip.Writer
	if compression == "gz" || compression == "bz2" || compression == "xz" {
		gzWriter = gzip.NewWriter(tmp)
		tarWriter = tar.NewWriter(gzWriter)
		defer func() {
			tarWriter.Close()
			gzWriter.Close()
		}()
	} else {
		tarWriter = tar.NewWriter(tmp)
		defer tarWriter.Close()
	}

	var processed int64
	for {
		if op.IsCancelled() {
			return removed, kept, fmt.Errorf("cancelled")
		}

		hdr, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return removed, kept, tarErr
		}

		innerPath := normalizePath(hdr.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			processed++
			op.UpdateProgress(processed, 0, hdr.Name, 0)
			continue
		}

		if writeErr := tarWriter.WriteHeader(hdr); writeErr != nil {
			return removed, kept, writeErr
		}

		n, copyErr := io.Copy(tarWriter, tarReader)
		if copyErr != nil {
			return removed, kept, copyErr
		}

		kept++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}

	return removed, kept, os.Rename(tmpPath, fsPath)
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
