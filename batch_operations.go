package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ============================================================================
// Conflict registry — lives on OperationManager
// ============================================================================
//
// When a copy/move goroutine detects a conflict it registers a pendingConflict
// in the registry and then blocks reading from its response channel.  The
// frontend calls App.ResolveConflict which looks up the channel and sends the
// user's decision, unblocking the goroutine.

type conflictRegistry struct {
	mu      sync.Mutex
	pending map[string]*pendingConflict // keyed by conflictID
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

// Attach a registry to OperationManager (zero-value safe, initialised lazily).
var globalConflicts = newConflictRegistry()

// ============================================================================
// Throttled progress emitter
// ============================================================================
//
// batchProgress wraps an ActiveOperation and ensures that UpdateProgress is
// only flushed to the frontend at most once per 100 ms, while still tracking
// counters accurately.

type batchProgress struct {
	op         *ActiveOperation
	lastEmit   time.Time
	throttleMs time.Duration

	mu             sync.Mutex
	processedItems int64
	processedBytes int64
	currentFile    string
	currentSize    int64
}

func newBatchProgress(op *ActiveOperation) *batchProgress {
	return &batchProgress{
		op:         op,
		lastEmit:   time.Now(),
		throttleMs: 100 * time.Millisecond,
	}
}

// update records progress; only flushes to the frontend if ≥100 ms have
// passed since the last flush, or if force is true (e.g. at operation end).
func (bp *batchProgress) update(items, bytes int64, file string, fileSize int64, force bool) {
	bp.mu.Lock()
	bp.processedItems = items
	bp.processedBytes = bytes
	bp.currentFile = file
	bp.currentSize = fileSize
	now := time.Now()
	shouldEmit := force || now.Sub(bp.lastEmit) >= bp.throttleMs
	if shouldEmit {
		bp.lastEmit = now
	}
	items2 := bp.processedItems
	bytes2 := bp.processedBytes
	file2 := bp.currentFile
	size2 := bp.currentSize
	bp.mu.Unlock()

	if shouldEmit {
		bp.op.UpdateProgress(items2, bytes2, file2, size2)
	}
}

func (bp *batchProgress) flush() {
	bp.mu.Lock()
	items := bp.processedItems
	bytes := bp.processedBytes
	file := bp.currentFile
	size := bp.currentSize
	bp.mu.Unlock()
	bp.op.UpdateProgress(items, bytes, file, size)
}

// ============================================================================
// Unique conflict ID generator
// ============================================================================

var conflictCounter int64

func nextConflictID(opID string) string {
	n := atomic.AddInt64(&conflictCounter, 1)
	return fmt.Sprintf("%s-conflict-%d", opID, n)
}

// ============================================================================
// Conflict check & resolution helper
// ============================================================================

// checkConflict examines whether dst exists.  If it does, it emits a
// "operation:conflict" event and blocks until the frontend resolves it.
//
// Returns (resolution, renamedDst, err).
// renamedDst is only non-empty when resolution == ConflictRename.
// When applyToAll is already set, the stored blanket resolution is returned
// immediately without emitting an event.
func (a *App) checkConflict(
	op *ActiveOperation,
	src, dst string,
	blanketResolution *ConflictResolution, // nil = ask each time
) (res ConflictResolution, finalDst string, err error) {
	finalDst = dst

	dstInfo, statErr := os.Lstat(dst)
	if os.IsNotExist(statErr) {
		return ConflictOverwrite, dst, nil // no conflict
	}
	if statErr != nil {
		return "", "", statErr
	}

	// Conflict exists.  If the caller already has a blanket decision, use it.
	if blanketResolution != nil {
		switch *blanketResolution {
		case ConflictSkip:
			return ConflictSkip, dst, nil
		case ConflictOverwrite:
			return ConflictOverwrite, dst, nil
		case ConflictRename:
			renamed, rerr := uniqueDestination(dst)
			if rerr != nil {
				return "", "", rerr
			}
			return ConflictRename, renamed, nil
		case ConflictCancel:
			return ConflictCancel, dst, nil
		}
	}

	// Gather source info for the event payload.
	srcInfo, srcStatErr := os.Lstat(src)
	var srcSize, srcMtime int64
	if srcStatErr == nil {
		srcSize = srcInfo.Size()
		srcMtime = srcInfo.ModTime().Unix()
	}

	conflictID := nextConflictID(op.ID)

	evt := ConflictEvent{
		OperationID: op.ID,
		ConflictID:  conflictID,
		SourcePath:  src,
		DestPath:    dst,
		SourceSize:  srcSize,
		DestSize:    dstInfo.Size(),
		SourceMtime: srcMtime,
		DestMtime:   dstInfo.ModTime().Unix(),
		IsDirectory: dstInfo.IsDir(),
	}

	// Register the pending conflict BEFORE emitting so the frontend can
	// call ResolveConflict synchronously without a race.
	pc := globalConflicts.register(conflictID)

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "operation:conflict", evt)
	}

	// Block until the frontend responds or the operation is cancelled.
	select {
	case resolution := <-pc.response:
		switch resolution.Resolution {
		case ConflictSkip:
			return ConflictSkip, dst, nil
		case ConflictOverwrite:
			return ConflictOverwrite, dst, nil
		case ConflictRename:
			renamed, rerr := uniqueDestination(dst)
			if rerr != nil {
				return "", "", rerr
			}
			return ConflictRename, renamed, nil
		case ConflictCancel:
			return ConflictCancel, dst, nil
		default:
			return ConflictSkip, dst, nil
		}
	case <-op.cancel:
		globalConflicts.cancelAll()
		return ConflictCancel, dst, nil
	}
}

