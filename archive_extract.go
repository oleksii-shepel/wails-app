package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

// errCancelled is returned when the user cancels an in-progress extraction.
var errCancelled = errors.New("cancelled")

// extractEntriesSync extracts all matching entries to req.DestinationDir and
// blocks until every file is fully written to disk.
// Directory root entries (e.g. the "some/dir/" header itself) are skipped via
// matchesEntriesForExtract - only the directory's children are extracted.
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
	case needsArchiveReader(req.FsPath):
		return a.extractArchiveReaderSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
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
		if matchesEntriesForExtract(normalizeArchiveEntryPath(f.Name), entries) {
			matches = append(matches, f)
		}
	}
	op.SetTotal(int64(len(matches)), 0)

	var processed int64
	for _, f := range matches {
		if op.IsCancelled() {
			errs = append(errs, batchErr(f.Name, "", errCancelled))
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
			errs = append(errs, batchErr(f.Name, "", err))
			failed++
			continue
		}

		rc, err := f.Open()
		if err != nil {
			errs = append(errs, batchErr(f.Name, "", err))
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, rc)
		rc.Close()
		if writeErr != nil {
			errs = append(errs, batchErr(f.Name, "", writeErr))
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
	// .tar.xz cannot be decompressed by stdlib; route through archiver.Walk
	// which uses the bundled ulikunitz/xz dependency.
	lower := strings.ToLower(fsPath)
	if strings.HasSuffix(lower, ".tar.xz") {
		return extractTarXzViaArchiver(fsPath, entries, stripPrefix, destDir, flatten, op)
	}

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
		if hdr.FileInfo().IsDir() || !matchesEntriesForExtract(normalizeArchiveEntryPath(hdr.Name), entries) {
			continue
		}

		relPath, ok := rebaseExtractPath(hdr.Name, stripPrefix)
		if !ok {
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, batchErr(hdr.Name, "", err))
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, batchErr(hdr.Name, "", writeErr))
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}

// extractTarXzViaArchiver extracts from a .tar.xz using archiver.Walk, which
// handles xz decompression via its bundled ulikunitz/xz dependency.
func extractTarXzViaArchiver(fsPath string, entries []string, stripPrefix, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	op.SetTotal(0, 0)
	var processed int64

	walkErr := archiver.Walk(fsPath, func(f archiver.File) error {
		if op.IsCancelled() {
			return errCancelled
		}
		hdr, ok := f.Header.(*tar.Header)
		if !ok || f.IsDir() {
			return nil
		}
		if !matchesEntriesForExtract(normalizeArchiveEntryPath(hdr.Name), entries) {
			return nil
		}
		relPath, ok := rebaseExtractPath(hdr.Name, stripPrefix)
		if !ok {
			return nil
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, batchErr(hdr.Name, "", err))
			failed++
			return nil
		}
		n, writeErr := extractWriteFile(destPath, f)
		if writeErr != nil {
			errs = append(errs, batchErr(hdr.Name, "", writeErr))
			failed++
			return nil
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errCancelled) {
		fatalErr = walkErr
	}
	return
}

// extractArchiveReaderSync extracts matching entries from an on-disk 7z or RAR
// archive to destDir using archiver.Reader - the only read-capable API for
// these formats. Used by extractEntriesSync for ArchiveCopyEntries/ArchiveMoveEntries.
func (a *App) extractArchiveReaderSync(fsPath string, entries []string, stripPrefix, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	f, err := os.Open(fsPath)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, 0, nil, err
	}

	aIface, err := archiver.ByExtension(fsPath)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("extractArchiveReaderSync: %w", err)
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return 0, 0, nil, fmt.Errorf("extractArchiveReaderSync: format %q is not readable", fsPath)
	}
	if err := reader.Open(f, fi.Size()); err != nil {
		return 0, 0, nil, err
	}
	defer reader.Close()

	op.SetTotal(0, 0)
	var processed int64

	for {
		if op.IsCancelled() {
			break
		}
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return succeeded, failed, errs, err
		}
		entryName := archiveEntryPath(entry)
		if entry.IsDir() || !matchesEntriesForExtract(normalizeArchiveEntryPath(entryName), entries) {
			entry.Close()
			continue
		}
		relPath, ok := rebaseExtractPath(entryName, stripPrefix)
		if !ok {
			entry.Close()
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, batchErr(entryName, "", err))
			failed++
			entry.Close()
			continue
		}
		n, writeErr := extractWriteFile(destPath, entry)
		entry.Close()
		if writeErr != nil {
			errs = append(errs, batchErr(entryName, "", writeErr))
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, entryName, n)
	}
	return
}

func extractFromBytesSync(data []byte, archiveName string, entries []string, stripPrefix string, destDir string, flatten bool, op *ActiveOperation) (succeeded, failed int, errs []BatchItemError, fatalErr error) {
	if !isZipMagic(data) && needsArchiveReader(archiveName) {
		reader, err := openArchiveReader(archiveName, data)
		if err != nil {
			return 0, 0, nil, err
		}
		defer reader.Close()
		op.SetTotal(0, 0)
		var processed int64
		for {
			if op.IsCancelled() {
				break
			}
			f, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return succeeded, failed, errs, err
			}
			entryName := archiveEntryPath(f)
			if f.IsDir() || !matchesEntriesForExtract(normalizeArchiveEntryPath(entryName), entries) {
				f.Close()
				continue
			}
			relPath, ok := rebaseExtractPath(entryName, stripPrefix)
			if !ok {
				f.Close()
				continue
			}
			destPath := extractDestPath(relPath, destDir, flatten)
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				errs = append(errs, batchErr(entryName, "", err))
				failed++
				f.Close()
				continue
			}
			n, writeErr := extractWriteFile(destPath, f)
			f.Close()
			if writeErr != nil {
				errs = append(errs, batchErr(entryName, "", writeErr))
				failed++
				continue
			}
			succeeded++
			processed++
			op.UpdateProgress(processed, n, entryName, n)
		}
		return
	}

	if isZipMagic(data) {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return 0, 0, nil, err
		}
		var matches []*zip.File
		for _, f := range r.File {
			if matchesEntriesForExtract(normalizeArchiveEntryPath(f.Name), entries) {
				matches = append(matches, f)
			}
		}
		op.SetTotal(int64(len(matches)), 0)
		var processed int64
		for _, f := range matches {
			if op.IsCancelled() {
				errs = append(errs, batchErr(f.Name, "", errCancelled))
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
				errs = append(errs, batchErr(f.Name, "", err))
				failed++
				continue
			}
			rc, err := f.Open()
			if err != nil {
				errs = append(errs, batchErr(f.Name, "", err))
				failed++
				continue
			}
			n, writeErr := extractWriteFile(destPath, rc)
			rc.Close()
			if writeErr != nil {
				errs = append(errs, batchErr(f.Name, "", writeErr))
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
		if hdr.FileInfo().IsDir() || !matchesEntriesForExtract(normalizeArchiveEntryPath(hdr.Name), entries) {
			continue
		}
		relPath, ok := rebaseExtractPath(hdr.Name, stripPrefix)
		if !ok {
			continue
		}
		destPath := extractDestPath(relPath, destDir, flatten)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, batchErr(hdr.Name, "", err))
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, batchErr(hdr.Name, "", writeErr))
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}
