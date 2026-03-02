package main

import (
	"encoding/base64"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================================================
// List Directory
// ============================================================================

func (a *App) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	var totalSize int64

	for _, entry := range entries {
		name := entry.Name()

		// Hidden file filtering
		if !showHidden && isHidden(name, path) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := toSlash(filepath.Join(path, name))
		fi := buildFileInfo(name, fullPath, info)
		files = append(files, fi)
		if !fi.IsHidden || showHidden {
			totalSize += fi.Size
		}
	}

	if files == nil {
		files = []FileInfo{}
	}

	return &DirectoryListing{
		Path:      toSlash(path),
		Entries:   files,
		TotalSize: totalSize,
	}, nil
}

// ============================================================================
// Get File Info
// ============================================================================

func (a *App) GetFileInfo(path string) (*FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	fi := buildFileInfo(name, path, info)
	return &fi, nil
}

// ============================================================================
// Read File
// ============================================================================

func (a *App) ReadFile(path string, encoding string) (*FileContent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if encoding == "" {
		encoding = "utf8"
	}

	var content string
	if encoding == "base64" {
		content = base64.StdEncoding.EncodeToString(data)
	} else {
		content = string(data)
		encoding = "utf8"
	}

	return &FileContent{
		Path:     path,
		Content:  content,
		Encoding: encoding,
		Size:     info.Size(),
	}, nil
}

// ============================================================================
// Write File
// ============================================================================

func (a *App) WriteFile(path string, content string, encoding string) (*OperationResult, error) {
	var data []byte
	var err error

	if encoding == "base64" {
		data, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, err
		}
	} else {
		data = []byte(content)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}

	return &OperationResult{
		Success:       true,
		Message:       "File written successfully",
		AffectedPaths: []string{path},
	}, nil
}

// ============================================================================
// Delete File / Directory
// ============================================================================

