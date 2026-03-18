package main

import (
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
	return internalcontracts.ClassifyError(err)
}

// ============================================================================
// OS Info
// ============================================================================

type OSType = internalcontracts.OSType

const (
	OSWindows OSType = internalcontracts.OSWindows
	OSLinux   OSType = internalcontracts.OSLinux
	OSDarwin  OSType = internalcontracts.OSDarwin
	OSUnknown OSType = internalcontracts.OSUnknown
)

type OSInfoResponse = internalcontracts.OSInfoResponse

// ============================================================================
// Drives
// ============================================================================

type DriveType = internalcontracts.DriveType

const (
	DriveFixed     DriveType = internalcontracts.DriveFixed
	DriveRemovable DriveType = internalcontracts.DriveRemovable
	DriveNetwork   DriveType = internalcontracts.DriveNetwork
	DriveCDROM     DriveType = internalcontracts.DriveCDROM
	DriveRAMDisk   DriveType = internalcontracts.DriveRAMDisk
	DriveUnknown   DriveType = internalcontracts.DriveUnknown
)

type DriveInfo = internalcontracts.DriveInfo

type DrivesList = internalcontracts.DrivesList

type SpecialFolder = internalcontracts.SpecialFolder

type SpecialFoldersResponse = internalcontracts.SpecialFoldersResponse

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
