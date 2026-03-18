package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	internalbatch "wails-app/internal/batch"
)

// batchErr builds a BatchItemError with the error message and its classified code.
// Use this helper everywhere a BatchItemError needs to be recorded.
func batchErr(source, destination string, err error) BatchItemError {
	return BatchItemError{
		Source:      source,
		Destination: destination,
		Error:       err.Error(),
		Code:        classifyError(err),
	}
}

var globalConflicts = internalbatch.NewConflictManager()

// ============================================================================
// Throttled progress
// ============================================================================

type batchProgress struct {
	tracker *internalbatch.ProgressTracker
}

func newBatchProgress(op *ActiveOperation) *batchProgress {
	return &batchProgress{tracker: internalbatch.NewProgressTracker(op)}
}

func (bp *batchProgress) update(items, byt int64, file string, fileSize int64, force bool) {
	bp.tracker.Set(items, byt, file, fileSize, force)
}

func (bp *batchProgress) flush() {
	bp.tracker.Flush()
}

func (bp *batchProgress) snapshot() (int64, int64) {
	return bp.tracker.Snapshot()
}

func (bp *batchProgress) advance(itemDelta, byteDelta int64, file string, fileSize int64, force bool) (int64, int64) {
	return bp.tracker.Advance(itemDelta, byteDelta, file, fileSize, force)
}

// ============================================================================
// checkConflict checks whether dst already exists and, if so, either applies
// blocks until the frontend calls ResolveConflict.
//
// blanket is **ConflictResolution so the caller's variable is updated when
// ApplyToAll is set — no extra return value needed.
//
// Returns (finalDst, skip, err):
//   finalDst — path to write to (may differ from dst on ConflictRename)
//   skip     — true: count as skipped and move on
//   err      — non-nil only on stat error or uniqueDestination failure

func (s *batchCoreService) checkConflict(
	op *ActiveOperation,
	srcLabel, dst string,
	blanket **ConflictResolution,
) (finalDst string, skip bool, err error) {
	dstInfo, statErr := os.Lstat(dst)
	if os.IsNotExist(statErr) {
		return dst, false, nil // no conflict
	}
	if statErr != nil {
		return "", false, statErr
	}

	// Apply stored blanket without prompting.
	if *blanket != nil {
		return internalbatch.ApplyResolution(**blanket, dst)
	}

	// Gather source metadata for the event (best-effort).
	var srcSize, srcMtime int64
	if info, serr := os.Lstat(srcLabel); serr == nil {
		srcSize = info.Size()
		srcMtime = info.ModTime().Unix()
	}

	conflictID := globalConflicts.NextID(op.ID)
	evt := ConflictEvent{
		OperationID: op.ID,
		ConflictID:  conflictID,
		SourcePath:  srcLabel,
		DestPath:    dst,
		SourceSize:  srcSize,
		DestSize:    dstInfo.Size(),
		SourceMtime: srcMtime,
		DestMtime:   dstInfo.ModTime().Unix(),
		IsDirectory: dstInfo.IsDir(),
	}

	// Register before emitting to avoid a race if the frontend responds instantly.
	response := globalConflicts.Register(conflictID)
	if s.emitEvent != nil {
		s.emitEvent("operation:conflict", evt)
	}

	select {
	case res := <-response:
		if res.ApplyToAll {
			r := res.Resolution
			*blanket = &r
		}
		return internalbatch.ApplyResolution(res.Resolution, dst)
	case <-op.CancelSignal():
		globalConflicts.CancelAll()
		return "", true, fmt.Errorf("operation cancelled")
	}
}

// ============================================================================
// Batch Copy
// ============================================================================

func (s *batchCoreService) BatchCopy(req BatchRequest) (*BatchResult, error) {
	op := s.operations.StartOperation(OpCopy, "Batch copy", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	fileOps := internalbatch.NewFileOperator(op.IsCancelled, bp.tracker)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		_, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.Source, "", err))
			continue
		}

		dst := item.Destination

		finalDst, skip, err := s.checkConflict(op, item.Source, dst, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.Source, dst, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		if err := fileOps.CopyPath(item.Source, finalDst, req.Recursive); err != nil {
			if isPermissionDenied(err) {
				resp, elevErr := s.runElevatedWithEvents("copy", opTargetPair(item.Source, finalDst), elevatedRequest{
					Op:          "copy",
					Source:      item.Source,
					Destination: finalDst,
					Recursive:   req.Recursive,
				})
				if elevErr == nil && resp.Success {
					result.Succeeded++
					continue
				}
			}
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.Source, finalDst, err))
		} else {
			result.Succeeded++
		}
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Move
// ============================================================================

