package archive

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// Service owns the archive-facing orchestration boundary while delegating
// concrete archive work to injected application closures.
type Service struct {
	archCache      *cache.Cache
	startOperation func(opType contracts.OperationType, title, source, dest string) Operation
	streamsMu      sync.RWMutex
	streamsCancel  map[string]context.CancelFunc
}

type Operation interface {
	ID() string
	Complete()
	SetError(error)
	SetTotal(totalItems, totalBytes int64)
	UpdateProgress(processedItems, processedBytes int64, currentFile string, currentFileSize int64)
	IsCancelled() bool
}

func NewService(
	archCache *cache.Cache,
	startOperation func(opType contracts.OperationType, title, source, dest string) Operation,
) *Service {
	return &Service{
		archCache:      archCache,
		startOperation: startOperation,
		streamsCancel:  make(map[string]context.CancelFunc),
	}
}

func (s *Service) Browse(fsPath string, nestedChain []string, innerPath string) (contracts.ArchiveListing, error) {
	ctx := context.Background()

	// Convert drive:// scheme paths to native OS paths
	nativePath := pathref.ToNativePath(fsPath)

	arch, err := s.archCache.Get(ctx, nativePath, nestedChain, "")
	if err != nil {
		return contracts.ArchiveListing{}, fmt.Errorf("vfs: open %q: %w", nativePath, err)
	}

	vfsEntries, err := arch.ReadDir(ctx, normalizePath(innerPath))
	if err != nil {
		return contracts.ArchiveListing{}, fmt.Errorf("vfs: readdir %q in %q: %w", innerPath, fsPath, err)
	}

	entries := make([]contracts.FileInfo, 0, len(vfsEntries))
	for _, entry := range vfsEntries {
		entries = append(entries, vfsEntryToFileInfo(entry, nestedChain))
	}

	return contracts.ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: nestedChain,
		Entries:     entries,
	}, nil
}

func (s *Service) ReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*contracts.FileContent, error) {
	entryPath = normalizePath(entryPath)

	ctx := context.Background()
	// Convert drive:// scheme paths to native OS paths
	nativePath := pathref.ToNativePath(fsPath)

	arch, err := s.archCache.Get(ctx, nativePath, nestedChain, "")
	if err != nil {
		return nil, fmt.Errorf("archive read: open %q: %w", nativePath, err)
	}

	f, err := arch.Open(ctx, entryPath)
	if err != nil {
		return nil, fmt.Errorf("archive read: open entry %q: %w", entryPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	if encoding == "" {
		encoding = "utf8"
	}

	content := string(data)
	if encoding == "base64" {
		content = base64.StdEncoding.EncodeToString(data)
	} else {
		encoding = "utf8"
	}

	return &contracts.FileContent{
		Path:     entryPath,
		Content:  content,
		Encoding: encoding,
		Size:     int64(len(data)),
	}, nil
}

func (s *Service) ReadFileStream(fsPath string, nestedChain []string, entryPath string, chunkSize int, emitEvent func(string, interface{})) (string, error) {
	entryPath = normalizePath(entryPath)

	if chunkSize <= 0 {
		chunkSize = 64 * 1024 // 64KB default chunk size
	}

	// Generate streamID
	streamID := fmt.Sprintf("archive-read-%d", time.Now().UnixNano())

	// Create cancellation context
	ctx, cancel := context.WithCancel(context.Background())
	s.streamsMu.Lock()
	s.streamsCancel[streamID] = cancel
	s.streamsMu.Unlock()

	// Start streaming in background goroutine
	go func() {
		defer func() {
			s.streamsMu.Lock()
			delete(s.streamsCancel, streamID)
			s.streamsMu.Unlock()
		}()

		// Convert drive:// scheme paths to native OS paths
		nativePath := pathref.ToNativePath(fsPath)

		arch, err := s.archCache.Get(ctx, nativePath, nestedChain, "")
		if err != nil {
			emitEvent("archive:read:error", map[string]interface{}{
				"streamID": streamID,
				"error":    fmt.Sprintf("archive read: open %q: %v", nativePath, err),
			})
			return
		}

		f, err := arch.Open(ctx, entryPath)
		if err != nil {
			emitEvent("archive:read:error", map[string]interface{}{
				"streamID": streamID,
				"error":    fmt.Sprintf("archive read: open entry %q: %v", entryPath, err),
			})
			return
		}
		defer f.Close()

		// Stream file in chunks
		bytesRead := int64(0)
		for {
			select {
			case <-ctx.Done():
				emitEvent("archive:read:cancelled", map[string]interface{}{
					"streamID": streamID,
				})
				return
			default:
			}

			chunk := make([]byte, chunkSize)
			n, err := f.Read(chunk)
			if err != nil && err != io.EOF {
				emitEvent("archive:read:error", map[string]interface{}{
					"streamID": streamID,
					"error":    fmt.Sprintf("archive read: failed to read chunk: %v", err),
				})
				return
			}

			if n > 0 {
				bytesRead += int64(n)
				emitEvent("archive:read:chunk", map[string]interface{}{
					"streamID":  streamID,
					"data":      chunk[:n],
					"bytesRead": bytesRead,
				})
			}

			if err == io.EOF {
				emitEvent("archive:read:done", map[string]interface{}{
					"streamID":   streamID,
					"totalBytes": bytesRead,
				})
				return
			}
		}
	}()

	return streamID, nil
}

func (s *Service) CancelReadStream(streamID string) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()

	if cancel, exists := s.streamsCancel[streamID]; exists {
		cancel()
		delete(s.streamsCancel, streamID)
	}
}

func (s *Service) CopyEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}

	op := s.startOperation(contracts.OpCopy, "Copy from archive", req.FsPath, req.DestinationDir)
	succeeded, failed, errs, err := s.ExtractEntries(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	op.Complete()
	return &contracts.ArchiveEntriesResult{
		OperationID: op.ID(),
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}, nil
}

