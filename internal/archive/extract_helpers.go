package archive

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

func extractWriteFile(destPath string, reader io.Reader) (int64, error) {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, reader)
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

func matchesEntriesForExtract(innerPath string, entries []string) bool {
	cleanInner := normalizePath(innerPath)
	isDirHeader := strings.HasSuffix(strings.ReplaceAll(innerPath, "\\", "/"), "/")
	for _, entry := range entries {
		entry = normalizePath(entry)
		if entry == "" {
			return true
		}
		if cleanInner == entry {
			if isDirHeader {
				return false
			}
			return true
		}
		if entry == "" || cleanInner == entry || strings.HasPrefix(cleanInner, entry+"/") {
			return true
		}
	}
	return false
}

func rebaseExtractPath(entryName, stripPrefix string) (string, bool) {
	entry := normalizePath(entryName)
	if entry == "" {
		return "", false
	}
	prefix := normalizePath(stripPrefix)
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