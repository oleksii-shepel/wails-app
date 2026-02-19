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

// GetParentPath returns the parent directory of the given path.
func (a *App) GetParentPath(path string) string {
	return filepath.Dir(path)
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

// Breadcrumbs returns the path split into navigable segments.
// e.g. "/home/user/docs" → [{label:"home", path:"/home"}, ...]
func (a *App) Breadcrumbs(path string) []map[string]string {
	path = filepath.Clean(path)
	parts := splitIntoParts(path)

	crumbs := make([]map[string]string, 0, len(parts))
	accumulated := ""

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 && isRootPart(part) {
			accumulated = part
		} else {
			if accumulated == "" || accumulated == string(filepath.Separator) {
				accumulated = string(filepath.Separator) + part
			} else {
				accumulated = accumulated + string(filepath.Separator) + part
			}
		}
		crumbs = append(crumbs, map[string]string{
			"label": part,
			"path":  accumulated,
		})
	}

	return crumbs
}

// splitIntoParts splits a path into its component parts.
func splitIntoParts(path string) []string {
	// filepath.SplitList is for PATH env; we want directory components
	var parts []string
	for {
		dir, file := filepath.Split(path)
		dir = strings.TrimSuffix(dir, string(filepath.Separator))
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == path || dir == "" {
			if dir != "" {
				parts = append([]string{dir}, parts...)
			}
			break
		}
		path = dir
	}
	return parts
}

func isRootPart(part string) bool {
	// Windows drive root like "C:", Unix root already handled
	return strings.HasSuffix(part, ":")
}
