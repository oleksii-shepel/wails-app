package main

import (
	"context"
	"fmt"

	internalarchive "wails-app/internal/archive"
	internalbatch "wails-app/internal/batch"
	internalcontracts "wails-app/internal/contracts"
	internalnav "wails-app/internal/navigation"
	internalops "wails-app/internal/ops"
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
	utility    *UtilityService
	navigation *internalnav.Service
	workspace  *internalworkspace.Service
	operations *OperationManager
	dirWatch   *PaneDirWatchManager
	elevation  *ElevationSessionService
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
	wm, err := internalworkspace.NewManager()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
		wm = internalworkspace.NewTransientManager()
	}

	navManager := internalnav.NewManager()

	app := &App{
		archive:    nil,
		runtime:    NewAppRuntimeService(),
		operations: internalops.NewManager(),
		archCache:  cache.New(resolveChain),
	}
	app.elevation = NewElevationSessionService(app.emitEvent)
	app.filesystem = NewFilesystemService(app.elevation, app.emitEvent, app.StartElevatedSession)
	app.dirStream = NewDirStreamService(app.emitEvent, app.filesystem.runElevatedWithEvents)
	app.utility = NewUtilityService(app.emitEvent)
	app.archive = internalarchive.NewService(
		app.archCache,
		func(opType internalcontracts.OperationType, title, source, dest string) internalarchive.Operation {
			return &archiveOperationAdapter{op: app.operations.StartOperation(opType, title, source, dest)}
		},
	)
	batchCore := newBatchCoreService(
		app.operations,
		app.emitEvent,
		app.filesystem.runElevatedWithEvents,
		app.archive.ExtractNestedArchive,
	)
	app.batch = internalbatch.NewService(
		batchCore.BatchCopy,
		batchCore.BatchMove,
		batchCore.BatchDelete,
		batchCore.BatchExtract,
		batchCore.BatchSelectiveExtract,
		batchCore.ResolveConflict,
	)
	app.navigation = internalnav.NewService(navManager, func(path string, showHidden bool) (*internalcontracts.DirectoryListing, error) {
		return app.filesystem.ListDirectory(path, showHidden)
	})
	app.workspace = internalworkspace.NewService(wm)
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
