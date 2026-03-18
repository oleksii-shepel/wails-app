package app

import (
	"context"
	"os"

	"github.com/mholt/archiver/v3"

	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
)

type BuildConfig struct {
	SetContext func(context.Context)

	CreatePaneWatch      func()
	InitVFS              func()
	HandleFileDrop       func(int, int, []string) error
	ClosePaneWatch       func()
	CancelDirStreams     func()
	StopElevationSession func()
	CloseArchiveCache    func()

	BuildFileInfo func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
	IsHidden      func(name, fullPath string) bool

	PromptConflict                 func(event contracts.ConflictEvent)
	ClassifyError                  func(error) contracts.AppErrorCode
	IsPermissionDenied             func(error) bool
	ShouldRetryDeleteWithElevation func(error) bool
	CalculateSize                  func(path string) (items, bytes int64)
	Normalize                      func(string) string
	ArchiveEntryPath               func(archiver.File) string
	IsArchive                      func(string) bool
}

func BuildState(core *Core, filesystem Filesystem, elevation *internalelevation.Service, config BuildConfig) *State {
	state := NewState(core)
	state.Elevation = elevation
	state.Filesystem = filesystem
	state.Runtime = NewRuntimeService(RuntimeConfig{
		SetContext:           config.SetContext,
		SetOperationEmitter:  state.Operations.SetEmitter,
		CreatePaneWatch:      config.CreatePaneWatch,
		InitVFS:              config.InitVFS,
		HandleFileDrop:       config.HandleFileDrop,
		ClosePaneWatch:       config.ClosePaneWatch,
		CancelDirStreams:     config.CancelDirStreams,
		StopElevationSession: config.StopElevationSession,
		CloseArchiveCache:    config.CloseArchiveCache,
	})

	services := NewServices(ServiceConfig{
		Core:                           core,
		Elevation:                      elevation,
		EmitEvent:                      func(name string, data interface{}) { state.Runtime.EmitEvent(state.Ctx, name, data) },
		ElevatedList:                   filesystem.ElevatedList,
		BuildFileInfo:                  config.BuildFileInfo,
		IsHidden:                       config.IsHidden,
		ListDirectory:                  filesystem.ListDirectory,
		StartOperation:                 state.Operations.StartOperation,
		PromptConflict:                 config.PromptConflict,
		ClassifyError:                  config.ClassifyError,
		IsPermissionDenied:             config.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: config.ShouldRetryDeleteWithElevation,
		RunElevatedCopy:                filesystem.RunElevatedCopy,
		RunElevatedMove:                filesystem.RunElevatedMove,
		RunElevatedDelete:              filesystem.RunElevatedDelete,
		CalculateSize:                  config.CalculateSize,
		ExtractNestedArchive: func(archivePath string, nestedChain []string) ([]byte, error) {
			return state.Archive.ExtractNestedArchive(archivePath, nestedChain)
		},
		Normalize:        config.Normalize,
		ArchiveEntryPath: config.ArchiveEntryPath,
		IsArchive:        config.IsArchive,
	})

	state.Archive = services.Archive
	state.Batch = services.Batch
	state.DirStream = services.DirStream
	state.Utility = services.Utility
	state.Navigation = services.Navigation
	state.Workspace = services.Workspace
	return state
}
