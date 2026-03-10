package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mholt/archiver/v3"
)

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
	for _, sfx := range []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".gz", ".bz2", ".xz"} {
		if strings.HasSuffix(lower, sfx) {
			return false
		}
	}
	if strings.HasSuffix(lower, ".zip") {
		return false
	}
	return true
}

// isRarArchiveName is kept for narrow RAR-only checks used by batch operations.
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
func matchesEntries(innerPath string, entries []string) bool {
	cleanInner := normalize(innerPath)
	for _, entry := range entries {
		entry = normalize(entry)
		if entry == "" || cleanInner == entry || strings.HasPrefix(cleanInner, entry+"/") {
			return true
		}
	}
	return false
}

// matchesEntriesForExtract is like matchesEntries but skips an entry that
// exactly equals a selected path when that entry is just the directory header.
func matchesEntriesForExtract(innerPath string, entries []string) bool {
	cleanInner := normalize(innerPath)
	isDirHeader := strings.HasSuffix(strings.ReplaceAll(innerPath, "\\", "/"), "/")
	for _, entry := range entries {
		entry = normalize(entry)
		if entry == "" {
			return true
		}
		if cleanInner == entry {
			if isDirHeader {
				return false
			}
			return true
		}
		if strings.HasPrefix(cleanInner, entry+"/") {
			return true
		}
	}
	return false
}
