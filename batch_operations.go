package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mholt/archiver/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Conflict registry
// ============================================================================
//
// An operation goroutine registers a pendingConflict and blocks on its response
// channel.  The frontend calls ResolveConflict which sends the user's decision,
// unblocking the goroutine.

type conflictRegistry struct {
	mu      sync.Mutex
	pending map[string]*pendingConflict
}

func newConflictRegistry() *conflictRegistry {
	return &conflictRegistry{pending: make(map[string]*pendingConflict)}
}

func (cr *conflictRegistry) register(id string) *pendingConflict {
	pc := &pendingConflict{response: make(chan ConflictResolutionRequest, 1)}
	cr.mu.Lock()
	cr.pending[id] = pc
	cr.mu.Unlock()
	return pc
}

func (cr *conflictRegistry) resolve(id string, req ConflictResolutionRequest) bool {
	cr.mu.Lock()
	pc, ok := cr.pending[id]
	if ok {
		delete(cr.pending, id)
	}
	cr.mu.Unlock()
	if !ok {
		return false
	}
	pc.response <- req
	return true
}

func (cr *conflictRegistry) cancelAll() {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	for id, pc := range cr.pending {
		pc.response <- ConflictResolutionRequest{Resolution: ConflictCancel}
		delete(cr.pending, id)
	}
}

var globalConflicts = newConflictRegistry()

// ============================================================================
// Throttled progress
// ============================================================================

type batchProgress struct {
	op       *ActiveOperation
	throttle time.Duration
	lastEmit time.Time

	mu             sync.Mutex
	processedItems int64
	processedBytes int64
	currentFile    string
	currentSize    int64
}

func newBatchProgress(op *ActiveOperation) *batchProgress {
	return &batchProgress{
		op:       op,
		throttle: 100 * time.Millisecond,
		lastEmit: time.Now(),
	}
}

// update stores counters and flushes to the frontend at most once per 100 ms.
// Pass force=true to emit unconditionally (e.g. at operation end).
func (bp *batchProgress) update(items, byt int64, file string, fileSize int64, force bool) {
	bp.mu.Lock()
	bp.processedItems = items
	bp.processedBytes = byt
	bp.currentFile = file
	bp.currentSize = fileSize
	now := time.Now()
	emit := force || now.Sub(bp.lastEmit) >= bp.throttle
	if emit {
		bp.lastEmit = now
	}
	i, b, f, s := bp.processedItems, bp.processedBytes, bp.currentFile, bp.currentSize
	bp.mu.Unlock()
	if emit {
		bp.op.UpdateProgress(i, b, f, s)
	}
}

func (bp *batchProgress) flush() {
	bp.mu.Lock()
	i, b, f, s := bp.processedItems, bp.processedBytes, bp.currentFile, bp.currentSize
	bp.mu.Unlock()
	bp.op.UpdateProgress(i, b, f, s)
}

// ============================================================================
// Conflict ID generator
// ============================================================================

var conflictCounter int64

func nextConflictID(opID string) string {
	return fmt.Sprintf("%s-conflict-%d", opID, atomic.AddInt64(&conflictCounter, 1))
}

// ============================================================================
// Conflict helper — single entry point for all batch operations
// ============================================================================
//
// checkConflict checks whether dst already exists and, if so, either applies
// the stored blanket decision immediately or emits "operation:conflict" and
// blocks until the frontend calls ResolveConflict.
//
// blanket is **ConflictResolution so the caller's variable is updated when
// ApplyToAll is set — no extra return value needed.
//
// Returns (finalDst, skip, err):
//   finalDst — path to write to (may differ from dst on ConflictRename)
//   skip     — true: count as skipped and move on
//   err      — non-nil only on stat error or uniqueDestination failure

