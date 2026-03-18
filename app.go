package main

import (
	"context"
	"fmt"

	internalapp "wails-app/internal/app"
	internalelevation "wails-app/internal/elevation"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	*internalapp.State
}

// NewApp creates a new App application struct.
func NewApp() *App {
	core, err := internalapp.NewCore()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
	}

	app := &App{State: internalapp.NewState(core)}
	app.Runtime = internalapp.NewRuntimeService(internalapp.RuntimeConfig{
		SetContext: func(ctx context.Context) {
			app.Ctx = ctx
		},
		SetOperationEmitter:  app.Operations.SetEmitter,
		CreatePaneWatch:      app.createPaneWatch,
		InitVFS:              app.initVFS,
		HandleFileDrop:       app.HandleFileDrop,
		ClosePaneWatch:       app.closePaneWatch,
		CancelDirStreams:     app.cancelDirStreams,
		StopElevationSession: app.stopElevationSession,
		CloseArchiveCache:    app.closeArchiveCache,
	})
	elevation := internalelevation.NewService(app.emitEvent, app.startElevatedClient)
	app.Elevation = elevation
	app.Filesystem = NewFilesystemService(elevation, app.emitEvent, app.StartElevatedSession)
	services := internalapp.NewServices(internalapp.ServiceConfig{
		Core:                           core,
		EmitEvent:                      app.emitEvent,
		StartElevatedClient:            app.startElevatedClient,
		ElevatedList:                   app.Filesystem.ElevatedList,
		BuildFileInfo:                  app.buildFileInfo,
		IsHidden:                       isHiddenFile,
		ListDirectory:                  app.Filesystem.ListDirectory,
		StartOperation:                 app.startOperation,
		PromptConflict:                 app.promptConflict,
		ClassifyError:                  classifyError,
		IsPermissionDenied:             isPermissionDenied,
		ShouldRetryDeleteWithElevation: shouldRetryDeleteWithElevation,
		RunElevatedCopy:                app.Filesystem.RunElevatedCopy,
		RunElevatedMove:                app.Filesystem.RunElevatedMove,
		RunElevatedDelete:              app.Filesystem.RunElevatedDelete,
		CalculateSize:                  calculateSize,
		ExtractNestedArchive:           app.extractNestedArchive,
		Normalize:                      normalize,
		ArchiveEntryPath:               archiveEntryPath,
		IsArchive:                      isArchive,
	})
	app.Elevation = services.Elevation
	app.Archive = services.Archive
	app.Batch = services.Batch
	app.DirStream = services.DirStream
	app.Utility = services.Utility
	app.Navigation = services.Navigation
	app.Workspace = services.Workspace
	return app
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.Runtime.Startup(ctx)
}

func (a *App) shutdown(_ context.Context) {
	a.Runtime.Shutdown()
}

func (a *App) emitEvent(name string, data interface{}) {
	a.Runtime.EmitEvent(a.Ctx, name, data)
}
