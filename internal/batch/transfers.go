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
	ResolvePath                    func(path string) (string, error)
	ToPublicPath                   func(path string) string
}

type TransferRunner struct {
	callbacks TransferCallbacks
}

func NewTransferRunner(callbacks TransferCallbacks) *TransferRunner {
	return &TransferRunner{callbacks: callbacks}
}

func (r *TransferRunner) resolvePath(path string) (string, error) {
	if r.callbacks.ResolvePath == nil {
		return path, nil
	}
	return r.callbacks.ResolvePath(path)
}

func (r *TransferRunner) toPublicPath(path string) string {
	if r.callbacks.ToPublicPath == nil {
		return path
	}
	return r.callbacks.ToPublicPath(path)
}

func (r *TransferRunner) normalizeItemDestination(sourceLabel, destinationLabel, resolvedSource string) (string, error) {
	resolvedDestination, err := r.resolvePath(destinationLabel)
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(resolvedDestination)
	if statErr == nil && info.IsDir() {
		normalized := r.toPublicPath(filepath.Join(resolvedDestination, filepath.Base(resolvedSource)))
		return normalized, nil
	}

	if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}

	return destinationLabel, nil
}

func (r *TransferRunner) Copy(op *internalops.ActiveOperation, req contracts.BatchRequest) (*contracts.BatchResult, error) {
	var totalItems, totalBytes int64
	for _, item := range req.Items {
		resolvedSource, err := r.resolvePath(item.Source)
		if err != nil {
			continue
		}
		n, b := r.callbacks.CalculateSize(resolvedSource)
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

		resolvedSource, err := r.resolvePath(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}

		if _, err := os.Stat(resolvedSource); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err))
			continue
		}

		destinationLabel, err := r.normalizeItemDestination(item.Source, item.Destination, resolvedSource)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}

		finalDst, skip, err := r.callbacks.CheckConflict(item.Source, destinationLabel, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, destinationLabel, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}
		publicDst := r.toPublicPath(finalDst)

		if err := fileOps.CopyPath(resolvedSource, finalDst, req.Recursive); err != nil {
			if r.callbacks.IsPermissionDenied(err) {
				success, elevErr := r.callbacks.RunElevatedCopy(resolvedSource, finalDst, req.Recursive)
				if elevErr == nil && success {
					result.Succeeded++
					continue
				}
			}
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, publicDst, err))
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
		resolvedSource, srcErr := r.resolvePath(item.Source)
		resolvedDestination, dstErr := r.resolvePath(item.Destination)
		if srcErr != nil || dstErr != nil || !SameDir(filepath.Dir(resolvedSource), filepath.Dir(resolvedDestination)) {
			allNoOp = false
			break
		}
	}
	if allNoOp {
		return &contracts.BatchResult{OperationID: op.ID, Total: len(req.Items), Skipped: len(req.Items)}, nil
	}

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		resolvedSource, err := r.resolvePath(item.Source)
		if err != nil {
			continue
		}
		n, b := r.callbacks.CalculateSize(resolvedSource)
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

		resolvedSource, err := r.resolvePath(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}
		destinationLabel, err := r.normalizeItemDestination(item.Source, item.Destination, resolvedSource)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, item.Destination, err))
			continue
		}
		resolvedDestination, err := r.resolvePath(destinationLabel)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, destinationLabel, err))
			continue
		}

		if SameDir(filepath.Dir(resolvedSource), filepath.Dir(resolvedDestination)) {
			result.Skipped++
			continue
		}

		srcInfo, err := os.Stat(resolvedSource)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err))
			continue
		}

		finalDst, skip, err := r.callbacks.CheckConflict(item.Source, destinationLabel, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, destinationLabel, err))
			continue
		}
		if skip {
			result.Skipped++
			continue
		}
		publicDst := r.toPublicPath(finalDst)

		if err = os.Rename(resolvedSource, finalDst); err != nil {
			if r.callbacks.IsPermissionDenied(err) {
				success, elevErr := r.callbacks.RunElevatedMove(resolvedSource, finalDst)
				if elevErr == nil && success {
					result.Succeeded++
					continue
				}
			}
			if err2 := fileOps.CopyPath(resolvedSource, finalDst, true); err2 != nil {
				if r.callbacks.IsPermissionDenied(err2) {
					success, elevErr := r.callbacks.RunElevatedMove(resolvedSource, finalDst)
					if elevErr == nil && success {
						result.Succeeded++
						continue
					}
				}
				result.Failed++
				result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, publicDst, err2))
				continue
			}
			if err2 := os.RemoveAll(resolvedSource); err2 != nil {
				if r.callbacks.IsPermissionDenied(err2) {
					useTrash := false
					success, elevErr := r.callbacks.RunElevatedDelete(resolvedSource, true, &useTrash)
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
		resolvedPath, err := r.resolvePath(path)
		if err != nil {
			continue
		}
		n, b := r.callbacks.CalculateSize(resolvedPath)
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

		resolvedPath, err := r.resolvePath(path)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
			continue
		}

		useTrash := req.UseTrash == nil || *req.UseTrash
		if useTrash {
			if err := r.callbacks.MoveToTrash(resolvedPath); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := true
					success, elevErr := r.callbacks.RunElevatedDelete(resolvedPath, false, &useTrashElev)
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

		info, err := os.Lstat(resolvedPath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(path, "", err))
			continue
		}

		if info.IsDir() && req.Recursive {
			if err := fileOps.DeleteDir(resolvedPath); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					success, elevErr := r.callbacks.RunElevatedDelete(resolvedPath, true, &useTrashElev)
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
			if err := os.Remove(resolvedPath); err != nil {
				if r.callbacks.ShouldRetryDeleteWithElevation(err) {
					useTrashElev := false
					success, elevErr := r.callbacks.RunElevatedDelete(resolvedPath, req.Recursive, &useTrashElev)
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
