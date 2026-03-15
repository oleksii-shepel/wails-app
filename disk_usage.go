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

	entries := make([]DiskUsageEntry, len(children))
	var totalSize, totalFiles, totalDirs atomic.Int64

	type job struct {
		idx   int
		entry os.DirEntry
	}
	jobs := make(chan job, len(children))
	for i, child := range children {
		jobs <- job{idx: i, entry: child}
	}
	close(jobs)

	var wg sync.WaitGroup
	workers := clampWorkers(len(children))
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				du := walkEntry(path, j.entry)
				accumulateTotals(&totalSize, &totalFiles, &totalDirs, du)
				entries[j.idx] = du
			}
		}()
	}
	wg.Wait()

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	return &DiskUsageResult{
		Path:       toSlash(path),
		TotalSize:  totalSize.Load(),
		TotalFiles: totalFiles.Load(),
		TotalDirs:  totalDirs.Load(),
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

	entries := make([]DiskUsageEntry, len(children))

	scanId := fmt.Sprintf("du-%d", time.Now().UnixNano())
	done := s.duStreams.register(scanId)

	go func() {
		start := time.Now()
		var totalSize, totalFiles, totalDirs atomic.Int64

		type job struct {
			idx   int
			entry os.DirEntry
		}
		jobs := make(chan job, len(children))
		for i, child := range children {
			jobs <- job{idx: i, entry: child}
		}
		close(jobs)

		var wg sync.WaitGroup
		workers := clampWorkers(len(children))
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for j := range jobs {
					select {
					case <-done:
						return
					default:
					}

					du := walkEntry(path, j.entry)
					accumulateTotals(&totalSize, &totalFiles, &totalDirs, du)
					entries[j.idx] = du

					select {
					case <-done:
						return
					default:
						s.emit("disk-usage:entry", map[string]interface{}{
							"scanId": scanId,
							"entry":  du,
						})
					}
				}
			}()
		}

		wg.Wait()
		s.duStreams.unregister(scanId)

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Size > entries[j].Size
		})

		select {
		case <-done:
			return
		default:
			s.emit("disk-usage:done", map[string]interface{}{
				"scanId": scanId,
				"result": DiskUsageResult{
					Path:       toSlash(path),
					TotalSize:  totalSize.Load(),
					TotalFiles: totalFiles.Load(),
					TotalDirs:  totalDirs.Load(),
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

func walkEntry(parent string, e os.DirEntry) DiskUsageEntry {
	name := e.Name()
	full := filepath.Join(parent, name)
	du := DiskUsageEntry{Name: name, Path: toSlash(full), IsDir: e.IsDir()}
	if e.IsDir() {
		sz, fc, dc, walkErr := bfsDirSize(full)
		du.Size, du.Files, du.Dirs = sz, fc, dc
		if walkErr != nil {
			du.Error = walkErr.Error()
		}
	} else {
		info, err := e.Info()
		if err != nil {
			du.Error = err.Error()
		} else {
			du.Size = info.Size()
		}
	}
	return du
}

func accumulateTotals(size, files, dirs *atomic.Int64, du DiskUsageEntry) {
	size.Add(du.Size)
	if du.IsDir {
		dirs.Add(1 + du.Dirs)
		files.Add(du.Files)
	} else {
		files.Add(1)
	}
}

func clampWorkers(n int) int {
	w := runtime.GOMAXPROCS(0)
	if w < 2 {
		w = 2
	}
	if w > n {
		w = n
	}
	return w
}

// ── Internal BFS walk ────────────────────────────────────────────────────────

func bfsDirSize(dir string) (size int64, files int64, dirs int64, firstErr error) {
	queue := []string{dir}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(current)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		for _, e := range entries {
			full := filepath.Join(current, e.Name())
			if e.IsDir() {
				dirs++
				queue = append(queue, full)
			} else {
				files++
				if info, err := e.Info(); err == nil {
					size += info.Size()
				}
			}
		}
	}
	return
}
