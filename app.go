package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	nav        *NavigationManager
	workspaces *WorkspaceManager
	operations *OperationManager
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
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ============================================================================
// OS Info
// ============================================================================

func (a *App) GetOSInfo_() OSInfoResponse {
	return a.GetOSInfo()
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

func (a *App) Delete(path string, recursive bool) (*OperationResult, error) {
	return a.DeleteFile(path, recursive)
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

// ArchiveBrowse is the unified entry point the frontend calls whenever it
// "opens" something that is an ARCHIVE type entry.
//
//   - fsPath      – absolute path to the on-disk archive file.
//   - nestedChain – empty for a top-level archive; each element is the entry
//     name of an archive-inside-an-archive, in order from
//     outermost to innermost.
//   - innerPath   – the sub-directory inside the (innermost) archive to list,
//     e.g. "" for the root or "src/pkg" for a subdirectory.
func (a *App) ArchiveBrowse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	if len(nestedChain) == 0 {
		return a.ArchiveList(fsPath, innerPath)
	}
	return a.ArchiveListNested(fsPath, nestedChain, innerPath)
}

// ArchiveRead returns the contents of a single file inside an archive.
// Use the same fsPath / nestedChain as ArchiveBrowse, and set entryPath to
// the full inner path of the file you want to read.
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
	parent := filepath.Dir(current)
	if parent == current {
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

func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}
