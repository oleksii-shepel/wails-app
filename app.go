package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	vfslib "wails-app/vfs"
	vfsarchive "wails-app/vfs/archive"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	nav        *NavigationManager
	workspaces *WorkspaceManager
	operations *OperationManager
	dirWatch   *PaneDirWatchManager
	elevMu     sync.Mutex
	elevWait   map[string]chan bool
	elevSvc    *ElevatedServiceClient // persistent JWT-authenticated elevated IPC channel
	streamMu   sync.Mutex
	dirStreams map[string]context.CancelFunc
	vfs        *vfslib.Unified
}

// NewApp creates a new App application struct.
func NewApp() *App {
	wm, err := NewWorkspaceManager()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
		wm = &WorkspaceManager{
			workspaces:  make(map[string]WorkspaceConfig),
			preferences: defaultPreferences(),
		}
	}

	return &App{
		nav:        NewNavigationManager(),
		workspaces: wm,
		operations: NewOperationManager(),
		elevWait:   make(map[string]chan bool),
		dirStreams: make(map[string]context.CancelFunc),
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Wire the App reference into the operation manager so it can emit
	// Wails events. This replaces the old package-level appInstance global.
	a.operations.SetApp(a)
	a.dirWatch = NewPaneDirWatchManager(a)
	a.initVFS()

	// Listen for native Wails file drop events from the OS
	runtime.EventsOn(ctx, "wails:file-drop", func(optionalData ...interface{}) {
		fmt.Println("[wails:file-drop] Backend listener fired!")
		if len(optionalData) > 0 {
			// Extract the data map sent by Wails
			if data, ok := optionalData[0].(map[string]interface{}); ok {
				fmt.Printf("[wails:file-drop] Native event received: %+v\n", data)

				// Extract x, y coordinates
				x := 0
				y := 0
				if xVal, ok := data["x"].(float64); ok {
					x = int(xVal)
				}
				if yVal, ok := data["y"].(float64); ok {
					y = int(yVal)
				}

				// Extract paths array
				paths := []string{}
				if pathsVal, ok := data["paths"].([]interface{}); ok {
					for _, p := range pathsVal {
						if str, ok := p.(string); ok {
							paths = append(paths, str)
						}
					}
				}

				fmt.Printf("[wails:file-drop] x=%d, y=%d, paths=%v\n", x, y, paths)

				// Process the file drop
				err := a.HandleFileDrop(x, y, paths)
				if err != nil {
					fmt.Printf("[wails:file-drop] HandleFileDrop error: %v\n", err)
				}
			}
		}
	})
}

func (a *App) shutdown(ctx context.Context) {
	if a.dirWatch != nil {
		a.dirWatch.Close()
	}
	a.cancelAllDirStreams()
	a.elevMu.Lock()
	svc := a.elevSvc
	a.elevSvc = nil
	a.elevMu.Unlock()
	if svc != nil {
		svc.Stop()
	}
}

// ============================================================================
// Drives
// ============================================================================

func (a *App) GetDrives() DrivesList {
	return a.ListDrives()
}

// ============================================================================
// File System
// ============================================================================

func (a *App) ListDir(path string, showHidden bool) (*DirectoryListing, error) {
	return a.ListDirectory(path, showHidden)
}

func (a *App) ListDirFiltered(path string, showHidden bool, filter FilterQuery) (*DirectoryListing, error) {
	listing, err := a.ListDirectory(path, showHidden)
	if err != nil {
		return nil, err
	}
	listing.Entries = filterEntries(listing.Entries, filter)
	return listing, nil
}

func (a *App) FileInfo(path string) (*FileInfo, error) {
	return a.GetFileInfo(path)
}

func (a *App) ReadFileContent(path string, encoding string) (*FileContent, error) {
	return a.ReadFile(path, encoding)
}

func (a *App) WriteFileContent(path string, content string, encoding string) (*OperationResult, error) {
	return a.WriteFile(path, content, encoding)
}

func (a *App) Delete(path string, recursive bool, useTrash *bool) (*OperationResult, error) {
	return a.DeleteFile(path, recursive, useTrash)
}

func (a *App) CreateDir(path string, recursive bool) (*OperationResult, error) {
	return a.CreateDirectory(path, recursive)
}

func (a *App) Move(source, destination string) (*OperationResult, error) {
	return a.MoveFile(source, destination)
}

func (a *App) Copy(source, destination string, recursive bool) (*OperationResult, error) {
	return a.CopyFile(source, destination, recursive)
}

func (a *App) Search(path, pattern string, recursive bool) (*SearchResult, error) {
	return a.SearchFiles(path, pattern, recursive)
}

// ============================================================================
// Progress Operations API (exposed to frontend)
// ============================================================================

