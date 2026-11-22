package runtime

import (
	"container/heap"
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
	"wails-app/internal/work"
)

const dirStreamBatchSize = 256
const dirStreamPreviewLimit = 1024

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
	WindowMS    int64                `json:"windowMs"`
	Entries     []contracts.FileInfo `json:"entries"`
}

type dirStreamMetaEvent struct {
	StreamID    string `json:"streamId"`
	PaneID      string `json:"paneId"`
	Path        string `json:"path"`
	ShowHidden  bool   `json:"showHidden"`
	SourceCount int    `json:"sourceCount"`
	TotalCount  int    `json:"totalCount"`
	Generation  uint64 `json:"generation"`
	EnumerateMS int64  `json:"enumerateMs"`
}

type dirStreamBatchEvent struct {
	StreamID   string               `json:"streamId"`
	Generation uint64               `json:"generation"`
	PaneID     string               `json:"paneId"`
	Path       string               `json:"path"`
	Entries    []contracts.FileInfo `json:"entries"`
}

type dirStreamDoneEvent struct {
	StreamID    string `json:"streamId"`
	PaneID      string `json:"paneId"`
	Path        string `json:"path"`
	EntryCount  int    `json:"entryCount"`
	SourceCount int    `json:"sourceCount"`
	TotalSize   int64  `json:"totalSize"`
	DurationMS  int64  `json:"durationMs"`
	EnumerateMS int64  `json:"enumerateMs"`
	IndexMS     int64  `json:"indexMs"`
	SortMS      int64  `json:"sortMs"`
	Generation  uint64 `json:"generation"`
	FromElevate bool   `json:"fromElevate"`
}

type dirStreamErrorEvent struct {
	StreamID   string `json:"streamId"`
	Generation uint64 `json:"generation"`
	PaneID     string `json:"paneId"`
	Path       string `json:"path"`
	Error      string `json:"error"`
	Partial    bool   `json:"partial"`
	EntryCount int    `json:"entryCount"`
}

type ElevatedListFunc func(path string, showHidden bool) ([]contracts.FileInfo, error)
type BuildFileInfoFunc func(name, fullPath string, info os.FileInfo, hidden bool) contracts.FileInfo
type IsHiddenFunc func(name, fullPath string) bool
type ResolvePublicPathFunc func(path string) (string, error)
type VFSListDirFunc func(path string) (*contracts.DirectoryListing, error)
type VFSListDirPageFunc func(ctx context.Context, path, token string, limit int) (*contracts.DirectoryPage, error)
type IsVFSPathFunc func(path string) bool

type DirStreamService struct {
	mu             sync.Mutex
	streams        map[string]context.CancelFunc
	prepared       map[string]preparedDirStream
	paneGeneration map[string]uint64
	nextGeneration uint64
	emitEvent      func(string, interface{})
	elevatedList   ElevatedListFunc
	buildFile      BuildFileInfoFunc
	isHidden       IsHiddenFunc
	resolvePath    ResolvePublicPathFunc
	vfsListDir     VFSListDirFunc
	vfsListDirPage VFSListDirPageFunc
	isVFSPath      IsVFSPathFunc
}

type preparedDirStream struct {
	paneID       string
	path         string
	generation   uint64
	options      DirStreamOptions
	sourceCount  int
	indexedCount int
	totalSize    int64
	complete     bool
	runs         [][]contracts.FileInfo
	entries      []contracts.FileInfo
}

func NewDirStreamService(
	emitEvent func(string, interface{}),
	elevatedList ElevatedListFunc,
	buildFile BuildFileInfoFunc,
	isHidden IsHiddenFunc,
	resolvePath ResolvePublicPathFunc,
) *DirStreamService {
	return &DirStreamService{
		streams:        make(map[string]context.CancelFunc),
		prepared:       make(map[string]preparedDirStream),
		paneGeneration: make(map[string]uint64),
		emitEvent:      emitEvent,
		elevatedList:   elevatedList,
		buildFile:      buildFile,
		isHidden:       isHidden,
		resolvePath:    resolvePath,
	}
}

