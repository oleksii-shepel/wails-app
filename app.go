package main

import (
	"context"
	"fmt"

	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	archive    *ArchiveService
	runtime    *AppRuntimeService
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
		runtime:    NewAppRuntimeService(),
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
	a.runtime.Startup(a, ctx)
}

func (a *App) shutdown(_ context.Context) {
	a.runtime.Shutdown(a)
}

func (a *App) emitEvent(name string, data interface{}) {
	a.runtime.EmitEvent(a.ctx, name, data)
}
