package main

import (
	"archive/tar"
	"archive/zip"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

type Entry struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"` // Full path for FS, InnerPath for Archive
	Type        string   `json:"type"` // "DIRECTORY", "FILE", "ARCHIVE"
	Size        int64    `json:"size"`
	Modified    int64    `json:"modified"`
	IsNested    bool     `json:"isNested"`
	NestedChain []string `json:"nestedChain,omitempty"`
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

func normalize(p string) string {
	return strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
}

func getRelativePath(full, base string) string {
	if base == "" {
		return full
	}
	if full == base {
		return ""
	}
	if strings.HasPrefix(full, base+"/") {
		return full[len(base)+1:]
	}
	return ""
}

func isArchiveFile(name string) bool {
	name = strings.ToLower(name)
	ext := detectExt(name)
	switch ext {
	case ".zip", ".tar", ".tgz", ".gz", ".bz2", ".xz", ".tar.gz", ".tar.bz2", ".tar.xz":
		return true
	}
	return false
}

func detectExt(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.HasSuffix(l, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(l, ".tar.bz2"):
		return ".tar.bz2"
	case strings.HasSuffix(l, ".tar.xz"):
		return ".tar.xz"
	case strings.HasSuffix(l, ".tgz"):
		return ".tgz"
	}
	return filepath.Ext(name)
}

// Add this function to fix "undefined: joinPath"
func joinPath(base, child string) string {
	base = normalize(base)
	child = normalize(child)
	if base == "" {
		return child
	}
	return base + "/" + child
}

// Ensure filterEntries uses the FileInfo type from types.go
func filterEntries(entries []FileInfo, filter FilterQuery) []FileInfo {
	if filter.Text == "" {
		return entries
	}
	var result []FileInfo
	q := strings.ToLower(filter.Text)
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			result = append(result, e)
		}
	}
	return result
}
