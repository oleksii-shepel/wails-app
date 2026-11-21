package batch

import (
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

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
	GetFileInfo                    func(path string) (*contracts.FileInfo, error)
	CopyPath                       func(source, destination string, recursive bool, streamToFile func(io.Reader, string, int64, string) (int64, error)) error
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

func (r *TransferRunner) getFileInfo(path string) (*contracts.FileInfo, error) {
	if r.callbacks.GetFileInfo == nil {
		return nil, os.ErrNotExist
	}
	return r.callbacks.GetFileInfo(path)
}

func (r *TransferRunner) sourceMetrics(path string) (items, bytes int64) {
	resolvedPath, err := r.resolvePath(path)
	if err == nil {
		if info, statErr := os.Stat(resolvedPath); statErr == nil {
			if info.IsDir() {
				return r.callbacks.CalculateSize(resolvedPath)
			}
			return 1, info.Size()
		}
	}

	info, infoErr := r.getFileInfo(path)
	if infoErr != nil || info == nil {
		return 0, 0
	}
	if info.Type == contracts.FileTypeDirectory {
		return 1, 0
	}
	return 1, info.Size
}

type sourceDescriptor struct {
	name       string
	size       int64
	isDir      bool
	customCopy bool
}

func (r *TransferRunner) describeSource(publicPath, resolvedPath string) (sourceDescriptor, error) {
	info, err := os.Stat(resolvedPath)
	if err == nil {
		return sourceDescriptor{
			name:  filepath.Base(resolvedPath),
			size:  info.Size(),
			isDir: info.IsDir(),
		}, nil
	}

	fileInfo, infoErr := r.getFileInfo(publicPath)
	if infoErr != nil || fileInfo == nil {
		return sourceDescriptor{}, err
	}

	name := fileInfo.Name
	if name == "" {
		name = pathBase(publicPath)
	}

	return sourceDescriptor{
		name:       name,
		size:       fileInfo.Size,
		isDir:      fileInfo.Type == contracts.FileTypeDirectory,
		customCopy: true,
	}, nil
}

func pathBase(path string) string {
	normalized := strings.TrimRight(strings.ReplaceAll(path, "\\", "/"), "/")
	if normalized == "" {
		return ""
	}
	return pathpkg.Base(normalized)
}

func (r *TransferRunner) normalizeItemDestination(item contracts.BatchItem, sourceName string) (string, error) {
	destinationLabel := item.Destination
	resolvedDestination, err := r.resolvePath(destinationLabel)
	if err != nil {
		return "", err
	}

	if item.ExactDestination {
		return destinationLabel, nil
	}

	info, statErr := os.Stat(resolvedDestination)
	if statErr == nil && info.IsDir() {
		normalized := r.toPublicPath(filepath.Join(resolvedDestination, sourceName))
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
		n, b := r.sourceMetrics(item.Source)
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

		source, err := r.describeSource(item.Source, resolvedSource)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, "", err))
			continue
		}

		destinationLabel, err := r.normalizeItemDestination(item, source.name)
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

		copyErr := error(nil)
		if source.customCopy && r.callbacks.CopyPath != nil {
			copyErr = r.callbacks.CopyPath(item.Source, publicDst, req.Recursive, fileOps.StreamToFile)
		} else {
			copyErr = fileOps.CopyPath(resolvedSource, finalDst, req.Recursive)
		}
		if copyErr != nil {
			if r.callbacks.IsPermissionDenied(copyErr) {
				success, elevErr := r.callbacks.RunElevatedCopy(resolvedSource, finalDst, req.Recursive)
				if elevErr == nil && success {
					result.Succeeded++
					result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
						Source: item.Source, Destination: publicDst,
					})
					continue
				}
				if elevErr != nil {
					copyErr = fmt.Errorf("%w (elevated copy failed: %v)", copyErr, elevErr)
				}
			}
			result.Failed++
			result.Errors = append(result.Errors, r.callbacks.BatchErr(item.Source, publicDst, copyErr))
			continue
		}
		result.Succeeded++
		result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
			Source: item.Source, Destination: publicDst,
		})
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
		if srcErr != nil || dstErr != nil || !SamePath(resolvedSource, resolvedDestination) {
			allNoOp = false
			break
		}
	}
	if allNoOp {
		op.Complete()
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
		destinationLabel, err := r.normalizeItemDestination(item, pathBase(item.Source))
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

		if SamePath(resolvedSource, resolvedDestination) {
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
					result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
						Source: item.Source, Destination: publicDst,
					})
					continue
				}
				if elevErr != nil {
					err = fmt.Errorf("%w (elevated move failed: %v)", err, elevErr)
				}
			}
			if err2 := fileOps.CopyPath(resolvedSource, finalDst, true); err2 != nil {
				if r.callbacks.IsPermissionDenied(err2) {
					success, elevErr := r.callbacks.RunElevatedMove(resolvedSource, finalDst)
					if elevErr == nil && success {
						result.Succeeded++
						result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
							Source: item.Source, Destination: publicDst,
						})
						continue
					}
					if elevErr != nil {
						err2 = fmt.Errorf("%w (elevated move failed: %v)", err2, elevErr)
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
						result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
							Source: item.Source, Destination: publicDst,
						})
						continue
					}
					if elevErr != nil {
						err2 = fmt.Errorf("%w (elevated delete failed: %v)", err2, elevErr)
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
		result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{
			Source: item.Source, Destination: publicDst,
		})
	}

	progress.Flush()
	op.Complete()
	return result, nil
}

func (r *TransferRunner) Delete(op *internalops.ActiveOperation, req contracts.BatchDeleteRequest) (*contracts.BatchResult, error) {
	type deleteWork struct {
		items int64
		bytes int64
	}

	work := make(map[string]deleteWork, len(req.Paths))
	var totalItems, totalBytes int64
	for _, path := range req.Paths {
		resolvedPath, err := r.resolvePath(path)
		if err != nil {
			continue
		}
		n, b := r.callbacks.CalculateSize(resolvedPath)
		work[path] = deleteWork{items: n, bytes: b}
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
						itemWork := work[path]
						progress.Advance(
							itemWork.items,
							itemWork.bytes,
							filepath.Base(path),
							itemWork.bytes,
							true,
						)
						result.Succeeded++
						result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{Source: path})
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
			itemWork := work[path]
			progress.Advance(
				itemWork.items,
				itemWork.bytes,
				filepath.Base(path),
				itemWork.bytes,
				true,
			)
			result.Succeeded++
			result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{Source: path})
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
						result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{Source: path})
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
						result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{Source: path})
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
		result.AffectedPaths = append(result.AffectedPaths, contracts.BatchAffectedPath{Source: path})
	}

	progress.Flush()
	op.Complete()
	return result, nil
}
