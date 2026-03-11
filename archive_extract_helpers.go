package main

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// wrapTarReader opens a tar.Reader over src, unwrapping the compression layer
// indicated by fsPath's extension. Returns an optional close func for the
// decompression layer.
func wrapTarReader(src *os.File, fsPath string) (*tar.Reader, func(), error) {
	lower := strings.ToLower(fsPath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gr, err := gzip.NewReader(src)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gr), func() { gr.Close() }, nil
	case strings.HasSuffix(lower, ".tar.bz2"):
		return tar.NewReader(bzip2.NewReader(src)), nil, nil
	case strings.HasSuffix(lower, ".tar.xz"):
		return nil, nil, fmt.Errorf("tar.xz extraction is handled via archiver.Walk, not wrapTarReader")
	default:
		return tar.NewReader(src), nil, nil
	}
}

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

func extractDestPath(entryName, destDir string, flatten bool) string {
	entryName = normalizeArchiveEntryPath(entryName)
	if flatten {
		return filepath.Join(destDir, filepath.Base(entryName))
	}
	return filepath.Join(destDir, filepath.FromSlash(entryName))
}
