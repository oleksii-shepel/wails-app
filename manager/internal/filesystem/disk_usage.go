package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
)

func (s *Service) AnalyzeDiskUsage(path string) (*contracts.DiskUsageResult, error) {
	requestedPath := pathref.Normalize(path)
	if cached, ok := s.duCache.Get(requestedPath); ok {
		return cached, nil
	}
	resolvedPath, err := s.resolvePublicPath(requestedPath)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	children, err := readNonLinks(resolvedPath)
	if err != nil {
		return nil, err
	}

	never := make(chan struct{})
	entries, _ := globalBFS(resolvedPath, children, never, nil, nil)

	result := s.mapDiskUsageResult(buildDiskUsageResult(resolvedPath, entries, start), requestedPath)
	s.duCache.Set(result)
	return result, nil
}

func (s *Service) StartDiskUsageStream(path string) (string, error) {
	requestedPath := pathref.Normalize(path)
	s.duCache.InvalidateSubtree(requestedPath)
	resolvedPath, err := s.resolvePublicPath(requestedPath)
	if err != nil {
		return "", err
	}

	children, err := readNonLinks(resolvedPath)
	if err != nil {
		return "", err
	}

	scanID := fmt.Sprintf("du-%d", time.Now().UnixNano())
	done := s.duStreams.register(scanID)

	go func() {
		start := time.Now()

		entries, _ := globalBFS(resolvedPath, children, done, func(_ int, entry contracts.DiskUsageEntry) {
			select {
			case <-done:
			default:
				s.emit("disk-usage:entry", map[string]interface{}{
					"scanId": scanID,
					"entry":  s.mapDiskUsageEntry(entry),
				})
			}
		}, func(progress contracts.DiskUsageProgress) {
			select {
			case <-done:
			default:
				s.emit("disk-usage:progress", map[string]interface{}{
					"scanId":   scanID,
					"progress": progress,
				})
			}
		})

		s.finishDiskUsageStream(scanID, done, resolvedPath, requestedPath, entries, start)
	}()

	return scanID, nil
}

// finishDiskUsageStream publishes the outcome of a background disk usage
// scan. Cancelled scans only hold partial totals, so they must not be
// cached as if they were complete.
func (s *Service) finishDiskUsageStream(scanID string, done <-chan struct{}, resolvedPath, requestedPath string, entries []contracts.DiskUsageEntry, start time.Time) bool {
	s.duStreams.unregister(scanID)

	select {
	case <-done:
		return false
	default:
	}

	result := s.mapDiskUsageResult(buildDiskUsageResult(resolvedPath, entries, start), requestedPath)
	s.duCache.Set(result)
	s.emit("disk-usage:done", map[string]interface{}{
		"scanId": scanID,
		"result": *result,
	})
	return true
}

func (s *Service) CancelDiskUsageStream(scanID string) {
	s.duStreams.cancel(scanID)
}

func (s *Service) mapDiskUsageEntry(entry contracts.DiskUsageEntry) contracts.DiskUsageEntry {
	entry.Path = s.toPublicPath(entry.Path)
	return entry
}

func (s *Service) mapDiskUsageResult(result *contracts.DiskUsageResult, requestedPath string) *contracts.DiskUsageResult {
	if result == nil {
		return nil
	}
	if requestedPath != "" {
		result.Path = pathref.Normalize(requestedPath)
	} else {
		result.Path = s.toPublicPath(result.Path)
	}
	for index := range result.Entries {
		result.Entries[index] = s.mapDiskUsageEntry(result.Entries[index])
	}
	return result
}

type duStreamRegistry struct {
	mu      sync.Mutex
	cancels map[string]func()
}

func newDuStreamRegistry() *duStreamRegistry {
	return &duStreamRegistry{cancels: make(map[string]func())}
}

func (r *duStreamRegistry) register(id string) <-chan struct{} {
	ch := make(chan struct{})
	var once sync.Once
	cancel := func() { once.Do(func() { close(ch) }) }
	r.mu.Lock()
	r.cancels[id] = cancel
	r.mu.Unlock()
	return ch
}

func (r *duStreamRegistry) unregister(id string) {
	r.mu.Lock()
	delete(r.cancels, id)
	r.mu.Unlock()
}

func (r *duStreamRegistry) cancel(id string) {
	r.mu.Lock()
	cancel, ok := r.cancels[id]
	if ok {
		delete(r.cancels, id)
	}
	r.mu.Unlock()
	if ok {
		cancel()
	}
}

