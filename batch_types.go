package main

// ============================================================================
// Conflict resolution
// ============================================================================

// ConflictResolution is the action the user has chosen for a file conflict.
type ConflictResolution string

const (
	// ConflictSkip leaves the destination untouched and moves on.
	ConflictSkip ConflictResolution = "skip"
	// ConflictOverwrite replaces the destination with the source.
	ConflictOverwrite ConflictResolution = "overwrite"
	// ConflictRename appends a numeric suffix to make the destination unique.
	ConflictRename ConflictResolution = "rename"
	// ConflictCancel aborts the entire batch immediately.
	ConflictCancel ConflictResolution = "cancel"
)

// ConflictEvent is the payload emitted as "operation:conflict" when a
// destination path already exists.  The frontend shows a dialog and calls
// ResolveConflict with the user's decision.
type ConflictEvent struct {
	OperationID string `json:"operationId"`
	// ConflictID uniquely identifies this conflict within the operation so the
	// frontend can match the resolution back to the right goroutine.
	ConflictID  string `json:"conflictId"`
	SourcePath  string `json:"sourcePath"`
	DestPath    string `json:"destPath"`
	SourceSize  int64  `json:"sourceSize"`
	DestSize    int64  `json:"destSize"`
	SourceMtime int64  `json:"sourceMtime"`
	DestMtime   int64  `json:"destMtime"`
	IsDirectory bool   `json:"isDirectory"`
}

// ConflictResolutionRequest is the payload the frontend sends back to
// ResolveConflict.
type ConflictResolutionRequest struct {
	Resolution ConflictResolution `json:"resolution"`
	// ApplyToAll, when true, applies this resolution to all remaining conflicts
	// in the operation without asking again.
	ApplyToAll bool `json:"applyToAll"`
}

// pendingConflict holds the channel pair used to pause the operation goroutine
// while the user decides what to do.
type pendingConflict struct {
	response chan ConflictResolutionRequest
}

// ============================================================================
// Batch request / result types
// ============================================================================

// BatchItem is a single source→destination pair in a batch request.
type BatchItem struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// BatchRequest describes a multi-item copy or move operation.
type BatchRequest struct {
	Items     []BatchItem `json:"items"`
	Recursive bool        `json:"recursive"`
}

// BatchResult is the summary returned after a batch finishes (or is cancelled).
type BatchResult struct {
	OperationID string           `json:"operationId"`
	Total       int              `json:"total"`
	Succeeded   int              `json:"succeeded"`
	Skipped     int              `json:"skipped"`
	Failed      int              `json:"failed"`
	Errors      []BatchItemError `json:"errors,omitempty"`
}

// BatchItemError records why a single item in a batch failed.
type BatchItemError struct {
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"`
	Error       string `json:"error"`
}

// BatchDeleteRequest is a list of paths to delete.
type BatchDeleteRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
}
