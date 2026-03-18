package main

import (
	"context"
	"time"

	internalcontracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
	vfslib "wails-app/vfs"
)

type appArchive interface {
	Browse(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error)
	ReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error)
	CopyEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error)
	MoveEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error)
	DeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error)
	ExtractNestedArchive(archivePath string, nestedChain []string) ([]byte, error)
}

type appRuntime interface {
	Startup(context.Context)
	Shutdown()
	EmitEvent(context.Context, string, interface{})
}

type appFilesystem interface {
	SetVFS(*vfslib.Unified)
	ListDirectory(path string, showHidden bool) (*DirectoryListing, error)
	ElevatedList(path string, showHidden bool) ([]FileInfo, error)
	DirSizeRecursive(path string) (int64, error)
	GetFileInfo(path string) (*FileInfo, error)
	ReadFile(path string, encoding string) (*FileContent, error)
	WriteFile(path string, content string, encoding string) (*OperationResult, error)
	DeleteFile(path string, recursive bool, useTrash *bool) (*OperationResult, error)
	CreateDirectory(path string, recursive bool) (*OperationResult, error)
	MoveFile(source, destination string) (*OperationResult, error)
	CopyFile(source, destination string, recursive bool) (*OperationResult, error)
	SearchFiles(path, pattern string, recursive bool) (*SearchResult, error)
	AnalyzeDiskUsage(path string) (*DiskUsageResult, error)
	StartDiskUsageStream(path string) (string, error)
	CancelDiskUsageStream(scanID string)
	RunElevatedCopy(src, dst string, recursive bool) (bool, error)
	RunElevatedMove(src, dst string) (bool, error)
	RunElevatedDelete(path string, recursive bool, useTrash *bool) (bool, error)
}

type appOperations interface {
	SetEmitter(func(string, interface{}))
	StartOperation(kind internalcontracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	ListActiveOperations() []internalcontracts.OperationProgress
	CancelOperation(id string) bool
	RemoveOperation(id string)
}

type appBatch interface {
	BatchCopy(req BatchRequest) (*BatchResult, error)
	BatchMove(req BatchRequest) (*BatchResult, error)
	BatchDelete(req BatchDeleteRequest) (*BatchResult, error)
	BatchExtract(req BatchExtractRequest) (*BatchExtractResult, error)
	BatchSelectiveExtract(req BatchSelectiveExtractRequest) (*BatchSelectiveExtractResult, error)
	ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool
}

type appDirStream interface {
	Start(paneID, path string, showHidden bool) (string, error)
	Cancel(streamID string)
	CancelAll()
}

type appUtility interface {
	OpenFile(path string) error
	ResolveLink(path string) (*ResolvedLink, error)
	HandleFileDrop(x int, y int, paths []string) error
}

type appNavigation interface {
	Navigate(paneID, path string) (*DirectoryListing, error)
	NavigateShowHidden(paneID, path string, showHidden bool) (*DirectoryListing, error)
	Back(paneID string) (*DirectoryListing, error)
	Forward(paneID string) (*DirectoryListing, error)
	GoUp(paneID string) (*DirectoryListing, error)
}

type appWorkspace interface {
	GetWorkspaces() WorkspaceList
	SaveWorkspace(ws WorkspaceConfig) error
	LoadWorkspace(id string) (*WorkspaceConfig, error)
	DeleteWorkspace(id string) error
	GetPreferences() UserPreferences
	SavePreferences(prefs UserPreferences) error
}

type appDirWatch interface {
	SetPaneDirectory(paneID, path string) error
	Close()
}

type appElevation interface {
	StartSession() (*ElevatedSessionStatus, error)
	StopSession()
	Status() ElevatedSessionStatus
	Confirm(requestID string, approve bool)
	WaitForApproval(requestID string, timeout time.Duration) (bool, error)
}
