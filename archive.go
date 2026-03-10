package main

// ────────────────────────────────────────────────────────────────────────────
// Types
// ────────────────────────────────────────────────────────────────────────────

type ArchiveEntriesRequest struct {
	// FsPath is the absolute on-disk path of the archive.
	FsPath string `json:"fsPath"`
	// NestedChain is non-empty when operating on a nested archive.
	// Copy/Move support it; Delete requires it to be empty.
	NestedChain []string `json:"nestedChain"`
	// Entries are the inner paths to act on. Empty = all entries.
	Entries []string `json:"entries"`
	// StripPrefix is removed from destination paths when extracting.
	// Example: stripPrefix="a/b" and entry="a/b/c.txt" -> "c.txt".
	StripPrefix string `json:"stripPrefix,omitempty"`
	// DestinationDir is only used by Copy and Move.
	DestinationDir string `json:"destinationDir"`
	// FlattenPaths strips directory structure on extraction.
	FlattenPaths bool `json:"flattenPaths"`
}

type ArchiveEntriesResult struct {
	OperationID string `json:"operationId"`
	Total       int    `json:"total"`
	Succeeded   int    `json:"succeeded"`
	Failed      int    `json:"failed"`
	// SourceRemoved is false when the delete step was skipped (nested chain).
	SourceRemoved bool             `json:"sourceRemoved"`
	Errors        []BatchItemError `json:"errors,omitempty"`
}
