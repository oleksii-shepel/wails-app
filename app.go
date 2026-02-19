package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	nav        *NavigationManager
	workspaces *WorkspaceManager
}

// NewApp creates a new App application struct.
func NewApp() *App {
	wm, err := NewWorkspaceManager()
	if err != nil {
		// Non-fatal: app still works, just without persistence
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
		wm = &WorkspaceManager{
			workspaces:  make(map[string]WorkspaceConfig),
			preferences: defaultPreferences(),
		}
	}

	return &App{
		nav:        NewNavigationManager(),
		workspaces: wm,
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ============================================================================
// OS Info
// ============================================================================

// GetOSInfo returns OS/platform information.
// Wails binding: window.go.GetOSInfo()
func (a *App) GetOSInfo_() OSInfoResponse {
	return a.GetOSInfo()
}

// ============================================================================
// Drives
// ============================================================================

// GetDrives returns all mounted drives/volumes.
// Wails binding: window.go.GetDrives()
func (a *App) GetDrives() DrivesList {
	return a.ListDrives()
}

// ============================================================================
// File System
// ============================================================================

// ListDir lists the contents of a directory.
// Wails binding: window.go.ListDir(path, showHidden)
func (a *App) ListDir(path string, showHidden bool) (*DirectoryListing, error) {
	return a.ListDirectory(path, showHidden)
}

// ListDirFiltered lists directory contents applying a filter.
// Wails binding: window.go.ListDirFiltered(path, showHidden, filter)
func (a *App) ListDirFiltered(path string, showHidden bool, filter FilterQuery) (*DirectoryListing, error) {
	listing, err := a.ListDirectory(path, showHidden)
	if err != nil {
		return nil, err
	}
	listing.Entries = filterEntries(listing.Entries, filter)
	return listing, nil
}

// FileInfo returns metadata for a single file or directory.
// Wails binding: window.go.FileInfo(path)
func (a *App) FileInfo(path string) (*FileInfo, error) {
	return a.GetFileInfo(path)
}

// ReadFileContent reads a file and returns its content.
// Wails binding: window.go.ReadFileContent(path, encoding)
func (a *App) ReadFileContent(path string, encoding string) (*FileContent, error) {
	return a.ReadFile(path, encoding)
}

// WriteFileContent writes content to a file.
// Wails binding: window.go.WriteFileContent(path, content, encoding)
func (a *App) WriteFileContent(path string, content string, encoding string) (*OperationResult, error) {
	return a.WriteFile(path, content, encoding)
}

// Delete deletes a file or directory.
// Wails binding: window.go.Delete(path, recursive)
func (a *App) Delete(path string, recursive bool) (*OperationResult, error) {
	return a.DeleteFile(path, recursive)
}

// CreateDir creates a directory.
// Wails binding: window.go.CreateDir(path, recursive)
func (a *App) CreateDir(path string, recursive bool) (*OperationResult, error) {
	return a.CreateDirectory(path, recursive)
}

// Move moves/renames a file or directory.
// Wails binding: window.go.Move(source, destination)
func (a *App) Move(source, destination string) (*OperationResult, error) {
	return a.MoveFile(source, destination)
}

// Copy copies a file or directory.
// Wails binding: window.go.Copy(source, destination, recursive)
func (a *App) Copy(source, destination string, recursive bool) (*OperationResult, error) {
	return a.CopyFile(source, destination, recursive)
}

// Search searches for files matching a pattern.
// Wails binding: window.go.Search(path, pattern, recursive)
func (a *App) Search(path, pattern string, recursive bool) (*SearchResult, error) {
	return a.SearchFiles(path, pattern, recursive)
}

// ============================================================================
// Navigation
// ============================================================================

// NavNavigate navigates a pane to a path (pushes history).
// Wails binding: window.go.NavNavigate(paneId, path)
func (a *App) NavNavigate(paneID, path string) (*DirectoryListing, error) {
	a.nav.Navigate(paneID, path)
	return a.ListDirectory(path, false)
}

// NavNavigateShowHidden navigates with control over hidden files.
// Wails binding: window.go.NavNavigateShowHidden(paneId, path, showHidden)
func (a *App) NavNavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error) {
	a.nav.Navigate(paneID, path)
	return a.ListDirectory(path, showHidden)
}

// NavBack navigates back and returns the new directory listing.
// Wails binding: window.go.NavBack(paneId)
func (a *App) NavBack(paneID string) (*DirectoryListing, error) {
	path := a.nav.GoBack(paneID)
	if path == "" {
		return nil, fmt.Errorf("no previous location for pane %q", paneID)
	}
	return a.ListDirectory(path, false)
}

// NavForward navigates forward and returns the new directory listing.
// Wails binding: window.go.NavForward(paneId)
func (a *App) NavForward(paneID string) (*DirectoryListing, error) {
	path := a.nav.GoForward(paneID)
	if path == "" {
		return nil, fmt.Errorf("no forward location for pane %q", paneID)
	}
	return a.ListDirectory(path, false)
}

// NavCanGoBack returns whether back navigation is possible.
// Wails binding: window.go.NavCanGoBack(paneId)
func (a *App) NavCanGoBack(paneID string) bool {
	return a.nav.CanGoBack(paneID)
}

// NavCanGoForward returns whether forward navigation is possible.
// Wails binding: window.go.NavCanGoForward(paneId)
func (a *App) NavCanGoForward(paneID string) bool {
	return a.nav.CanGoForward(paneID)
}

// NavCurrentPath returns the current path for a pane.
// Wails binding: window.go.NavCurrentPath(paneId)
func (a *App) NavCurrentPath(paneID string) string {
	return a.nav.CurrentPath(paneID)
}

// NavGetHistory returns the navigation history for a pane.
// Wails binding: window.go.NavGetHistory(paneId)
func (a *App) NavGetHistory(paneID string) NavigationHistory {
	return a.nav.GetHistory(paneID)
}

// NavClearHistory clears the navigation history for a pane.
// Wails binding: window.go.NavClearHistory(paneId)
func (a *App) NavClearHistory(paneID string) {
	a.nav.ClearHistory(paneID)
}

// NavGoUp navigates to the parent directory of the current path.
// Wails binding: window.go.NavGoUp(paneId)
func (a *App) NavGoUp(paneID string) (*DirectoryListing, error) {
	current := a.nav.CurrentPath(paneID)
	if current == "" {
		return nil, fmt.Errorf("no current path for pane %q", paneID)
	}
	parent := parentPath(current)
	if parent == current {
		return nil, fmt.Errorf("already at root")
	}
	return a.NavNavigate(paneID, parent)
}

// ============================================================================
// Workspaces
// ============================================================================

// GetWorkspaces lists all saved workspaces.
// Wails binding: window.go.GetWorkspaces()
func (a *App) GetWorkspaces() WorkspaceList {
	return a.workspaces.ListWorkspaces()
}

// SaveWorkspace saves or updates a workspace.
// Wails binding: window.go.SaveWorkspace(workspace)
func (a *App) SaveWorkspace(ws WorkspaceConfig) error {
	return a.workspaces.SaveWorkspace(ws)
}

// LoadWorkspace loads a workspace by ID and sets it as active.
// Wails binding: window.go.LoadWorkspace(id)
func (a *App) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return a.workspaces.LoadWorkspace(id)
}

