package main

import (
	"fmt"
	"os"

	internalplatform "wails-app/internal/platform"
)

// UtilityService owns small app-level utilities that don't belong to a larger
// domain service.
type UtilityService struct {
	emitEvent func(string, interface{})
}

func NewUtilityService(emitEvent func(string, interface{})) *UtilityService {
	return &UtilityService{emitEvent: emitEvent}
}

// ResolvedLink is the result returned by ResolveLink.
type ResolvedLink struct {
	Target string `json:"target"`
	IsDir  bool   `json:"isDir"`
}

func (s *UtilityService) OpenFile(path string) error {
	return internalplatform.OpenFileWithDefaultApp(path)
}

func (s *UtilityService) ResolveLink(path string) (*ResolvedLink, error) {
	target, isDir, err := internalplatform.ResolveLinkPath(path)
	if err != nil {
		return nil, err
	}
	return &ResolvedLink{Target: target, IsDir: isDir}, nil
}

func (s *UtilityService) HandleFileDrop(x int, y int, paths []string) error {
	fmt.Printf("[HandleFileDrop] Called with x=%d, y=%d, %d paths: %v\n", x, y, len(paths), paths)

	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}

	providedPaths := paths
	isRelativeOnly := false
	for _, p := range paths {
		if !isAbsolutePath(p) {
			isRelativeOnly = true
			fmt.Printf("[HandleFileDrop] Path is relative: %s\n", p)
			break
		}
	}

	if isRelativeOnly {
		fmt.Println("[HandleFileDrop] Detected filename-only paths, attempting clipboard fallback...")
		clipboardPaths, err := GetClipboardFilePaths()
		if err == nil && len(clipboardPaths) > 0 {
			fmt.Printf("[HandleFileDrop] Got %d paths from clipboard: %v\n", len(clipboardPaths), clipboardPaths)
			providedPaths = clipboardPaths
		} else {
			fmt.Println("[HandleFileDrop] Clipboard fallback failed:", err)
			fmt.Printf("[HandleFileDrop] Continuing with provided %d paths\n", len(paths))
		}
	}

	validPaths := make([]string, 0, len(providedPaths))
	for _, p := range providedPaths {
		if p == "" {
			continue
		}
		fmt.Printf("[HandleFileDrop] Checking path: %s\n", p)
		if stat, err := os.Stat(p); err == nil {
			validPaths = append(validPaths, p)
			fmt.Printf("[HandleFileDrop] Path exists: %s (isDir=%v)\n", p, stat.IsDir())
		} else {
			fmt.Printf("[HandleFileDrop] Path does not exist: %s (%v)\n", p, err)
		}
	}

	if len(validPaths) == 0 {
		fmt.Println("[HandleFileDrop] No valid file paths after validation")
		return fmt.Errorf("no valid file paths after validation")
	}

	fmt.Printf("[HandleFileDrop] Emitting file:drop event with %d valid files\n", len(validPaths))
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
	if path[0] == '/' {
		return true
	}
	return false
}
