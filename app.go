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

	app := &App{}
	elevation := internalelevation.NewService(app.emitEvent, app.startElevatedClient)
	filesystem := NewFilesystemService(elevation, app.emitEvent, app.StartElevatedSession)
	app.State = internalapp.BuildState(core, filesystem, elevation, internalapp.BuildConfig{
		SetContext:                     func(ctx context.Context) { app.Ctx = ctx },
		CreatePaneWatch:                app.createPaneWatch,
		InitVFS:                        app.initVFS,
		HandleFileDrop:                 app.HandleFileDrop,
		ClosePaneWatch:                 app.closePaneWatch,
		CancelDirStreams:               app.cancelDirStreams,
		StopElevationSession:           app.stopElevationSession,
		CloseArchiveCache:              app.closeArchiveCache,
		BuildFileInfo:                  app.buildFileInfo,
		IsHidden:                       isHiddenFile,
		PromptConflict:                 app.promptConflict,
		ClassifyError:                  classifyError,
		IsPermissionDenied:             isPermissionDenied,
		ShouldRetryDeleteWithElevation: shouldRetryDeleteWithElevation,
		CalculateSize:                  calculateSize,
		Normalize:                      normalize,
		ArchiveEntryPath:               archiveEntryPath,
		IsArchive:                      isArchive,
	})
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
