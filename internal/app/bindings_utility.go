package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
	internalutility "wails-app/internal/utility"
)

type ResolvedLink = internalutility.ResolvedLink

func (a *App) Ping() int64 {
	return time.Now().UnixMilli()
}

func (a *App) OpenFile(path string) error {
	return a.Utility.OpenFile(path)
}

func (a *App) ResolveLink(path string) (*ResolvedLink, error) {
	return a.Utility.ResolveLink(path)
}

func (a *App) IsAbsPath(path string) bool {
	if pathref.IsDrive(pathref.Normalize(path)) {
		return true
	}
	return filepath.IsAbs(path)
}

func (a *App) JoinPaths(parts []string) string {
	if joined, ok := joinAliasParts(parts); ok {
		return joined
	}
	return internalplatform.Join(parts...)
}

func joinAliasParts(parts []string) (string, bool) {
	if len(parts) == 0 {
		return "", false
	}

	first := pathref.Normalize(parts[0])
	if !pathref.IsAlias(first) && !pathref.IsDrive(first) {
		return "", false
	}

	joined := first
	for _, part := range parts[1:] {
		normalizedPart := strings.Trim(strings.ReplaceAll(part, "\\", "/"), "/")
		if normalizedPart == "" {
			continue
		}
		joined = strings.TrimRight(joined, "/") + "/" + normalizedPart
	}

	return pathref.Normalize(joined), true
}

func (a *App) SplitPath(path string) map[string]string {
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		resolvedPath = pathref.ToNativePath(pathref.Normalize(path))
	}
	dir, file, name, ext := internalplatform.SplitPath(resolvedPath)
	return map[string]string{
		"dir":  toPublicMemoryPath(dir),
		"base": file,
		"name": name,
		"ext":  ext,
	}
}

func (a *App) GetParentPath(path string) string {
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		resolvedPath = pathref.ToNativePath(pathref.Normalize(path))
	}
	return toPublicMemoryPath(internalplatform.ParentPath(resolvedPath))
}

func (a *App) PathExists(path string) bool {
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		return false
	}
	_, err = os.Lstat(resolvedPath)
	return err == nil
}

func (a *App) IsDirectory(path string) bool {
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		return false
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (a *App) IsRoot(path string) bool {
	return filepath.Dir(path) == path
}

func (a *App) Breadcrumbs(path string) []map[string]string {
	return internalplatform.Breadcrumbs(path)
}

func (a *App) GetSpecialFolders() contracts.SpecialFoldersResponse {
	return internalplatform.GetSpecialFolders()
}
