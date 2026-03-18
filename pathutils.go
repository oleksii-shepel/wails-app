package main

import (
	"path/filepath"

	internalplatform "wails-app/internal/platform"
)

// toSlash converts any OS-native path separators to forward slashes.
// This is the single point where the backend normalises paths before
// returning them to the frontend, so the frontend never sees backslashes.
func toSlash(path string) string {
	return internalplatform.ToSlash(path)
}

// cleanPath normalises a path: resolves "..", cleans separators.
func cleanPath(path string) string {
	return internalplatform.CleanPath(path)
}

// IsAbsPath returns true if the path is absolute.
func (a *App) IsAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

// JoinPaths joins path segments safely.
func (a *App) JoinPaths(parts []string) string {
	return internalplatform.Join(parts...)
}

// SplitPath splits a path into directory and file components.
func (a *App) SplitPath(path string) map[string]string {
	dir, file, name, ext := internalplatform.SplitPath(path)
	return map[string]string{
		"dir":  dir,
		"base": file,
		"name": name,
		"ext":  ext,
	}
}

// GetParentPath returns the parent directory of the given path,
// stopping at the filesystem root.
func (a *App) GetParentPath(path string) string {
	return internalplatform.ParentPath(path)
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
	return internalplatform.Breadcrumbs(path)
}

// parentPath returns the parent directory, stopping at filesystem root.
// Use filepath.Dir which correctly handles both Unix "/" and Windows "C:\".
func parentPath(path string) string {
	return internalplatform.ParentPath(path)
}