func (a *App) GetActiveOperations() []OperationProgress {
	return a.operations.ListActiveOperations()
}

func (a *App) CancelOperation(id string) bool {
	return a.operations.CancelOperation(id)
}

func (a *App) DismissOperation(id string) {
	a.operations.RemoveOperation(id)
}

// ============================================================================
// Archive traversal  (treat archives like directories, no temp files)
// ============================================================================

// ArchiveBrowse routes ALL supported archive formats through the VFS handler
// registry (zip, tar/tar.gz/tar.bz2, rar, 7z). The handler is selected by
// archive.ForPath() based on the innermost name in the chain, so the calling
// convention from the frontend is unchanged.
func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	return a.vfsBrowseArchive(fsPath, nestedChain, innerPath)
}

// vfsBrowseArchive delegates archive listing to whichever VFS handler is
// registered for the given path (zip, tar, rar, 7z, …).
// It converts vfs.FileInfo entries back into the app's wire FileInfo type so
// the rest of the frontend/backend pipeline is completely unchanged.
func (a *App) vfsBrowseArchive(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	ctx := context.Background()

	// The innermost name in the chain determines the handler.
	handlerPath := fsPath
	if len(nestedChain) > 0 {
		handlerPath = nestedChain[len(nestedChain)-1]
	}

	h, ok := vfsarchive.ForPath(handlerPath)
	if !ok {
		return ArchiveListing{}, fmt.Errorf("vfs: no archive handler for %q", handlerPath)
	}

	arch, err := h.Open(ctx, fsPath, vfsarchive.OpenOptions{
		ReadOnly:    true,
		NestedChain: nestedChain,
	})
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("vfs: open %q: %w", fsPath, err)
	}
	defer arch.Close()

	vfsEntries, err := arch.ReadDir(ctx, normalize(innerPath))
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("vfs: readdir %q in %q: %w", innerPath, fsPath, err)
	}

	entries := make([]FileInfo, 0, len(vfsEntries))
	for _, e := range vfsEntries {
		entries = append(entries, vfsEntryToFileInfo(e, nestedChain))
	}

	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: nestedChain,
		Entries:     entries,
	}, nil
}

// vfsEntryToFileInfo converts a vfs.FileInfo (from the archive provider) into
// the app's wire FileInfo type, preserving the NestedChain that tells the
// frontend which chain to pass back when opening a nested archive.
func vfsEntryToFileInfo(e vfslib.FileInfo, parentChain []string) FileInfo {
	ft := FileTypeFile
	switch {
	case e.IsDir():
		ft = FileTypeDirectory
	case e.IsArchive():
		ft = FileTypeArchive
	case e.IsSymlink():
		ft = FileTypeSymlink
	}

	var nestedChain []string
	if ft == FileTypeArchive {
		// Build the chain the frontend will pass when opening this nested archive:
		// the parent's chain + this entry's path within the archive.
		nestedChain = append(append([]string{}, parentChain...), e.Path())
	}

	return FileInfo{
		Name:        e.Name(),
		Path:        e.Path(),
		Type:        ft,
		Size:        e.Size(),
		Modified:    e.ModTime().UnixMilli(),
		IsHidden:    e.IsHidden(),
		IsSymlink:   e.IsSymlink(),
		IsNested:    ft == FileTypeArchive,
		NestedChain: nestedChain,
	}
}

func (a *App) ArchiveRead(fsPath string, nestedChain []string, entryPath string) (*FileContent, error) {
	return a.ArchiveReadFile(fsPath, nestedChain, entryPath)
}

// ============================================================================
// Navigation
// ============================================================================

func (a *App) NavNavigate(paneID, path string) (*DirectoryListing, error) {
	a.nav.Navigate(paneID, path)
	return a.ListDirectory(path, false)
}

func (a *App) NavNavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error) {
	a.nav.Navigate(paneID, path)
	return a.ListDirectory(path, showHidden)
}

func (a *App) NavBack(paneID string) (*DirectoryListing, error) {
	path := a.nav.GoBack(paneID)
	if path == "" {
		return nil, fmt.Errorf("no previous location for pane %q", paneID)
	}
	return a.ListDirectory(path, false)
}

func (a *App) NavForward(paneID string) (*DirectoryListing, error) {
	path := a.nav.GoForward(paneID)
	if path == "" {
		return nil, fmt.Errorf("no forward location for pane %q", paneID)
	}
	return a.ListDirectory(path, false)
}

func (a *App) NavGoUp(paneID string) (*DirectoryListing, error) {
	current := a.nav.CurrentPath(paneID)
	if current == "" {
		return nil, fmt.Errorf("no current path for pane %q", paneID)
	}
	parent := toSlash(filepath.Dir(current))
	if parent == toSlash(current) {
		return nil, fmt.Errorf("already at root")
	}
	return a.NavNavigate(paneID, parent)
}

