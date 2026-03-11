package main

import "strings"

// normalizeArchiveEntryPath preserves a trailing slash on raw archive header
// names so extract/rewrite matching can still distinguish a directory header
// from a regular file entry. This intentionally does not share VFS pathutil
// semantics, which trim both ends for browsing-oriented paths.
func normalizeArchiveEntryPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.TrimPrefix(p, "/")
}

// matchesEntries returns true when innerPath matches any of the listed entries.
// A directory entry matches the entry itself and all children.
func matchesEntries(innerPath string, entries []string) bool {
	cleanInner := normalize(innerPath)
	for _, entry := range entries {
		entry = normalize(entry)
		if matchesArchiveEntry(cleanInner, entry) {
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
		if matchesArchiveEntry(cleanInner, entry) {
			return true
		}
	}
	return false
}

func rebaseExtractPath(entryName, stripPrefix string) (string, bool) {
	entry := normalize(entryName)
	if entry == "" {
		return "", false
	}

	prefix := normalize(stripPrefix)
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

func matchesArchiveEntry(cleanInner, entry string) bool {
	return entry == "" || cleanInner == entry || strings.HasPrefix(cleanInner, entry+"/")
}
