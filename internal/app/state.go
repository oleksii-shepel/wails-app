package app

import (
	"context"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
	internalelevation "wails-app/internal/elevation"
	internalops "wails-app/internal/ops"
	internalutility "wails-app/internal/utility"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

type Archive interface {
	Browse(fsPath string, nestedChain []string, innerPath string) (contracts.ArchiveListing, error)
	ReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*contracts.FileContent, error)
	ReadFileStream(fsPath string, nestedChain []string, entryPath string, chunkSize int, emitEvent func(string, interface{})) (string, error)
	CancelReadStream(streamID string)
	CopyEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error)
	MoveEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error)
	DeleteEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error)
	ExtractNestedArchive(archivePath string, nestedChain []string) ([]byte, error)
}

type Runtime interface {
	Startup(context.Context)
	Shutdown()
	EmitEvent(context.Context, string, interface{})
}

type Filesystem interface {
	SetVFS(*vfslib.Unified)
	ListDirectory(path string, showHidden bool) (*contracts.DirectoryListing, error)
	ElevatedList(path string, showHidden bool) ([]contracts.FileInfo, error)
	DirSizeRecursive(path string) (int64, error)
	GetFileInfo(path string) (*contracts.FileInfo, error)
	ReadFile(path string, encoding string) (*contracts.FileContent, error)
	ReadFileStream(path string, chunkSize int, emitEvent func(string, interface{})) (string, error)
	CancelReadStream(streamID string)
	WriteFile(path string, content string, encoding string) (*contracts.OperationResult, error)
	DeleteFile(path string, recursive bool, useTrash *bool) (*contracts.OperationResult, error)
	CreateDirectory(path string, recursive bool) (*contracts.OperationResult, error)
	MoveFile(source, destination string) (*contracts.OperationResult, error)
	CopyFile(source, destination string, recursive bool) (*contracts.OperationResult, error)
	SearchFiles(path, pattern string, recursive bool) (*contracts.SearchResult, error)
	AnalyzeDiskUsage(path string) (*contracts.DiskUsageResult, error)
	StartDiskUsageStream(path string) (string, error)
	CancelDiskUsageStream(scanID string)
	RunElevatedCopy(src, dst string, recursive bool) (bool, error)
	RunElevatedMove(src, dst string) (bool, error)
	RunElevatedDelete(path string, recursive bool, useTrash *bool) (bool, error)
	RealPath(path string) (string, error)
}

type Operations interface {
	SetEmitter(func(string, interface{}))
	StartOperation(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	ListActiveOperations() []contracts.OperationProgress
	CancelOperation(id string) bool
	RemoveOperation(id string)
}

type Batch interface {
	BatchCopy(req contracts.BatchRequest) (*contracts.BatchResult, error)
	BatchMove(req contracts.BatchRequest) (*contracts.BatchResult, error)
	BatchDelete(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error)
	BatchExtract(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error)
	BatchSelectiveExtract(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error)
	ResolveConflict(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool
}

type Converter interface {
	ConvertFormat(inputPath string, outputFormat string) ([]byte, error)
	IsSupportedSourceFormat(extension string) bool
	GetAvailableConversionFormats(sourceExt string) []string
}

type OfficeConverter interface {
	ConvertToPDF(inputPath string) (string, error)
	IsSupportedFormat(ext string) bool
	IsAvailable() bool
}

type DirStream interface {
	Start(paneID, path string, showHidden bool) (string, error)
	Cancel(streamID string)
	CancelAll()
}

type Utility interface {
	OpenFile(path string) error
	ResolveLink(path string) (*internalutility.ResolvedLink, error)
	HandleFileDrop(x int, y int, paths []string) error
}

type Navigation interface {
	Navigate(paneID, path string) (*contracts.DirectoryListing, error)
	NavigateShowHidden(paneID, path string, showHidden bool) (*contracts.DirectoryListing, error)
	Back(paneID string) (*contracts.DirectoryListing, error)
	Forward(paneID string) (*contracts.DirectoryListing, error)
	GoUp(paneID string) (*contracts.DirectoryListing, error)
}

type Workspace interface {
	GetWorkspaces() contracts.WorkspaceList
	SaveWorkspace(ws contracts.WorkspaceConfig) error
	LoadWorkspace(id string) (*contracts.WorkspaceConfig, error)
	DeleteWorkspace(id string) error
	GetPreferences() contracts.UserPreferences
	SavePreferences(prefs contracts.UserPreferences) error
}

type DirWatch interface {
	SetPaneDirectory(paneID, path string) error
	Close()
}

type Elevation interface {
	StartSession() (*internalelevation.SessionStatus, error)
	StopSession()
	Status() internalelevation.SessionStatus
	Confirm(requestID string, approve bool)
	WaitForApproval(requestID string, timeout time.Duration) (bool, error)
}

type State struct {
	Ctx             context.Context
	Archive         Archive
	Runtime         Runtime
	Filesystem      Filesystem
	Batch           Batch
	Converter       Converter
	OfficeConverter OfficeConverter
	DirStream       DirStream
	Utility         Utility
	Navigation      Navigation
	Workspace       Workspace
	Operations      Operations
	DirWatch        DirWatch
	Elevation       Elevation
	vfs             *vfslib.Unified
	archCache       *cache.Cache
	remotesMu       sync.Mutex
	remotes         map[string]contracts.RemoteMount // keyed by VFS mount point
}

func NewState(core *Core) *State {
	return &State{
		Operations: core.Operations,
		archCache:  core.ArchiveCache,
		remotes:    make(map[string]contracts.RemoteMount),
	}
}
