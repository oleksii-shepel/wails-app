package app

import (
	"os"

	internalarchive "wails-app/internal/archive"
	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalnav "wails-app/internal/navigation"
	internalruntime "wails-app/internal/runtime"
	internalutility "wails-app/internal/utility"
	internalworkspace "wails-app/internal/workspace"
)

type Services struct {
	Elevation *internalelevation.Service
	Archive   *internalarchive.Service
	DirStream *internalruntime.DirStreamService
	Utility   *internalutility.Service
	Navigation *internalnav.Service
	Workspace *internalworkspace.Service
}

type ServiceConfig struct {
	Core *Core

	EmitEvent func(string, interface{})

	StartElevatedClient func(string) (internalelevation.Client, error)

	ArchiveOperationFactory func(opType contracts.OperationType, title, source, dest string) internalarchive.Operation

	ElevatedList func(path string, showHidden bool) ([]contracts.FileInfo, error)
	BuildFileInfo func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
	IsHidden func(name, fullPath string) bool

	ListDirectory func(path string, showHidden bool) (*contracts.DirectoryListing, error)
}

func NewServices(config ServiceConfig) *Services {
	return &Services{
		Elevation: internalelevation.NewService(config.EmitEvent, config.StartElevatedClient),
		Archive: internalarchive.NewService(
			config.Core.ArchiveCache,
			config.ArchiveOperationFactory,
		),
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
