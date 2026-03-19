package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
)

const dirStreamBatchSize = 256

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

type DirStreamService struct {
	mu           sync.Mutex
	streams      map[string]context.CancelFunc
	emitEvent    func(string, interface{})
	elevatedList ElevatedListFunc
	buildFile    BuildFileInfoFunc
	isHidden     IsHiddenFunc
}

func NewDirStreamService(
	emitEvent func(string, interface{}),
	elevatedList ElevatedListFunc,
	buildFile BuildFileInfoFunc,
	isHidden IsHiddenFunc,
) *DirStreamService {
	return &DirStreamService{
		streams:      make(map[string]context.CancelFunc),
		emitEvent:    emitEvent,
		elevatedList: elevatedList,
		buildFile:    buildFile,
		isHidden:     isHidden,
	}
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

func (s *DirStreamService) Start(paneID, path string, showHidden bool) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	streamID := fmt.Sprintf("%s-%d", paneID, time.Now().UnixNano())
	ctx, cancel := context.WithCancel(context.Background())
	s.Register(streamID, cancel)
	go s.run(ctx, streamID, paneID, internalplatform.ToSlash(path), showHidden)
	return streamID, nil
}

func (s *DirStreamService) Cancel(streamID string) {
	if streamID == "" {
		return
	}
	s.mu.Lock()
	cancel := s.streams[streamID]
	delete(s.streams, streamID)
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
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *DirStreamService) run(ctx context.Context, streamID, paneID, path string, showHidden bool) {
	defer s.Unregister(streamID)

	started := time.Now()
	var entryCount int
	var totalSize int64

	s.Emit("fs:listStream:meta", dirStreamMetaEvent{
		StreamID:   streamID,
		PaneID:     paneID,
		Path:       path,
		ShowHidden: showHidden,
	})

	scan := s.scanDirectoryStream
	if pathref.IsSearch(path) {
		scan = s.scanSearchStream
	}

	err := scan(ctx, path, showHidden, dirStreamBatchSize, func(batch []contracts.FileInfo, batchSize int64) {
		entryCount += len(batch)
		totalSize += batchSize
		s.Emit("fs:listStream:batch", dirStreamBatchEvent{
			StreamID: streamID,
			PaneID:   paneID,
			Path:     path,
			Entries:  batch,
		})
	})
	if err == nil {
		s.Emit("fs:listStream:done", dirStreamDoneEvent{
			StreamID:   streamID,
			PaneID:     paneID,
			Path:       path,
			EntryCount: entryCount,
			TotalSize:  totalSize,
			DurationMS: time.Since(started).Milliseconds(),
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
			size := int64(0)
			for _, fi := range entries {
				size += fi.Size
			}
			s.Emit("fs:listStream:batch", dirStreamBatchEvent{
				StreamID: streamID,
				PaneID:   paneID,
				Path:     path,
				Entries:  entries,
			})
			s.Emit("fs:listStream:done", dirStreamDoneEvent{
				StreamID:    streamID,
				PaneID:      paneID,
				Path:        path,
				EntryCount:  len(entries),
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

func (s *DirStreamService) scanDirectoryStream(
	ctx context.Context,
	path string,
	showHidden bool,
	chunkSize int,
	onBatch func(batch []contracts.FileInfo, batchSize int64),
) error {
	dir, err := os.Open(path)
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
				fullPath := internalplatform.ToSlash(filepath.Join(path, name))
				hidden := s.isHidden != nil && s.isHidden(name, fullPath)
				if !showHidden && hidden {
					continue
				}
				fi := s.buildFile(name, fullPath, info, hidden)
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

	err := filepath.WalkDir(query.BasePath, func(currentPath string, entry fs.DirEntry, walkErr error) error {
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

		if currentPath == query.BasePath {
			return nil
		}

		name := entry.Name()
		fullPath := internalplatform.ToSlash(currentPath)
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
				fi.SearchBasePath = basePath
				fi.SearchPattern = query.Pattern
				fi.SearchParentPath = internalplatform.ToSlash(filepath.Dir(fullPath))
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
