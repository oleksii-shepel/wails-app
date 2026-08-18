package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
)

const dirStreamBatchSize = 256

// DirStreamOptions describes presentation work that must never run on the
// renderer thread for a large directory.
type DirStreamOptions struct {
	SortField        string `json:"sortField,omitempty"`
	SortDirection    string `json:"sortDirection,omitempty"`
	DirectoriesFirst bool   `json:"directoriesFirst"`
	FilterText       string `json:"filterText,omitempty"`
}

type DirStreamWindow struct {
	StreamID    string               `json:"streamId"`
	PaneID      string               `json:"paneId"`
	Path        string               `json:"path"`
	Offset      int                  `json:"offset"`
	Limit       int                  `json:"limit"`
	TotalCount  int                  `json:"totalCount"`
	SourceCount int                  `json:"sourceCount"`
	Entries     []contracts.FileInfo `json:"entries"`
}

type dirStreamMetaEvent struct {
	StreamID   string `json:"streamId"`
	PaneID     string `json:"paneId"`
	Path       string `json:"path"`
	ShowHidden bool   `json:"showHidden"`
}

type dirStreamBatchEvent struct {
	StreamID string               `json:"streamId"`
	PaneID   string               `json:"paneId"`
	Path     string               `json:"path"`
	Entries  []contracts.FileInfo `json:"entries"`
}

type dirStreamDoneEvent struct {
	StreamID    string `json:"streamId"`
	PaneID      string `json:"paneId"`
	Path        string `json:"path"`
	EntryCount  int    `json:"entryCount"`
	SourceCount int    `json:"sourceCount"`
	TotalSize   int64  `json:"totalSize"`
	DurationMS  int64  `json:"durationMs"`
	FromElevate bool   `json:"fromElevate"`
}

type dirStreamErrorEvent struct {
	StreamID string `json:"streamId"`
	PaneID   string `json:"paneId"`
	Path     string `json:"path"`
	Error    string `json:"error"`
}

type ElevatedListFunc func(path string, showHidden bool) ([]contracts.FileInfo, error)
type BuildFileInfoFunc func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
type IsHiddenFunc func(name, fullPath string) bool
type ResolvePublicPathFunc func(path string) (string, error)
type VFSListDirFunc func(path string) (*contracts.DirectoryListing, error)
type IsVFSPathFunc func(path string) bool

type DirStreamService struct {
	mu           sync.Mutex
	streams      map[string]context.CancelFunc
	prepared     map[string]preparedDirStream
	emitEvent    func(string, interface{})
	elevatedList ElevatedListFunc
	buildFile    BuildFileInfoFunc
	isHidden     IsHiddenFunc
	resolvePath  ResolvePublicPathFunc
	vfsListDir   VFSListDirFunc
	isVFSPath    IsVFSPathFunc
}

type preparedDirStream struct {
	paneID      string
	path        string
	sourceCount int
	totalSize   int64
	entries     []contracts.FileInfo
}

func NewDirStreamService(
	emitEvent func(string, interface{}),
	elevatedList ElevatedListFunc,
	buildFile BuildFileInfoFunc,
	isHidden IsHiddenFunc,
	resolvePath ResolvePublicPathFunc,
) *DirStreamService {
	return &DirStreamService{
		streams:      make(map[string]context.CancelFunc),
		prepared:     make(map[string]preparedDirStream),
		emitEvent:    emitEvent,
		elevatedList: elevatedList,
		buildFile:    buildFile,
		isHidden:     isHidden,
		resolvePath:  resolvePath,
	}
}

func (s *DirStreamService) SetVFS(listDir VFSListDirFunc, isVFS IsVFSPathFunc) {
	s.vfsListDir = listDir
	s.isVFSPath = isVFS
}

func (s *DirStreamService) Emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}

func (s *DirStreamService) Register(streamID string, cancel context.CancelFunc) {
	s.mu.Lock()
	s.streams[streamID] = cancel
	s.mu.Unlock()
}

func (s *DirStreamService) Unregister(streamID string) {
	s.mu.Lock()
	delete(s.streams, streamID)
	s.mu.Unlock()
}

func (s *DirStreamService) rememberPrepared(streamID string, stream preparedDirStream) {
	s.mu.Lock()
	s.prepared[streamID] = stream
	s.mu.Unlock()
}

func (s *DirStreamService) Window(streamID string, offset, limit int) (*DirStreamWindow, error) {
	s.mu.Lock()
	stream, ok := s.prepared[streamID]
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("directory stream %q is unavailable", streamID)
	}

	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	if offset > len(stream.entries) {
		offset = len(stream.entries)
	}
	end := offset + limit
	if end > len(stream.entries) {
		end = len(stream.entries)
	}
	windowEntries := stream.entries[offset:end]
	if windowEntries == nil {
		windowEntries = []contracts.FileInfo{}
	}

	return &DirStreamWindow{
		StreamID:    streamID,
		PaneID:      stream.paneID,
		Path:        stream.path,
		Offset:      offset,
		Limit:       limit,
		TotalCount:  len(stream.entries),
		SourceCount: stream.sourceCount,
		Entries:     windowEntries,
	}, nil
}

