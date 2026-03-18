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
	emitEvent := emitAppEvent(app)
	elevation := internalelevation.NewService(emitEvent, startElevatedClient)
	filesystem := NewFilesystemService(elevation, emitEvent, app.StartElevatedSession)
	app.State = internalapp.BuildState(core, filesystem, elevation, internalapp.BuildConfig{
		SetContext:                     func(ctx context.Context) { app.Ctx = ctx },
		InitVFS:                        app.initVFS,
		HandleFileDrop:                 app.HandleFileDrop,
		BuildFileInfo:                  buildFileInfoWithHidden,
		IsHidden:                       isHiddenFile,
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
