package main

import (
	"context"
	"fmt"

	internalapp "wails-app/internal/app"
	internalarchive "wails-app/internal/archive"
	internalbatch "wails-app/internal/batch"
	internalelevation "wails-app/internal/elevation"
	internalnav "wails-app/internal/navigation"
	internalruntime "wails-app/internal/runtime"
	internalutility "wails-app/internal/utility"
	internalworkspace "wails-app/internal/workspace"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	archive    *internalarchive.Service
	runtime    *internalapp.RuntimeService
	filesystem appFilesystem
	batch      *internalbatch.Service
	dirStream  *internalruntime.DirStreamService
	utility    *internalutility.Service
	navigation *internalnav.Service
	workspace  *internalworkspace.Service
	operations appOperations
	dirWatch   *internalruntime.PaneWatchManager
	elevation  *internalelevation.Service
	vfs        *vfslib.Unified
	archCache  *cache.Cache // lazy-open, LRU cache of open archive.Archive objects
}

// NewApp creates a new App application struct.
func NewApp() *App {
	core, err := internalapp.NewCore()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
	}

	app := &App{
		archive:    nil,
		operations: core.Operations,
		archCache:  core.ArchiveCache,
	}
	app.runtime = internalapp.NewRuntimeService(internalapp.RuntimeConfig{
		SetContext: func(ctx context.Context) {
			app.ctx = ctx
		},
		SetOperationEmitter:  app.operations.SetEmitter,
		CreatePaneWatch:      app.createPaneWatch,
		InitVFS:              app.initVFS,
		HandleFileDrop:       app.HandleFileDrop,
		ClosePaneWatch:       app.closePaneWatch,
		CancelDirStreams:     app.cancelDirStreams,
		StopElevationSession: app.stopElevationSession,
		CloseArchiveCache:    app.closeArchiveCache,
	})
	app.elevation = internalelevation.NewService(app.emitEvent, app.startElevatedClient)
	app.filesystem = NewFilesystemService(app.elevation, app.emitEvent, app.StartElevatedSession)
	services := internalapp.NewServices(internalapp.ServiceConfig{
		Core:                           core,
		EmitEvent:                      app.emitEvent,
		StartElevatedClient:            app.startElevatedClient,
		ElevatedList:                   app.filesystem.ElevatedList,
		BuildFileInfo:                  app.buildFileInfo,
		IsHidden:                       isHiddenFile,
		ListDirectory:                  app.filesystem.ListDirectory,
		StartOperation:                 app.startOperation,
		PromptConflict:                 app.promptConflict,
		ClassifyError:                  classifyError,
		IsPermissionDenied:             isPermissionDenied,
		ShouldRetryDeleteWithElevation: shouldRetryDeleteWithElevation,
		RunElevatedCopy:                app.filesystem.RunElevatedCopy,
		RunElevatedMove:                app.filesystem.RunElevatedMove,
		RunElevatedDelete:              app.filesystem.RunElevatedDelete,
		CalculateSize:                  calculateSize,
		ExtractNestedArchive:           app.extractNestedArchive,
		Normalize:                      normalize,
		ArchiveEntryPath:               archiveEntryPath,
		IsArchive:                      isArchive,
	})
	app.elevation = services.Elevation
	app.archive = services.Archive
	app.batch = services.Batch
	app.dirStream = services.DirStream
	app.utility = services.Utility
	app.navigation = services.Navigation
	app.workspace = services.Workspace
	return app
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.runtime.Startup(ctx)
}

func (a *App) shutdown(_ context.Context) {
	a.runtime.Shutdown()
}

func (a *App) emitEvent(name string, data interface{}) {
	a.runtime.EmitEvent(a.ctx, name, data)
}
