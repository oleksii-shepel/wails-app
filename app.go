package main

import (
	"context"
	"fmt"

	internalapp "wails-app/internal/app"
	internalarchive "wails-app/internal/archive"
	internalbatch "wails-app/internal/batch"
	internalcontracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalnav "wails-app/internal/navigation"
	internalops "wails-app/internal/ops"
	internalplatform "wails-app/internal/platform"
	internalutility "wails-app/internal/utility"
	internalworkspace "wails-app/internal/workspace"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx        context.Context
	archive    *internalarchive.Service
	runtime    *AppRuntimeService
	filesystem *FilesystemService
	batch      *internalbatch.Service
	dirStream  *DirStreamService
	utility    *internalutility.Service
	navigation *internalnav.Service
	workspace  *internalworkspace.Service
	operations *OperationManager
	dirWatch   *PaneDirWatchManager
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
		runtime:    NewAppRuntimeService(),
		operations: core.Operations,
		archCache:  core.ArchiveCache,
	}
	app.elevation = internalelevation.NewService(app.emitEvent, func(exePath string) (internalelevation.Client, error) {
		return StartElevatedServiceClient(exePath)
	})
	app.filesystem = NewFilesystemService(app.elevation, app.emitEvent, app.StartElevatedSession)
	app.dirStream = NewDirStreamService(app.emitEvent, app.filesystem.runElevatedWithEvents)
	app.utility = internalutility.NewService(app.emitEvent)
	app.archive = internalarchive.NewService(
		app.archCache,
		func(opType internalcontracts.OperationType, title, source, dest string) internalarchive.Operation {
			return &archiveOperationAdapter{op: app.operations.StartOperation(opType, title, source, dest)}
		},
	)
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
	app.navigation = internalnav.NewService(core.NavigationManager, func(path string, showHidden bool) (*internalcontracts.DirectoryListing, error) {
		return app.filesystem.ListDirectory(path, showHidden)
	})
	app.workspace = internalworkspace.NewService(core.WorkspaceManager)
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