func (s *Service) MoveEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	if req.DestinationDir == "" {
		return nil, fmt.Errorf("destinationDir is required")
	}
	if len(req.Entries) == 0 {
		return &contracts.ArchiveEntriesResult{}, nil
	}

	op := s.startOperation(contracts.OpMove, "Move from archive", req.FsPath, req.DestinationDir)
	succeeded, failed, errs, err := s.ExtractEntries(req, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	result := &contracts.ArchiveEntriesResult{
		OperationID: op.ID(),
		Total:       succeeded + failed,
		Succeeded:   succeeded,
		Failed:      failed,
		Errors:      errs,
	}

	if failed > 0 {
		op.SetError(fmt.Errorf("%d file(s) failed to extract; archive not modified", failed))
		return result, nil
	}

	if len(req.NestedChain) > 0 {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, contracts.BatchItemError{
			Source: req.FsPath,
			Error:  "entries copied but NOT removed: deleting from nested archives is not supported",
		})
		op.Complete()
		return result, nil
	}

	deleteReq := contracts.ArchiveEntriesRequest{FsPath: req.FsPath, Entries: req.Entries}
	delResult, delErr := s.DeleteEntries(deleteReq)
	if delErr != nil {
		result.SourceRemoved = false
		result.Errors = append(result.Errors, contracts.BatchItemError{
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

func (s *Service) ExtractEntries(req contracts.ArchiveEntriesRequest, op Operation) (succeeded, failed int, errs []contracts.BatchItemError, fatalErr error) {
	if err := os.MkdirAll(req.DestinationDir, 0755); err != nil {
		return 0, 0, nil, fmt.Errorf("create destination: %w", err)
	}

	if len(req.NestedChain) > 0 {
		data, err := s.ExtractNestedArchive(req.FsPath, req.NestedChain)
		if err != nil {
			return 0, 0, nil, err
		}
		return s.extractEntriesFromBytes(data, req.NestedChain[len(req.NestedChain)-1], req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	}

	lower := strings.ToLower(req.FsPath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return s.extractZipSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	case needsArchiveReader(req.FsPath):
		return s.extractArchiveReaderSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	default:
		return s.extractTarSync(req.FsPath, req.Entries, req.StripPrefix, req.DestinationDir, req.FlattenPaths, op)
	}
}

func (s *Service) DeleteEntries(req contracts.ArchiveEntriesRequest) (*contracts.ArchiveEntriesResult, error) {
	if len(req.NestedChain) > 0 {
		return nil, fmt.Errorf("deleting entries from nested archives is not supported")
	}
	if len(req.Entries) == 0 {
		return &contracts.ArchiveEntriesResult{}, nil
	}

	op := s.startOperation(contracts.OpDelete, "Delete from archive", req.FsPath, "")
	op.SetTotal(0, 0)

	removed, kept, rewriteErr := s.rewriteEntries(req.FsPath, req.Entries, op)
	if rewriteErr != nil {
		op.SetError(rewriteErr)
	}

	result := &contracts.ArchiveEntriesResult{
		OperationID:   op.ID(),
		Total:         removed + kept,
		Succeeded:     removed,
		SourceRemoved: rewriteErr == nil,
	}

	if rewriteErr != nil {
		result.Failed = 1
		result.Errors = []contracts.BatchItemError{{Source: req.FsPath, Error: rewriteErr.Error()}}
		return result, rewriteErr
	}

	op.Complete()
	return result, nil
}

func (s *Service) rewriteEntries(fsPath string, entries []string, op Operation) (removed, kept int, err error) {
	lower := strings.ToLower(fsPath)
	switch {
	case isZipArchiveName(fsPath):
		return rewriteZip(fsPath, entries, op)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return rewriteTar(fsPath, entries, "gz", op)
	case strings.HasSuffix(lower, ".tar.bz2"):
		return rewriteTar(fsPath, entries, "bz2", op)
	case strings.HasSuffix(lower, ".tar.xz"):
		return rewriteTar(fsPath, entries, "xz", op)
	case strings.HasSuffix(lower, ".tar"):
		return rewriteTar(fsPath, entries, "", op)
	case needsArchiveReader(fsPath):
		return 0, 0, unsupportedRewriteFormatError(fsPath)
	default:
		return 0, 0, fmt.Errorf("unsupported archive format: %q", fsPath)
	}
}

func unsupportedRewriteFormatError(fsPath string) error {
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(strings.ToLower(fsPath)), "."))
	if ext == "" {
		ext = "UNKNOWN"
	}
	return fmt.Errorf("deleting entries from %s archives is not supported (format is read-only)", ext)
}

func normalizePath(path string) string {
	return strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
}

func vfsEntryToFileInfo(entry vfslib.FileInfo, parentChain []string) contracts.FileInfo {
	entryType := contracts.FileTypeFile
	switch {
	case entry.IsDir():
		entryType = contracts.FileTypeDirectory
	case entry.IsArchive():
		entryType = contracts.FileTypeArchive
	case entry.IsSymlink():
		entryType = contracts.FileTypeSymlink
	}

	var nestedChain []string
	if entryType == contracts.FileTypeArchive {
		nestedChain = append(append([]string{}, parentChain...), entry.Path())
	}

	return contracts.FileInfo{
		Name:        entry.Name(),
		Path:        entry.Path(),
		Type:        entryType,
		Size:        entry.Size(),
		Modified:    entry.ModTime().Unix(),
		IsHidden:    entry.IsHidden(),
		IsSymlink:   entry.IsSymlink(),
		IsNested:    entryType == contracts.FileTypeArchive,
		NestedChain: nestedChain,
	}
}
