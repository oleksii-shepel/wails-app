package main

import (
	"fmt"
)

// ArchiveDeleteEntries rewrites the archive atomically (temp file → rename),
// omitting the listed entries. Emits operation:start / :progress / :complete.
// NestedChain must be empty — nested rewrite is not supported.
func (a *App) archiveDeleteEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if len(req.NestedChain) > 0 {
		return nil, fmt.Errorf("deleting entries from nested archives is not supported")
	}
	if len(req.Entries) == 0 {
		return &ArchiveEntriesResult{}, nil
	}

	op := a.operations.StartOperation(OpDelete, "Delete from archive",
		req.FsPath, "")
	op.SetTotal(0, 0)

	var (
		removed, kept int
		rewriteErr    error
	)

	removed, kept, rewriteErr = rewriteArchiveByFormat(req.FsPath, req.Entries, op)
	if rewriteErr != nil {
		op.SetError(rewriteErr)
	}

	result := &ArchiveEntriesResult{
		OperationID:   op.ID,
		Total:         removed + kept,
		Succeeded:     removed,
		SourceRemoved: rewriteErr == nil,
	}

	if rewriteErr != nil {
		result.Failed = 1
		result.Errors = []BatchItemError{{Source: req.FsPath, Error: rewriteErr.Error()}}
		return result, rewriteErr
	}

	op.Complete()
	return result, nil
}
