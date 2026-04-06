package app

import (
	"os"
	"path/filepath"
	"regexp"
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
		joined = trimAliasChildBase(joined) + "/" + normalizedPart
	}

	return pathref.Normalize(joined), true
}

var aliasRootPattern = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://$`)

func trimAliasChildBase(path string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	if aliasRootPattern.MatchString(strings.ToLower(trimmed)) {
		return trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func (a *App) SplitPath(path string) map[string]string {
	if dir, base, name, ext, ok := splitAliasPath(path); ok {
		return map[string]string{
			"dir":  dir,
			"base": base,
			"name": name,
			"ext":  ext,
		}
	}
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
	normalized := pathref.Normalize(path)
	if parent, ok := pathref.Parent(normalized); ok {
		return parent
	}
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		resolvedPath = pathref.ToNativePath(normalized)
	}
	return toPublicMemoryPath(internalplatform.ParentPath(resolvedPath))
}

func (a *App) PathExists(path string) bool {
	normalized := pathref.Normalize(path)
	if pathref.IsWeb(normalized) {
		_, err := a.State.GetFileInfo(normalized)
		return err == nil
	}
	resolvedPath, err := resolveMemoryAlias(path)
	if err != nil {
		return false
	}
	_, err = os.Lstat(resolvedPath)
	return err == nil
}

func (a *App) IsDirectory(path string) bool {
	normalized := pathref.Normalize(path)
	if pathref.IsWeb(normalized) {
		info, err := a.State.GetFileInfo(normalized)
		return err == nil && info != nil && info.Type == contracts.FileTypeDirectory
	}
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

func splitAliasPath(path string) (dir string, base string, name string, ext string, ok bool) {
	normalized := pathref.Normalize(path)
	if !pathref.IsAlias(normalized) && !pathref.IsDrive(normalized) {
		return "", "", "", "", false
	}

	dir = normalized
	if parent, hasParent := pathref.Parent(normalized); hasParent {
		dir = parent
	}

	trimmed := strings.TrimRight(normalized, "/")
	if trimmed == strings.TrimRight(dir, "/") {
		return dir, "", "", "", true
	}

	slashIndex := strings.LastIndex(trimmed, "/")
	if slashIndex >= 0 {
		base = trimmed[slashIndex+1:]
	} else {
		base = trimmed
	}
	ext = filepath.Ext(base)
	name = strings.TrimSuffix(base, ext)
	return dir, base, name, ext, true
}