// SetVFS installs the directory listing callbacks. Field writes take the
// service mutex: run() reads them from worker goroutines concurrently.
func (s *DirStreamService) SetVFS(listDir VFSListDirFunc, isVFS IsVFSPathFunc) {
	s.mu.Lock()
	s.vfsListDir = listDir
	s.isVFSPath = isVFS
	s.mu.Unlock()
}

func (s *DirStreamService) SetVFSPaged(fn VFSListDirPageFunc) {
	s.mu.Lock()
	s.vfsListDirPage = fn
	s.mu.Unlock()
}

// vfsCallbacks snapshots the VFS listing callbacks under the mutex so
// worker goroutines never race a concurrent SetVFS.
func (s *DirStreamService) vfsCallbacks() (VFSListDirFunc, VFSListDirPageFunc, IsVFSPathFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vfsListDir, s.vfsListDirPage, s.isVFSPath
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

func (s *DirStreamService) isCurrent(streamID, paneID string, generation uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, active := s.streams[streamID]
	return active && s.paneGeneration[paneID] == generation
}

func (s *DirStreamService) rememberPrepared(streamID string, generation uint64, stream preparedDirStream) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, active := s.streams[streamID]; !active || s.paneGeneration[stream.paneID] != generation {
		return false
	}
	stream.generation = generation
	s.prepared[streamID] = stream
	return true
}

