package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	internalfilesystem "wails-app/internal/filesystem"
	internalplatform "wails-app/internal/platform"
	internalruntime "wails-app/internal/runtime"
)

// ============================================================================
// List Directory
// ============================================================================

func (a *App) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	return a.filesystem.ListDirectory(path, showHidden)
}

func (s *FilesystemService) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	// Route through VFS if the path lives inside a mounted volume (e.g. an archive).
	if s.vfs != nil && s.vfs.IsMounted(path) {
		return s.listVFSDirectory(path)
	}

	listing, err := listDirectoryRaw(path, showHidden)
	if err == nil {
		return listing, nil
	}
	if !shouldRetryDeleteWithElevation(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("list", s.elevationTargetSingle(path), elevatedRequest{
		Op:         "list",
		Path:       path,
		ShowHidden: showHidden,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.DirListing, nil
}

func (s *FilesystemService) listVFSDirectory(path string) (*DirectoryListing, error) {
	return internalruntime.VFSListDir(s.vfs, path)
}

func listDirectoryRaw(path string, showHidden bool) (*DirectoryListing, error) {
	return rawFilesystemOps.ListDirectory(path, showHidden)
}

// ============================================================================
// Dir Size Recursive (BFS)
// ============================================================================

// DirSizeRecursive returns the total byte size of all regular files under path
// using breadth-first traversal.  Unreadable sub-directories are silently skipped
// so that a single permission error does not abort the entire walk.
func (a *App) DirSizeRecursive(path string) (int64, error) {
	return a.filesystem.DirSizeRecursive(path)
}

func (s *FilesystemService) DirSizeRecursive(path string) (int64, error) {
	var total int64
	queue := []string{path}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		entries, err := os.ReadDir(current)
		if err != nil {
			continue // skip unreadable dirs
		}
		for _, e := range entries {
			full := filepath.Join(current, e.Name())
			if e.IsDir() {
				queue = append(queue, full)
			} else {
				info, err := e.Info()
				if err == nil {
					total += info.Size()
				}
			}
		}
	}
	return total, nil
}

// ============================================================================
// Get File Info
// ============================================================================

func (a *App) GetFileInfo(path string) (*FileInfo, error) {
	return a.filesystem.GetFileInfo(path)
}

func (s *FilesystemService) GetFileInfo(path string) (*FileInfo, error) {
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
	return a.filesystem.ReadFile(path, encoding)
}

func (s *FilesystemService) ReadFile(path string, encoding string) (*FileContent, error) {
	file, err := readFileRaw(path, encoding)
	if err == nil {
		return file, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("read", s.elevationTargetSingle(path), elevatedRequest{
		Op:       "read",
		Path:     path,
		Encoding: encoding,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.File, nil
}

func readFileRaw(path string, encoding string) (*FileContent, error) {
	return rawFilesystemOps.ReadFile(path, encoding)
}

// ============================================================================
// Write File
// ============================================================================

func (a *App) WriteFile(path string, content string, encoding string) (*OperationResult, error) {
	return a.filesystem.WriteFile(path, content, encoding)
}

func (s *FilesystemService) WriteFile(path string, content string, encoding string) (*OperationResult, error) {
	op, err := writeFileRaw(path, content, encoding)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("write", s.elevationTargetSingle(path), elevatedRequest{
		Op:       "write",
		Path:     path,
		Content:  content,
		Encoding: encoding,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.Operation, nil
}

func writeFileRaw(path string, content string, encoding string) (*OperationResult, error) {
	return rawFilesystemOps.WriteFile(path, content, encoding)
}

// ============================================================================
// Delete File / Directory
// ============================================================================

func (a *App) DeleteFile(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	return a.filesystem.DeleteFile(path, recursive, useTrash)
}

func (s *FilesystemService) DeleteFile(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	op, err := deleteFileRaw(path, recursive, useTrash)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("delete", s.elevationTargetSingle(path), elevatedRequest{
		Op:        "delete",
		Path:      path,
		Recursive: recursive,
		UseTrash:  useTrash,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.Operation, nil
}

func deleteFileRaw(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	return rawFilesystemOps.DeleteFile(path, recursive, useTrash)
}

// ============================================================================
// Create Directory
// ============================================================================

func (a *App) CreateDirectory(path string, recursive bool) (*OperationResult, error) {
	return a.filesystem.CreateDirectory(path, recursive)
}

func (s *FilesystemService) CreateDirectory(path string, recursive bool) (*OperationResult, error) {
	op, err := createDirectoryRaw(path, recursive)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("mkdir", s.elevationTargetSingle(path), elevatedRequest{
		Op:        "mkdir",
		Path:      path,
		Recursive: recursive,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.Operation, nil
}

func createDirectoryRaw(path string, recursive bool) (*OperationResult, error) {
	return rawFilesystemOps.CreateDirectory(path, recursive)
}

// ============================================================================
// Move / Rename
// ============================================================================

func (a *App) MoveFile(source, destination string) (*OperationResult, error) {
	return a.filesystem.MoveFile(source, destination)
}

func (s *FilesystemService) MoveFile(source, destination string) (*OperationResult, error) {
	op, err := moveFileRaw(source, destination)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("move", s.elevationTargetPair(source, destination), elevatedRequest{
		Op:          "move",
		Source:      source,
		Destination: destination,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.Operation, nil
}

func moveFileRaw(source, destination string) (*OperationResult, error) {
	return rawFilesystemOps.MoveFile(source, destination)
}

// ============================================================================
// Copy
// ============================================================================

func (a *App) CopyFile(source, destination string, recursive bool) (*OperationResult, error) {
	return a.filesystem.CopyFile(source, destination, recursive)
}

func (s *FilesystemService) CopyFile(source, destination string, recursive bool) (*OperationResult, error) {
	op, err := copyFileRaw(source, destination, recursive)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := s.runElevatedWithEvents("copy", s.elevationTargetPair(source, destination), elevatedRequest{
		Op:          "copy",
		Source:      source,
		Destination: destination,
		Recursive:   recursive,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.Operation, nil
}

func copyFileRaw(source, destination string, recursive bool) (*OperationResult, error) {
	return rawFilesystemOps.CopyFile(source, destination, recursive)
}

// ============================================================================
// Search
// ============================================================================

func (a *App) SearchFiles(path, pattern string, recursive bool) (*SearchResult, error) {
	return a.filesystem.SearchFiles(path, pattern, recursive)
}

func (s *FilesystemService) SearchFiles(path, pattern string, recursive bool) (*SearchResult, error) {
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
	return internalfilesystem.BuildFileInfo(rawFilesystemMetadataCallbacks(), name, fullPath, info)
}

func buildFileInfoWithHidden(name, fullPath string, info os.FileInfo, hidden bool) FileInfo {
	return internalfilesystem.BuildFileInfoWithHidden(rawFilesystemMetadataCallbacks(), name, fullPath, info, hidden)
}

func isHidden(name, dir string) bool {
	return isHiddenFile(name, filepath.Join(dir, name))
}

func copyPath(src, dst string) error {
	return rawFilesystemOps.CopyPath(src, dst)
}

// isHiddenFile is the cross-platform hidden check stub —
// platform-specific logic lives in platform_{windows,unix}.go
func isHiddenFile(name, path string) bool {
	return internalplatform.CheckHidden(name, path)
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

func buildFallbackLinkInfo(name, fullPath string, entry fs.DirEntry, hidden bool) (FileInfo, bool) {
	return internalfilesystem.BuildFallbackLinkInfo(rawFilesystemMetadataCallbacks(), name, fullPath, entry, hidden)
}

type elevationEvent struct {
	RequestID string `json:"requestId,omitempty"`
	Phase     string `json:"phase"`
	Operation string `json:"operation"`
	Target    string `json:"target"`
	Message   string `json:"message,omitempty"`
	Success   bool   `json:"success,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *FilesystemService) emitElevationRequested(requestID, op, target string) {
	s.emit("elevation:requested", elevationEvent{
		RequestID: requestID,
		Phase:     "requested",
		Operation: op,
		Target:    target,
		Message:   "Administrative privileges required. Confirm to show OS elevation prompt.",
	})
}

func (s *FilesystemService) emitElevationResult(requestID, op, target string, success bool, err error) {
	evt := elevationEvent{
		RequestID: requestID,
		Phase:     "result",
		Operation: op,
		Target:    target,
		Success:   success,
	}
	if err != nil {
		evt.Error = err.Error()
		evt.Message = "Elevation failed or was cancelled."
	} else {
		evt.Message = "Elevated operation completed."
	}
	s.emit("elevation:result", evt)
}

func (s *FilesystemService) runElevatedWithEvents(op, target string, req elevatedRequest) (elevatedResponse, error) {
	// ── Tier 1: process is already admin/root ────────────────────────────────
	if isProcessElevated() {
		return executeElevatedRequest(req), nil
	}

	// ── Tier 2: route through the persistent elevated service session ─────────
	// If a session is already alive, dispatch immediately.
	// If not, try to auto-start one now — this shows exactly ONE UAC/sudo
	// prompt and then all future operations reuse the same elevated channel.
	svc := s.elevation.Client()

	if svc == nil || !svc.Alive() {
		// Serialize startup via StartElevatedSession so concurrent denied
		// operations do not trigger multiple overlapping UAC prompts.
		s.emit("elevation:session-starting", map[string]string{"op": op, "target": target})
		if status, startErr := s.startElevatedSession(); startErr == nil {
			s.emit("elevation:session-started", status)
			svc = s.elevation.Client()
		}
		// If start failed (e.g. user clicked "No") svc stays nil and we
		// fall through to the per-op dialog below.
	}

	if svc != nil && svc.Alive() {
		resp, err := svc.Dispatch(req)
		if err != nil {
			s.emit("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
			return elevatedResponse{}, err
		}
		s.emit("elevation:auto-started", map[string]string{"op": op, "target": target})
		return resp, nil
	}

	// ── Tier 3: service unavailable — direct per-op elevation fallback ───────
	// Do not show an in-app confirmation dialog; let OS UAC prompt directly.
	s.emit("elevation:uac-requested", map[string]string{
		"phase":     "requested",
		"operation": op,
		"target":    target,
	})
	resp, err := runElevatedRequest(req)
	if err != nil {
		s.emit("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
		return elevatedResponse{}, err
	}
	s.emit("elevation:auto-started", map[string]string{"op": op, "target": target})
	return resp, nil
}