func (a *App) checkConflict(
	op *ActiveOperation,
	srcLabel, dst string,
	blanket **ConflictResolution,
) (finalDst string, skip bool, err error) {
	dstInfo, statErr := os.Lstat(dst)
	if os.IsNotExist(statErr) {
		return dst, false, nil // no conflict
	}
	if statErr != nil {
		return "", false, statErr
	}

	// Apply stored blanket without prompting.
	if *blanket != nil {
		return applyResolution(**blanket, dst)
	}

	// Gather source metadata for the event (best-effort).
	var srcSize, srcMtime int64
	if info, serr := os.Lstat(srcLabel); serr == nil {
		srcSize = info.Size()
		srcMtime = info.ModTime().Unix()
	}

	conflictID := nextConflictID(op.ID)
	evt := ConflictEvent{
		OperationID: op.ID,
		ConflictID:  conflictID,
		SourcePath:  srcLabel,
		DestPath:    dst,
		SourceSize:  srcSize,
		DestSize:    dstInfo.Size(),
		SourceMtime: srcMtime,
		DestMtime:   dstInfo.ModTime().Unix(),
		IsDirectory: dstInfo.IsDir(),
	}

	// Register before emitting to avoid a race if the frontend responds instantly.
	pc := globalConflicts.register(conflictID)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "operation:conflict", evt)
	}

	select {
	case res := <-pc.response:
		if res.ApplyToAll {
			r := res.Resolution
			*blanket = &r
		}
		return applyResolution(res.Resolution, dst)
	case <-op.cancel:
		globalConflicts.cancelAll()
		return "", true, fmt.Errorf("operation cancelled")
	}
}

// applyResolution converts a ConflictResolution into the (finalDst, skip, err)
// triple that checkConflict returns.
func applyResolution(res ConflictResolution, dst string) (string, bool, error) {
	switch res {
	case ConflictSkip:
		return dst, true, nil
	case ConflictOverwrite:
		return dst, false, nil
	case ConflictRename:
		renamed, err := uniqueDestination(dst)
		return renamed, false, err
	case ConflictCancel:
		return "", true, fmt.Errorf("operation cancelled by user")
	default:
		return dst, true, nil
	}
}

// uniqueDestination finds a free path by appending " (N)" before the extension.
func uniqueDestination(dst string) (string, error) {
	ext := filepath.Ext(dst)
	base := dst[:len(dst)-len(ext)]
	for i := 2; i <= 999; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free name for %q", dst)
}

// ============================================================================
// Shared streaming writer
// ============================================================================
//
// streamToFile is the single 32 KB copy loop used by both file copy and all
// archive streaming paths.  It updates throttled progress on every chunk and
// removes the partial destination file if the operation is cancelled mid-write.

func (a *App) streamToFile(r io.Reader, dst string, size int64, label string, op *ActiveOperation, bp *batchProgress) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	bp.mu.Lock()
	baseItems := bp.processedItems
	baseBytes := bp.processedBytes
	bp.mu.Unlock()
	bp.update(baseItems, baseBytes, label, size, false)

	buf := make([]byte, 32*1024)
	var written int64
	for {
		if op.IsCancelled() {
			out.Close()
			os.Remove(dst)
			return 0, fmt.Errorf("operation cancelled")
		}
		nr, readErr := r.Read(buf)
		if nr > 0 {
			nw, writeErr := out.Write(buf[:nr])
			if writeErr != nil {
				return written, writeErr
			}
			written += int64(nw)
			bp.update(baseItems, baseBytes+written, label, size, false)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, readErr
		}
	}

	bp.mu.Lock()
	bp.processedItems = baseItems + 1
	bp.processedBytes = baseBytes + written
	i, b := bp.processedItems, bp.processedBytes
	bp.mu.Unlock()
	bp.update(i, b, "", 0, false)
	return written, nil
}

// ============================================================================
// Batch Copy
// ============================================================================

func (a *App) BatchCopy(req BatchRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpCopy, "Batch copy", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		_, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Error: err.Error()})
			continue
		}

		dst := item.Destination

		finalDst, skip, err := a.checkConflict(op, item.Source, dst, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Destination: dst, Error: err.Error()})
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		if err := a.copyPath(item.Source, finalDst, req.Recursive, op, bp); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Destination: finalDst, Error: err.Error()})
		} else {
			result.Succeeded++
		}
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Move
// ============================================================================

