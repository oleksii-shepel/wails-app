package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"github.com/mholt/archiver/v3"
)

var archiveStdlibSuffixes = []string{".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".gz", ".bz2", ".xz", ".zip"}

func isZipMagic(data []byte) bool {
	return len(data) >= 4 &&
		data[0] == 'P' && data[1] == 'K' &&
		((data[2] == 0x03 && data[3] == 0x04) ||
			(data[2] == 0x05 && data[3] == 0x06))
}

func IsZipMagic(data []byte) bool {
	return isZipMagic(data)
}

func needsArchiveReader(name string) bool {
	if name == "" {
		return false
	}
	return !usesStdlibArchiveHandling(name)
}

func NeedsArchiveReader(name string) bool {
	return needsArchiveReader(name)
}

func isRarArchiveName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".rar")
}

func IsRarArchiveName(name string) bool {
	return isRarArchiveName(name)
}

func isZipArchiveName(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(name), ".zip")
}

func usesStdlibArchiveHandling(name string) bool {
	lower := strings.ToLower(name)
	for _, sfx := range archiveStdlibSuffixes {
		if strings.HasSuffix(lower, sfx) {
			return true
		}
	}
	return false
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

func OpenArchiveReader(archiveName string, data []byte) (archiver.Reader, error) {
	return openArchiveReader(archiveName, data)
}

func archiveEntryPath(f archiver.File) string {
	switch hdr := f.Header.(type) {
	case *zip.FileHeader:
		return strings.TrimSuffix(hdr.Name, "/")
	case *tar.Header:
		return strings.TrimSuffix(hdr.Name, "/")
	}
	return f.Name()
}

func normalizeArchiveEntryPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.TrimPrefix(path, "/")
}

func matchesEntries(innerPath string, entries []string) bool {
	cleanInner := normalizePath(innerPath)
	for _, entry := range entries {
		entry = normalizePath(entry)
		if entry == "" || cleanInner == entry || strings.HasPrefix(cleanInner, entry+"/") {
			return true
		}
	}
	return false
}