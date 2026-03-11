package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	archive    *ArchiveService
	filesystem *FilesystemService
	batch      *BatchService
	dirStream  *DirStreamService
	utility    *UtilityService
	navigation *NavigationService
	workspace  *WorkspaceService
	operations *OperationManager
	dirWatch   *PaneDirWatchManager
	elevation  *ElevationSessionService
	vfs        *vfslib.Unified
	archCache  *cache.Cache // lazy-open, LRU cache of open archive.Archive objects
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

	navManager := NewNavigationManager()

	app := &App{
		archive:    nil,
		operations: NewOperationManager(),
		archCache:  cache.New(resolveChain),
	}
	app.elevation = NewElevationSessionService(app.emitEvent)
	app.filesystem = NewFilesystemService(app.elevation, app.emitEvent, app.StartElevatedSession)
	app.dirStream = NewDirStreamService(app.emitEvent, app.filesystem.runElevatedWithEvents)
	app.utility = NewUtilityService(app.emitEvent)
	app.batch = NewBatchService(
		app.operations,
		app.emitEvent,
		app.filesystem.runElevatedWithEvents,
		app.extractNestedArchive,
	)
	app.navigation = NewNavigationService(navManager, app.filesystem.ListDirectory)
	app.workspace = NewWorkspaceService(wm)
	app.archive = NewArchiveService(app)
	return app
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
	a.dirStream.CancelAll()
	if a.elevation != nil {
		a.elevation.StopSession()
	}
	a.archCache.CloseAll()
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
// vfsBrowseArchive delegates archive listing to whichever VFS handler is
// registered for the given path (zip, tar, rar, 7z, …).
// Archives are opened once and kept alive in archCache; subsequent calls for
// any innerPath within the same (fsPath, nestedChain) identity are instant —
// no extraction, no temp files, no re-parse of the archive header.
func (a *App) vfsBrowseArchive(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	ctx := context.Background()

	// Retrieve (or lazily open) the archive from the LRU cache.
	arch, err := a.archCache.Get(ctx, fsPath, nestedChain, "")
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("vfs: open %q: %w", fsPath, err)
	}
	// NOTE: do NOT defer arch.Close() — the cache owns the lifetime.

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

// ============================================================================
// Navigation
// ============================================================================

func (a *App) NavNavigate(paneID, path string) (*DirectoryListing, error) {
	return a.navigation.Navigate(paneID, path)
}

func (a *App) NavNavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error) {
	return a.navigation.NavigateShowHidden(paneID, path, showHidden)
}

func (a *App) NavBack(paneID string) (*DirectoryListing, error) {
	return a.navigation.Back(paneID)
}

func (a *App) NavForward(paneID string) (*DirectoryListing, error) {
	return a.navigation.Forward(paneID)
}

func (a *App) NavGoUp(paneID string) (*DirectoryListing, error) {
	return a.navigation.GoUp(paneID)
}

// ============================================================================
// Workspaces & Preferences
// ============================================================================

func (a *App) GetWorkspaces() WorkspaceList           { return a.workspace.GetWorkspaces() }
func (a *App) SaveWorkspace(ws WorkspaceConfig) error { return a.workspace.SaveWorkspace(ws) }
func (a *App) LoadWorkspace(id string) (*WorkspaceConfig, error) {
	return a.workspace.LoadWorkspace(id)
}
func (a *App) DeleteWorkspace(id string) error { return a.workspace.DeleteWorkspace(id) }
func (a *App) GetPreferences() UserPreferences { return a.workspace.GetPreferences() }
func (a *App) SavePreferences(prefs UserPreferences) error {
	return a.workspace.SavePreferences(prefs)
}

// ============================================================================
// Utility
// ============================================================================

func (a *App) Ping() int64 {
	return time.Now().UnixMilli()
}

func (a *App) OpenFile(path string) error {
	return a.utility.OpenFile(path)
}

func (a *App) ResolveLink(path string) (*ResolvedLink, error) {
	return a.utility.ResolveLink(path)
}

func (a *App) SetPaneDirectory(paneID, path string) error {
	if a.dirWatch == nil {
		return nil
	}
	return a.dirWatch.SetPaneDirectory(paneID, path)
}

func (a *App) StartListDirStream(paneID, path string, showHidden bool) (string, error) {
	return a.dirStream.Start(paneID, path, showHidden)
}

func (a *App) CancelListDirStream(streamID string) {
	a.dirStream.Cancel(streamID)
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
	return a.elevation.StartSession()
}

// StopElevatedSession closes the elevated IPC channel and terminates the
// helper process. Future permission-denied operations will fall back to the
// interactive per-op UAC dialog.
func (a *App) StopElevatedSession() {
	a.elevation.StopSession()
}

// GetElevatedSessionStatus returns the live status of the elevated service
// channel without starting or stopping it.
func (a *App) GetElevatedSessionStatus() ElevatedSessionStatus {
	return a.elevation.Status()
}

// ============================================================================
// IsProcessElevated returns true when the current process is running with
// administrator / root privileges.
func (a *App) IsProcessElevated() bool {
	return isProcessElevated()
}

func (a *App) ConfirmElevationRequest(requestID string, approve bool) {
	a.elevation.Confirm(requestID, approve)
}

func (a *App) waitForElevationApproval(requestID string, timeout time.Duration) (bool, error) {
	return a.elevation.WaitForApproval(requestID, timeout)
}

func (a *App) HandleFileDrop(x int, y int, paths []string) error {
	return a.utility.HandleFileDrop(x, y, paths)
}
