package main

import "fmt"

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