// ============================================================================
// Workspaces & Preferences
// ============================================================================

func (a *App) GetWorkspaces() WorkspaceList           { return a.workspaces.ListWorkspaces() }
func (a *App) SaveWorkspace(ws WorkspaceConfig) error { return a.workspaces.SaveWorkspace(ws) }
func (a *App) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return a.workspaces.LoadWorkspace(id)
}
func (a *App) DeleteWorkspace(id string) error { return a.workspaces.DeleteWorkspace(id) }
func (a *App) GetPreferences() UserPreferences { return a.workspaces.GetPreferences() }
func (a *App) SavePreferences(prefs UserPreferences) error {
	return a.workspaces.SavePreferences(prefs)
}

// ============================================================================
// Utility
// ============================================================================

func (a *App) Ping() int64 {
	return time.Now().UnixMilli()
}

// OpenFile opens the given path with the OS default application.
func (a *App) OpenFile(path string) error {
	return openFileWithDefaultApp(path)
}

// ResolvedLink is the result returned by ResolveLink.
type ResolvedLink struct {
	Target string `json:"target"`
	IsDir  bool   `json:"isDir"`
}

// ResolveLink resolves a Shell Link (.lnk on Windows) or a Unix symlink and
// returns the real target path plus whether the target is a directory.
func (a *App) ResolveLink(path string) (*ResolvedLink, error) {
	target, isDir, err := resolveLinkPath(path)
	if err != nil {
		return nil, err
	}
	return &ResolvedLink{Target: target, IsDir: isDir}, nil
}

func (a *App) SetPaneDirectory(paneID, path string) error {
	if a.dirWatch == nil {
		return nil
	}
	return a.dirWatch.SetPaneDirectory(paneID, path)
}

func (a *App) StartListDirStream(paneID, path string, showHidden bool) (string, error) {
	return a.startListDirectoryStream(paneID, path, showHidden)
}

func (a *App) CancelListDirStream(streamID string) {
	a.cancelDirStream(streamID)
}

func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

func (a *App) registerDirStream(streamID string, cancel context.CancelFunc) {
	a.streamMu.Lock()
	a.dirStreams[streamID] = cancel
	a.streamMu.Unlock()
}

func (a *App) unregisterDirStream(streamID string) {
	a.streamMu.Lock()
	delete(a.dirStreams, streamID)
	a.streamMu.Unlock()
}

func (a *App) cancelDirStream(streamID string) {
	if streamID == "" {
		return
	}
	a.streamMu.Lock()
	cancel := a.dirStreams[streamID]
	delete(a.dirStreams, streamID)
	a.streamMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) cancelAllDirStreams() {
	a.streamMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(a.dirStreams))
	for id, cancel := range a.dirStreams {
		delete(a.dirStreams, id)
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	a.streamMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// ============================================================================
// Elevated service session (persistent JWT-secured IPC channel)
// ============================================================================

// StartElevatedSession launches the elevated helper process (one UAC/sudo
// prompt) and establishes a persistent JWT-authenticated TCP channel to it.
// All subsequent permission-denied operations are routed through this channel
// with no further OS prompts.
func (a *App) StartElevatedSession() (*ElevatedSessionStatus, error) {
	a.elevMu.Lock()
	defer a.elevMu.Unlock()

	if a.elevSvc != nil && a.elevSvc.Alive() {
		status := a.elevSvc.Status()
		a.emitEvent("elevation:session-started", status)
		return &status, nil
	}

	exePath, err := executablePath()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}

	a.emitEvent("elevation:uac-requested", map[string]string{
		"phase":     "requested",
		"operation": "session-start",
		"target":    "elevated-service",
		"message":   "Administrative privileges requested to start elevated service session.",
	})

	svc, err := StartElevatedServiceClient(exePath)
	if err != nil {
		return nil, err
	}
	a.elevSvc = svc
	status := svc.Status()
	a.emitEvent("elevation:session-started", status)
	return &status, nil
}

// StopElevatedSession closes the elevated IPC channel and terminates the
// helper process. Future permission-denied operations will fall back to the
// interactive per-op UAC dialog.
func (a *App) StopElevatedSession() {
	a.elevMu.Lock()
	svc := a.elevSvc
	a.elevSvc = nil
	a.elevMu.Unlock()
	if svc != nil {
		svc.Stop()
	}
	a.emitEvent("elevation:session-stopped", map[string]bool{"active": false})
}

