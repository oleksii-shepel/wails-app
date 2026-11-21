package archive

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

	contracts "wails-app/internal/contracts"

	"github.com/mholt/archiver/v3"
)

var errCancelled = errors.New("cancelled")

func (s *Service) extractZipSync(fsPath string, entries []string, stripPrefix string, destDir string, flatten bool, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
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
			errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: errCancelled.Error()})
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
		destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

		if destPathErr != nil {

			errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

			failed++

			continue

		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: err.Error()})
			failed++
			continue
		}
		rc, err := f.Open()
		if err != nil {
			errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, rc)
		rc.Close()
		if writeErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, f.Name, n)
	}
	return
}

func (s *Service) extractTarSync(fsPath string, entries []string, stripPrefix string, destDir string, flatten bool, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
	lower := strings.ToLower(fsPath)
	if strings.HasSuffix(lower, ".tar.xz") {
		return s.extractTarXzViaArchiver(fsPath, entries, stripPrefix, destDir, flatten, op)
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
		if errors.Is(err, io.EOF) {
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
		destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

		if destPathErr != nil {

			errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

			failed++

			continue

		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, contracts.BatchItemError{Source: hdr.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: hdr.Name, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}

func (s *Service) extractTarXzViaArchiver(fsPath string, entries []string, stripPrefix, destDir string, flatten bool, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
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
		destPath, destPathErr := extractDestPath(relPath, destDir, flatten)
		if destPathErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})
			failed++
			return nil
		}
		n, writeErr := extractWriteFile(destPath, f)
		if writeErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: hdr.Name, Error: writeErr.Error()})
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

func (s *Service) extractArchiveReaderSync(fsPath string, entries []string, stripPrefix, destDir string, flatten bool, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
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
		if errors.Is(err, io.EOF) {
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
		destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

		if destPathErr != nil {

			errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

			failed++

			continue

		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, contracts.BatchItemError{Source: entryName, Error: err.Error()})
			failed++
			entry.Close()
			continue
		}
		n, writeErr := extractWriteFile(destPath, entry)
		entry.Close()
		if writeErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: entryName, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, entryName, n)
	}
	return
}

func (s *Service) extractEntriesFromBytes(data []byte, archiveName string, entries []string, stripPrefix string, destDir string, flatten bool, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
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
			if errors.Is(err, io.EOF) {
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
			destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

			if destPathErr != nil {

				errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

				failed++

				continue

			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				errs = append(errs, contracts.BatchItemError{Source: entryName, Error: err.Error()})
				failed++
				f.Close()
				continue
			}
			n, writeErr := extractWriteFile(destPath, f)
			f.Close()
			if writeErr != nil {
				errs = append(errs, contracts.BatchItemError{Source: entryName, Error: writeErr.Error()})
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
				errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: errCancelled.Error()})
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
			destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

			if destPathErr != nil {

				errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

				failed++

				continue

			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: err.Error()})
				failed++
				continue
			}
			rc, err := f.Open()
			if err != nil {
				errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: err.Error()})
				failed++
				continue
			}
			n, writeErr := extractWriteFile(destPath, rc)
			rc.Close()
			if writeErr != nil {
				errs = append(errs, contracts.BatchItemError{Source: f.Name, Error: writeErr.Error()})
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
		if errors.Is(err, io.EOF) {
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
		destPath, destPathErr := extractDestPath(relPath, destDir, flatten)

		if destPathErr != nil {

			errs = append(errs, contracts.BatchItemError{Source: relPath, Error: destPathErr.Error()})

			failed++

			continue

		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			errs = append(errs, contracts.BatchItemError{Source: hdr.Name, Error: err.Error()})
			failed++
			continue
		}
		n, writeErr := extractWriteFile(destPath, tr)
		if writeErr != nil {
			errs = append(errs, contracts.BatchItemError{Source: hdr.Name, Error: writeErr.Error()})
			failed++
			continue
		}
		succeeded++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}
	return
}