func (s *DirStreamService) Start(paneID, path string, showHidden bool) (string, error) {
	return s.StartWithOptions(paneID, path, showHidden, DirStreamOptions{
		SortField: "name", SortDirection: "asc", DirectoriesFirst: true,
	})
}

func (s *DirStreamService) StartWithOptions(paneID, path string, showHidden bool, options DirStreamOptions) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	streamID := fmt.Sprintf("%s-%d", paneID, time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	s.Register(streamID, cancel)
	go s.run(ctx, streamID, paneID, internalplatform.ToSlash(path), showHidden, options)
	return streamID, nil
}

func (s *DirStreamService) Cancel(streamID string) {
	if streamID == "" {
		return
	}
	s.mu.Lock()
	cancel := s.streams[streamID]
	delete(s.streams, streamID)
	delete(s.prepared, streamID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *DirStreamService) CancelAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.streams))
	for id, cancel := range s.streams {
		delete(s.streams, id)
		delete(s.prepared, id)
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *DirStreamService) run(ctx context.Context, streamID, paneID, path string, showHidden bool, options DirStreamOptions) {
	defer s.Unregister(streamID)

	started := time.Now()

	s.Emit("fs:listStream:meta", dirStreamMetaEvent{
		StreamID:   streamID,
		PaneID:     paneID,
		Path:       path,
		ShowHidden: showHidden,
	})

	scan := s.scanDirectoryStream
	if pathref.IsSearch(path) {
		scan = s.scanSearchStream
	} else if s.isVFSPath != nil && s.isVFSPath(path) {
		scan = s.scanVFSStream
	}

	entries := make([]contracts.FileInfo, 0, dirStreamBatchSize)
	err := scan(ctx, path, showHidden, dirStreamBatchSize, func(batch []contracts.FileInfo, _ int64) {
		entries = append(entries, batch...)
	})
	if err == nil {
		sourceCount := len(entries)
		entries = prepareStreamEntries(entries, options)
		entryCount, totalSize := sumPreparedEntries(entries)
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		s.rememberPrepared(streamID, preparedDirStream{
			paneID:      paneID,
			path:        path,
			sourceCount: sourceCount,
			totalSize:   totalSize,
			entries:     entries,
		})
		s.Emit("fs:listStream:done", dirStreamDoneEvent{
			StreamID:    streamID,
			PaneID:      paneID,
			Path:        path,
			EntryCount:  entryCount,
			SourceCount: sourceCount,
			TotalSize:   totalSize,
			DurationMS:  time.Since(started).Milliseconds(),
		})
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if s.elevatedList != nil {
		entries, elevErr := s.elevatedList(path, showHidden)
		if elevErr == nil {
			if entries == nil {
				entries = []contracts.FileInfo{}
			}
			sourceCount := len(entries)
			entries = prepareStreamEntries(entries, options)
			count, size := sumPreparedEntries(entries)
			if errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			s.rememberPrepared(streamID, preparedDirStream{
				paneID:      paneID,
				path:        path,
				sourceCount: sourceCount,
				totalSize:   size,
				entries:     entries,
			})
			s.Emit("fs:listStream:done", dirStreamDoneEvent{
				StreamID:    streamID,
				PaneID:      paneID,
				Path:        path,
				EntryCount:  count,
				SourceCount: sourceCount,
				TotalSize:   size,
				DurationMS:  time.Since(started).Milliseconds(),
				FromElevate: true,
			})
			return
		}
	}

	s.Emit("fs:listStream:error", dirStreamErrorEvent{
		StreamID: streamID,
		PaneID:   paneID,
		Path:     path,
		Error:    err.Error(),
	})
}

func sumPreparedEntries(entries []contracts.FileInfo) (int, int64) {
	var totalSize int64
	for _, entry := range entries {
		totalSize += entry.Size
	}
	return len(entries), totalSize
}

func prepareStreamEntries(entries []contracts.FileInfo, options DirStreamOptions) []contracts.FileInfo {
	needle := strings.ToLower(strings.TrimSpace(options.FilterText))
	if needle != "" {
		filtered := make([]contracts.FileInfo, 0, len(entries))
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry.Name), needle) || strings.Contains(strings.ToLower(entry.Path), needle) {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	descending := strings.EqualFold(options.SortDirection, "desc")
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Path == "__parent__" || right.Path == "__parent__" {
			return left.Path == "__parent__"
		}
		if options.DirectoriesFirst && (left.Type == contracts.FileTypeDirectory) != (right.Type == contracts.FileTypeDirectory) {
			return left.Type == contracts.FileTypeDirectory
		}
		var comparison int
		switch options.SortField {
		case "size":
			comparison = compareInt64(left.Size, right.Size)
		case "modified":
			comparison = compareInt64(left.Modified, right.Modified)
		case "type":
			comparison = strings.Compare(string(left.Type), string(right.Type))
		default:
			comparison = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		}
		if comparison == 0 && options.SortField != "name" {
			comparison = strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
	return entries
}

func compareInt64(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func (s *DirStreamService) scanVFSStream(
	ctx context.Context,
	path string,
	showHidden bool,
	chunkSize int,
	onBatch func(batch []contracts.FileInfo, batchSize int64),
) error {
	if s.vfsListDir == nil {
		return fmt.Errorf("vfs listing not available")
	}
	listing, err := s.vfsListDir(path)
	if err != nil {
		return err
	}
	batch := make([]contracts.FileInfo, 0, chunkSize)
	batchSize := int64(0)
	for _, fi := range listing.Entries {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}
		if !showHidden && fi.IsHidden {
			continue
		}
		batch = append(batch, fi)
		batchSize += fi.Size
		if len(batch) >= chunkSize {
			onBatch(batch, batchSize)
			batch = make([]contracts.FileInfo, 0, chunkSize)
			batchSize = 0
		}
	}
	if len(batch) > 0 {
		onBatch(batch, batchSize)
	}
	return nil
}

func (s *DirStreamService) scanDirectoryStream(
	ctx context.Context,
	path string,
	showHidden bool,
	chunkSize int,
	onBatch func(batch []contracts.FileInfo, batchSize int64),
) error {
	nativePath, err := s.resolveNativePath(path)
	if err != nil {
		return err
	}

	dir, err := os.Open(nativePath)
	if err != nil {
		return err
	}
	defer dir.Close()

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		infos, readErr := dir.Readdir(chunkSize)
		if len(infos) > 0 {
			batch := make([]contracts.FileInfo, 0, len(infos))
			batchSize := int64(0)
			for _, info := range infos {
				name := info.Name()
				nativeFullPath := internalplatform.ToSlash(filepath.Join(nativePath, name))
				publicFullPath := joinPublicPath(path, name)
				hidden := s.isHidden != nil && s.isHidden(name, nativeFullPath)
				if !showHidden && hidden {
					continue
				}
				fi := s.buildFile(name, nativeFullPath, info, hidden)
				fi.Path = publicFullPath
				batch = append(batch, fi)
				batchSize += fi.Size
			}
			if len(batch) > 0 {
				onBatch(batch, batchSize)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func (s *DirStreamService) scanSearchStream(
	ctx context.Context,
	path string,
	showHidden bool,
	chunkSize int,
	onBatch func(batch []contracts.FileInfo, batchSize int64),
) error {
	if path == pathref.SearchScheme {
		return nil
	}

	query, ok := pathref.ParseSearch(path)
	if !ok {
		return fmt.Errorf("invalid search alias path")
	}

	basePath := internalplatform.ToSlash(query.BasePath)
	nativeBasePath, err := s.resolveNativePath(query.BasePath)
	if err != nil {
		return err
	}
	batch := make([]contracts.FileInfo, 0, chunkSize)
	batchSize := int64(0)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		out := append([]contracts.FileInfo(nil), batch...)
		onBatch(out, batchSize)
		batch = batch[:0]
		batchSize = 0
	}

	err = filepath.WalkDir(nativeBasePath, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		if walkErr != nil {
			if errors.Is(walkErr, fs.ErrPermission) || errors.Is(walkErr, os.ErrPermission) {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return nil
		}

		if currentPath == nativeBasePath {
			return nil
		}

		name := entry.Name()
		fullPath := internalplatform.ToSlash(currentPath)
		publicFullPath := pathref.FromNativePath(fullPath)
		hidden := s.isHidden != nil && s.isHidden(name, fullPath)
		if !showHidden && hidden {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		matched, matchErr := pathref.MatchSearchPattern(query.Pattern, name)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			info, infoErr := entry.Info()
			if infoErr == nil {
				fi := s.buildFile(name, fullPath, info, hidden)
				fi.Path = publicFullPath
				fi.SearchBasePath = basePath
				fi.SearchPattern = query.Pattern
				fi.SearchParentPath = pathref.FromNativePath(internalplatform.ToSlash(filepath.Dir(fullPath)))
				batch = append(batch, fi)
				batchSize += fi.Size
				if len(batch) >= chunkSize {
					flush()
				}
			}
		}

		if !query.Recursive && entry.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return err
	}

	flush()
	return nil
}

func (s *DirStreamService) resolveNativePath(path string) (string, error) {
	if s.resolvePath != nil {
		return s.resolvePath(path)
	}
	return pathref.ToNativePath(path), nil
}

func joinPublicPath(basePath string, name string) string {
	trimmedBase := strings.TrimRight(internalplatform.ToSlash(basePath), "/")
	if trimmedBase == "" || trimmedBase == "/" {
		return "/" + name
	}
	return trimmedBase + "/" + name
}
