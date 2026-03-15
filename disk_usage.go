package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ── Types ────────────────────────────────────────────────────────────────────

// DiskUsageEntry represents one direct child of the analysed directory.
type DiskUsageEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	Files int64  `json:"files"`
	Dirs  int64  `json:"dirs"`
	Error string `json:"error,omitempty"`
}

// DiskUsageResult is the wire payload returned by AnalyzeDiskUsage and
// emitted as the final event by StartDiskUsageStream.
type DiskUsageResult struct {
	Path       string           `json:"path"`
	TotalSize  int64            `json:"totalSize"`
	TotalFiles int64            `json:"totalFiles"`
	TotalDirs  int64            `json:"totalDirs"`
	Entries    []DiskUsageEntry `json:"entries"`
	ElapsedMs  int64            `json:"elapsedMs"`
}

// ── Wails-exposed App shims ───────────────────────────────────────────────────
// Thin pass-throughs — all real logic lives on FilesystemService.

func (a *App) AnalyzeDiskUsage(path string) (*DiskUsageResult, error) {
	return a.filesystem.AnalyzeDiskUsage(path)
}

// StartDiskUsageStream starts a streaming scan.
//
// Events emitted (scoped by scanId so concurrent scans never cross):
//
//	"disk-usage:entry"  — { scanId, entry: DiskUsageEntry }   one per child
//	"disk-usage:done"   — { scanId, result: DiskUsageResult } final summary
//	"disk-usage:error"  — { scanId, error: string }
//
// The returned scanId must be passed to CancelDiskUsageStream when the
// caller navigates away, so the goroutine stops emitting to a dead surface.
func (a *App) StartDiskUsageStream(path string) (string, error) {
	return a.filesystem.StartDiskUsageStream(path)
}

func (a *App) CancelDiskUsageStream(scanId string) {
	a.filesystem.CancelDiskUsageStream(scanId)
}

// ── FilesystemService methods ─────────────────────────────────────────────────

// AnalyzeDiskUsage is the original blocking call — kept for compatibility.
func (s *FilesystemService) AnalyzeDiskUsage(path string) (*DiskUsageResult, error) {
	start := time.Now()

	children, err := readNonLinks(path)
	if err != nil {
		return nil, err
	}

	never := make(chan struct{}) // never closed — scan always runs to completion
	entries, _ := globalBFS(path, children, never, nil)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	var totalSize, totalFiles, totalDirs int64
	for _, e := range entries {
		totalSize += e.Size
		if e.IsDir {
			totalDirs += 1 + e.Dirs
			totalFiles += e.Files
		} else {
			totalFiles++
		}
	}

	return &DiskUsageResult{
		Path:       toSlash(path),
		TotalSize:  totalSize,
		TotalFiles: totalFiles,
		TotalDirs:  totalDirs,
		Entries:    entries,
		ElapsedMs:  time.Since(start).Milliseconds(),
	}, nil
}

// StartDiskUsageStream starts the parallel scan and emits events per-entry.
func (s *FilesystemService) StartDiskUsageStream(path string) (string, error) {
	children, err := readNonLinks(path)
	if err != nil {
		return "", err
	}

	scanId := fmt.Sprintf("du-%d", time.Now().UnixNano())
	done := s.duStreams.register(scanId)

	go func() {
		start := time.Now()

		entries, _ := globalBFS(path, children, done, func(_ int, entry DiskUsageEntry) {
			select {
			case <-done:
			default:
				s.emit("disk-usage:entry", map[string]interface{}{
					"scanId": scanId,
					"entry":  entry,
				})
			}
		})

		s.duStreams.unregister(scanId)

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Size > entries[j].Size
		})

		var totalSize, totalFiles, totalDirs int64
		for _, e := range entries {
			totalSize += e.Size
			if e.IsDir {
				totalDirs += 1 + e.Dirs
				totalFiles += e.Files
			} else {
				totalFiles++
			}
		}

		select {
		case <-done:
			return
		default:
			s.emit("disk-usage:done", map[string]interface{}{
				"scanId": scanId,
				"result": DiskUsageResult{
					Path:       toSlash(path),
					TotalSize:  totalSize,
					TotalFiles: totalFiles,
					TotalDirs:  totalDirs,
					Entries:    entries,
					ElapsedMs:  time.Since(start).Milliseconds(),
				},
			})
		}
	}()

	return scanId, nil
}

// CancelDiskUsageStream stops an in-progress stream. Safe to call after completion.
func (s *FilesystemService) CancelDiskUsageStream(scanId string) {
	s.duStreams.cancel(scanId)
}

// ── Stream registry ───────────────────────────────────────────────────────────
// duStreamRegistry is a field on FilesystemService — see filesystem_service.go.