// uniqueDestination returns a path that does not yet exist by appending
// " (N)" before the extension until a free slot is found (up to 999).
func uniqueDestination(dst string) (string, error) {
	ext := filepath.Ext(dst)
	base := dst[:len(dst)-len(ext)]
	for i := 2; i <= 999; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free destination name for %q", dst)
}

// ============================================================================
// Batch Copy
// ============================================================================

func (a *App) BatchCopyWithProgress(req BatchRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpCopy, "Batch copy", "", "")

	// Calculate totals across all items.
	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := a.calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{
		OperationID: op.ID,
		Total:       len(req.Items),
	}

	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() {
			break
		}

		dst := item.Destination

		// For directories we place source *inside* dst if dst already is a dir.
		srcInfo, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Destination: dst, Error: err.Error(),
			})
			continue
		}

		if srcInfo.IsDir() {
			dst = filepath.Join(item.Destination, filepath.Base(item.Source))
		}

		resolution, finalDst, confErr := a.checkConflict(op, item.Source, dst, blanket)
		if confErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Destination: dst, Error: confErr.Error(),
			})
			continue
		}

		switch resolution {
		case ConflictCancel:
			op.Complete()
			bp.flush()
			return result, nil
		case ConflictSkip:
			result.Skipped++
			continue
		}

		// resolution is Overwrite or Rename (finalDst is already correct).
		if err := a.copyWithProgressBatch(item.Source, finalDst, req.Recursive, op, bp); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Destination: finalDst, Error: err.Error(),
			})
		} else {
			result.Succeeded++
		}

		// If the user chose ApplyToAll in the conflict dialog, the blanket was
		// already stored by checkConflict's caller.  We update it here by
		// checking whether the resolution we got was explicit.
		_ = blanket
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// BatchCopyWithProgressApplyToAll is the full version that honours the
// applyToAll flag returned in ConflictResolutionRequest.  The frontend can
// still call the simpler BatchCopyWithProgress which never stores a blanket.
func (a *App) BatchCopyWithProgressEx(req BatchRequest) (*BatchResult, error) {
	return a.runBatchCopy(req)
}

func (a *App) runBatchCopy(req BatchRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpCopy, "Batch copy", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := a.calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}

	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() {
			break
		}
		succeeded, skipped, err := a.processBatchCopyItem(item, req.Recursive, op, bp, &blanket)
		switch {
		case err != nil:
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Destination: item.Destination, Error: err.Error(),
			})
		case skipped:
			result.Skipped++
		case succeeded:
			result.Succeeded++
		}
		if blanket != nil && *blanket == ConflictCancel {
			break
		}
	}

	bp.flush()
	op.Complete()
	return result, nil
}