func (a *App) BatchMove(req BatchRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpMove, "Batch move", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		srcInfo, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Error: err.Error()})
			continue
		}

		dst := item.Destination

		// For directories, always use the destination as provided (avoid nesting duplicate folder names)
		finalDst := dst
		skip := false
		finalDst, skip, err = a.checkConflict(op, item.Source, dst, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Destination: dst, Error: err.Error()})
			continue
		}
		if skip {
			result.Skipped++
			continue
		}

		// Atomic rename first; cross-device falls back to copy+delete.
		if err = os.Rename(item.Source, finalDst); err != nil {
			if err2 := a.copyPath(item.Source, finalDst, true, op, bp); err2 != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Destination: finalDst, Error: err2.Error()})
				continue
			}
			if err2 := os.RemoveAll(item.Source); err2 != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{Source: item.Source, Error: err2.Error()})
				continue
			}
		} else {
			// Atomic rename: advance counters without a byte loop.
			bp.mu.Lock()
			bp.processedItems++
			bp.processedBytes += srcInfo.Size()
			i, b := bp.processedItems, bp.processedBytes
			bp.mu.Unlock()
			bp.update(i, b, filepath.Base(item.Source), srcInfo.Size(), false)
		}
		result.Succeeded++
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Delete
// ============================================================================

func (a *App) BatchDelete(req BatchDeleteRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpDelete, "Batch delete", "", "")

	var totalItems, totalBytes int64
	for _, p := range req.Paths {
		n, b := calculateSize(p)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Paths)}

	for _, p := range req.Paths {
		if op.IsCancelled() {
			break
		}

		useTrash := req.UseTrash == nil || *req.UseTrash
		if useTrash {
			// Delegate entirely to the OS trash / recycle bin. The OS handles
			// directories recursively, so no need to walk them ourselves.
			if err := moveToTrash(p); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{Source: p, Error: err.Error()})
				continue
			}
			bp.mu.Lock()
			bp.processedItems++
			i, b := bp.processedItems, bp.processedBytes
			bp.mu.Unlock()
			bp.update(i, b, filepath.Base(p), 0, false)
			result.Succeeded++
			continue
		}

		info, err := os.Lstat(p)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: p, Error: err.Error()})
			continue
		}

		if info.IsDir() && req.Recursive {
			if err := a.deleteDir(p, op, bp); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{Source: p, Error: err.Error()})
				continue
			}
		} else {
			size := info.Size()
			if err := os.Remove(p); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{Source: p, Error: err.Error()})
				continue
			}
			bp.mu.Lock()
			bp.processedItems++
			bp.processedBytes += size
			i, b := bp.processedItems, bp.processedBytes
			bp.mu.Unlock()
			bp.update(i, b, filepath.Base(p), size, false)
		}
		result.Succeeded++
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Extract — full archives
// ============================================================================

func (a *App) BatchExtract(req BatchExtractRequest) (*BatchExtractResult, error) {
	op := a.operations.StartOperation(OpExtract, "Batch extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Archives)), 0)

	bp := newBatchProgress(op)
	result := &BatchExtractResult{OperationID: op.ID, Total: len(req.Archives)}
	var blanket *ConflictResolution

	for _, archivePath := range req.Archives {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		destRoot := req.DestinationDir
		if req.CreateSubdirectory {
			destRoot = filepath.Join(req.DestinationDir, archiveBaseName(archivePath))
		}

		ec := extractCtx{op: op, bp: bp, req: &req, blanket: blanket, destRoot: destRoot, depth: 0}
		files, byt, err := a.extractOnDisk(archivePath, &ec)
		blanket = ec.blanket

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: archivePath, Destination: destRoot, Error: err.Error()})
			continue
		}
		result.Succeeded++
		result.ExtractedFiles += files
		result.ExtractedBytes += byt

		bp.mu.Lock()
		bp.processedItems++
		i, b := bp.processedItems, bp.processedBytes
		bp.mu.Unlock()
		bp.update(i, b, filepath.Base(archivePath), 0, false)
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Batch Selective Extract — specific files from archives / nested chains
// ============================================================================
//
// Memory model:
//   Top-level archives  → only matched entries buffered; streamed from disk.
//   Nested archives     → innermost archive resolved in memory (no seekable
//                         handle exists), then only matched entries buffered.
//                         Intermediate archive bytes released after collection.