type duStreamRegistry struct {
	mu      sync.Mutex
	cancels map[string]func()
}

func newDuStreamRegistry() *duStreamRegistry {
	return &duStreamRegistry{cancels: make(map[string]func())}
}

// register creates a cancel channel for scanId and returns the done channel.
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

// ── Shared helpers ────────────────────────────────────────────────────────────

func readNonLinks(path string) ([]os.DirEntry, error) {
	all, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, e := range all {
		if !isLink(e) {
			out = append(out, e)
		}
	}
	return out, nil
}

// ── Global concurrent BFS ────────────────────────────────────────────────────

// scanJob is one directory to process, tagged with its top-level ancestor index.
type scanJob struct {
	dir     string
	rootIdx int
}

// globalBFS scans all direct children of root using a single shared concurrent
// work queue. All workers serve the entire tree so no worker idles while a
// large subtree monopolises another — unlike the old two-tier approach.
//
// onChildDone is called exactly once per directory child when its entire
// subtree is scanned; pass nil when immediate notification is not needed.
func globalBFS(
	root string,
	children []os.DirEntry,
	done <-chan struct{},
	onChildDone func(idx int, entry DiskUsageEntry),
) ([]DiskUsageEntry, error) {
	n := len(children)
	entries := make([]DiskUsageEntry, n)

	// Per-root-child accumulators.
	type rootAccum struct {
		size     atomic.Int64
		files    atomic.Int64
		dirs     atomic.Int64
		inFlight atomic.Int32
	}
	accums := make([]rootAccum, n)

	var firstErr error
	var errMu sync.Mutex

	// Initialise entry metadata; plain-file children are handled immediately.
	for i, ch := range children {
		full := filepath.Join(root, ch.Name())
		entries[i] = DiskUsageEntry{
			Name:  ch.Name(),
			Path:  toSlash(full),
			IsDir: ch.IsDir(),
		}
		if !ch.IsDir() {
			if info, err := ch.Info(); err == nil {
				entries[i].Size = info.Size()
			} else {
				entries[i].Error = err.Error()
			}
			if onChildDone != nil {
				onChildDone(i, entries[i])
			}
		}
	}

	// ── Job queue ─────────────────────────────────────────────────────────────
	// A relay goroutine bridges a buffered input channel and a buffered output
	// channel with an in-memory overflow slice so workers never block on send.
	jobsIn := make(chan scanJob, 1024)
	jobsOut := make(chan scanJob, 1024)
	go func() { // relay
		defer close(jobsOut)
		var buf []scanJob
		for {
			if len(buf) == 0 {
				j, ok := <-jobsIn
				if !ok {
					return
				}
				buf = append(buf, j)
			} else {
				select {
				case j, ok := <-jobsIn:
					if !ok {
						for _, bj := range buf {
							jobsOut <- bj
						}
						return
					}
					buf = append(buf, j)
				case jobsOut <- buf[0]:
					buf = buf[1:]
				}
			}
		}
	}()

	var wg sync.WaitGroup
	go func() { wg.Wait(); close(jobsIn) }() // closer

	submit := func(job scanJob) {
		wg.Add(1)
		accums[job.rootIdx].inFlight.Add(1)
		select {
		case jobsIn <- job:
		default:
			// Buffer full: park in a goroutine so the calling worker stays unblocked.
			// wg.Add(1) is already done above so wg.Wait() accounts for this goroutine.
			go func() { jobsIn <- job }()
		}
	}

	// Seed with top-level directory children.
	for i, ch := range children {
		if ch.IsDir() {
			submit(scanJob{filepath.Join(root, ch.Name()), i})
		}
	}

	// ── Workers ───────────────────────────────────────────────────────────────
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
						for _, e := range dirEntries {
							if isLink(e) {
								continue
							}
							full := filepath.Join(job.dir, e.Name())
							if e.IsDir() {
								accums[job.rootIdx].dirs.Add(1)
								submit(scanJob{full, job.rootIdx})
							} else {
								accums[job.rootIdx].files.Add(1)
								if info, err := e.Info(); err == nil {
									accums[job.rootIdx].size.Add(info.Size())
								}
							}
						}
					}
				}

				if accums[job.rootIdx].inFlight.Add(-1) == 0 {
					// All work for this root child is done — materialise entry.
					idx := job.rootIdx
					entries[idx].Size = accums[idx].size.Load()
					entries[idx].Files = accums[idx].files.Load()
					entries[idx].Dirs = accums[idx].dirs.Load()
					if onChildDone != nil {
						onChildDone(idx, entries[idx])
					}
				}
				wg.Done()
			}
		}()
	}

	workerWG.Wait()
	return entries, firstErr
}
