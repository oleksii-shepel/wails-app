package main

import (
	"context"
	"fmt"
	"os"

	internalapp "wails-app/internal/app"
	internalarchive "wails-app/internal/archive"
	internalbatch "wails-app/internal/batch"
	internalcontracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalnav "wails-app/internal/navigation"
	internalops "wails-app/internal/ops"
	internalplatform "wails-app/internal/platform"
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
	filesystem *FilesystemService
	batch      *internalbatch.Service
	dirStream  *internalruntime.DirStreamService
	utility    *internalutility.Service
	navigation *internalnav.Service
	workspace  *internalworkspace.Service
	operations *OperationManager
	dirWatch   *internalruntime.PaneWatchManager
	elevation  *internalelevation.Service
	vfs        *vfslib.Unified
	archCache  *cache.Cache // lazy-open, LRU cache of open archive.Archive objects
}

type archiveOperationAdapter struct {
	op *internalops.ActiveOperation
}

func (a *archiveOperationAdapter) ID() string {
	return a.op.ID
}

func (a *archiveOperationAdapter) Complete() {
	a.op.Complete()
}

func (a *archiveOperationAdapter) SetError(err error) {
	a.op.SetError(err)
}

func (a *archiveOperationAdapter) SetTotal(totalItems, totalBytes int64) {
	a.op.SetTotal(totalItems, totalBytes)
}

func (a *archiveOperationAdapter) UpdateProgress(processedItems, processedBytes int64, currentFile string, currentFileSize int64) {
	a.op.UpdateProgress(processedItems, processedBytes, currentFile, currentFileSize)
}

func (a *archiveOperationAdapter) IsCancelled() bool {
	return a.op.IsCancelled()
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
		SetOperationEmitter: app.operations.SetEmitter,
		CreatePaneWatch: func() {
			app.dirWatch = internalruntime.NewPaneWatchManager(app.emitEvent)
		},
		InitVFS: app.initVFS,
		HandleFileDrop: func(x, y int, paths []string) error {
			return app.HandleFileDrop(x, y, paths)
		},
		ClosePaneWatch: func() {
			if app.dirWatch != nil {
				app.dirWatch.Close()
			}
		},
		CancelDirStreams: func() {
			app.dirStream.CancelAll()
		},
		StopElevationSession: func() {
			if app.elevation != nil {
				app.elevation.StopSession()
			}
		},
		CloseArchiveCache: func() {
			app.archCache.CloseAll()
		},
	})
	app.elevation = internalelevation.NewService(app.emitEvent, func(exePath string) (internalelevation.Client, error) {
		return StartElevatedServiceClient(exePath)
	})
	app.filesystem = NewFilesystemService(app.elevation, app.emitEvent, app.StartElevatedSession)
	services := internalapp.NewServices(internalapp.ServiceConfig{
		Core: core,
		EmitEvent: app.emitEvent,
		StartElevatedClient: func(exePath string) (internalelevation.Client, error) {
			return StartElevatedServiceClient(exePath)
		},
		ArchiveOperationFactory: func(opType internalcontracts.OperationType, title, source, dest string) internalarchive.Operation {
			return &archiveOperationAdapter{op: app.operations.StartOperation(opType, title, source, dest)}
		},
		ElevatedList: func(path string, showHidden bool) ([]internalcontracts.FileInfo, error) {
			resp, err := app.filesystem.runElevatedWithEvents("list", path, elevatedRequest{
				Op:         "list",
				Path:       path,
				ShowHidden: showHidden,
			})
			if err != nil || resp.DirListing == nil {
				return nil, err
			}
			return resp.DirListing.Entries, nil
		},
		BuildFileInfo: func(name, fullPath string, info os.FileInfo, hidden bool) internalcontracts.FileInfo {
			return buildFileInfoWithHidden(name, fullPath, info, hidden)
		},
		IsHidden:      isHiddenFile,
		ListDirectory: func(path string, showHidden bool) (*internalcontracts.DirectoryListing, error) { return app.filesystem.ListDirectory(path, showHidden) },
	})
	app.elevation = services.Elevation
	app.archive = services.Archive
	app.dirStream = services.DirStream
	app.utility = services.Utility
	app.navigation = services.Navigation
	app.workspace = services.Workspace
	app.batch = internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: func(kind internalcontracts.OperationType, title, source, dest string) *internalops.ActiveOperation {
			return app.operations.StartOperation(kind, title, source, dest)
		},
		ConflictPrompter: internalbatch.ConflictPrompterFunc(func(event internalcontracts.ConflictEvent) {
			app.emitEvent("operation:conflict", event)
		}),
		BatchErr: func(source, destination string, err error) internalcontracts.BatchItemError {
			return internalcontracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        classifyError(err),
			}
		},
		IsPermissionDenied:             isPermissionDenied,
		ShouldRetryDeleteWithElevation: shouldRetryDeleteWithElevation,
		RunElevatedCopy: func(src, dst string, recursive bool) (bool, error) {
			resp, err := app.filesystem.runElevatedWithEvents("copy", app.filesystem.elevationTargetPair(src, dst), elevatedRequest{Op: "copy", Source: src, Destination: dst, Recursive: recursive})
			return err == nil && resp.Success, err
		},
		RunElevatedMove: func(src, dst string) (bool, error) {
			resp, err := app.filesystem.runElevatedWithEvents("move", app.filesystem.elevationTargetPair(src, dst), elevatedRequest{Op: "move", Source: src, Destination: dst})
			return err == nil && resp.Success, err
		},
		RunElevatedDelete: func(path string, recursive bool, useTrash *bool) (bool, error) {
			resp, err := app.filesystem.runElevatedWithEvents("delete", app.filesystem.elevationTargetSingle(path), elevatedRequest{Op: "delete", Path: path, Recursive: recursive, UseTrash: useTrash})
			return err == nil && resp.Success, err
		},
		MoveToTrash:          internalplatform.MoveToTrash,
		CalculateSize:        calculateSize,
		ExtractNestedArchive: app.archive.ExtractNestedArchive,
		Normalize:            normalize,
		ArchiveEntryPath:     archiveEntryPath,
		IsArchive:            isArchive,
	})
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
