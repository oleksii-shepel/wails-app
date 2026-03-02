package main

// ============================================================================
// Conflict resolution
// ============================================================================

// ConflictResolution is the action the user has chosen for a file conflict.
type ConflictResolution string

const (
	ConflictSkip      ConflictResolution = "skip"
	ConflictOverwrite ConflictResolution = "overwrite"
	ConflictRename    ConflictResolution = "rename" // appends " (N)" before extension
	ConflictCancel    ConflictResolution = "cancel"
)

// ConflictEvent is the payload emitted as "operation:conflict" when a
// destination path already exists.  The frontend shows a dialog and calls
// ResolveConflict with the user's decision.
type ConflictEvent struct {
	OperationID string `json:"operationId"`
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
	// ApplyToAll stores this resolution as the blanket for all subsequent
	// conflicts in the same operation — no further dialogs will appear.
	ApplyToAll bool `json:"applyToAll"`
}

// pendingConflict holds the channel used to pause an operation goroutine
// while the user decides what to do.
type pendingConflict struct {
	response chan ConflictResolutionRequest
}

// ============================================================================
// Copy / Move / Delete
// ============================================================================

type BatchItem struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type BatchRequest struct {
	Items     []BatchItem `json:"items"`
	Recursive bool        `json:"recursive"`
}

type BatchResult struct {
	OperationID string           `json:"operationId"`
	Total       int              `json:"total"`
	Succeeded   int              `json:"succeeded"`
	Skipped     int              `json:"skipped"`
	Failed      int              `json:"failed"`
	Errors      []BatchItemError `json:"errors,omitempty"`
}

type BatchItemError struct {
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"`
	Error       string `json:"error"`
}

type BatchDeleteRequest struct {
	Paths     []string `json:"paths"`
	Recursive bool     `json:"recursive"`
	// UseTrash moves items to the OS recycle bin / trash instead of
	// permanently deleting them. Supported on Windows (Recycle Bin),
	// macOS (~/.Trash), and Linux (FreeDesktop Trash spec).
	// Defaults to true; set to false for permanent deletion.
	UseTrash *bool `json:"useTrash"`
}

// ============================================================================
// Full-archive extraction
// ============================================================================

// BatchExtractRequest extracts one or more complete on-disk archives.
type BatchExtractRequest struct {
	// Archives is the list of on-disk archive paths to extract.
	Archives []string `json:"archives"`
	// DestinationDir is the root directory all archives are extracted into.
	DestinationDir string `json:"destinationDir"`
	// CreateSubdirectory extracts each archive into its own named subdirectory
	// (<DestinationDir>/<archive-name-without-ext>/).
	CreateSubdirectory bool `json:"createSubdirectory"`
	// ExpandNestedArchives recurses into archive-inside-archive entries instead
	// of writing them as plain files.
	ExpandNestedArchives bool `json:"expandNestedArchives"`
	// MaxNestDepth caps recursion depth (-1 = unlimited, 0 = top-level only).
	MaxNestDepth int `json:"maxNestDepth"`
	// FlattenSingleRoot skips a sole top-level directory inside the archive
	// (common in source tarballs).
	FlattenSingleRoot bool `json:"flattenSingleRoot"`
}

type BatchExtractResult struct {
	OperationID    string           `json:"operationId"`
	Total          int              `json:"total"`
	Succeeded      int              `json:"succeeded"`
	Skipped        int              `json:"skipped"`
	Failed         int              `json:"failed"`
	ExtractedFiles int64            `json:"extractedFiles"`
	ExtractedBytes int64            `json:"extractedBytes"`
	Errors         []BatchItemError `json:"errors,omitempty"`
}

// ============================================================================
// Selective extraction (specific files/dirs from an archive or nested chain)
// ============================================================================

// SelectiveExtractItem extracts specific entries from one archive, which may
// be reached through a chain of nested archives.
//
// NestedChain mirrors the same field used in ArchiveBrowse / ArchiveRead:
//
//	[]                    → entries live directly in the top-level on-disk archive
//	["inner.zip"]         → entries are inside inner.zip, which is inside the disk archive
//	["a.tar.gz","b.zip"]  → two levels deep
//
// Entries lists inner paths to extract.  A directory prefix extracts the whole
// subtree below it.  An empty slice (or [""]) extracts everything at that level.
//
// FlattenPaths strips directory structure and dumps all files directly into
// DestinationDir (conflicts handled normally).
type SelectiveExtractItem struct {
	ArchivePath    string   `json:"archivePath"`
	NestedChain    []string `json:"nestedChain"`
	Entries        []string `json:"entries"`
	StripPrefix    string   `json:"stripPrefix,omitempty"`
	DestinationDir string   `json:"destinationDir,omitempty"` // overrides batch-level default
	FlattenPaths   bool     `json:"flattenPaths"`
}

type BatchSelectiveExtractRequest struct {
	Items          []SelectiveExtractItem `json:"items"`
	DestinationDir string                 `json:"destinationDir"` // fallback when item.DestinationDir is empty
}

type BatchSelectiveExtractResult struct {
	OperationID    string           `json:"operationId"`
	Total          int              `json:"total"`
	Succeeded      int              `json:"succeeded"`
	Skipped        int              `json:"skipped"`
	Failed         int              `json:"failed"`
	ExtractedFiles int64            `json:"extractedFiles"`
	ExtractedBytes int64            `json:"extractedBytes"`
	Errors         []BatchItemError `json:"errors,omitempty"`
}

// archiveEntry is a single file held in memory after archive layers are
// resolved.  Only matched files are ever buffered — never the whole archive.
type archiveEntry struct {
	innerPath string // normalised path inside the innermost archive
	data      []byte // uncompressed bytes
	size      int64  // declared uncompressed size from archive header
	mtime     int64  // modification time (unix seconds)
}
