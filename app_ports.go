package main

import (
	internalcontracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
	vfslib "wails-app/vfs"
)

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
	runElevatedWithEvents(op, target string, req elevatedRequest) (elevatedResponse, error)
	elevationTargetSingle(path string) string
	elevationTargetPair(source, destination string) string
}

type appOperations interface {
	SetEmitter(func(string, interface{}))
	StartOperation(kind internalcontracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	ListActiveOperations() []internalcontracts.OperationProgress
	CancelOperation(id string) bool
	RemoveOperation(id string)
}
