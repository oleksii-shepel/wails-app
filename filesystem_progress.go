package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyFileWithProgress copies with real-time progress tracking.
func (a *App) CopyFileWithProgress(source, destination string, recursive bool) (*OperationResult, error) {
	op := a.operations.StartOperation(OpCopy, "Copying files", source, destination)

	totalItems, totalBytes := a.calculateSize(source)
	op.SetTotal(totalItems, totalBytes)

	err := a.copyWithProgress(source, destination, recursive, op)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	op.Complete()
	return &OperationResult{
		Success:       true,
		Message:       "Copied successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

func (a *App) calculateSize(path string) (items, bytes int64) {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		items++
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				bytes += info.Size()
			}
		}
		return nil
	})
	return
}

func (a *App) copyWithProgress(src, dst string, recursive bool, op *ActiveOperation) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("source is a directory; use recursive copy")
		}
		return a.copyDirWithProgress(src, dst, op)
	}
	return a.copySingleFileWithProgress(src, dst, info.Size(), op)
}

// copySingleFileWithProgress copies one file and emits granular byte-level
// progress updates.
//
// The progress counters on the operation accumulate across all files in a
// multi-file copy. This function receives baseBytes — the number of bytes
// already accounted for by previously completed files — and adds only the
// bytes written for THIS file on top of that value.
func (a *App) copySingleFileWithProgress(src, dst string, size int64, op *ActiveOperation) error {
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

	// Snapshot the already-processed byte count BEFORE this file starts so
	// we can add only the delta for this file to the running total.
	op.mu.RLock()
	baseBytes := op.Progress.ProcessedBytes
	baseItems := op.Progress.ProcessedItems
	op.mu.RUnlock()

	// Announce we have started this file (0 bytes written yet).
	op.UpdateProgress(baseItems, baseBytes, filepath.Base(src), size)

	buf := make([]byte, 32*1024) // 32 KB
	var written int64

	for {
		if op.IsCancelled() {
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
			// baseBytes + written gives the correct cumulative total across
			// ALL files processed so far.
			op.UpdateProgress(baseItems, baseBytes+written, filepath.Base(src), size)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	// Mark this file as fully processed: increment the item counter.
	op.UpdateProgress(baseItems+1, baseBytes+written, "", 0)
	return nil
}

func (a *App) copyDirWithProgress(src, dst string, op *ActiveOperation) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if op.IsCancelled() {
			return fmt.Errorf("operation cancelled")
		}

		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		info, _ := d.Info()
		return a.copySingleFileWithProgress(path, target, info.Size(), op)
	})
}

// MoveFileWithProgress moves with progress tracking.
func (a *App) MoveFileWithProgress(source, destination string) (*OperationResult, error) {
	op := a.operations.StartOperation(OpMove, "Moving files", source, destination)

	// Try atomic rename first — instant, no byte-level progress needed.
	err := os.Rename(source, destination)
	if err != nil {
		// Cross-device: fall back to copy-then-delete with progress.
		totalItems, totalBytes := a.calculateSize(source)
		op.SetTotal(totalItems, totalBytes)

		err = a.copyWithProgress(source, destination, true, op)
		if err != nil {
			op.SetError(err)
			return nil, err
		}

		if err := os.RemoveAll(source); err != nil {
			op.SetError(err)
			return nil, err
		}
	}

	op.Complete()
	return &OperationResult{
		Success:       true,
		Message:       "Moved successfully",
		AffectedPaths: []string{source, destination},
	}, nil
}

// DeleteFileWithProgress deletes with progress tracking.
func (a *App) DeleteFileWithProgress(path string, recursive bool) (*OperationResult, error) {
	op := a.operations.StartOperation(OpDelete, "Deleting files", path, "")

	info, err := os.Stat(path)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	if info.IsDir() && recursive {
		totalItems, totalBytes := a.calculateSize(path)
		op.SetTotal(totalItems, totalBytes)
		err = a.deleteDirWithProgress(path, op)
	} else {
		op.SetTotal(1, info.Size())
		err = os.Remove(path)
		op.UpdateProgress(1, info.Size(), filepath.Base(path), info.Size())
	}

	if err != nil {
		op.SetError(err)
		return nil, err
	}

	op.Complete()
	return &OperationResult{
		Success:       true,
		Message:       "Deleted successfully",
		AffectedPaths: []string{path},
	}, nil
}

func (a *App) deleteDirWithProgress(path string, op *ActiveOperation) error {
	var files []string
	var dirs []string

	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == path {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})

	// Delete files with progress.
	for _, f := range files {
		if op.IsCancelled() {
			return fmt.Errorf("operation cancelled")
		}

		var size int64
		if info, err := os.Stat(f); err == nil {
			size = info.Size()
		}

		os.Remove(f)

		op.mu.RLock()
		prevItems := op.Progress.ProcessedItems
		prevBytes := op.Progress.ProcessedBytes
		op.mu.RUnlock()

		op.UpdateProgress(prevItems+1, prevBytes+size, filepath.Base(f), size)
	}

	// Delete directories in reverse order (deepest first).
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}

	return os.Remove(path)
}
