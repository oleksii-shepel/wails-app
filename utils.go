package main

import (
	"archive/tar"
	"archive/zip"
	"strings"

	"github.com/mholt/archiver/v3"

	internalplatform "wails-app/internal/platform"
	"wails-app/vfs/archive"
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
	return internalplatform.Normalize(p)
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
// It delegates to the format registry so it stays in sync automatically
// when new handlers are registered — no separate extension list to maintain.
func isArchive(name string) bool {
	return archive.IsArchiveExt(name)
}

// joinPath joins an inner-path base and a child name using forward slashes.
func joinPath(base, child string) string {
	return internalplatform.JoinNormalized(base, child)
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