func readNonLinks(path string) ([]os.DirEntry, error) {
	all, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, entry := range all {
		if !isLink(entry) {
			out = append(out, entry)
		}
	}
	return out, nil
}

type scanJob struct {
	dir     string
	rootIdx int
}

func globalBFS(
	root string,
	children []os.DirEntry,
	done <-chan struct{},
	onChildUpdate func(idx int, entry contracts.DiskUsageEntry),
	onProgress func(progress contracts.DiskUsageProgress),
) ([]contracts.DiskUsageEntry, error) {
	n := len(children)
	entries := make([]contracts.DiskUsageEntry, n)
	start := time.Now()
	var progressSize atomic.Int64
	var progressFiles atomic.Int64
	var progressDirs atomic.Int64
	progressStop := make(chan struct{})

	type rootAccum struct {
		size     atomic.Int64
		files    atomic.Int64
		dirs     atomic.Int64
		inFlight atomic.Int32
		dirty    atomic.Bool
		typesMu  sync.Mutex
		types    map[string]contracts.DiskUsageTypeStat
	}
	accums := make([]rootAccum, n)

	buildChildSnapshot := func(idx int) contracts.DiskUsageEntry {
		snapshot := entries[idx]
		snapshot.Size = accums[idx].size.Load()
		snapshot.Files = accums[idx].files.Load()
		snapshot.Dirs = accums[idx].dirs.Load()
		snapshot.Types = cloneDiskUsageTypes(accums[idx].types, &accums[idx].typesMu)
		return snapshot
	}

	emitChildSnapshot := func(idx int) {
		if onChildUpdate != nil {
			onChildUpdate(idx, buildChildSnapshot(idx))
		}
	}

	var firstErr error
	var errMu sync.Mutex

	for i, child := range children {
		full := filepath.Join(root, child.Name())
		accums[i].types = make(map[string]contracts.DiskUsageTypeStat)
		entries[i] = contracts.DiskUsageEntry{
			Name:  child.Name(),
			Path:  filepath.ToSlash(full),
			IsDir: child.IsDir(),
		}
		if !child.IsDir() {
			if info, err := child.Info(); err == nil {
				entries[i].Size = info.Size()
				entries[i].Types = buildSingleFileTypes(child.Name(), info.Size())
				progressSize.Add(info.Size())
			} else {
				entries[i].Error = err.Error()
			}
			progressFiles.Add(1)
			if onChildUpdate != nil {
				onChildUpdate(i, entries[i])
			}
		} else {
			progressDirs.Add(1)
			accums[i].dirty.Store(true)
		}
	}

	if onProgress != nil || onChildUpdate != nil {
		emitProgress := func() {
			if onProgress != nil {
				onProgress(contracts.DiskUsageProgress{
					TotalSize:  progressSize.Load(),
					TotalFiles: progressFiles.Load(),
					TotalDirs:  progressDirs.Load(),
					ElapsedMs:  time.Since(start).Milliseconds(),
				})
			}
			if onChildUpdate != nil {
				for idx := range accums {
					if entries[idx].IsDir && accums[idx].inFlight.Load() > 0 && accums[idx].dirty.Swap(false) {
						emitChildSnapshot(idx)
					}
				}
			}
		}
		emitProgress()
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-progressStop:
					return
				case <-ticker.C:
					emitProgress()
				}
			}
		}()
	}

	jobsIn := make(chan scanJob, 1024)
	jobsOut := make(chan scanJob, 1024)
	go func() {
		defer close(jobsOut)
		var buf []scanJob
		for {
			if len(buf) == 0 {
				job, ok := <-jobsIn
				if !ok {
					return
				}
				buf = append(buf, job)
			} else {
				select {
				case job, ok := <-jobsIn:
					if !ok {
						for _, buffered := range buf {
							jobsOut <- buffered
						}
						return
					}
					buf = append(buf, job)
				case jobsOut <- buf[0]:
					buf = buf[1:]
				}
			}
		}
	}()

	var inFlight atomic.Int64
	var closeJobsOnce sync.Once

	submit := func(job scanJob) {
		inFlight.Add(1)
		accums[job.rootIdx].inFlight.Add(1)
		select {
		case jobsIn <- job:
		case <-done:
			inFlight.Add(-1)
			accums[job.rootIdx].inFlight.Add(-1)
		}
	}

	for i, child := range children {
		if child.IsDir() {
			submit(scanJob{filepath.Join(root, child.Name()), i})
		}
	}

	numWorkers := runtime.GOMAXPROCS(0) * 2
	if numWorkers < 4 {
		numWorkers = 4
	}
	if numWorkers > 64 {
		numWorkers = 64
	}

	var workerWG sync.WaitGroup
	workerWG.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func() {
			defer workerWG.Done()
			for job := range jobsOut {
				cancelled := false
				select {
				case <-done:
					cancelled = true
				default:
				}

				if !cancelled {
					dirEntries, err := os.ReadDir(job.dir)
					if err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					} else {
						for _, entry := range dirEntries {
							if isLink(entry) {
								continue
							}
							full := filepath.Join(job.dir, entry.Name())
							if entry.IsDir() {
								accums[job.rootIdx].dirs.Add(1)
								accums[job.rootIdx].dirty.Store(true)
								progressDirs.Add(1)
								submit(scanJob{full, job.rootIdx})
							} else {
								accums[job.rootIdx].files.Add(1)
								accums[job.rootIdx].dirty.Store(true)
								progressFiles.Add(1)
								if info, err := entry.Info(); err == nil {
									accums[job.rootIdx].size.Add(info.Size())
									progressSize.Add(info.Size())
									addDiskUsageType(accums[job.rootIdx].types, &accums[job.rootIdx].typesMu, entry.Name(), info.Size())
								}
							}
						}
					}
				}

				if accums[job.rootIdx].inFlight.Add(-1) == 0 {
					idx := job.rootIdx
					accums[idx].dirty.Store(false)
					entries[idx] = buildChildSnapshot(idx)
					emitChildSnapshot(idx)
				}
				if inFlight.Add(-1) == 0 {
					closeJobsOnce.Do(func() { close(jobsIn) })
				}
			}
		}()
	}

	workerWG.Wait()
	close(progressStop)
	if onProgress != nil {
		onProgress(contracts.DiskUsageProgress{
			TotalSize:  progressSize.Load(),
			TotalFiles: progressFiles.Load(),
			TotalDirs:  progressDirs.Load(),
			ElapsedMs:  time.Since(start).Milliseconds(),
		})
	}
	return entries, firstErr
}