func (a *App) BatchSelectiveExtract(req BatchSelectiveExtractRequest) (*BatchSelectiveExtractResult, error) {
	op := a.operations.StartOperation(OpExtract, "Selective extract", "", req.DestinationDir)
	op.SetTotal(int64(len(req.Items)), 0)

	bp := newBatchProgress(op)
	result := &BatchSelectiveExtractResult{OperationID: op.ID, Total: len(req.Items)}
	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() || (blanket != nil && *blanket == ConflictCancel) {
			break
		}

		dest := item.DestinationDir
		if dest == "" {
			dest = req.DestinationDir
		}

		files, byt, err := a.selectiveExtractItem(item, dest, op, bp, &blanket)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: item.ArchivePath, Error: err.Error()})
		} else {
			result.Succeeded++
			result.ExtractedFiles += files
			result.ExtractedBytes += byt
		}

		bp.mu.Lock()
		bp.processedItems++
		i, b := bp.processedItems, bp.processedBytes
		bp.mu.Unlock()
		bp.update(i, b, filepath.Base(item.ArchivePath), 0, false)
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// selectiveExtractItem handles one SelectiveExtractItem end-to-end.
func (a *App) selectiveExtractItem(
	item SelectiveExtractItem,
	destRoot string,
	op *ActiveOperation,
	bp *batchProgress,
	blanket **ConflictResolution,
) (int64, int64, error) {
	if op.IsCancelled() {
		return 0, 0, fmt.Errorf("operation cancelled")
	}

	// Phase 1: collect matched entries into memory.
	var collected []archiveEntry
	var err error
	if len(item.NestedChain) == 0 {
		collected, err = a.collectFromDisk(item.ArchivePath, item.Entries)
	} else {
		collected, err = a.collectFromNested(item.ArchivePath, item.NestedChain, item.Entries)
	}
	if err != nil {
		return 0, 0, err
	}
	if len(collected) == 0 {
		return 0, 0, nil
	}

	// Update total byte estimate now that we know the collected sizes.
	var totalSize int64
	for _, e := range collected {
		totalSize += e.size
	}
	op.mu.Lock()
	op.Progress.TotalBytes += totalSize
	op.mu.Unlock()

	// Phase 2: write collected entries to disk.
	return a.writeEntries(collected, destRoot, item.StripPrefix, item.FlattenPaths, op, bp, blanket)
}

// ============================================================================
// Phase 1 — collect matched entries into memory
// ============================================================================

// wantEntry reports whether innerPath should be extracted given the entry list.
// An empty list means "everything".  A directory prefix extracts its subtree.
func wantEntry(innerPath string, entries []string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, e := range entries {
		e = normalize(e)
		if e == "" || e == "." {
			return true
		}
		if innerPath == e || strings.HasPrefix(innerPath, e+"/") {
			return true
		}
	}
	return false
}

func (a *App) collectFromDisk(archivePath string, entries []string) ([]archiveEntry, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return collectZipFile(archivePath, entries)
	}
	return collectTarFile(archivePath, entries)
}

func collectZipFile(archivePath string, entries []string) ([]archiveEntry, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var out []archiveEntry
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ep := normalize(f.Name)
		if !wantEntry(ep, entries) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f.Name, err)
		}
		out = append(out, archiveEntry{ep, data, int64(f.UncompressedSize64), f.Modified.Unix()})
	}
	return out, nil
}

func collectTarFile(archivePath string, entries []string) ([]archiveEntry, error) {
	var out []archiveEntry
	err := archiver.Walk(archivePath, func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		ep := normalize(archiveEntryPath(f))
		if !wantEntry(ep, entries) {
			return nil
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("read %q: %w", ep, err)
		}
		var mtime int64
		if hdr, ok := f.Header.(*tar.Header); ok {
			mtime = hdr.ModTime.Unix()
		}
		out = append(out, archiveEntry{ep, data, f.Size(), mtime})
		return nil
	})
	return out, err
}

// collectFromNested resolves a nested archive chain then collects only matched
// entries from the innermost archive bytes.
func (a *App) collectFromNested(archivePath string, nestedChain []string, entries []string) ([]archiveEntry, error) {
	// extractNestedArchive is defined in archive.go — resolves chain fully in memory.
	data, err := a.extractNestedArchive(archivePath, nestedChain)
	if err != nil {
		return nil, fmt.Errorf("resolve nested chain: %w", err)
	}
	return collectFromBytes(data, nestedChain[len(nestedChain)-1], entries)
}

func collectFromBytes(data []byte, archiveName string, entries []string) ([]archiveEntry, error) {
	if isZipMagic(data) {
		return collectZipBytes(data, entries)
	}
	if isRarArchiveName(archiveName) {
		return collectRarBytes(data, archiveName, entries)
	}
	return collectTarBytes(data, entries)
}

func collectZipBytes(data []byte, entries []string) ([]archiveEntry, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out []archiveEntry
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ep := normalize(f.Name)
		if !wantEntry(ep, entries) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f.Name, err)
		}
		out = append(out, archiveEntry{ep, buf, int64(f.UncompressedSize64), f.Modified.Unix()})
	}
	return out, nil
}

