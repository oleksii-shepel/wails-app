package main

import (
	"path/filepath"
	"time"

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
	return filepath.IsAbs(path)
}

func (a *App) JoinPaths(parts []string) string {
	return internalplatform.Join(parts...)
}

func (a *App) SplitPath(path string) map[string]string {
	dir, file, name, ext := internalplatform.SplitPath(path)
	return map[string]string{
		"dir":  dir,
		"base": file,
		"name": name,
		"ext":  ext,
	}
}

func (a *App) GetParentPath(path string) string {
	return internalplatform.ParentPath(path)
}

func (a *App) PathExists(path string) bool {
	_, err := statPath(path)
	return err == nil
}

func (a *App) IsDirectory(path string) bool {
	info, err := statPath(path)
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

func (a *App) GetSpecialFolders() SpecialFoldersResponse {
	return internalplatform.GetSpecialFolders()
}