func (s *batchCoreService) BatchMove(req BatchRequest) (*BatchResult, error) {
	// Pre-scan: if every item is a no-op move within the same directory, skip the operation entirely.
	allNoOp := true
	for _, item := range req.Items {
		srcDir := filepath.Dir(item.Source)
		dstDir := filepath.Dir(item.Destination)
		if !internalbatch.SameDir(srcDir, dstDir) {
			allNoOp = false
			break
		}
	}
	if allNoOp {
		return &BatchResult{Total: len(req.Items), Skipped: len(req.Items)}, nil
	}

	op := s.operations.StartOperation(OpMove, "Batch move", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	fileOps := internalbatch.NewFileOperator(op.IsCancelled, bp.tracker)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		// No-op: moving within the same directory should do nothing.
		// This avoids surfacing a conflict dialog and mirrors common UX (skip silently).
		srcDir := filepath.Dir(item.Source)
		dstDir := filepath.Dir(item.Destination)
		if internalbatch.SameDir(srcDir, dstDir) {
			result.Skipped++
			continue
		}

		srcInfo, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.Source, "", err))
			continue
		}

		dst := item.Destination

		// For directories, always use the destination as provided (avoid nesting duplicate folder names)
		finalDst := dst
		skip := false
		finalDst, skip, err = s.checkConflict(op, item.Source, dst, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.Source, dst, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		// Atomic rename first; cross-device falls back to copy+delete.
		if err = os.Rename(item.Source, finalDst); err != nil {
			if isPermissionDenied(err) {
				resp, elevErr := s.runElevatedWithEvents("move", opTargetPair(item.Source, finalDst), elevatedRequest{
					Op:          "move",
					Source:      item.Source,
					Destination: finalDst,
				})
				if elevErr == nil && resp.Success {
					result.Succeeded++
					continue
				}
			}
			if err2 := fileOps.CopyPath(item.Source, finalDst, true); err2 != nil {
				if isPermissionDenied(err2) {
					resp, elevErr := s.runElevatedWithEvents("move", opTargetPair(item.Source, finalDst), elevatedRequest{
						Op:          "move",
						Source:      item.Source,
						Destination: finalDst,
					})
					if elevErr == nil && resp.Success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, batchErr(item.Source, finalDst, err2))
				continue
			}
			if err2 := os.RemoveAll(item.Source); err2 != nil {
				if isPermissionDenied(err2) {
					useTrash := false
					resp, elevErr := s.runElevatedWithEvents("delete", opTargetSingle(item.Source), elevatedRequest{
						Op:        "delete",
						Path:      item.Source,
						Recursive: true,
						UseTrash:  &useTrash,
					})
					if elevErr == nil && resp.Success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, batchErr(item.Source, "", err2))
				continue
			}
		} else {
			// Atomic rename: advance counters without a byte loop.
			bp.advance(1, srcInfo.Size(), filepath.Base(item.Source), srcInfo.Size(), false)
		}
		result.Succeeded++
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Delete
// ============================================================================

func (s *batchCoreService) BatchDelete(req BatchDeleteRequest) (*BatchResult, error) {
	op := s.operations.StartOperation(OpDelete, "Batch delete", "", "")

	var totalItems, totalBytes int64
	for _, p := range req.Paths {
		n, b := calculateSize(p)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	fileOps := internalbatch.NewFileOperator(op.IsCancelled, bp.tracker)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Paths)}

	for _, p := range req.Paths {
		if op.IsCancelled() {
			break
		}

		useTrash := req.UseTrash == nil || *req.UseTrash
		if useTrash {
			// Delegate entirely to the OS trash / recycle bin. The OS handles
			// directories recursively, so no need to walk them ourselves.
			if err := moveToTrash(p); err != nil {
				if shouldRetryDeleteWithElevation(err) {
					useTrashElev := true
					resp, elevErr := s.runElevatedWithEvents("delete", opTargetSingle(p), elevatedRequest{
						Op:       "delete",
						Path:     p,
						UseTrash: &useTrashElev,
					})
					if elevErr == nil && resp.Success {
						bp.advance(1, 0, filepath.Base(p), 0, false)
						result.Succeeded++
						continue
					}
					if elevErr != nil {
						err = elevErr
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, batchErr(p, "", err))
				continue
			}
			bp.advance(1, 0, filepath.Base(p), 0, false)
			result.Succeeded++
			continue
		}

		info, err := os.Lstat(p)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(p, "", err))
			continue
		}

		if info.IsDir() && req.Recursive {
			if err := fileOps.DeleteDir(p); err != nil {
				if shouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					resp, elevErr := s.runElevatedWithEvents("delete", opTargetSingle(p), elevatedRequest{
						Op:        "delete",
						Path:      p,
						Recursive: true,
						UseTrash:  &useTrashElev,
					})
					if elevErr == nil && resp.Success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, batchErr(p, "", err))
				continue
			}
		} else {
			size := info.Size()
			if err := os.Remove(p); err != nil {
				if shouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					resp, elevErr := s.runElevatedWithEvents("delete", opTargetSingle(p), elevatedRequest{
						Op:        "delete",
						Path:      p,
						Recursive: req.Recursive,
						UseTrash:  &useTrashElev,
					})
					if elevErr == nil && resp.Success {
						bp.advance(1, size, filepath.Base(p), size, false)
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, batchErr(p, "", err))
				continue
			}
			bp.advance(1, size, filepath.Base(p), size, false)
		}
		result.Succeeded++
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Extract — full archives
// ============================================================================

