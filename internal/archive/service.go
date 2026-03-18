package archive

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	contracts "wails-app/internal/contracts"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

// Service owns the archive-facing orchestration boundary while delegating
// concrete archive work to injected application closures.
type Service struct {
	archCache      *cache.Cache
	startOperation func(opType contracts.OperationType, title, source, dest string) Operation
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
	}
}

func (s *Service) Browse(fsPath string, nestedChain []string, innerPath string) (contracts.ArchiveListing, error) {
	ctx := context.Background()

	arch, err := s.archCache.Get(ctx, fsPath, nestedChain, "")
	if err != nil {
		return contracts.ArchiveListing{}, fmt.Errorf("vfs: open %q: %w", fsPath, err)
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
	arch, err := s.archCache.Get(ctx, fsPath, nestedChain, "")
	if err != nil {
		return nil, fmt.Errorf("archive read: open %q: %w", fsPath, err)
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
		Modified:    entry.ModTime().UnixMilli(),
		IsHidden:    entry.IsHidden(),
		IsSymlink:   entry.IsSymlink(),
		IsNested:    entryType == contracts.FileTypeArchive,
		NestedChain: nestedChain,
	}
}