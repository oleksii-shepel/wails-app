package main

import (
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

// DiskUsageResult is the wire payload returned by AnalyzeDiskUsage.
type DiskUsageResult struct {
	Path       string           `json:"path"`
	TotalSize  int64            `json:"totalSize"`
	TotalFiles int64            `json:"totalFiles"`
	TotalDirs  int64            `json:"totalDirs"`
	Entries    []DiskUsageEntry `json:"entries"`
	ElapsedMs  int64            `json:"elapsedMs"`
}

// ── Wails-exposed method ─────────────────────────────────────────────────────

// AnalyzeDiskUsage calculates the size of every direct child in path using a
// parallel BFS worker pool. Returns per-child totals sorted descending by size.
func (a *App) AnalyzeDiskUsage(path string) (*DiskUsageResult, error) {
	start := time.Now()

	all, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// Exclude symlinks, junctions, and all other reparse points.
	children := all[:0]
	for _, e := range all {
		if !isLink(e) {
			children = append(children, e)
		}
	}

	entries := make([]DiskUsageEntry, len(children))

	// Use a worker pool sized to GOMAXPROCS to walk children in parallel.
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		workers = 2
	}
	if workers > len(children) {
		workers = len(children)
	}

	var totalSize, totalFiles, totalDirs atomic.Int64
	var wg sync.WaitGroup

	type job struct {
		idx   int
		entry os.DirEntry
	}
	jobs := make(chan job, len(children))
	for i, child := range children {
		jobs <- job{idx: i, entry: child}
	}
	close(jobs)

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				e := j.entry
				name := e.Name()
				full := filepath.Join(path, name)

				var du DiskUsageEntry
				du.Name = name
				du.Path = toSlash(full)
				du.IsDir = e.IsDir()

				if e.IsDir() {
					sz, fc, dc, walkErr := bfsDirSize(full)
					du.Size = sz
					du.Files = fc
					du.Dirs = dc
					if walkErr != nil {
						du.Error = walkErr.Error()
					}
					totalDirs.Add(1 + dc)
					totalFiles.Add(fc)
					totalSize.Add(sz)
				} else {
					info, infoErr := e.Info()
					if infoErr != nil {
						du.Error = infoErr.Error()
					} else {
						du.Size = info.Size()
					}
					totalFiles.Add(1)
					totalSize.Add(du.Size)
				}

				entries[j.idx] = du
			}
		}()
	}

	wg.Wait()

	// Sort children descending by size (largest first).
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

// ── Internal BFS walk ────────────────────────────────────────────────────────

// bfsDirSize computes the total byte size, file count and subdirectory count
// of dir using breadth-first traversal. Unreadable directories are silently
// skipped so a single permission error never aborts the whole walk.
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