func collectTarBytes(data []byte, entries []string) ([]archiveEntry, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var out []archiveEntry
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.FileInfo().IsDir() {
			io.Copy(io.Discard, tr) // drain required for sequential TAR
			continue
		}
		ep := normalize(h.Name)
		if !wantEntry(ep, entries) {
			io.Copy(io.Discard, tr)
			continue
		}
		buf, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", h.Name, err)
		}
		out = append(out, archiveEntry{ep, buf, h.Size, h.ModTime.Unix()})
	}
	return out, nil
}

func collectRarBytes(data []byte, archiveName string, entries []string) ([]archiveEntry, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var out []archiveEntry
	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(archiveEntryPath(f))
		if f.IsDir() || !wantEntry(ep, entries) {
			f.Close()
			continue
		}
		buf, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", ep, err)
		}
		out = append(out, archiveEntry{ep, buf, f.Size(), f.ModTime().Unix()})
	}
	return out, nil
}

// ============================================================================
// Phase 2 — write collected entries to disk
// ============================================================================

func (a *App) writeEntries(
	collected []archiveEntry,
	destRoot string,
	stripPrefix string,
	flatten bool,
	op *ActiveOperation,
	bp *batchProgress,
	blanket **ConflictResolution,
) (int64, int64, error) {
	var totalFiles, totalBytes int64
	rebasePrefix := normalize(stripPrefix)

	for _, entry := range collected {
		if op.IsCancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}

		relPath := normalize(entry.innerPath)
		if rebasePrefix != "" {
			if relPath == rebasePrefix {
				continue
			}
			prefixWithSep := rebasePrefix + "/"
			relPath = strings.TrimPrefix(relPath, prefixWithSep)
		}
		if relPath == "" {
			continue
		}

		var destPath string
		if flatten {
			destPath = filepath.Join(destRoot, filepath.Base(filepath.FromSlash(relPath)))
		} else {
			destPath = filepath.Join(destRoot, filepath.FromSlash(relPath))
		}

		finalDst, skip, err := a.checkConflict(op, entry.innerPath, destPath, blanket)
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if skip {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			return totalFiles, totalBytes, err
		}

		// Buffered entry: single WriteFile call, still updates progress.
		bp.mu.Lock()
		baseItems := bp.processedItems
		baseBytes := bp.processedBytes
		bp.mu.Unlock()
		bp.update(baseItems, baseBytes, filepath.Base(entry.innerPath), entry.size, false)

		if err := os.WriteFile(finalDst, entry.data, 0644); err != nil {
			return totalFiles, totalBytes, fmt.Errorf("write %q: %w", finalDst, err)
		}

		written := int64(len(entry.data))
		bp.mu.Lock()
		bp.processedItems = baseItems + 1
		bp.processedBytes = baseBytes + written
		i, b := bp.processedItems, bp.processedBytes
		bp.mu.Unlock()
		bp.update(i, b, "", 0, false)

		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

// ============================================================================
// Full-archive extraction internals
// ============================================================================

type extractCtx struct {
	op       *ActiveOperation
	bp       *batchProgress
	req      *BatchExtractRequest
	blanket  *ConflictResolution
	destRoot string
	depth    int
}

func (ec *extractCtx) cancelled() bool { return ec.op.IsCancelled() }

func (ec *extractCtx) tooDeep() bool {
	if !ec.req.ExpandNestedArchives {
		return true
	}
	if ec.req.MaxNestDepth < 0 {
		return false // unlimited
	}
	return ec.depth >= ec.req.MaxNestDepth
}

func (a *App) extractOnDisk(archivePath string, ec *extractCtx) (int64, int64, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return a.extractZipOnDisk(archivePath, ec)
	}
	return a.extractTarOnDisk(archivePath, ec)
}

