package pathutil

import "strings"

// NormalizePath converts backslashes to forward slashes and collapses the
// path into clean "/"-joined segments, dropping empty segments (from
// duplicate or leading/trailing slashes) and "." segments (from archives
// whose entries are stored with a "./" prefix, e.g. those produced by
// `zip -r out.zip .`). Without this, an entry like "./readme.txt" would
// keep its "." as the first path segment, causing archive browsing to
// collapse everything under a single synthetic "." folder instead of
// listing the real contents.
func NormalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	segments := strings.Split(path, "/")
	out := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || seg == "." {
			continue
		}
		out = append(out, seg)
	}
	return strings.Join(out, "/")
}

// NormalizeTarPath is now equivalent to NormalizePath since "./" prefixes
// are stripped generically there. Kept as a thin wrapper so existing call
// sites don't need to change.
func NormalizeTarPath(path string) string {
	return NormalizePath(path)
}

func ChildOf(entryPath, parent string) string {
	entryPath = NormalizePath(entryPath)
	parent = NormalizePath(parent)
	if parent == "" {
		return entryPath
	}
	if entryPath == parent {
		return ""
	}
	prefix := parent + "/"
	if strings.HasPrefix(entryPath, prefix) {
		return entryPath[len(prefix):]
	}
	return ""
}

func MatchesEntries(entryPath string, entries []string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, entry := range entries {
		entry = NormalizePath(entry)
		if entryPath == entry || strings.HasPrefix(entryPath, entry+"/") {
			return true
		}
	}
	return false
}

func RebasePath(entryPath, stripPrefix string) string {
	if stripPrefix == "" {
		return entryPath
	}
	prefix := NormalizePath(stripPrefix)
	if entryPath == prefix {
		return ""
	}
	if strings.HasPrefix(entryPath, prefix+"/") {
		return entryPath[len(prefix)+1:]
	}
	return entryPath
}