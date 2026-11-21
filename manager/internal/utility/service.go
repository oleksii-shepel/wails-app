package utility

import (
	"fmt"
	"os"

	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
	vfslib "wails-app/vfs"
)

type memoryPathResolver interface {
	ResolvePublicPath(path string) (string, bool, error)
	ToPublicPath(path string) string
}

type ServiceConfig struct {
	EmitEvent func(string, interface{})
}

type Service struct {
	emitEvent func(string, interface{})
	vfs       *vfslib.Unified
}

type ResolvedLink struct {
	Target string `json:"target"`
	IsDir  bool   `json:"isDir"`
}

func NewService(config ServiceConfig) *Service {
	return &Service{
		emitEvent: config.EmitEvent,
	}
}

func (s *Service) SetVFS(v *vfslib.Unified) {
	s.vfs = v
}

func (s *Service) resolvePublicPath(path string) (string, error) {
	normalized := pathref.Normalize(path)
	if pathref.IsWeb(normalized) {
		return normalized, nil
	}
	if resolved, ok := pathref.ResolveLocalCloudPath(normalized); ok {
		return resolved, nil
	}
	if s.vfs != nil {
		resolvedPath, err := s.vfs.ResolvePublicPath(normalized)
		if err != nil {
			return "", err
		}
		if resolvedPath == "" {
			return "", fmt.Errorf("path %q is virtual-only and not available to the OS", normalized)
		}
		return resolvedPath, nil
	}
	nativePath := pathref.ToNativePath(normalized)
	return nativePath, nil
}

func (s *Service) toPublicPath(path string) string {
	return pathref.FromNativePath(path)
}

func (s *Service) OpenFile(path string) error {
	normalized := pathref.Normalize(path)
	if pathref.IsWeb(normalized) {
		return internalplatform.OpenFileWithDefaultApp(normalized)
	}
	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return err
	}
	return internalplatform.OpenFileWithDefaultApp(resolvedPath)
}

func (s *Service) ResolveLink(path string) (*ResolvedLink, error) {
	normalized := pathref.Normalize(path)
	resolvedPath, err := s.resolvePublicPath(path)
	if err != nil {
		return nil, err
	}
	target, isDir, err := internalplatform.ResolveLinkPath(resolvedPath)
	if err != nil {
		return nil, err
	}
	publicTarget := s.toPublicPath(target)
	if pathref.IsLocalCloud(normalized) {
		if mapped, ok := pathref.LocalCloudAliasFromNativePath(target); ok {
			publicTarget = mapped
		}
	}
	return &ResolvedLink{Target: publicTarget, IsDir: isDir}, nil
}

func (s *Service) HandleFileDrop(x int, y int, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}

	providedPaths := paths
	isRelativeOnly := false
	for _, path := range paths {
		if !isAbsolutePath(path) {
			isRelativeOnly = true
			break
		}
	}

	if isRelativeOnly {
		// Drag sources such as browsers only report dropped filenames, so
		// fall back to the file paths copied alongside the drop.
		if clipboardPaths, err := GetFilePaths(); err == nil && len(clipboardPaths) > 0 {
			providedPaths = clipboardPaths
		}
	}

	validPaths := make([]string, 0, len(providedPaths))
	for _, path := range providedPaths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			validPaths = append(validPaths, path)
		}
	}

	if len(validPaths) == 0 {
		return fmt.Errorf("no valid file paths after validation")
	}

	if s.emitEvent != nil {
		s.emitEvent("file:drop", map[string]interface{}{
			"x":     x,
			"y":     y,
			"paths": validPaths,
		})
	}
	return nil
}

func isAbsolutePath(path string) bool {
	if len(path) == 0 {
		return false
	}
	if len(path) > 1 && path[1] == ':' {
		return true
	}
	if len(path) > 1 && path[0] == '\\' && path[1] == '\\' {
		return true
	}
	return path[0] == '/'
}
