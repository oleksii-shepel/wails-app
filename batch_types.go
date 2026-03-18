package main

import internalcontracts "wails-app/internal/contracts"

// ============================================================================
// Conflict resolution
// ============================================================================

// ConflictResolution is the action the user has chosen for a file conflict.
type ConflictResolution = internalcontracts.ConflictResolution

const (
	ConflictSkip      ConflictResolution = internalcontracts.ConflictSkip
	ConflictOverwrite ConflictResolution = internalcontracts.ConflictOverwrite
	ConflictRename    ConflictResolution = internalcontracts.ConflictRename
	ConflictCancel    ConflictResolution = internalcontracts.ConflictCancel
)

// ConflictEvent is the payload emitted as "operation:conflict" when a
// destination path already exists.  The frontend shows a dialog and calls
// ResolveConflict with the user's decision.
type ConflictEvent = internalcontracts.ConflictEvent

// ConflictResolutionRequest is the payload the frontend sends back to
// ResolveConflict.
type ConflictResolutionRequest = internalcontracts.ConflictResolutionRequest

// ============================================================================
// Copy / Move / Delete
// ============================================================================

type BatchItem = internalcontracts.BatchItem

type BatchRequest = internalcontracts.BatchRequest

type BatchResult = internalcontracts.BatchResult

type BatchDeleteRequest = internalcontracts.BatchDeleteRequest

// ============================================================================
// Full-archive extraction
// ============================================================================

// BatchExtractRequest extracts one or more complete on-disk archives.
type BatchExtractRequest = internalcontracts.BatchExtractRequest

type BatchExtractResult = internalcontracts.BatchExtractResult

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
type SelectiveExtractItem = internalcontracts.SelectiveExtractItem

type BatchSelectiveExtractRequest = internalcontracts.BatchSelectiveExtractRequest

type BatchSelectiveExtractResult = internalcontracts.BatchSelectiveExtractResult