func (s *batchCoreService) BatchExtract(req BatchExtractRequest) (*BatchExtractResult, error) {
	op := s.operations.StartOperation(OpExtract, "Batch extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Archives)), 0)

	bp := newBatchProgress(op)
	fileOps := internalbatch.NewFileOperator(op.IsCancelled, bp.tracker)
	extractor := internalbatch.NewArchiveExtractor(internalbatch.ExtractCallbacks{
		IsCancelled: op.IsCancelled,
		CheckConflict: func(srcLabel, dst string, blanket **ConflictResolution) (string, bool, error) {
			return s.checkConflict(op, srcLabel, dst, blanket)
		},
		StreamToFile: func(r io.Reader, dst string, size int64, label string) (int64, error) {
			return fileOps.StreamToFile(r, dst, size, label)
		},
		ArchiveEntryPath: archiveEntryPath,
		Normalize:        normalize,
		IsArchive:        isArchive,
	})
	result := &BatchExtractResult{OperationID: op.ID, Total: len(req.Archives)}
	var blanket *ConflictResolution

	for _, archivePath := range req.Archives {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		destRoot := req.DestinationDir
		if req.CreateSubdirectory {
			destRoot = filepath.Join(req.DestinationDir, internalbatch.ArchiveBaseName(archivePath))
		}

		files, byt, nextBlanket, err := extractor.ExtractOnDisk(archivePath, &req, destRoot, blanket)
		blanket = nextBlanket

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(archivePath, destRoot, err))
			continue
		}
		result.Succeeded++
		result.ExtractedFiles += files
		result.ExtractedBytes += byt

		bp.advance(1, 0, filepath.Base(archivePath), 0, false)
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Selective Extract — specific files from archives / nested chains
// ============================================================================
//
// Memory model:
//   Top-level archives  → only matched entries buffered; streamed from disk.
//   Nested archives     → innermost archive resolved in memory (no seekable
//                         handle exists), then only matched entries buffered.
//                         Intermediate archive bytes released after collection.

func (s *batchCoreService) BatchSelectiveExtract(req BatchSelectiveExtractRequest) (*BatchSelectiveExtractResult, error) {
	op := s.operations.StartOperation(OpExtract, "Selective extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Items)), 0)

	bp := newBatchProgress(op)
	extractor := internalbatch.NewSelectiveExtractor(
		internalbatch.SelectiveExtractCallbacks{
			IsCancelled:          op.IsCancelled,
			CheckConflict:        func(srcLabel, dst string, blanket **ConflictResolution) (string, bool, error) { return s.checkConflict(op, srcLabel, dst, blanket) },
			ExtractNestedArchive: s.extractNestedArchive,
			Normalize:            normalize,
			ArchiveEntryPath:     archiveEntryPath,
		},
		bp.tracker,
		op.AddTotalBytes,
	)
	result := &BatchSelectiveExtractResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		files, byt, nextBlanket, err := extractor.ExtractItem(item, req.DestinationDir, blanket)
		blanket = nextBlanket
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, batchErr(item.ArchivePath, "", err))
		} else {
			result.Succeeded++
			result.ExtractedFiles += files
			result.ExtractedBytes += byt
		}

		bp.advance(1, 0, filepath.Base(item.ArchivePath), 0, false)
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Frontend binding
// ============================================================================

// ResolveConflict unblocks a paused operation by supplying the user's decision.
// conflictID comes from ConflictEvent.ConflictID emitted on "operation:conflict".
// Returns false when the ID is not recognised (already resolved or cancelled).
func (s *batchCoreService) ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool {
	return globalConflicts.Resolve(conflictID, ConflictResolutionRequest{
		Resolution: resolution,
		ApplyToAll: applyToAll,
	})
}
