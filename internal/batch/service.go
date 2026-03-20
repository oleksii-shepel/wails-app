package batch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"

	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

type ConflictPrompter interface {
	PromptConflict(event contracts.ConflictEvent)
}

type ConflictPrompterFunc func(event contracts.ConflictEvent)

func (fn ConflictPrompterFunc) PromptConflict(event contracts.ConflictEvent) {
	if fn != nil {
		fn(event)
	}
}

type ServiceConfig struct {
	StartOperation                 func(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation
	ConflictPrompter               ConflictPrompter
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
	ExtractNestedArchive           func(archivePath string, nestedChain []string) ([]byte, error)
	Normalize                      func(string) string
	ArchiveEntryPath               func(archiver.File) string
	IsArchive                      func(string) bool
}

type Service struct {
	config    ServiceConfig
	conflicts *ConflictManager
}

func NewService(config ServiceConfig) *Service {
	return &Service{config: config, conflicts: NewConflictManager()}
}

func (s *Service) resolvePath(path string) (string, error) {
	if s.config.ResolvePath == nil {
		return path, nil
	}
	return s.config.ResolvePath(path)
}

func (s *Service) toPublicPath(path string) string {
	if s.config.ToPublicPath == nil {
		return path
	}
	return s.config.ToPublicPath(path)
}

func (s *Service) checkConflict(
	op *internalops.ActiveOperation,
	srcLabel, dst string,
	blanket **contracts.ConflictResolution,
) (finalDst string, skip bool, err error) {
	resolvedDst, err := s.resolvePath(dst)
	if err != nil {
		return "", false, err
	}

	dstInfo, statErr := os.Lstat(resolvedDst)
	if os.IsNotExist(statErr) {
		return resolvedDst, false, nil
	}
	if statErr != nil {
		return "", false, statErr
	}

	if *blanket != nil {
		return ApplyResolution(**blanket, resolvedDst)
	}

	var srcSize, srcMtime int64
	resolvedSrc, resolveErr := s.resolvePath(srcLabel)
	if resolveErr == nil {
		if info, sourceErr := os.Lstat(resolvedSrc); sourceErr == nil {
			srcSize = info.Size()
			srcMtime = info.ModTime().Unix()
		}
	} else if info, sourceErr := os.Lstat(srcLabel); sourceErr == nil {
		srcSize = info.Size()
		srcMtime = info.ModTime().Unix()
	}

	conflictID := s.conflicts.NextID(op.ID)
	response := s.conflicts.Register(conflictID)
	if s.config.ConflictPrompter != nil {
		s.config.ConflictPrompter.PromptConflict(contracts.ConflictEvent{
			OperationID: op.ID,
			ConflictID:  conflictID,
			SourcePath:  srcLabel,
			DestPath:    dst,
			SourceSize:  srcSize,
			DestSize:    dstInfo.Size(),
			SourceMtime: srcMtime,
			DestMtime:   dstInfo.ModTime().Unix(),
			IsDirectory: dstInfo.IsDir(),
		})
	}

	select {
	case res := <-response:
		if res.ApplyToAll {
			r := res.Resolution
			*blanket = &r
		}
		return ApplyResolution(res.Resolution, resolvedDst)
	case <-op.CancelSignal():
		s.conflicts.CancelAll()
		return "", true, fmt.Errorf("operation cancelled")
	}
}

func (s *Service) newTransferRunner(op *internalops.ActiveOperation) *TransferRunner {
	return NewTransferRunner(TransferCallbacks{
		CheckConflict: func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error) {
			return s.checkConflict(op, srcLabel, dst, blanket)
		},
		BatchErr:                       s.config.BatchErr,
		IsPermissionDenied:             s.config.IsPermissionDenied,
		ShouldRetryDeleteWithElevation: s.config.ShouldRetryDeleteWithElevation,
		RunElevatedCopy:                s.config.RunElevatedCopy,
		RunElevatedMove:                s.config.RunElevatedMove,
		RunElevatedDelete:              s.config.RunElevatedDelete,
		MoveToTrash:                    s.config.MoveToTrash,
		CalculateSize:                  s.config.CalculateSize,
		ResolvePath:                    s.config.ResolvePath,
		ToPublicPath:                   s.config.ToPublicPath,
	})
}