func (a *App) processBatchCopyItem(
	item BatchItem,
	recursive bool,
	op *ActiveOperation,
	bp *batchProgress,
	blanket **ConflictResolution,
) (succeeded, skipped bool, err error) {
	srcInfo, err := os.Stat(item.Source)
	if err != nil {
		return false, false, err
	}

	dst := item.Destination
	if srcInfo.IsDir() {
		dst = filepath.Join(item.Destination, filepath.Base(item.Source))
	}

	resolution, finalDst, confErr := a.checkConflictEx(op, item.Source, dst, blanket)
	if confErr != nil {
		return false, false, confErr
	}

	switch resolution {
	case ConflictCancel:
		cancel := ConflictCancel
		*blanket = &cancel
		return false, false, nil
	case ConflictSkip:
		return false, true, nil
	}

	if err := a.copyWithProgressBatch(item.Source, finalDst, recursive, op, bp); err != nil {
		return false, false, err
	}
	return true, false, nil
}

// ============================================================================
// Batch Move
// ============================================================================

func (a *App) BatchMoveWithProgress(req BatchRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpMove, "Batch move", "", "")

	var totalItems, totalBytes int64
	for _, item := range req.Items {
		n, b := a.calculateSize(item.Source)
		totalItems += n
		totalBytes += b
	}
	op.SetTotal(totalItems, totalBytes)

	bp := newBatchProgress(op)
	result := &BatchResult{OperationID: op.ID, Total: len(req.Items)}

	var blanket *ConflictResolution

	for _, item := range req.Items {
		if op.IsCancelled() {
			break
		}
		if blanket != nil && *blanket == ConflictCancel {
			break
		}

		srcInfo, err := os.Stat(item.Source)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Error: err.Error(),
			})
			continue
		}

		dst := item.Destination
		if srcInfo.IsDir() {
			dst = filepath.Join(item.Destination, filepath.Base(item.Source))
		}

		resolution, finalDst, confErr := a.checkConflictEx(op, item.Source, dst, &blanket)
		if confErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{
				Source: item.Source, Destination: dst, Error: confErr.Error(),
			})
			continue
		}

		switch resolution {
		case ConflictCancel:
			op.Complete()
			bp.flush()
			return result, nil
		case ConflictSkip:
			result.Skipped++
			continue
		}

		// Try atomic rename; fall back to copy+delete with progress.
		if err := os.Rename(item.Source, finalDst); err != nil {
			if err2 := a.copyWithProgressBatch(item.Source, finalDst, true, op, bp); err2 != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{
					Source: item.Source, Destination: finalDst, Error: err2.Error(),
				})
				continue
			}
			if err2 := os.RemoveAll(item.Source); err2 != nil {
				result.Failed++
				result.Errors = append(result.Errors, BatchItemError{
					Source: item.Source, Error: err2.Error(),
				})
				continue
			}
		} else {
			// Atomic rename succeeded — advance progress counters manually.
			bp.mu.Lock()
			bp.processedItems++
			bp.processedBytes += srcInfo.Size()
			items := bp.processedItems
			bytes := bp.processedBytes
			bp.mu.Unlock()
			bp.update(items, bytes, filepath.Base(item.Source), srcInfo.Size(), false)
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

func (a *App) BatchDeleteWithProgress(req BatchDeleteRequest) (*BatchResult, error) {
	op := a.operations.StartOperation(OpDelete, "Batch delete", "", "")

	var totalItems, totalBytes int64
	for _, p := range req.Paths {
		n, b := a.calculateSize(p)
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

		info, err := os.Lstat(p)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchItemError{Source: p, Error: err.Error()})
			continue
		}

		if info.IsDir() && req.Recursive {
			if err := a.deleteDirWithProgressBatch(p, op, bp); err != nil {
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
			items := bp.processedItems
			bytes := bp.processedBytes
			bp.mu.Unlock()
			bp.update(items, bytes, filepath.Base(p), size, false)
		}

		result.Succeeded++
	}

	bp.flush()
	op.Complete()
	return result, nil
}

// ============================================================================
// Internal copy with batchProgress (throttled)
// ============================================================================

func (a *App) copyWithProgressBatch(src, dst string, recursive bool, op *ActiveOperation, bp *batchProgress) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("source is a directory; use recursive copy")
		}
		return a.copyDirWithProgressBatch(src, dst, op, bp)
	}
	return a.copySingleFileWithProgressBatch(src, dst, info.Size(), op, bp)
}