func (a *App) DeleteFile(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	trash := useTrash == nil || *useTrash
	if trash {
		if err := moveToTrash(path); err != nil {
			return nil, err
		}
		return &OperationResult{
			Success:       true,
			Message:       "Moved to trash successfully",
			AffectedPaths: []string{path},
		}, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		if recursive {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return nil, err
	}

	return &OperationResult{
		Success:       true,
		Message:       "Deleted successfully",
		AffectedPaths: []string{path},
	}, nil
}

// ============================================================================
// Create Directory
// ============================================================================

func (a *App) CreateDirectory(path string, recursive bool) (*OperationResult, error) {
	var err error
	if recursive {
		err = os.MkdirAll(path, 0755)
	} else {
		err = os.Mkdir(path, 0755)
	}
	if err != nil {
		return nil, err
	}

	return &OperationResult{
		Success:       true,
		Message:       "Directory created",
		AffectedPaths: []string{path},
	}, nil
}

// ============================================================================
// Move / Rename
// ============================================================================

func (a *App) MoveFile(source, destination string) (*OperationResult, error) {
	// Try atomic rename first
	err := os.Rename(source, destination)
	if err != nil {
		// Cross-device: copy then delete
		if err2 := copyPath(source, destination); err2 != nil {
			return nil, err2
		}
		if err2 := os.RemoveAll(source); err2 != nil {
			return nil, err2
		}
	}

	return &OperationResult{
		Success:       true,
		Message:       "Moved successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

// ============================================================================
// Copy
// ============================================================================

func (a *App) CopyFile(source, destination string, recursive bool) (*OperationResult, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}

	if info.IsDir() && !recursive {
		return &OperationResult{
			Success: false,
			Message: "Source is a directory; set recursive=true to copy directories",
		}, nil
	}

	if err := copyPath(source, destination); err != nil {
		return nil, err
	}

	return &OperationResult{
		Success:       true,
		Message:       "Copied successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

// ============================================================================
// Search
// ============================================================================

func (a *App) SearchFiles(path, pattern string, recursive bool) (*SearchResult, error) {
	var matches []FileInfo

	walkFn := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if p == path {
			return nil
		}
		// If not recursive, skip subdirectories' contents
		if !recursive && d.IsDir() && p != path {
			return filepath.SkipDir
		}

		name := d.Name()
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return err
		}
		if !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		matches = append(matches, buildFileInfo(name, toSlash(p), info))
		return nil
	}

	if err := filepath.WalkDir(path, walkFn); err != nil {
		return nil, err
	}

	if matches == nil {
		matches = []FileInfo{}
	}

	return &SearchResult{
		Path:         toSlash(path),
		Matches:      matches,
		TotalMatches: len(matches),
	}, nil
}

// ============================================================================
// Helpers
// ============================================================================

func buildFileInfo(name, fullPath string, info os.FileInfo) FileInfo {
	ft := FileTypeFile
	isSymlink := false
	if info.IsDir() {
		ft = FileTypeDirectory
	} else if info.Mode()&os.ModeSymlink != 0 {
		isSymlink = true
		// Use the platform helper so that protected Windows junctions (e.g.
		// "Documents and Settings") are correctly identified as directories
		// without needing traversal access.
		if symlinkPointsToDir(fullPath) {
			ft = FileTypeDirectory
		} else {
			ft = FileTypeSymlink
		}
	} else if isArchive(name) {
		ft = FileTypeArchive
	}

	return FileInfo{
		Name:        name,
		Path:        fullPath,
		Type:        ft,
		Size:        info.Size(),
		Created:     getCreatedTime(info),
		Modified:    info.ModTime().Unix(),
		Accessed:    getAccessedTime(info),
		Permissions: formatPermissions(info),
		IsHidden:    isHiddenFile(name, fullPath),
		IsSymlink:   isSymlink,
	}
}

func isHidden(name, dir string) bool {
	return isHiddenFile(name, filepath.Join(dir, name))
}

func formatPermissions(info os.FileInfo) string {
	if runtime.GOOS == "windows" {
		// Simplified Windows permissions
		if info.Mode()&0200 != 0 {
			return "rw-"
		}
		return "r--"
	}
	return info.Mode().Perm().String()
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(p, target)
	})
}

// isHiddenFile is the cross-platform hidden check stub —
// platform-specific logic lives in platform_{windows,unix}.go
func isHiddenFile(name, path string) bool {
	return checkHidden(name, path)
}

func matchesFilter(e FileInfo, q FilterQuery) bool {
	// Simple text search
	if q.Text != "" {
		name := e.Name
		text := q.Text
		if !q.CaseSensitive {
			name = strings.ToLower(name)
			text = strings.ToLower(text)
		}
		if !strings.Contains(name, text) {
			return false
		}
	}

	// Advanced criteria
	for _, c := range q.Criteria {
		if !matchesCriteria(e, c, q.CaseSensitive) {
			return false
		}
	}

	return true
}

func matchesCriteria(e FileInfo, c FilterCriteria, caseSensitive bool) bool {
	switch c.Type {
	case FilterName:
		return compareStrings(e.Name, c.Value, c.Operator, caseSensitive)
	case FilterExtension:
		ext := strings.TrimPrefix(filepath.Ext(e.Name), ".")
		return compareStrings(ext, c.Value, c.Operator, caseSensitive)
	case FilterGlob:
		matched, _ := filepath.Match(c.Value, e.Name)
		return matched
	case FilterType_:
		return strings.EqualFold(string(e.Type), c.Value)
	}
	return true
}

func compareStrings(a, b, operator string, caseSensitive bool) bool {
	if !caseSensitive {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	switch operator {
	case "equals":
		return a == b
	case "startsWith":
		return strings.HasPrefix(a, b)
	case "endsWith":
		return strings.HasSuffix(a, b)
	case "contains", "":
		return strings.Contains(a, b)
	}
	return strings.Contains(a, b)
}
