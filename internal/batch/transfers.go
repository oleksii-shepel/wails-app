package batch

import (
	"os"
	"path/filepath"

	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

type TransferCallbacks struct {
	CheckConflict                  func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error)
	BatchErr                       func(source, destination string, err error) contracts.BatchItemError
	IsPermissionDenied             func(error) bool
	ShouldRetryDeleteWithElevation func(error) bool
	RunElevatedCopy                func(src, dst string, recursive bool) (bool, error)
	RunElevatedMove                func(src, dst string) (bool, error)
	RunElevatedDelete              func(path string, recursive bool, useTrash *bool) (bool, error)
	MoveToTrash                    func(path string) error
	CalculateSize                  func(path string) (int64, int64)
}

type TransferRunner struct {
	callbacks TransferCallbacks
}

func NewTransferRunner(callbacks TransferCallbacks) *TransferRunner {
	return &TransferRunner{callbacks: callbacks}
}

func (r *TransferRunner) Copy(op *internalops.ActiveOperation, req contracts.BatchRequest) (*contracts.BatchResult, error) {
	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := r.callbacks.CalculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	progress := NewProgressTracker(op)
	fileOps := NewFileOperator(op.IsCancelled, progress)
	result := &contracts.BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *contracts.ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == contracts.ConflictCancel) {
			break
		}

		if _, err := os.Stat(item.Source); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err))
			continue
		}

		finalDst, skip, err := r.callbacks.CheckConflict(item.Source, item.Destination, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		if err := fileOps.CopyPath(item.Source, finalDst, req.Recursive); err != nil {
			if r.callbacks.IsPermissionDenied(err) {
				success, elevErr := r.callbacks.RunElevatedCopy(item.Source, finalDst, req.Recursive)
				if elevErr == nil && success {
					result.Succeeded++
					continue
				}
			}
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, finalDst, err))
			continue
		}
		result.Succeeded++
	}

	progress.Flush()
	op.Complete()
	return result, nil
}

func (r *TransferRunner) Move(op *internalops.ActiveOperation, req contracts.BatchRequest) (*contracts.BatchResult, error) {
	allNoOp := true
	for _, item := range req.Items {
		if !SameDir(filepath.Dir(item.Source), filepath.Dir(item.Destination)) {
			allNoOp = false
			break
		}
	}
	if allNoOp {
		return &contracts.BatchResult{OperationID: op.ID, Total: len(req.Items), Skipped: len(req.Items)}, nil
	}

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := r.callbacks.CalculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	progress := NewProgressTracker(op)
	fileOps := NewFileOperator(op.IsCancelled, progress)
	result := &contracts.BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *contracts.ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == contracts.ConflictCancel) {
			break
		}

		if SameDir(filepath.Dir(item.Source), filepath.Dir(item.Destination)) {
			result.Skipped++
			continue
		}

		srcInfo, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err))
			continue
		}

		finalDst, skip, err := r.callbacks.CheckConflict(item.Source, item.Destination, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		if err = os.Rename(item.Source, finalDst); err != nil {
			if r.callbacks.IsPermissionDenied(err) {
				success, elevErr := r.callbacks.RunElevatedMove(item.Source, finalDst)
				if elevErr == nil && success {
					result.Succeeded++
					continue
				}
			}
			if err2 := fileOps.CopyPath(item.Source, finalDst, true); err2 != nil {
				if r.callbacks.IsPermissionDenied(err2) {
					success, elevErr := r.callbacks.RunElevatedMove(item.Source, finalDst)
					if elevErr == nil && success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, finalDst, err2))
				continue
			}
			if err2 := os.RemoveAll(item.Source); err2 != nil {
				if r.callbacks.IsPermissionDenied(err2) {
					useTrash := false
					success, elevErr := r.callbacks.RunElevatedDelete(item.Source, true, &useTrash)
					if elevErr == nil && success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err2))
				continue
			}
		} else {
			progress.Advance(1, srcInfo.Size(), filepath.Base(item.Source), srcInfo.Size(), false)
		}
		result.Succeeded++
	}

	progress.Flush()
	op.Complete()
	return result, nil
}

func (r *TransferRunner) Delete(op *internalops.ActiveOperation, req contracts.BatchDeleteRequest) (*contracts.BatchResult, error) {
	var totalItems, totalBytes int64
	for _, path := range req.Paths {
		n, b := r.callbacks.CalculateSize(path)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	progress := NewProgressTracker(op)
	fileOps := NewFileOperator(op.IsCancelled, progress)
	result := &contracts.BatchResult{OperationID: op.ID, Total: len(req.Paths)}

	for _, path := range req.Paths {
		if op.IsCancelled() {
			break
		}

		useTrash := req.UseTrash == nil || *req.UseTrash
		if useTrash {
			if err := r.callbacks.MoveToTrash(path); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := true
					success, elevErr := r.callbacks.RunElevatedDelete(path, false, &useTrashElev)
					if elevErr == nil && success {
						progress.Advance(1, 0, filepath.Base(path), 0, false)
						result.Succeeded++
						continue
					}
					if elevErr != nil {
						err = elevErr
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
				continue
			}
			progress.Advance(1, 0, filepath.Base(path), 0, false)
			result.Succeeded++
			continue
		}

		info, err := os.Lstat(path)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
			continue
		}

		if info.IsDir() && req.Recursive {
			if err := fileOps.DeleteDir(path); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					success, elevErr := r.callbacks.RunElevatedDelete(path, true, &useTrashElev)
					if elevErr == nil && success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
				continue
			}
		} else {
			size := info.Size()
			if err := os.Remove(path); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					success, elevErr := r.callbacks.RunElevatedDelete(path, req.Recursive, &useTrashElev)
					if elevErr == nil && success {
						progress.Advance(1, size, filepath.Base(path), size, false)
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
				continue
			}
			progress.Advance(1, size, filepath.Base(path), size, false)
		}
		result.Succeeded++
	}

	progress.Flush()
	op.Complete()
	return result, nil
}
