package main

import (
	"errors"
	"io/fs"
	"os"

	internalcontracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

// ============================================================================
// Typed Error Codes
// ============================================================================

// AppErrorCode is a stable machine-readable string sent with every
// BatchItemError so the frontend can show localised / actionable messages.
type AppErrorCode = internalcontracts.AppErrorCode

type BatchItemError = internalcontracts.BatchItemError

const (
	ErrCodePermissionDenied AppErrorCode = internalcontracts.ErrCodePermissionDenied
	ErrCodeNotFound         AppErrorCode = internalcontracts.ErrCodeNotFound
	ErrCodeDiskFull         AppErrorCode = internalcontracts.ErrCodeDiskFull
	ErrCodeReadOnly         AppErrorCode = internalcontracts.ErrCodeReadOnly
	ErrCodeAlreadyExists    AppErrorCode = internalcontracts.ErrCodeAlreadyExists
	ErrCodeInvalidPath      AppErrorCode = internalcontracts.ErrCodeInvalidPath
	ErrCodeCancelled        AppErrorCode = internalcontracts.ErrCodeCancelled
	ErrCodeUnknown          AppErrorCode = internalcontracts.ErrCodeUnknown
)

// classifyError maps a Go error to a stable AppErrorCode.
func classifyError(err error) AppErrorCode {
	if err == nil {
		return ErrCodeUnknown
	}
	msg := err.Error()
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, fs.ErrNotExist):
		return ErrCodeNotFound
	case errors.Is(err, os.ErrPermission), errors.Is(err, fs.ErrPermission):
		return ErrCodePermissionDenied
	case errors.Is(err, os.ErrExist), errors.Is(err, fs.ErrExist):
		return ErrCodeAlreadyExists
	case errors.Is(err, os.ErrInvalid), errors.Is(err, fs.ErrInvalid):
		return ErrCodeInvalidPath
	}
	// Heuristic string-based classification for cases os.Is cannot reach.
	for _, sub := range []string{"no space left", "disk full", "not enough space"} {
		if containsFold(msg, sub) {
			return ErrCodeDiskFull
		}
	}
	for _, sub := range []string{"read-only", "read only", "write-protected"} {
		if containsFold(msg, sub) {
			return ErrCodeReadOnly
		}
	}
	for _, sub := range []string{"cancelled", "canceled"} {
		if containsFold(msg, sub) {
			return ErrCodeCancelled
		}
	}
	return ErrCodeUnknown
}

// ============================================================================
// OS Info
// ============================================================================

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "macos"
	OSUnknown OSType = "unknown"
)

type OSInfoResponse struct {
	OS          OSType  `json:"os"`
	Version     string  `json:"version"`
	Arch        string  `json:"arch"`
	Hostname    string  `json:"hostname"`
	SystemDrive *string `json:"systemDrive"`
}

// ============================================================================
// Drives
// ============================================================================

type DriveType string

const (
	DriveFixed     DriveType = "fixed"
	DriveRemovable DriveType = "removable"
	DriveNetwork   DriveType = "network"
	DriveCDROM     DriveType = "cdrom"
	DriveRAMDisk   DriveType = "ramdisk"
	DriveUnknown   DriveType = "unknown"
)

type DriveInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	DriveType  DriveType `json:"driveType"`
	TotalSpace int64     `json:"totalSpace"`
	FreeSpace  int64     `json:"freeSpace"`
	FileSystem *string   `json:"fileSystem"`
}

type DrivesList struct {
	Drives []DriveInfo `json:"drives"`
}

// ============================================================================
// File System Data Types
// ============================================================================

type FileType = internalcontracts.FileType

const (
	FileTypeFile      FileType = internalcontracts.FileTypeFile
	FileTypeArchive   FileType = internalcontracts.FileTypeArchive
	FileTypeDirectory FileType = internalcontracts.FileTypeDirectory
	FileTypeSymlink   FileType = internalcontracts.FileTypeSymlink
)

type FileInfo = internalcontracts.FileInfo

type ArchiveListing = internalcontracts.ArchiveListing

type DirectoryListing = internalcontracts.DirectoryListing

type FileContent = internalcontracts.FileContent

type OperationResult = internalcontracts.OperationResult

type SearchResult = internalcontracts.SearchResult

// ============================================================================
// Response Envelope
// ============================================================================

type ResponseStatus string

const (
	ResponseStatusSuccess ResponseStatus = "SUCCESS"
	ResponseStatusError   ResponseStatus = "ERROR"
)

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type AppResponse struct {
	CommandID string         `json:"commandId"`
	Status    ResponseStatus `json:"status"`
	Timestamp int64          `json:"timestamp"`
	Data      interface{}    `json:"data,omitempty"`
	Error     *ErrorDetail   `json:"error,omitempty"`
}

// ============================================================================
// Navigation History
// ============================================================================

type HistoryEntry = internalcontracts.HistoryEntry

type NavigationHistory = internalcontracts.NavigationHistory

// ============================================================================
// Tabs & Workspace
// ============================================================================

type TabInfo = internalcontracts.TabInfo

type ViewerTab = internalcontracts.ViewerTab

type PaneState = internalcontracts.PaneState

type WorkspaceConfig = internalcontracts.WorkspaceConfig

type WorkspaceList = internalcontracts.WorkspaceList

// ============================================================================
// Filter System
// ============================================================================

type FilterType string

const (
	FilterName      FilterType = "name"
	FilterExtension FilterType = "ext"
	FilterType_     FilterType = "type"
	FilterSize      FilterType = "size"
	FilterModified  FilterType = "modified"
	FilterGlob      FilterType = "glob"
)

type FilterCriteria struct {
	Type     FilterType `json:"type"`
	Value    string     `json:"value"`
	Operator string     `json:"operator,omitempty"`
}

type FilterQuery struct {
	Text          string           `json:"text,omitempty"`
	Criteria      []FilterCriteria `json:"criteria,omitempty"`
	CaseSensitive bool             `json:"caseSensitive"`
}

// ============================================================================
// User Preferences
// ============================================================================

type UserPreferences = internalcontracts.UserPreferences

// ============================================================================
// Operations
// ============================================================================

type OperationType = internalcontracts.OperationType

const (
	OpCopy      OperationType = internalcontracts.OpCopy
	OpMove      OperationType = internalcontracts.OpMove
	OpDelete    OperationType = internalcontracts.OpDelete
	OpArchive   OperationType = internalcontracts.OpArchive
	OpExtract   OperationType = internalcontracts.OpExtract
	OpSearch    OperationType = internalcontracts.OpSearch
	OpCalculate OperationType = internalcontracts.OpCalculate
)

type OperationStatus = internalcontracts.OperationStatus

const (
	StatusPending   OperationStatus = internalcontracts.StatusPending
	StatusRunning   OperationStatus = internalcontracts.StatusRunning
	StatusPaused    OperationStatus = internalcontracts.StatusPaused
	StatusCompleted OperationStatus = internalcontracts.StatusCompleted
	StatusCancelled OperationStatus = internalcontracts.StatusCancelled
	StatusError     OperationStatus = internalcontracts.StatusError
)

// OperationProgress is the shared wire model; ActiveOperation keeps runtime-only
// bookkeeping locally via progressSnapshot.
type OperationProgress = internalcontracts.OperationProgress
type ActiveOperation = internalops.ActiveOperation

type OperationManager = internalops.Manager