func (a *App) extractZipOnDisk(archivePath string, ec *extractCtx) (int64, int64, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()

	rootStrip := zipRootPrefix(r.File, ec.req.FlattenSingleRoot)
	var totalFiles, totalBytes int64

	for _, f := range r.File {
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}

		ep := normalize(f.Name)
		if rootStrip != "" {
			if ep = strings.TrimPrefix(ep, rootStrip); ep == "" {
				continue
			}
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		// Nested archive: recurse in memory.
		if isArchive(f.Name) && !ec.tooDeep() {
			rc, err := f.Open()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := extractCtx{op: ec.op, bp: ec.bp, req: ec.req, blanket: ec.blanket,
				destRoot: filepath.Join(ec.destRoot, archiveBaseName(f.Name)), depth: ec.depth + 1}
			nf, nb, nerr := a.extractNestedFromBytes(data, f.Name, &child)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return totalFiles, totalBytes, err
		}
		finalDst, skip, cerr := a.checkConflict(ec.op, f.Name, destPath, &ec.blanket)
		if cerr != nil {
			rc.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			rc.Close()
			continue
		}
		written, werr := a.streamToFile(rc, finalDst, int64(f.UncompressedSize64), f.Name, ec.op, ec.bp)
		rc.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (a *App) extractTarOnDisk(archivePath string, ec *extractCtx) (int64, int64, error) {
	// Single-root detection requires a pre-scan when FlattenSingleRoot is set.
	var rootStrip string
	if ec.req.FlattenSingleRoot {
		var tops []string
		_ = archiver.Walk(archivePath, func(f archiver.File) error {
			parts := strings.SplitN(normalize(archiveEntryPath(f)), "/", 2)
			tops = appendUnique(tops, parts[0])
			return nil
		})
		if len(tops) == 1 {
			rootStrip = tops[0] + "/"
		}
	}

	var totalFiles, totalBytes int64
	err := archiver.Walk(archivePath, func(f archiver.File) error {
		if ec.cancelled() {
			return fmt.Errorf("operation cancelled")
		}
		ep := normalize(archiveEntryPath(f))
		if rootStrip != "" {
			if ep = strings.TrimPrefix(ep, rootStrip); ep == "" {
				return nil
			}
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))

		if f.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		if isArchive(f.Name()) && !ec.tooDeep() {
			data, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			child := extractCtx{op: ec.op, bp: ec.bp, req: ec.req, blanket: ec.blanket,
				destRoot: filepath.Join(ec.destRoot, archiveBaseName(f.Name())), depth: ec.depth + 1}
			nf, nb, nerr := a.extractNestedFromBytes(data, f.Name(), &child)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			return nerr
		}

		finalDst, skip, err := a.checkConflict(ec.op, f.Name(), destPath, &ec.blanket)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
		written, err := a.streamToFile(f, finalDst, f.Size(), f.Name(), ec.op, ec.bp)
		if err != nil {
			return err
		}
		totalFiles++
		totalBytes += written
		return nil
	})
	return totalFiles, totalBytes, err
}

// extractFromBytes handles a nested (in-memory) archive layer.
func (a *App) extractNestedFromBytes(data []byte, archiveName string, ec *extractCtx) (int64, int64, error) {
	if isZipMagic(data) {
		return a.extractZipBytes(data, ec)
	}
	if isRarArchiveName(archiveName) {
		return a.extractRarBytes(data, archiveName, ec)
	}
	return a.extractTarBytes(data, ec)
}

func (a *App) extractZipBytes(data []byte, ec *extractCtx) (int64, int64, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, 0, err
	}
	var totalFiles, totalBytes int64
	for _, f := range r.File {
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := normalize(f.Name)
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		if isArchive(f.Name) && !ec.tooDeep() {
			rc, err := f.Open()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			nested, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := extractCtx{op: ec.op, bp: ec.bp, req: ec.req, blanket: ec.blanket,
				destRoot: filepath.Join(ec.destRoot, archiveBaseName(f.Name)), depth: ec.depth + 1}
			nf, nb, nerr := a.extractNestedFromBytes(nested, f.Name, &child)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return totalFiles, totalBytes, err
		}
		finalDst, skip, cerr := a.checkConflict(ec.op, f.Name, destPath, &ec.blanket)
		if cerr != nil {
			rc.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			rc.Close()
			continue
		}
		written, werr := a.streamToFile(rc, finalDst, int64(f.UncompressedSize64), f.Name, ec.op, ec.bp)
		rc.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (a *App) extractTarBytes(data []byte, ec *extractCtx) (int64, int64, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var totalFiles, totalBytes int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := normalize(h.Name)
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if h.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		if isArchive(h.Name) && !ec.tooDeep() {
			nested, err := io.ReadAll(tr)
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := extractCtx{op: ec.op, bp: ec.bp, req: ec.req, blanket: ec.blanket,
				destRoot: filepath.Join(ec.destRoot, archiveBaseName(h.Name)), depth: ec.depth + 1}
			nf, nb, nerr := a.extractNestedFromBytes(nested, h.Name, &child)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		finalDst, skip, cerr := a.checkConflict(ec.op, h.Name, destPath, &ec.blanket)
		if cerr != nil {
			return totalFiles, totalBytes, cerr
		}
		if skip {
			continue
		}
		written, werr := a.streamToFile(tr, finalDst, h.Size, h.Name, ec.op, ec.bp)
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (a *App) extractRarBytes(data []byte, archiveName string, ec *extractCtx) (int64, int64, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	var totalFiles, totalBytes int64
	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if ec.cancelled() {
			f.Close()
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := normalize(archiveEntryPath(f))
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if f.IsDir() {
			f.Close()
			os.MkdirAll(destPath, 0755)
			continue
		}
		if isArchive(f.Name()) && !ec.tooDeep() {
			nested, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := extractCtx{op: ec.op, bp: ec.bp, req: ec.req, blanket: ec.blanket,
				destRoot: filepath.Join(ec.destRoot, archiveBaseName(f.Name())), depth: ec.depth + 1}
			nf, nb, nerr := a.extractNestedFromBytes(nested, f.Name(), &child)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			f.Close()
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		finalDst, skip, cerr := a.checkConflict(ec.op, f.Name(), destPath, &ec.blanket)
		if cerr != nil {
			f.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			f.Close()
			continue
		}
		written, werr := a.streamToFile(f, finalDst, f.Size(), f.Name(), ec.op, ec.bp)
		f.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

// ============================================================================
// File copy / delete internals
// ============================================================================

func (a *App) copyPath(src, dst string, recursive bool, op *ActiveOperation, bp *batchProgress) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("source is a directory; use recursive=true")
		}
		return walkDirOrdered(src, func(p string, isDir bool, size int64) error {
			if op.IsCancelled() {
				return fmt.Errorf("operation cancelled")
			}
			rel, _ := filepath.Rel(src, p)
			target := filepath.Join(dst, rel)
			if isDir {
				return os.MkdirAll(target, 0755)
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = a.streamToFile(f, target, size, filepath.Base(p), op, bp)
			return err
		})
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = a.streamToFile(f, dst, info.Size(), filepath.Base(src), op, bp)
	return err
}

func (a *App) deleteDir(path string, op *ActiveOperation, bp *batchProgress) error {
	var files, dirs []string
	walkDirOrdered(path, func(p string, isDir bool, _ int64) error {
		if p == path {
			return nil
		}
		if isDir {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})

	for _, f := range files {
		if op.IsCancelled() {
			return fmt.Errorf("operation cancelled")
		}
		var size int64
		if info, err := os.Stat(f); err == nil {
			size = info.Size()
		}
		os.Remove(f)
		bp.mu.Lock()
		bp.processedItems++
		bp.processedBytes += size
		i, b := bp.processedItems, bp.processedBytes
		bp.mu.Unlock()
		bp.update(i, b, filepath.Base(f), size, false)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
	return os.Remove(path)
}

func walkDirOrdered(src string, fn func(path string, isDir bool, size int64) error) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		return fn(p, info.IsDir(), info.Size())
	})
}

// ============================================================================
// Frontend binding
// ============================================================================

// ResolveConflict unblocks a paused operation by supplying the user's decision.
// conflictID comes from ConflictEvent.ConflictID emitted on "operation:conflict".
// Returns false when the ID is not recognised (already resolved or cancelled).
func (a *App) ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool {
	return globalConflicts.resolve(conflictID, ConflictResolutionRequest{
		Resolution: resolution,
		ApplyToAll: applyToAll,
	})
}

// ============================================================================
// Archive helpers
// ============================================================================

func archiveBaseName(name string) string {
	base := filepath.Base(name)
	for _, suffix := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(strings.ToLower(base), suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	if ext := filepath.Ext(base); ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

func zipRootPrefix(files []*zip.File, flatten bool) string {
	if !flatten {
		return ""
	}
	var tops []string
	for _, f := range files {
		parts := strings.SplitN(normalize(f.Name), "/", 2)
		tops = appendUnique(tops, parts[0])
	}
	if len(tops) == 1 {
		return tops[0] + "/"
	}
	return ""
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