func buildSingleFileTypes(name string, size int64) map[string]contracts.DiskUsageTypeStat {
	ext := diskUsageExt(name)
	if ext == "" {
		return nil
	}
	return map[string]contracts.DiskUsageTypeStat{
		ext: {Count: 1, Size: size},
	}
}

func addDiskUsageType(target map[string]contracts.DiskUsageTypeStat, mu *sync.Mutex, name string, size int64) {
	ext := diskUsageExt(name)
	if ext == "" {
		return
	}
	mu.Lock()
	stat := target[ext]
	stat.Count++
	stat.Size += size
	target[ext] = stat
	mu.Unlock()
}

func cloneDiskUsageTypes(source map[string]contracts.DiskUsageTypeStat, mu *sync.Mutex) map[string]contracts.DiskUsageTypeStat {
	mu.Lock()
	defer mu.Unlock()
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]contracts.DiskUsageTypeStat, len(source))
	for ext, stat := range source {
		cloned[ext] = stat
	}
	return cloned
}

func mergeDiskUsageTypes(entries []contracts.DiskUsageEntry) map[string]contracts.DiskUsageTypeStat {
	merged := make(map[string]contracts.DiskUsageTypeStat)
	for _, entry := range entries {
		for ext, stat := range entry.Types {
			current := merged[ext]
			current.Count += stat.Count
			current.Size += stat.Size
			merged[ext] = current
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func diskUsageExt(name string) string {
	ext := filepath.Ext(name)
	if len(ext) <= 1 {
		return ""
	}
	return strings.ToLower(ext[1:])
}

func buildDiskUsageResult(path string, entries []contracts.DiskUsageEntry, start time.Time) *contracts.DiskUsageResult {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	var totalSize, totalFiles, totalDirs int64
	for _, entry := range entries {
		totalSize += entry.Size
		if entry.IsDir {
			totalDirs += 1 + entry.Dirs
			totalFiles += entry.Files
		} else {
			totalFiles++
		}
	}

	return &contracts.DiskUsageResult{
		Path:       filepath.ToSlash(path),
		TotalSize:  totalSize,
		TotalFiles: totalFiles,
		TotalDirs:  totalDirs,
		Types:      mergeDiskUsageTypes(entries),
		Entries:    entries,
		ElapsedMs:  time.Since(start).Milliseconds(),
	}
}
