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

	EmitEvent func(string, interface{})

	StartElevatedClient func(string) (internalelevation.Client, error)

	ArchiveOperationFactory func(opType contracts.OperationType, title, source, dest string) internalarchive.Operation

	ElevatedList  func(path string, showHidden bool) ([]contracts.FileInfo, error)
	BuildFileInfo func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
	IsHidden      func(name, fullPath string) bool

	ListDirectory func(path string, showHidden bool) (*contracts.DirectoryListing, error)

	StartOperation                 func(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	PromptConflict                 func(event contracts.ConflictEvent)
	BatchErr                       func(source, destination string, err error) contracts.BatchItemError
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
		Elevation: internalelevation.NewService(config.EmitEvent, config.StartElevatedClient),
		Archive: internalarchive.NewService(
			config.Core.ArchiveCache,
			config.ArchiveOperationFactory,
		),
		Batch: internalbatch.NewService(internalbatch.ServiceConfig{
			StartOperation:                 config.StartOperation,
			ConflictPrompter:               internalbatch.ConflictPrompterFunc(config.PromptConflict),
			BatchErr:                       config.BatchErr,
			IsPermissionDenied:             config.IsPermissionDenied,
			ShouldRetryDeleteWithElevation: config.ShouldRetryDeleteWithElevation,
			RunElevatedCopy:                config.RunElevatedCopy,
			RunElevatedMove:                config.RunElevatedMove,
			RunElevatedDelete:              config.RunElevatedDelete,
			MoveToTrash:                    internalplatform.MoveToTrash,
			CalculateSize:                  config.CalculateSize,
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
	return internalutility.NewService(c.EmitEvent)
}
