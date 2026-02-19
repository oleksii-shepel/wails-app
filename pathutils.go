package main

import (
	"path/filepath"
	"strings"
)

// cleanPath normalises a path: resolves "..", cleans separators.
func cleanPath(path string) string {
	return filepath.Clean(path)
}

// IsAbsPath returns true if the path is absolute.
func (a *App) IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// JoinPaths joins path segments safely.
func (a *App) JoinPaths(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return filepath.Join(parts...)
}

// SplitPath splits a path into directory and file components.
func (a *App) SplitPath(path string) map[string]string {
	dir, file := filepath.Split(path)
	ext := filepath.Ext(file)
	name := strings.TrimSuffix(file, ext)
	return map[string]string{
		"dir":  filepath.Clean(dir),
		"base": file,
		"name": name,
		"ext":  ext,
	}
}

// GetParentPath returns the parent directory of the given path,
// stopping at the filesystem root.
func (a *App) GetParentPath(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		return path // already at root
	}
	return parent
}

// PathExists checks whether a path exists.
func (a *App) PathExists(path string) bool {
	_, err := statPath(path)
	return err == nil
}

// IsDirectory returns true if the path is an existing directory.
func (a *App) IsDirectory(path string) bool {
	info, err := statPath(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsRoot returns true if the path is a filesystem root.
// On Unix: "/"
// On Windows: "C:\", "D:\", etc.
func (a *App) IsRoot(path string) bool {
	return filepath.Dir(path) == path
}

// Breadcrumbs returns the path split into navigable segments.
//
// Examples:
//
//	"/home/user/docs" → [{label:"/", path:"/"}, {label:"home", path:"/home"}, ...]
//	`C:\Users\alice` → [{label:`C:\`, path:`C:\`}, {label:"Users", path:`C:\Users`}, ...]
func (a *App) Breadcrumbs(path string) []map[string]string {
	path = filepath.Clean(path)

	// Walk upward collecting segments, then reverse.
	type segment struct {
		label string
		path  string
	}
	var segments []segment

	current := path
	for {
		parent := filepath.Dir(current)
		isRoot := parent == current

		var label string
		if isRoot {
			// Root segment: use the root path itself as the label.
			// On Unix this is "/"; on Windows "C:\".
			label = current
		} else {
			label = filepath.Base(current)
		}

		segments = append(segments, segment{label: label, path: current})

		if isRoot {
			break
		}
		current = parent
	}

	// Reverse so we go root → leaf.
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}

	crumbs := make([]map[string]string, len(segments))
	for i, s := range segments {
		crumbs[i] = map[string]string{
			"label": s.label,
			"path":  s.path,
		}
	}
	return crumbs
}

// parentPath returns the parent directory, stopping at filesystem root.
// Use filepath.Dir which correctly handles both Unix "/" and Windows "C:\".
func parentPath(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		return path // already at root
	}
	return parent
}