func (s *Service) BatchCopy(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	op := s.config.StartOperation(contracts.OpCopy, "Batch copy", "", "")
	return s.newTransferRunner(op).Copy(op, req)
}

func (s *Service) BatchMove(req contracts.BatchRequest) (*contracts.BatchResult, error) {
	op := s.config.StartOperation(contracts.OpMove, "Batch move", "", "")
	return s.newTransferRunner(op).Move(op, req)
}

func (s *Service) BatchDelete(req contracts.BatchDeleteRequest) (*contracts.BatchResult, error) {
	op := s.config.StartOperation(contracts.OpDelete, "Batch delete", "", "")
	return s.newTransferRunner(op).Delete(op, req)
}

func (s *Service) BatchExtract(req contracts.BatchExtractRequest) (*contracts.BatchExtractResult, error) {
	op := s.config.StartOperation(contracts.OpExtract, "Batch extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Archives)), 0)

	progress := NewProgressTracker(op)
	fileOps := NewFileOperator(op.IsCancelled, progress)
	extractor := NewArchiveExtractor(ExtractCallbacks{
		IsCancelled: op.IsCancelled,
		CheckConflict: func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error) {
			return s.checkConflict(op, srcLabel, dst, blanket)
		},
		StreamToFile: func(r io.Reader, dst string, size int64, label string) (int64, error) {
			return fileOps.StreamToFile(r, dst, size, label)
		},
		ArchiveEntryPath: s.config.ArchiveEntryPath,
		Normalize:        s.config.Normalize,
		IsArchive:        s.config.IsArchive,
	})
	result := &contracts.BatchExtractResult{OperationID: op.ID, Total: len(req.Archives)}
	var blanket *contracts.ConflictResolution

	for _, archivePath := range req.Archives {
		if op.IsCancelled() || (blanket != nil && *blanket == contracts.ConflictCancel) {
			break
		}

		destRoot := req.DestinationDir
		if req.CreateSubdirectory {
			destRoot = filepath.Join(req.DestinationDir, ArchiveBaseName(archivePath))
		}

		files, bytes, nextBlanket, err := extractor.ExtractOnDisk(archivePath, &req, destRoot, blanket)
		blanket = nextBlanket
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, s.config.BatchErr(archivePath, destRoot, err))
			continue
		}

		result.Succeeded++
		result.ExtractedFiles += files
		result.ExtractedBytes += bytes
		progress.Advance(1, 0, filepath.Base(archivePath), 0, false)
	}

	progress.Flush()
	op.Complete()
	return result, nil
}

func (s *Service) BatchSelectiveExtract(req contracts.BatchSelectiveExtractRequest) (*contracts.BatchSelectiveExtractResult, error) {
	op := s.config.StartOperation(contracts.OpExtract, "Selective extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Items)), 0)

	progress := NewProgressTracker(op)
	extractor := NewSelectiveExtractor(
		SelectiveExtractCallbacks{
			IsCancelled: op.IsCancelled,
			CheckConflict: func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error) {
				return s.checkConflict(op, srcLabel, dst, blanket)
			},
			ExtractNestedArchive: s.config.ExtractNestedArchive,
			Normalize:            s.config.Normalize,
			ArchiveEntryPath:     s.config.ArchiveEntryPath,
		},
		progress,
		op.AddTotalBytes,
	)
	result := &contracts.BatchSelectiveExtractResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *contracts.ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == contracts.ConflictCancel) {
			break
		}

		files, bytes, nextBlanket, err := extractor.ExtractItem(item, req.DestinationDir, blanket)
		blanket = nextBlanket
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, s.config.BatchErr(item.ArchivePath, "", err))
		} else {
			result.Succeeded++
			result.ExtractedFiles += files
			result.ExtractedBytes += bytes
		}

		progress.Advance(1, 0, filepath.Base(item.ArchivePath), 0, false)
	}

	progress.Flush()
	op.Complete()
	return result, nil
}

func (s *Service) ResolveConflict(conflictID string, resolution contracts.ConflictResolution, applyToAll bool) bool {
	return s.conflicts.Resolve(conflictID, contracts.ConflictResolutionRequest{
		Resolution: resolution,
		ApplyToAll: applyToAll,
	})
}
