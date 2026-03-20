package app

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	internalarchive "wails-app/internal/archive"
	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalfilesystem "wails-app/internal/filesystem"
	internalmemoryfs "wails-app/internal/memoryfs"
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
	memoryPathOnce  sync.Once
	memoryPath      *internalmemoryfs.PathResolver
	memoryPathErr   error
	memoryRootFn    = defaultMemoryRootProvider
)

const memoryAppName = "wails-file-manager"

func defaultMemoryRootProvider() string {
	if root := os.Getenv("WAILS_MEMFS_ROOT"); strings.TrimSpace(root) != "" {
		return root
	}
	if root := os.Getenv("MEMFS_HIDDEN_PATH"); strings.TrimSpace(root) != "" {
		return root
	}
	if runtime.GOOS == "windows" {
		if info, err := internalmemoryfs.GetConnectionInfo(); err == nil {
			if root := strings.TrimSpace(info.HiddenPath); root != "" {
				return root
			}
		}
	}
	return ""
}

func configureMemoryRootProvider(provider func() string) {
	if provider == nil {
		memoryRootFn = defaultMemoryRootProvider
		return
	}
	memoryRootFn = provider
}

func getElevatedVFS() (*vfslib.Unified, error) {
	elevatedVFSOnce.Do(func() {
		elevatedVFS, elevatedVFSErr = internalruntime.InitVFS()
	})
	return elevatedVFS, elevatedVFSErr
}

func getMemoryPathResolver() (*internalmemoryfs.PathResolver, error) {
	memoryPathOnce.Do(func() {
		memoryPath, memoryPathErr = internalmemoryfs.NewPathResolver(memoryAppName, func() string {
			return memoryRootFn()
		})
	})
	return memoryPath, memoryPathErr
}

func resolveMemoryAlias(path string) (string, error) {
	if !pathref.IsMemory(path) {
		return path, nil
	}
	resolver, err := getMemoryPathResolver()
	if err != nil {
		return "", err
	}
	resolved, _, err := resolver.ResolvePublicPath(path)
	return resolved, err
}

func toPublicMemoryPath(path string) string {
	resolver, err := getMemoryPathResolver()
	if err != nil || resolver == nil {
		return path
	}
	return resolver.ToPublicPath(path)
}

func mapMemoryListing(listing *contracts.DirectoryListing, requestedPath string) *contracts.DirectoryListing {
	if listing == nil {
		return nil
	}
	listing.Path = pathref.Normalize(requestedPath)
	for index := range listing.Entries {
		listing.Entries[index].Path = toPublicMemoryPath(listing.Entries[index].Path)
	}
	return listing
}

func mapMemoryFile(file *contracts.FileContent, requestedPath string) *contracts.FileContent {
	if file == nil {
		return nil
	}
	file.Path = pathref.Normalize(requestedPath)
	return file
}

func mapMemoryOperation(op *contracts.OperationResult) *contracts.OperationResult {
	if op == nil {
		return nil
	}
	for index := range op.AffectedPaths {
		op.AffectedPaths[index] = toPublicMemoryPath(op.AffectedPaths[index])
	}
	return op
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
		resolvedPath, err := resolveMemoryAlias(path)
		if err != nil {
			return nil, err
		}
		listing, err := rawFilesystemOps.ListDirectory(resolvedPath, showHidden)
		if err != nil {
			return nil, err
		}
		return mapMemoryListing(listing, path), nil
	},
	ReadFile: func(path string, encoding string) (*contracts.FileContent, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSReadFile(vfs, path, encoding)
		}
		resolvedPath, err := resolveMemoryAlias(path)
		if err != nil {
			return nil, err
		}
		file, err := rawFilesystemOps.ReadFile(resolvedPath, encoding)
		if err != nil {
			return nil, err
		}
		return mapMemoryFile(file, path), nil
	},
	WriteFile: func(path string, content string, encoding string) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSWriteFile(vfs, path, content, encoding)
		}
		resolvedPath, err := resolveMemoryAlias(path)
		if err != nil {
			return nil, err
		}
		op, err := rawFilesystemOps.WriteFile(resolvedPath, content, encoding)
		if err != nil {
			return nil, err
		}
		return mapMemoryOperation(op), nil
	},
	DeleteFile: func(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSRemove(vfs, path)
		}
		resolvedPath, err := resolveMemoryAlias(path)
		if err != nil {
			return nil, err
		}
		op, err := rawFilesystemOps.DeleteFile(resolvedPath, recursive, useTrash)
		if err != nil {
			return nil, err
		}
		return mapMemoryOperation(op), nil
	},
	CreateDirectory: func(path string, recursive bool) (*contracts.OperationResult, error) {
		if pathref.IsRegistry(path) {
			vfs, err := getElevatedVFS()
			if err != nil {
				return nil, err
			}
			return internalruntime.VFSMkdir(vfs, path)
		}
		resolvedPath, err := resolveMemoryAlias(path)
		if err != nil {
			return nil, err
		}
		op, err := rawFilesystemOps.CreateDirectory(resolvedPath, recursive)
		if err != nil {
			return nil, err
		}
		return mapMemoryOperation(op), nil
	},
	MoveFile: func(source, destination string) (*contracts.OperationResult, error) {
		resolvedSource, err := resolveMemoryAlias(source)
		if err != nil {
			return nil, err
		}
		resolvedDestination, err := resolveMemoryAlias(destination)
		if err != nil {
			return nil, err
		}
		op, err := rawFilesystemOps.MoveFile(resolvedSource, resolvedDestination)
		if err != nil {
			return nil, err
		}
		return mapMemoryOperation(op), nil
	},
	CopyFile: func(source, destination string, recursive bool) (*contracts.OperationResult, error) {
		resolvedSource, err := resolveMemoryAlias(source)
		if err != nil {
			return nil, err
		}
		resolvedDestination, err := resolveMemoryAlias(destination)
		if err != nil {
			return nil, err
		}
		op, err := rawFilesystemOps.CopyFile(resolvedSource, resolvedDestination, recursive)
		if err != nil {
			return nil, err
		}
		return mapMemoryOperation(op), nil
	},
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
	resolver, _ := getMemoryPathResolver()
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
		MemoryResolver:                 resolver,
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

	configureMemoryRootProvider(func() string {
		if root := defaultMemoryRootProvider(); root != "" {
			return root
		}
		if core != nil && core.WorkspaceManager != nil {
			return strings.TrimSpace(core.WorkspaceManager.GetPreferences().MemoryRoot)
		}
		return ""
	})

	app := NewApp(internalelevation.IsProcessElevated)
	elevation := internalelevation.NewService(app.EmitEvent, startElevatedClient)
	filesystem := newFilesystemService(elevation, app.EmitEvent, app.StartElevatedSession)
	app.BindState(BuildState(core, filesystem, elevation, newBuildConfig(app)))
	return app
}
