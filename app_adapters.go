package main

import (
	"context"
	"fmt"

	internalapp "wails-app/internal/app"
	internalarchive "wails-app/internal/archive"
	internalelevation "wails-app/internal/elevation"
	internalops "wails-app/internal/ops"
	internalplatform "wails-app/internal/platform"
	vfsarchive "wails-app/vfs/archive"
)

func startElevatedClient(exePath string) (internalelevation.Client, error) {
	return StartElevatedServiceClient(exePath)
}

func newAppBuildConfig(app *internalapp.App) internalapp.BuildConfig {
	return internalapp.BuildConfig{
		SetContext:                     func(ctx context.Context) { app.Ctx = ctx },
		InitVFS:                        app.InitVFS,
		HandleFileDrop:                 app.HandleFileDrop,
		BuildFileInfo:                  buildFileInfoWithHidden,
		IsHidden:                       isHiddenFile,
		ClassifyError:                  classifyError,
		IsPermissionDenied:             internalelevation.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: internalelevation.ShouldRetryDeleteWithElevation,
		CalculateSize:                  internalops.CalculateSize,
		Normalize:                      internalplatform.Normalize,
		ArchiveEntryPath:               internalarchive.ArchiveEntryPath,
		IsArchive:                      vfsarchive.IsArchiveExt,
	}
}

func buildApp() *internalapp.App {
	core, err := internalapp.NewCore()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
	}

	app := internalapp.NewApp(isProcessElevated)
	elevation := internalelevation.NewService(app.EmitEvent, startElevatedClient)
	filesystem := NewFilesystemService(elevation, app.EmitEvent, app.StartElevatedSession)
	app.BindState(internalapp.BuildState(core, filesystem, elevation, newAppBuildConfig(app)))
	return app
}
