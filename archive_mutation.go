package main

import (
	"fmt"
)

// ArchiveCopyEntries extracts the listed entries to DestinationDir
// synchronously, blocking until every file is fully written to disk.
//
// BatchSelectiveExtract is NOT used here because:
//  1. It is fire-and-forget (async), so callers cannot know when files are
//     actually on disk.
//  2. It uses matchesEntries which exact-matches directory root headers,
//     writing them as empty files at the destination instead of skipping them.
//
// extractEntriesSync uses matchesEntriesForExtract which correctly skips the
// directory root entry itself while still extracting all of its children.
func (a *App) archiveCopyEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}

	op := a.operations.StartOperation(OpCopy, "Copy from archive",
		req.FsPath, req.DestinationDir)

	succeeded, failed, errs, err := a.extractEntriesSync(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	op.Complete()
	return &ArchiveEntriesResult{
		OperationID: op.ID,
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}, nil
}

// ArchiveMoveEntries extracts entries to DestinationDir synchronously, then
// rewrites the archive without them only after all files are fully on disk.
//
// BatchSelectiveExtract is NOT used here because it is fire-and-forget (async):
// it starts an operation and returns before files are written to disk, so
// ArchiveDeleteEntries would rewrite the archive while extraction is still
// in progress, corrupting or losing files.
//
// extractEntriesSync blocks until every byte is written, then the archive
// rewrite via ArchiveDeleteEntries is safe to run.
func (a *App) archiveMoveEntries(req ArchiveEntriesRequest) (*ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}
	if len(req.Entries) == 0 {
		return &ArchiveEntriesResult{}, nil
	}

	op := a.operations.StartOperation(OpMove, "Move from archive",
		req.FsPath, req.DestinationDir)

	// Phase 1: synchronous extraction — blocks until all files are on disk.
	succeeded, failed, errs, err := a.extractEntriesSync(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	result := &ArchiveEntriesResult{
		OperationID: op.ID,
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}

	if failed > 0 {
		op.SetError(fmt.Errorf("%d file(s) failed to extract; archive not modified", failed))
		return result, nil
	}

	// Phase 2: atomic archive rewrite — safe because phase 1 has completed.
	if len(req.NestedChain) > 0 {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, BatchItemError{
			Source: req.FsPath,
			Error:  "entries copied but NOT removed: deleting from nested archives is not supported",
		})
		op.Complete()
		return result, nil
	}

	deleteReq := ArchiveEntriesRequest{FsPath: req.FsPath, Entries: req.Entries}
	delResult, delErr := a.archiveDeleteEntries(deleteReq)
	if delErr != nil {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, BatchItemError{
			Source: req.FsPath,
			Error:  "entries extracted but NOT removed from archive: " + delErr.Error(),
		})
		op.Complete()
		return result, nil
	}

	result.SourceRemoved = true
	if delResult.Failed > 0 {
		result.Errors = append(result.Errors, delResult.Errors...)
	}
	op.Complete()
	return result, nil
}
