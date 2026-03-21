package app

import (
	"os"

	"github.com/mholt/archiver/v3"

	internalarchive "wails-app/internal/archive"
	internalbatch "wails-app/internal/batch"
	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalnav "wails-app/internal/navigation"
	internalops "wails-app/internal/ops"
	internalplatform "wails-app/internal/platform"
	internalruntime "wails-app/internal/runtime"
	internalutility "wails-app/internal/utility"
	internalworkspace "wails-app/internal/workspace"
)

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

type Services struct {
	Elevation  *internalelevation.Service
	Archive    *internalarchive.Service
	Batch      *internalbatch.Service
	DirStream  *internalruntime.DirStreamService
	Utility    *internalutility.Service
	Navigation *internalnav.Service
	Workspace  *internalworkspace.Service
}

type ServiceConfig struct {
	Core *Core

	Elevation *internalelevation.Service
	EmitEvent func(string, interface{})

	ElevatedList  func(path string, showHidden bool) ([]contracts.FileInfo, error)
	BuildFileInfo func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
	IsHidden      func(name, fullPath string) bool

	ListDirectory func(path string, showHidden bool) (*contracts.DirectoryListing, error)

	StartOperation                 func(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	PromptConflict                 func(event contracts.ConflictEvent)
	ClassifyError                  func(error) contracts.AppErrorCode
	IsPermissionDenied             func(error) bool
	ShouldRetryDeleteWithElevation func(error) bool
	RunElevatedCopy                func(src, dst string, recursive bool) (bool, error)
	RunElevatedMove                func(src, dst string) (bool, error)
	RunElevatedDelete              func(path string, recursive bool, useTrash *bool) (bool, error)
	CalculateSize                  func(path string) (items, bytes int64)
	ExtractNestedArchive           func(archivePath string, nestedChain []string) ([]byte, error)
	Normalize                      func(string) string
	ArchiveEntryPath               func(archiver.File) string
	IsArchive                      func(string) bool
}

func NewServices(config ServiceConfig) *Services {
	return &Services{
		Elevation: config.Elevation,
		Archive: internalarchive.NewService(
			config.Core.ArchiveCache,
			func(opType contracts.OperationType, title, source, dest string) internalarchive.Operation {
				return &archiveOperationAdapter{op: config.StartOperation(opType, title, source, dest)}
			},
		),
		Batch: internalbatch.NewService(internalbatch.ServiceConfig{
			StartOperation:   config.StartOperation,
			ConflictPrompter: internalbatch.ConflictPrompterFunc(config.PromptConflict),
			BatchErr: func(source, destination string, err error) contracts.BatchItemError {
				return contracts.BatchItemError{
					Source:      source,
					Destination: destination,
					Error:       err.Error(),
					Code:        config.ClassifyError(err),
				}
			},
			IsPermissionDenied:             config.IsPermissionDenied,
			ShouldRetryDeleteWithElevation: config.ShouldRetryDeleteWithElevation,
			RunElevatedCopy:                config.RunElevatedCopy,
			RunElevatedMove:                config.RunElevatedMove,
			RunElevatedDelete:              config.RunElevatedDelete,
			MoveToTrash:                    internalplatform.MoveToTrash,
			CalculateSize:                  config.CalculateSize,
			ResolvePath:                    resolveMemoryAlias,
			ToPublicPath:                   toPublicMemoryPath,
			ExtractNestedArchive:           config.ExtractNestedArchive,
			Normalize:                      config.Normalize,
			ArchiveEntryPath:               config.ArchiveEntryPath,
			IsArchive:                      config.IsArchive,
		}),
		DirStream: internalruntime.NewDirStreamService(
			config.EmitEvent,
			config.ElevatedList,
			config.BuildFileInfo,
			config.IsHidden,
		),
		Utility: config.newUtility(),
		Navigation: internalnav.NewService(
			config.Core.NavigationManager,
			config.ListDirectory,
		),
		Workspace: internalworkspace.NewService(config.Core.WorkspaceManager),
	}
}

func (c ServiceConfig) newUtility() *internalutility.Service {
	memoryPlugin, _ := getMemoryFS()
	return internalutility.NewService(internalutility.ServiceConfig{
		EmitEvent:      c.EmitEvent,
		MemoryResolver: memoryPlugin,
	})
}