func (a *App) copySingleFileWithProgressBatch(src, dst string, size int64, op *ActiveOperation, bp *batchProgress) error {
	if op.IsCancelled() {
		return fmt.Errorf("operation cancelled")
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	bp.mu.Lock()
	baseBytes := bp.processedBytes
	baseItems := bp.processedItems
	bp.mu.Unlock()

	// Announce start of this file.
	bp.update(baseItems, baseBytes, filepath.Base(src), size, false)

	buf := make([]byte, 32*1024)
	var written int64

	for {
		if op.IsCancelled() {
			dstFile.Close()
			os.Remove(dst)
			return fmt.Errorf("operation cancelled")
		}

		nr, readErr := srcFile.Read(buf)
		if nr > 0 {
			nw, writeErr := dstFile.Write(buf[:nr])
			if writeErr != nil {
				return writeErr
			}
			written += int64(nw)
			bp.update(baseItems, baseBytes+written, filepath.Base(src), size, false)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	bp.mu.Lock()
	bp.processedItems = baseItems + 1
	bp.processedBytes = baseBytes + written
	bp.mu.Unlock()
	bp.update(baseItems+1, baseBytes+written, "", 0, false)
	return nil
}

func (a *App) copyDirWithProgressBatch(src, dst string, op *ActiveOperation, bp *batchProgress) error {
	return walkDirOrdered(src, func(path string, isDir bool, size int64) error {
		if op.IsCancelled() {
			return fmt.Errorf("operation cancelled")
		}

		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if isDir {
			return os.MkdirAll(target, 0755)
		}
		return a.copySingleFileWithProgressBatch(path, target, size, op, bp)
	})
}

func (a *App) deleteDirWithProgressBatch(path string, op *ActiveOperation, bp *batchProgress) error {
	var files []string
	var dirs []string

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
		items := bp.processedItems
		bytes := bp.processedBytes
		bp.mu.Unlock()
		bp.update(items, bytes, filepath.Base(f), size, false)
	}

	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}
	return os.Remove(path)
}

// walkDirOrdered walks src calling fn for every entry.
func walkDirOrdered(src string, fn func(path string, isDir bool, size int64) error) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		return fn(p, info.IsDir(), info.Size())
	})
}

// ============================================================================
// checkConflictEx — variant that updates *blanket on ApplyToAll responses
// ============================================================================

func (a *App) checkConflictEx(
	op *ActiveOperation,
	src, dst string,
	blanket **ConflictResolution,
) (res ConflictResolution, finalDst string, err error) {
	finalDst = dst

	dstInfo, statErr := os.Lstat(dst)
	if os.IsNotExist(statErr) {
		return ConflictOverwrite, dst, nil
	}
	if statErr != nil {
		return "", "", statErr
	}

	// Use blanket decision if already set.
	if *blanket != nil {
		switch **blanket {
		case ConflictSkip:
			return ConflictSkip, dst, nil
		case ConflictOverwrite:
			return ConflictOverwrite, dst, nil
		case ConflictRename:
			renamed, rerr := uniqueDestination(dst)
			if rerr != nil {
				return "", "", rerr
			}
			return ConflictRename, renamed, nil
		case ConflictCancel:
			return ConflictCancel, dst, nil
		}
	}

	srcInfo, srcStatErr := os.Lstat(src)
	var srcSize, srcMtime int64
	if srcStatErr == nil {
		srcSize = srcInfo.Size()
		srcMtime = srcInfo.ModTime().Unix()
	}

	conflictID := nextConflictID(op.ID)
	evt := ConflictEvent{
		OperationID: op.ID,
		ConflictID:  conflictID,
		SourcePath:  src,
		DestPath:    dst,
		SourceSize:  srcSize,
		DestSize:    dstInfo.Size(),
		SourceMtime: srcMtime,
		DestMtime:   dstInfo.ModTime().Unix(),
		IsDirectory: dstInfo.IsDir(),
	}

	pc := globalConflicts.register(conflictID)

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "operation:conflict", evt)
	}

	select {
	case resolution := <-pc.response:
		if resolution.ApplyToAll {
			r := resolution.Resolution
			*blanket = &r
		}
		switch resolution.Resolution {
		case ConflictSkip:
			return ConflictSkip, dst, nil
		case ConflictOverwrite:
			return ConflictOverwrite, dst, nil
		case ConflictRename:
			renamed, rerr := uniqueDestination(dst)
			if rerr != nil {
				return "", "", rerr
			}
			return ConflictRename, renamed, nil
		case ConflictCancel:
			return ConflictCancel, dst, nil
		default:
			return ConflictSkip, dst, nil
		}
	case <-op.cancel:
		globalConflicts.cancelAll()
		return ConflictCancel, dst, nil
	}
}