func (s *DirStreamService) Window(streamID string, offset, limit int) (*DirStreamWindow, error) {
	started := time.Now()
	s.mu.Lock()
	stream, ok := s.prepared[streamID]
	current := ok && s.paneGeneration[stream.paneID] == stream.generation
	s.mu.Unlock()
	if ok && !current {
		return nil, fmt.Errorf("directory stream %q is stale", streamID)
	}
	if !ok {
		return nil, fmt.Errorf("directory stream %q is unavailable", streamID)
	}

	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	totalCount := stream.indexedCount
	if offset > totalCount {
		offset = totalCount
	}
	windowEntries := []contracts.FileInfo{}
	if stream.complete {
		end := offset + limit
		if end > len(stream.entries) {
			end = len(stream.entries)
		}
		windowEntries = stream.entries[offset:end]
	} else if limit > 0 && totalCount > 0 {
		windowEntries = mergeSortedRunsWindow(stream.runs, stream.options, offset, limit)
	}

	return &DirStreamWindow{
		StreamID:    streamID,
		PaneID:      stream.paneID,
		Path:        stream.path,
		Offset:      offset,
		Limit:       limit,
		TotalCount:  stream.indexedCount,
		SourceCount: stream.sourceCount,
		WindowMS:    time.Since(started).Milliseconds(),
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

	s.mu.Lock()
	s.nextGeneration++
	generation := s.nextGeneration
	s.paneGeneration[paneID] = generation
	// Retire the pane's previous streams: superseded prepared listings can
	// never be read again and would otherwise stay resident until an
	// explicit cancel.
	var staleCancels []context.CancelFunc
	for staleID, stale := range s.prepared {
		if stale.paneID == paneID && stale.generation != generation {
			if staleCancel, ok := s.streams[staleID]; ok {
				staleCancels = append(staleCancels, staleCancel)
			}
			delete(s.streams, staleID)
			delete(s.prepared, staleID)
		}
	}
	s.streams[streamID] = cancel
	s.prepared[streamID] = preparedDirStream{
		paneID:     paneID,
		path:       internalplatform.ToSlash(path),
		generation: generation,
		options:    options,
		runs:       [][]contracts.FileInfo{},
		entries:    []contracts.FileInfo{},
	}
	s.mu.Unlock()
	for _, staleCancel := range staleCancels {
		staleCancel()
	}

	work.Default.Submit(ctx, work.Task{
		Key:      "directory-stream:" + streamID,
		Priority: work.PriorityVisible,
		Kind:     "directory",
		Run: func(taskCtx context.Context, reporter work.Reporter) error {
			s.run(
				taskCtx,
				streamID,
				paneID,
				internalplatform.ToSlash(path),
				showHidden,
				options,
				generation,
			)
			if err := taskCtx.Err(); err != nil {
				return err
			}
			return nil
		},
	})
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

type localDirPopulation struct {
	nativePath string
	entries    []fs.DirEntry
}

func (s *DirStreamService) enumerateLocalDirectory(
	ctx context.Context,
	path string,
	showHidden bool,
) (*localDirPopulation, error) {
	nativePath, err := s.resolveNativePath(path)
	if err != nil {
		return nil, err
	}

	dir, err := os.Open(nativePath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	raw, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, context.Canceled
	}

	if showHidden || s.isHidden == nil {
		return &localDirPopulation{nativePath: nativePath, entries: raw}, nil
	}

	filtered := make([]fs.DirEntry, 0, len(raw))
	for _, entry := range raw {
		if err := ctx.Err(); err != nil {
			return nil, context.Canceled
		}
		name := entry.Name()
		nativeFullPath := internalplatform.ToSlash(filepath.Join(nativePath, name))
		if !s.isHidden(name, nativeFullPath) {
			filtered = append(filtered, entry)
		}
	}

	return &localDirPopulation{nativePath: nativePath, entries: filtered}, nil
}

func (s *DirStreamService) indexLocalPopulation(
	ctx context.Context,
	path string,
	population *localDirPopulation,
	chunkSize int,
	onBatch func(batch []contracts.FileInfo, batchSize int64),
) error {
	if population == nil {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = dirStreamBatchSize
	}

	for offset := 0; offset < len(population.entries); offset += chunkSize {
		if err := ctx.Err(); err != nil {
			return context.Canceled
		}
		end := offset + chunkSize
		if end > len(population.entries) {
			end = len(population.entries)
		}

		batch := make([]contracts.FileInfo, 0, end-offset)
		var batchSize int64
		for _, entry := range population.entries[offset:end] {
			if err := ctx.Err(); err != nil {
				return context.Canceled
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			name := entry.Name()
			nativeFullPath := internalplatform.ToSlash(filepath.Join(population.nativePath, name))
			hidden := s.isHidden != nil && s.isHidden(name, nativeFullPath)
			fi := s.buildFile(name, nativeFullPath, info, hidden)
			fi.Path = joinPublicPath(path, name)
			batch = append(batch, fi)
			batchSize += fi.Size
		}
		if len(batch) > 0 {
			onBatch(batch, batchSize)
		}
	}
	return nil
}

func (s *DirStreamService) run(
	ctx context.Context,
	streamID, paneID, path string,
	showHidden bool,
	options DirStreamOptions,
	generation uint64,
) {
	defer s.Unregister(streamID)

	started := time.Now()
	var enumerateDuration time.Duration
	var indexDuration time.Duration
	var sortDuration time.Duration

	// Snapshot the VFS callbacks once: SetVFS may swap them concurrently.
	_, _, isVFSPath := s.vfsCallbacks()

	var scan func(
		context.Context,
		string,
		bool,
		int,
		func([]contracts.FileInfo, int64),
	) error

	var localPopulation *localDirPopulation
	if !pathref.IsSearch(path) && (isVFSPath == nil || !isVFSPath(path)) {
		enumerateStarted := time.Now()
		population, err := s.enumerateLocalDirectory(ctx, path, showHidden)
		enumerateDuration = time.Since(enumerateStarted)
		if err == nil {
			localPopulation = population
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			s.finishStreamError(ctx, streamID, paneID, path, showHidden, options, generation, started, enumerateDuration, err)
			return
		}

		totalCount := len(localPopulation.entries)
		if strings.TrimSpace(options.FilterText) != "" {
			totalCount = -1
		}
		if !s.isCurrent(streamID, paneID, generation) {
			return
		}
		s.Emit("fs:listStream:meta", dirStreamMetaEvent{
			StreamID:    streamID,
			PaneID:      paneID,
			Path:        path,
			ShowHidden:  showHidden,
			SourceCount: len(localPopulation.entries),
			TotalCount:  totalCount,
			Generation:  generation,
			EnumerateMS: enumerateDuration.Milliseconds(),
		})

		scan = func(
			ctx context.Context,
			path string,
			_ bool,
			chunkSize int,
			onBatch func([]contracts.FileInfo, int64),
		) error {
			return s.indexLocalPopulation(ctx, path, localPopulation, chunkSize, onBatch)
		}
	} else {
		if !s.isCurrent(streamID, paneID, generation) {
			return
		}
		s.Emit("fs:listStream:meta", dirStreamMetaEvent{
			StreamID:    streamID,
			PaneID:      paneID,
			Path:        path,
			ShowHidden:  showHidden,
			SourceCount: -1,
			TotalCount:  -1,
			Generation:  generation,
			EnumerateMS: 0,
		})
		scan = s.scanDirectoryStream
		if pathref.IsSearch(path) {
			scan = s.scanSearchStream
		} else if isVFSPath != nil && isVFSPath(path) {
			scan = s.scanVFSStream
		}
	}

	runs := make([][]contracts.FileInfo, 0, 8)
	sourceCount := 0
	var totalSize int64
	previewCount := 0

	scanStarted := time.Now()
	err := scan(ctx, path, showHidden, dirStreamBatchSize, func(batch []contracts.FileInfo, _ int64) {
		if len(batch) == 0 || ctx.Err() != nil || !s.isCurrent(streamID, paneID, generation) {
			return
		}

		sourceCount += len(batch)

		preparedInput := append([]contracts.FileInfo(nil), batch...)

		sortStarted := time.Now()
		preparedBatch := prepareStreamEntries(preparedInput, options)
		sortDuration += time.Since(sortStarted)

		if len(preparedBatch) > 0 {
			runs = append(runs, preparedBatch)
			for _, entry := range preparedBatch {
				totalSize += entry.Size
			}
			if !s.rememberPrepared(streamID, generation, preparedDirStream{
				paneID:       paneID,
				path:         path,
				generation:   generation,
				options:      options,
				sourceCount:  sourceCount,
				indexedCount: countRunEntries(runs),
				totalSize:    totalSize,
				complete:     false,
				runs:         append([][]contracts.FileInfo(nil), runs...),
				entries:      []contracts.FileInfo{},
			}) {
				return
			}
		}

		if previewCount >= dirStreamPreviewLimit || !s.isCurrent(streamID, paneID, generation) {
			return
		}
		remaining := dirStreamPreviewLimit - previewCount
		previewBatch := batch
		if len(previewBatch) > remaining {
			previewBatch = previewBatch[:remaining]
		}
		previewCount += len(previewBatch)
		s.Emit("fs:listStream:batch", dirStreamBatchEvent{
			StreamID:   streamID,
			Generation: generation,
			PaneID:     paneID,
			Path:       path,
			Entries:    previewBatch,
		})
	})
	scanDuration := time.Since(scanStarted)
	if scanDuration > sortDuration {
		indexDuration = scanDuration - sortDuration
	} else {
		indexDuration = scanDuration
	}

	if err == nil {
		sortStarted := time.Now()
		entries := mergeSortedRunsAll(runs, options)
		sortDuration += time.Since(sortStarted)

		entryCount, finalTotalSize := sumPreparedEntries(entries)
		if ctx.Err() != nil || !s.isCurrent(streamID, paneID, generation) {
			return
		}
		if !s.rememberPrepared(streamID, generation, preparedDirStream{
			paneID:       paneID,
			path:         path,
			generation:   generation,
			options:      options,
			sourceCount:  sourceCount,
			indexedCount: entryCount,
			totalSize:    finalTotalSize,
			complete:     true,
			runs:         nil,
			entries:      entries,
		}) {
			return
		}

		s.Emit("fs:listStream:done", dirStreamDoneEvent{
			StreamID:    streamID,
			PaneID:      paneID,
			Path:        path,
			EntryCount:  entryCount,
			SourceCount: sourceCount,
			TotalSize:   finalTotalSize,
			DurationMS:  time.Since(started).Milliseconds(),
			EnumerateMS: enumerateDuration.Milliseconds(),
			IndexMS:     indexDuration.Milliseconds(),
			SortMS:      sortDuration.Milliseconds(),
			Generation:  generation,
		})
		return
	}

	if errors.Is(err, context.Canceled) || ctx.Err() != nil || !s.isCurrent(streamID, paneID, generation) {
		return
	}

	// A remote/provider failure after some pages were indexed is partial, not
	// catastrophic. Seal the rows we already own so window reads remain valid.
	if len(runs) > 0 {
		entries := mergeSortedRunsAll(runs, options)
		count, size := sumPreparedEntries(entries)
		if s.rememberPrepared(streamID, generation, preparedDirStream{
			paneID:       paneID,
			path:         path,
			generation:   generation,
			options:      options,
			sourceCount:  sourceCount,
			indexedCount: count,
			totalSize:    size,
			complete:     true,
			runs:         nil,
			entries:      entries,
		}) {
			s.Emit("fs:listStream:error", dirStreamErrorEvent{
				StreamID:   streamID,
				Generation: generation,
				PaneID:     paneID,
				Path:       path,
				Error:      err.Error(),
				Partial:    true,
				EntryCount: count,
			})
			return
		}
	}

	s.finishStreamError(ctx, streamID, paneID, path, showHidden, options, generation, started, enumerateDuration, err)
}

func (s *DirStreamService) finishStreamError(
	ctx context.Context,
	streamID, paneID, path string,
	showHidden bool,
	options DirStreamOptions,
	generation uint64,
	started time.Time,
	enumerateDuration time.Duration,
	err error,
) {
	if ctx.Err() != nil || !s.isCurrent(streamID, paneID, generation) {
		return
	}
	if s.elevatedList != nil {
		entries, elevErr := s.elevatedList(path, showHidden)
		if elevErr == nil {
			if entries == nil {
				entries = []contracts.FileInfo{}
			}
			sourceCount := len(entries)
			sortStarted := time.Now()
			entries = prepareStreamEntries(entries, options)
			sortDuration := time.Since(sortStarted)
			count, size := sumPreparedEntries(entries)
			if ctx.Err() != nil || !s.isCurrent(streamID, paneID, generation) {
				return
			}
			if !s.rememberPrepared(streamID, generation, preparedDirStream{
				paneID:       paneID,
				path:         path,
				generation:   generation,
				options:      options,
				sourceCount:  sourceCount,
				indexedCount: count,
				totalSize:    size,
				complete:     true,
				runs:         nil,
				entries:      entries,
			}) {
				return
			}
			s.Emit("fs:listStream:done", dirStreamDoneEvent{
				StreamID:    streamID,
				PaneID:      paneID,
				Path:        path,
				EntryCount:  count,
				SourceCount: sourceCount,
				TotalSize:   size,
				DurationMS:  time.Since(started).Milliseconds(),
				EnumerateMS: enumerateDuration.Milliseconds(),
				SortMS:      sortDuration.Milliseconds(),
				Generation:  generation,
				FromElevate: true,
			})
			return
		}
	}

	if !s.isCurrent(streamID, paneID, generation) {
		return
	}
	s.Emit("fs:listStream:error", dirStreamErrorEvent{
		StreamID:   streamID,
		Generation: generation,
		PaneID:     paneID,
		Path:       path,
		Error:      err.Error(),
		Partial:    false,
		EntryCount: 0,
	})
}

func sumPreparedEntries(entries []contracts.FileInfo) (int, int64) {
	var totalSize int64
	for _, entry := range entries {
		totalSize += entry.Size
	}
	return len(entries), totalSize
}

func countRunEntries(runs [][]contracts.FileInfo) int {
	total := 0
	for _, run := range runs {
		total += len(run)
	}
	return total
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
		comparison := comparePreparedEntries(entries[i], entries[j], options)
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
	return entries
}

func comparePreparedEntries(left, right contracts.FileInfo, options DirStreamOptions) int {
	if left.Path == "__parent__" || right.Path == "__parent__" {
		if left.Path == right.Path {
			return 0
		}
		if left.Path == "__parent__" {
			return -1
		}
		return 1
	}
	if options.DirectoriesFirst && (left.Type == contracts.FileTypeDirectory) != (right.Type == contracts.FileTypeDirectory) {
		if left.Type == contracts.FileTypeDirectory {
			return -1
		}
		return 1
	}
	switch options.SortField {
	case "size":
		if comparison := compareInt64(left.Size, right.Size); comparison != 0 {
			return comparison
		}
	case "modified":
		if comparison := compareInt64(left.Modified, right.Modified); comparison != 0 {
			return comparison
		}
	case "type":
		if comparison := strings.Compare(string(left.Type), string(right.Type)); comparison != 0 {
			return comparison
		}
	default:
		if comparison := strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name)); comparison != 0 {
			return comparison
		}
	}
	return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
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

type sortedRunCursor struct {
	runIndex   int
	entryIndex int
	entry      contracts.FileInfo
}

type sortedRunHeap struct {
	items   []sortedRunCursor
	options DirStreamOptions
}

func (h sortedRunHeap) Len() int { return len(h.items) }

func (h sortedRunHeap) Less(i, j int) bool {
	comparison := comparePreparedEntries(h.items[i].entry, h.items[j].entry, h.options)
	if strings.EqualFold(h.options.SortDirection, "desc") {
		comparison = -comparison
	}
	if comparison == 0 {
		if h.items[i].runIndex != h.items[j].runIndex {
			return h.items[i].runIndex < h.items[j].runIndex
		}
		return h.items[i].entryIndex < h.items[j].entryIndex
	}
	return comparison < 0
}

func (h sortedRunHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

func (h *sortedRunHeap) Push(x interface{}) {
	h.items = append(h.items, x.(sortedRunCursor))
}

func (h *sortedRunHeap) Pop() interface{} {
	last := len(h.items) - 1
	item := h.items[last]
	h.items = h.items[:last]
	return item
}

func mergeSortedRunsWindow(runs [][]contracts.FileInfo, options DirStreamOptions, offset, limit int) []contracts.FileInfo {
	if limit <= 0 || len(runs) == 0 {
		return []contracts.FileInfo{}
	}
	queue := &sortedRunHeap{options: options}
	for runIndex, run := range runs {
		if len(run) == 0 {
			continue
		}
		heap.Push(queue, sortedRunCursor{
			runIndex:   runIndex,
			entryIndex: 0,
			entry:      run[0],
		})
	}
	if queue.Len() == 0 {
		return []contracts.FileInfo{}
	}
	window := make([]contracts.FileInfo, 0, limit)
	needed := offset + limit
	position := 0
	for queue.Len() > 0 && position < needed {
		cursor := heap.Pop(queue).(sortedRunCursor)
		if position >= offset {
			window = append(window, cursor.entry)
		}
		position++
		nextIndex := cursor.entryIndex + 1
		if nextIndex < len(runs[cursor.runIndex]) {
			heap.Push(queue, sortedRunCursor{
				runIndex:   cursor.runIndex,
				entryIndex: nextIndex,
				entry:      runs[cursor.runIndex][nextIndex],
			})
		}
	}
	return window
}

func mergeSortedRunsAll(runs [][]contracts.FileInfo, options DirStreamOptions) []contracts.FileInfo {
	total := countRunEntries(runs)
	if total == 0 {
		return []contracts.FileInfo{}
	}
	return mergeSortedRunsWindow(runs, options, 0, total)
}

func (s *DirStreamService) scanVFSStream(ctx context.Context, path string, showHidden bool, chunkSize int, onBatch func([]contracts.FileInfo, int64)) error {
	vfsListDir, vfsListDirPage, _ := s.vfsCallbacks()
	if vfsListDirPage != nil {
		token := ""
		for {
			if ctx.Err() != nil {
				return context.Canceled
			}
			page, err := vfsListDirPage(ctx, path, token, chunkSize)
			if err != nil {
				return err
			}
			batch := make([]contracts.FileInfo, 0, len(page.Entries))
			var size int64
			for _, fi := range page.Entries {
				if ctx.Err() != nil {
					return context.Canceled
				}
				if !showHidden && fi.IsHidden {
					continue
				}
				batch = append(batch, fi)
				size += fi.Size
			}
			if len(batch) > 0 {
				onBatch(batch, size)
			}
			if page.NextToken == "" {
				return nil
			}
			token = page.NextToken
		}
	}
	if vfsListDir == nil {
		return fmt.Errorf("vfs listing not available")
	}
	listing, err := vfsListDir(path)
	if err != nil {
		return err
	}
	batch := make([]contracts.FileInfo, 0, chunkSize)
	var size int64
	for _, fi := range listing.Entries {
		if ctx.Err() != nil {
			return context.Canceled
		}
		if !showHidden && fi.IsHidden {
			continue
		}
		batch = append(batch, fi)
		size += fi.Size
		if len(batch) >= chunkSize {
			onBatch(batch, size)
			batch = make([]contracts.FileInfo, 0, chunkSize)
			size = 0
		}
	}
	if len(batch) > 0 {
		onBatch(batch, size)
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