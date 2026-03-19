package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	internalarchive "wails-app/internal/archive"
	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalfilesystem "wails-app/internal/filesystem"
	internalops "wails-app/internal/ops"
	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
	internalruntime "wails-app/internal/runtime"
	vfslib "wails-app/vfs"
	vfsarchive "wails-app/vfs/archive"
)

type filesystemElevation interface {
	Client() internalelevation.Client
}

type FilesystemService = internalfilesystem.Service

var rawFilesystemOps = internalfilesystem.NewRawService(internalfilesystem.RawCallbacks{
	Metadata: rawFilesystemMetadataCallbacks(),
	MoveToTrash: func(path string) error {
		return internalplatform.MoveToTrash(path)
	},
})

var (
	elevatedVFSOnce sync.Once
	elevatedVFS     *vfslib.Unified
	elevatedVFSErr  error
)

func getElevatedVFS() (*vfslib.Unified, error) {
	elevatedVFSOnce.Do(func() {
		elevatedVFS, elevatedVFSErr = internalruntime.InitVFS()
	})
	return elevatedVFS, elevatedVFSErr
}

var elevatedRawHandler = internalfilesystem.NewElevatedHandler(internalfilesystem.RawOperations{
	ListDirectory: func(path string, showHidden bool) (*contracts.DirectoryListing, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSListDir(vfs, path)
		}
		return rawFilesystemOps.ListDirectory(path, showHidden)
	},
	ReadFile: func(path string, encoding string) (*contracts.FileContent, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSReadFile(vfs, path, encoding)
		}
		return rawFilesystemOps.ReadFile(path, encoding)
	},
	WriteFile: func(path string, content string, encoding string) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSWriteFile(vfs, path, content, encoding)
		}
		return rawFilesystemOps.WriteFile(path, content, encoding)
	},
	DeleteFile: func(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSRemove(vfs, path)
		}
		return rawFilesystemOps.DeleteFile(path, recursive, useTrash)
	},
	CreateDirectory: func(path string, recursive bool) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSMkdir(vfs, path)
		}
		return rawFilesystemOps.CreateDirectory(path, recursive)
	},
	MoveFile: rawFilesystemOps.MoveFile,
	CopyFile: rawFilesystemOps.CopyFile,
})

func MaybeRunElevatedService(args []string) (bool, error) {
	return internalelevation.MaybeRunService(args, elevatedRawHandler)
}

func MaybeRunElevatedHelper(args []string) (bool, error) {
	return internalelevation.MaybeRunHelper(args, elevatedRawHandler)
}

func rawFilesystemMetadataCallbacks() internalfilesystem.MetadataCallbacks {
	return internalfilesystem.MetadataCallbacks{
		ToSlash:                      internalplatform.ToSlash,
		IsHiddenFile:                 internalplatform.CheckHidden,
		IsArchive:                    vfsarchive.IsArchiveExt,
		SymlinkPointsToDir:           internalplatform.SymlinkPointsToDir,
		IsUntrackedDirectoryJunction: internalplatform.IsUntrackedDirectoryJunction,
		GetCreatedTime:               internalplatform.GetCreatedTime,
		GetAccessedTime:              internalplatform.GetAccessedTime,
		FormatPermissions:            formatPermissions,
	}
}

func formatPermissions(info os.FileInfo) string {
	if runtime.GOOS == "windows" {
		if info.Mode()&0200 != 0 {
			return "rw-"
		}
		return "r--"
	}
	return info.Mode().Perm().String()
}

func newFilesystemService(
	elevation filesystemElevation,
	emitEvent func(string, interface{}),
	startElevatedSession func() (*internalelevation.SessionStatus, error),
) *FilesystemService {
	return internalfilesystem.NewService(internalfilesystem.ServiceConfig{
		Elevation:                      elevation,
		EmitEvent:                      emitEvent,
		StartElevatedSession:           startElevatedSession,
		Raw:                            rawFilesystemOps,
		Metadata:                       rawFilesystemMetadataCallbacks(),
		IsProcessElevated:              internalelevation.IsProcessElevated,
		ExecuteElevatedRequest:         executeElevatedRequest,
		RunElevatedRequest:             runElevatedRequest,
		IsPermissionDenied:             internalelevation.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: internalelevation.ShouldRetryDeleteWithElevation,
	})
}

func executeElevatedRequest(req internalelevation.Request) internalelevation.Response {
	return elevatedRawHandler(req)
}

func runElevatedRequest(req internalelevation.Request) (internalelevation.Response, error) {
	return internalelevation.RunRequest(req, os.Executable, internalelevation.LaunchProcess)
}

func startElevatedClient(exePath string) (internalelevation.Client, error) {
	return internalelevation.StartClient(exePath, internalelevation.LaunchService)
}

func newBuildConfig(app *App) BuildConfig {
	return BuildConfig{
		SetContext:     func(ctx context.Context) { app.Ctx = ctx },
		InitVFS:        app.InitVFS,
		HandleFileDrop: app.HandleFileDrop,
		BuildFileInfo: func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo {
			return internalfilesystem.BuildFileInfoWithHidden(rawFilesystemMetadataCallbacks(), name, fullPath, info, hidden)
		},
		IsHidden: func(name, path string) bool {
			return internalplatform.CheckHidden(name, path)
		},
		ClassifyError:                  contracts.ClassifyError,
		IsPermissionDenied:             internalelevation.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: internalelevation.ShouldRetryDeleteWithElevation,
		CalculateSize:                  internalops.CalculateSize,
		Normalize:                      internalplatform.Normalize,
		ArchiveEntryPath:               internalarchive.ArchiveEntryPath,
		IsArchive:                      vfsarchive.IsArchiveExt,
	}
}

func BuildApp() *App {
	core, err := NewCore()
	if err != nil {
		fmt.Printf("Warning: could not initialize workspace manager: %v\n", err)
	}

	app := NewApp(internalelevation.IsProcessElevated)
	elevation := internalelevation.NewService(app.EmitEvent, startElevatedClient)
	filesystem := newFilesystemService(elevation, app.EmitEvent, app.StartElevatedSession)
	app.BindState(BuildState(core, filesystem, elevation, newBuildConfig(app)))
	return app
}