// DeleteWorkspace deletes a workspace by ID.
// Wails binding: window.go.DeleteWorkspace(id)
func (a *App) DeleteWorkspace(id string) error {
	return a.workspaces.DeleteWorkspace(id)
}

// SetActiveWorkspace sets the active workspace.
// Wails binding: window.go.SetActiveWorkspace(id)
func (a *App) SetActiveWorkspace(id string) error {
	return a.workspaces.SetActiveWorkspace(id)
}

// ============================================================================
// Preferences
// ============================================================================

// GetPreferences returns the user preferences.
// Wails binding: window.go.GetPreferences()
func (a *App) GetPreferences() UserPreferences {
	return a.workspaces.GetPreferences()
}

// SavePreferences saves the user preferences.
// Wails binding: window.go.SavePreferences(prefs)
func (a *App) SavePreferences(prefs UserPreferences) error {
	return a.workspaces.SavePreferences(prefs)
}

// ============================================================================
// Utility
// ============================================================================

// Ping returns a timestamp — useful for health checks and latency measurement.
// Wails binding: window.go.Ping()
func (a *App) Ping() int64 {
	return time.Now().UnixMilli()
}

// EmitEvent is a helper to broadcast events to the frontend.
func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

// parentPath returns the parent directory, handling both Unix and Windows roots.
func parentPath(path string) string {
	parent := cleanPath(path + "/..")
	if parent == path {
		return path // already at root
	}
	return parent
}