// GetElevatedSessionStatus returns the live status of the elevated service
// channel without starting or stopping it.
func (a *App) GetElevatedSessionStatus() ElevatedSessionStatus {
	a.elevMu.Lock()
	svc := a.elevSvc
	a.elevMu.Unlock()
	if svc == nil {
		return ElevatedSessionStatus{}
	}
	return svc.Status()
}

// ============================================================================
// IsProcessElevated returns true when the current process is running with
// administrator / root privileges.
func (a *App) IsProcessElevated() bool {
	return isProcessElevated()
}

func (a *App) ConfirmElevationRequest(requestID string, approve bool) {
	a.elevMu.Lock()
	ch, ok := a.elevWait[requestID]
	if ok {
		delete(a.elevWait, requestID)
	}
	a.elevMu.Unlock()

	if ok {
		select {
		case ch <- approve:
		default:
		}
	}
}

func (a *App) waitForElevationApproval(requestID string, timeout time.Duration) (bool, error) {
	ch := make(chan bool, 1)
	a.elevMu.Lock()
	a.elevWait[requestID] = ch
	a.elevMu.Unlock()

	defer func() {
		a.elevMu.Lock()
		delete(a.elevWait, requestID)
		a.elevMu.Unlock()
	}()

	select {
	case approve := <-ch:
		return approve, nil
	case <-time.After(timeout):
		return false, fmt.Errorf("elevation approval timed out")
	}
}

// executablePath returns the absolute path of the running binary.
func executablePath() (string, error) {
	return os.Executable()
}

func newElevationRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("elev-%d", time.Now().UnixNano())
	}
	return "elev-" + base64.RawURLEncoding.EncodeToString(b)
}

// HandleFileDrop processes files dropped onto the app and emits an event.
// This is called by the frontend when files are dropped.
// If browser doesn't provide filesystem paths, it attempts to read from Windows clipboard.
func (a *App) HandleFileDrop(x int, y int, paths []string) error {
	fmt.Printf("[HandleFileDrop] Called with x=%d, y=%d, %d paths: %v\n", x, y, len(paths), paths)

	if len(paths) == 0 {
		return fmt.Errorf("no paths provided")
	}

	providedPaths := paths

	// Check if paths are relative/filename-only (from browser drop)
	isRelativeOnly := false
	for _, p := range paths {
		if !isAbsolutePath(p) {
			isRelativeOnly = true
			fmt.Printf("[HandleFileDrop] Path is relative: %s\n", p)
			break
		}
	}

	// If paths are just filenames (no directory), try to get from clipboard
	if isRelativeOnly {
		fmt.Println("[HandleFileDrop] Detected filename-only paths, attempting clipboard fallback...")
		clipboardPaths, err := GetClipboardFilePaths()
		if err == nil && len(clipboardPaths) > 0 {
			fmt.Printf("[HandleFileDrop] ✓ Got %d paths from clipboard: %v\n", len(clipboardPaths), clipboardPaths)
			providedPaths = clipboardPaths
		} else {
			fmt.Println("[HandleFileDrop] ✗ Clipboard fallback failed:", err)
			// Try to use what we have anyway
			fmt.Printf("[HandleFileDrop] Continuing with provided %d paths\n", len(paths))
		}
	}

	// Validate each path and collect valid ones
	validPaths := []string{}
	for _, p := range providedPaths {
		if p == "" {
			continue
		}

		fmt.Printf("[HandleFileDrop] Checking path: %s\n", p)

		// Verify path exists
		if stat, err := os.Stat(p); err == nil {
			validPaths = append(validPaths, p)
			fmt.Printf("[HandleFileDrop] ✓ Path exists: %s (isDir=%v)\n", p, stat.IsDir())
		} else {
			fmt.Printf("[HandleFileDrop] ✗ Path does not exist: %s (%v)\n", p, err)
		}
	}

	if len(validPaths) == 0 {
		fmt.Println("[HandleFileDrop] ✗ No valid file paths after validation")
		return fmt.Errorf("no valid file paths after validation")
	}

	fmt.Printf("[HandleFileDrop] ✓ Emitting file:drop event with %d valid files\n", len(validPaths))

	// Emit a Wails event to the frontend with the drop data
	payload := map[string]interface{}{
		"x":     x,
		"y":     y,
		"paths": validPaths,
	}

	runtime.EventsEmit(a.ctx, "file:drop", payload)
	return nil
}

// isAbsolutePath checks if a path is absolute (not just a filename)
func isAbsolutePath(path string) bool {
	if len(path) == 0 {
		return false
	}
	// Windows absolute paths start with drive letter (C:) or UNC (\\)
	if len(path) > 1 && path[1] == ':' {
		return true
	}
	if len(path) > 1 && path[0] == '\\' && path[1] == '\\' {
		return true
	}
	// Unix paths start with /
	if path[0] == '/' {
		return true
	}
	return false
}
