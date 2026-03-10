package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ArchiveDeleteEntries rewrites the archive atomically (temp file → rename),
// omitting the listed entries. Emits operation:start / :progress / :complete.
// NestedChain must be empty — nested rewrite is not supported.
func (a *App) archiveDeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if len(req.NestedChain) > 0 {
		return nil, fmt.Errorf("deleting entries from nested archives is not supported")
	}
	if len(req.Entries) == 0 {
		return &ArchiveEntriesResult{}, nil
	}

	op := a.operations.StartOperation(OpDelete, "Delete from archive",
		req.FsPath, "")
	op.SetTotal(0, 0)

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
	case needsArchiveReader(req.FsPath):
		ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(strings.ToLower(req.FsPath)), "."))
		err := fmt.Errorf("deleting entries from %s archives is not supported (format is read-only)", ext)
		op.SetError(err)
		return nil, err
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

		innerPath := normalizeArchiveEntryPath(f.Name)
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

// Reading .tar.bz2:  stdlib compress/bzip2 (read-only, correct).
// Reading .tar.xz:   archiver.Walk (handles xz via its internal ulikunitz/xz dep).
// Writing bz2 / xz:  Go stdlib has no bz2/xz write support; rewriting bz2/xz
//
//	archives is done via archiver's TarBz2/TarXz types.
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
	case "bz2":
		tarReader = tar.NewReader(bzip2.NewReader(srcFile))
	default:
		tarReader = tar.NewReader(srcFile)
	}

	if compression == "xz" {
		return rewriteTarXzViaArchiver(fsPath, entries, op)
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
	if compression == "gz" || compression == "bz2" {
		gzWriter = gzip.NewWriter(tmp)
		tarWriter = tar.NewWriter(gzWriter)
	} else {
		tarWriter = tar.NewWriter(tmp)
	}

	var processed int64
	for {
		if op.IsCancelled() {
			if gzWriter != nil {
				tarWriter.Close()
				gzWriter.Close()
			} else {
				tarWriter.Close()
			}
			return removed, kept, fmt.Errorf("cancelled")
		}

		hdr, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return removed, kept, tarErr
		}

		innerPath := normalizeArchiveEntryPath(hdr.Name)
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

	if gzWriter != nil {
		if e := tarWriter.Close(); e != nil {
			return removed, kept, e
		}
		if e := gzWriter.Close(); e != nil {
			return removed, kept, e
		}
	} else {
		if e := tarWriter.Close(); e != nil {
			return removed, kept, e
		}
	}
	tmp.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}

func rewriteTarXzViaArchiver(fsPath string, entries []string, op *ActiveOperation) (removed, kept int, err error) {
	type keptEntry struct {
		hdr  *tar.Header
		data []byte
	}
	var keptEntries []keptEntry

	walkErr := archiver.Walk(fsPath, func(f archiver.File) error {
		if op.IsCancelled() {
			return fmt.Errorf("cancelled")
		}
		hdr, ok := f.Header.(*tar.Header)
		if !ok {
			return nil
		}
		innerPath := normalizeArchiveEntryPath(hdr.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			return nil
		}
		data, rerr := io.ReadAll(f)
		if rerr != nil {
			return rerr
		}
		keptEntries = append(keptEntries, keptEntry{hdr: hdr, data: data})
		kept++
		return nil
	})
	if walkErr != nil {
		return removed, kept, walkErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*.tar.xz")
	if err != nil {
		return removed, kept, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	txz := archiver.NewTarXz()
	if e := txz.Create(tmp); e != nil {
		return removed, kept, fmt.Errorf("create tar.xz: %w", e)
	}

	var processed int64
	for _, ke := range keptEntries {
		if err = txz.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   ke.hdr.FileInfo(),
				CustomName: ke.hdr.Name,
			},
			ReadCloser: io.NopCloser(bytes.NewReader(ke.data)),
		}); err != nil {
			txz.Close()
			return removed, kept, fmt.Errorf("write %q: %w", ke.hdr.Name, err)
		}
		processed++
		op.UpdateProgress(processed, int64(len(ke.data)), ke.hdr.Name, int64(len(ke.data)))
	}

	if err = txz.Close(); err != nil {
		return removed, kept, err
	}
	tmp.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}
