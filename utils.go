package main

import (
	"archive/tar"
	"archive/zip"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

// Entry is kept for any frontend types that reference it directly.
type Entry struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Size        int64    `json:"size"`
	Modified    int64    `json:"modified"`
	IsNested    bool     `json:"isNested"`
	NestedChain []string `json:"nestedChain,omitempty"`
}

// archiveEntryPath returns the canonical inner path for an archiver.File,
// preferring the typed header (which carries the full path) over f.Name()
// (which may only be the base name for some formats).
func archiveEntryPath(f archiver.File) string {
	switch hdr := f.Header.(type) {
	case *zip.FileHeader:
		return strings.TrimSuffix(hdr.Name, "/")
	case *tar.Header:
		return strings.TrimSuffix(hdr.Name, "/")
	}
	return f.Name()
}

// normalize converts backslashes to forward slashes and strips leading/
// trailing slashes so all paths are in a consistent "a/b/c" form.
func normalize(p string) string {
	return strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
}

// getRelativePath returns the portion of `full` that follows `base/`.
// Returns "" when full == base (the directory itself) or when full is not
// under base at all.
func getRelativePath(full, base string) string {
	if base == "" {
		return full
	}
	if full == base {
		return ""
	}
	prefix := base + "/"
	if strings.HasPrefix(full, prefix) {
		return full[len(prefix):]
	}
	return ""
}

// isArchive reports whether a file name looks like a supported archive.
func isArchive(name string) bool {
	switch detectExt(strings.ToLower(name)) {
	case ".zip", ".tar", ".tgz", ".gz", ".bz2", ".xz", ".rar", ".7z", ".tar.gz", ".tar.bz2", ".tar.xz":
		return true
	}
	return false
}

// detectExt returns the effective extension, handling compound suffixes like
// ".tar.gz" before falling back to filepath.Ext.
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

// joinPath joins an inner-path base and a child name using forward slashes.
func joinPath(base, child string) string {
	base = normalize(base)
	child = normalize(child)
	if base == "" {
		return child
	}
	return base + "/" + child
}

// filterEntries filters a slice of FileInfo entries using a FilterQuery.
// It applies both the quick Text search and the structured Criteria list.
func filterEntries(entries []FileInfo, filter FilterQuery) []FileInfo {
	// Short-circuit: nothing to filter.
	if filter.Text == "" && len(filter.Criteria) == 0 {
		return entries
	}

	result := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		if matchesFilter(e, filter) {
			result = append(result, e)
		}
	}
	return result
}
