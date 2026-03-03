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
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Wire the App reference into the operation manager so it can emit
	// Wails events. This replaces the old package-level appInstance global.
	a.operations.SetApp(a)
	a.dirWatch = NewPaneDirWatchManager(a)
}

func (a *App) shutdown(ctx context.Context) {
	if a.dirWatch != nil {
		a.dirWatch.Close()
	}
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

func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	if len(nestedChain) == 0 {
		return a.ArchiveList(fsPath, innerPath)
	}
	return a.ArchiveListNested(fsPath, nestedChain, innerPath)
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

func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
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
