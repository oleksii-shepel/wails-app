package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ============================================================================
// List Directory
// ============================================================================

func (a *App) ListDirectory(path string, showHidden bool) (*DirectoryListing, error) {
	listing, err := listDirectoryRaw(path, showHidden)
	if err == nil {
		return listing, nil
	}
	if !shouldRetryDeleteWithElevation(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("list", opTargetSingle(path), elevatedRequest{
		Op:         "list",
		Path:       path,
		ShowHidden: showHidden,
	})
	if elevErr != nil {
		return nil, err
	}
	return resp.DirListing, nil
}

func listDirectoryRaw(path string, showHidden bool) (*DirectoryListing, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		// For Windows junctions/reparse-points (e.g. "Documents and Settings"),
		// os.ReadDir may fail with access denied on the junction path itself even
		// though the target directory is readable. Try resolving the reparse point
		// and retrying on the real path.
		if resolved, rerr := os.Readlink(path); rerr == nil {
			if entries2, rerr2 := os.ReadDir(resolved); rerr2 == nil {
				entries = entries2
				path = toSlash(resolved)
				err = nil
			}
		}
		if err != nil {
			return nil, err
		}
	}

	var files []FileInfo
	var totalSize int64

	for _, entry := range entries {
		name := entry.Name()
		fullPath := toSlash(filepath.Join(path, name))
		hidden := isHiddenFile(name, fullPath)

		// Hidden file filtering
		if !showHidden && hidden {
			continue
		}

		// Use Lstat so symlinks (including broken/inaccessible targets) still
		// appear in listings; DirEntry.Info can fail for those and hide them.
		info, err := os.Lstat(fullPath)
		if err != nil {
			if fallback, ok := buildFallbackLinkInfo(name, fullPath, entry, hidden); ok {
				files = append(files, fallback)
				if !fallback.IsHidden || showHidden {
					totalSize += fallback.Size
				}
			}
			continue
		}

		fi := buildFileInfoWithHidden(name, fullPath, info, hidden)
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
	file, err := readFileRaw(path, encoding)
	if err == nil {
		return file, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("read", opTargetSingle(path), elevatedRequest{
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
	op, err := writeFileRaw(path, content, encoding)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("write", opTargetSingle(path), elevatedRequest{
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
	op, err := deleteFileRaw(path, recursive, useTrash)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("delete", opTargetSingle(path), elevatedRequest{
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
	op, err := createDirectoryRaw(path, recursive)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("mkdir", opTargetSingle(path), elevatedRequest{
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
	op, err := moveFileRaw(source, destination)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("move", opTargetPair(source, destination), elevatedRequest{
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
	op, err := copyFileRaw(source, destination, recursive)
	if err == nil {
		return op, nil
	}
	if !isPermissionDenied(err) {
		return nil, err
	}

	resp, elevErr := a.runElevatedWithEvents("copy", opTargetPair(source, destination), elevatedRequest{
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
	return buildFileInfoWithHidden(name, fullPath, info, isHiddenFile(name, fullPath))
}

func buildFileInfoWithHidden(name, fullPath string, info os.FileInfo, hidden bool) FileInfo {
	ft := FileTypeFile
	isSymlink := false
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		isSymlink = true
		// Use the platform helper so that protected Windows junctions (e.g.
		// "Documents and Settings") are correctly identified as directories
		// without needing traversal access.
		if symlinkPointsToDir(fullPath) {
			ft = FileTypeDirectory
		} else {
			ft = FileTypeSymlink
		}
	} else if info.IsDir() {
		ft = FileTypeDirectory
	} else if isArchive(name) {
		ft = FileTypeArchive
	}

	// Last-resort: some Windows junctions (e.g. "Documents and Settings")
	// are reported by Go's os.Lstat with IsDir()=false and ModeSymlink=0
	// because the reparse tag is not a plain symlink. GetFileAttributes still
	// reports FILE_ATTRIBUTE_DIRECTORY|FILE_ATTRIBUTE_REPARSE_POINT for them.
	// On Unix this is always a no-op (returns false immediately).
	if !info.IsDir() && mode&os.ModeSymlink == 0 && isUntrackedDirectoryJunction(fullPath) {
		ft = FileTypeDirectory
		isSymlink = true
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
		IsHidden:    hidden,
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

func buildFallbackLinkInfo(name, fullPath string, entry fs.DirEntry, hidden bool) (FileInfo, bool) {
	// Include inaccessible reparse points/symlinks instead of silently dropping them.
	mode := entry.Type()
	isSymlink := mode&os.ModeSymlink != 0
	isJunction := isUntrackedDirectoryJunction(fullPath)
	isLinkLike := isSymlink || isJunction
	if !isLinkLike {
		return FileInfo{}, false
	}

	ft := FileTypeSymlink
	if entry.IsDir() || isJunction || (isSymlink && symlinkPointsToDir(fullPath)) {
		ft = FileTypeDirectory
	}

	now := time.Now().Unix()
	return FileInfo{
		Name:        name,
		Path:        fullPath,
		Type:        ft,
		Size:        0,
		Created:     now,
		Modified:    now,
		Accessed:    now,
		Permissions: "",
		IsHidden:    hidden,
		IsSymlink:   true,
	}, true
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

func (a *App) emitElevationRequested(requestID, op, target string) {
	a.emitEvent("elevation:requested", elevationEvent{
		RequestID: requestID,
		Phase:     "requested",
		Operation: op,
		Target:    target,
		Message:   "Administrative privileges required. Confirm to show OS elevation prompt.",
	})
}

func (a *App) emitElevationResult(requestID, op, target string, success bool, err error) {
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
	a.emitEvent("elevation:result", evt)
}

func (a *App) runElevatedWithEvents(op, target string, req elevatedRequest) (elevatedResponse, error) {
	// ── Tier 1: process is already admin/root ────────────────────────────────
	if isProcessElevated() {
		return executeElevatedRequest(req), nil
	}

	// ── Tier 2: route through the persistent elevated service session ─────────
	// If a session is already alive, dispatch immediately.
	// If not, try to auto-start one now — this shows exactly ONE UAC/sudo
	// prompt and then all future operations reuse the same elevated channel.
	a.elevMu.Lock()
	svc := a.elevSvc
	a.elevMu.Unlock()

	if svc == nil || !svc.Alive() {
		// Serialize startup via StartElevatedSession so concurrent denied
		// operations do not trigger multiple overlapping UAC prompts.
		a.emitEvent("elevation:session-starting", map[string]string{"op": op, "target": target})
		if status, startErr := a.StartElevatedSession(); startErr == nil {
			a.emitEvent("elevation:session-started", status)
			a.elevMu.Lock()
			svc = a.elevSvc
			a.elevMu.Unlock()
		}
		// If start failed (e.g. user clicked "No") svc stays nil and we
		// fall through to the per-op dialog below.
	}

	if svc != nil && svc.Alive() {
		resp, err := svc.Dispatch(req)
		if err != nil {
			a.emitEvent("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
			return elevatedResponse{}, err
		}
		a.emitEvent("elevation:auto-started", map[string]string{"op": op, "target": target})
		return resp, nil
	}

	// ── Tier 3: service unavailable — direct per-op elevation fallback ───────
	// Do not show an in-app confirmation dialog; let OS UAC prompt directly.
	a.emitEvent("elevation:uac-requested", map[string]string{
		"phase":     "requested",
		"operation": op,
		"target":    target,
	})
	resp, err := runElevatedRequest(req)
	if err != nil {
		a.emitEvent("elevation:auto-failed", map[string]interface{}{"op": op, "target": target, "error": err.Error()})
		return elevatedResponse{}, err
	}
	a.emitEvent("elevation:auto-started", map[string]string{"op": op, "target": target})
	return resp, nil
}

func opTargetSingle(path string) string {
	return path
}

func opTargetPair(src, dst string) string {
	return fmt.Sprintf("%s -> %s", src, dst)
}
