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

// Change the parameter and return type to FileInfo to match your DirectoryListing
func filterEntries(entries []FileInfo, filter FilterQuery) []FileInfo {
	// If FilterQuery is from types.go, use filter.Text.
	// If it's the simple one from utils.go, use filter.Query.
	searchQuery := filter.Text
	if searchQuery == "" {
		return entries
	}

	var result []FileInfo
	q := strings.ToLower(searchQuery)
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name), q) {
			result = append(result, e)
		}
	}
	return result
}
