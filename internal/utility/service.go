package utility

import (
	"fmt"
	"os"

	internalplatform "wails-app/internal/platform"
)

type Service struct {
	emitEvent func(string, interface{})
}

type ResolvedLink struct {
	Target string `json:"target"`
	IsDir  bool   `json:"isDir"`
}

func NewService(emitEvent func(string, interface{})) *Service {
	return &Service{emitEvent: emitEvent}
}

func (s *Service) OpenFile(path string) error {
	return internalplatform.OpenFileWithDefaultApp(path)
}

func (s *Service) ResolveLink(path string) (*ResolvedLink, error) {
	target, isDir, err := internalplatform.ResolveLinkPath(path)
	if err != nil {
		return nil, err
	}
	return &ResolvedLink{Target: target, IsDir: isDir}, nil
}

func (s *Service) HandleFileDrop(x int, y int, paths []string) error {
	fmt.Printf("[HandleFileDrop] Called with x=%d, y=%d, %d paths: %v\n", x, y, len(paths), paths)

	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}

	providedPaths := paths
	isRelativeOnly := false
	for _, path := range paths {
		if !isAbsolutePath(path) {
			isRelativeOnly = true
			fmt.Printf("[HandleFileDrop] Path is relative: %s\n", path)
			break
		}
	}

	if isRelativeOnly {
		fmt.Println("[HandleFileDrop] Detected filename-only paths, attempting clipboard fallback...")
		clipboardPaths, err := GetFilePaths()
		if err == nil && len(clipboardPaths) > 0 {
			fmt.Printf("[HandleFileDrop] Got %d paths from clipboard: %v\n", len(clipboardPaths), clipboardPaths)
			providedPaths = clipboardPaths
		} else {
			fmt.Println("[HandleFileDrop] Clipboard fallback failed:", err)
			fmt.Printf("[HandleFileDrop] Continuing with provided %d paths\n", len(paths))
		}
	}

	validPaths := make([]string, 0, len(providedPaths))
	for _, path := range providedPaths {
		if path == "" {
			continue
		}
		fmt.Printf("[HandleFileDrop] Checking path: %s\n", path)
		if stat, err := os.Stat(path); err == nil {
			validPaths = append(validPaths, path)
			fmt.Printf("[HandleFileDrop] Path exists: %s (isDir=%v)\n", path, stat.IsDir())
		} else {
			fmt.Printf("[HandleFileDrop] Path does not exist: %s (%v)\n", path, err)
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
	return path[0] == '/'
}
