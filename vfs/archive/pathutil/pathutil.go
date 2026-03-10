package pathutil

import "strings"

func NormalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.Trim(path, "/")
}

func NormalizeTarPath(path string) string {
	path = NormalizePath(path)
	return strings.TrimPrefix(path, "./")
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
