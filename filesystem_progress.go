package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyFileWithProgress copies with real-time progress tracking
func (a *App) CopyFileWithProgress(source, destination string, recursive bool) (*OperationResult, error) {
	// First, calculate total size
	op := a.operations.StartOperation(OpCopy, "Copying files", source, destination)

	// Calculate totals in background
	totalItems, totalBytes := a.calculateSize(source)
	op.SetTotal(totalItems, totalBytes)

	// Perform copy with progress
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

func (a *App) copySingleFileWithProgress(src, dst string, size int64, op *ActiveOperation) error {
	if op.IsCancelled() {
		return fmt.Errorf("operation cancelled")
	}

	// Ensure dest directory exists
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

	// Update progress with current file
	op.UpdateProgress(op.Progress.ProcessedItems, op.Progress.ProcessedBytes, filepath.Base(src), size)

	// Copy with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	var written int64
	for {
		if op.IsCancelled() {
			os.Remove(dst) // Clean up partial file
			return fmt.Errorf("operation cancelled")
		}

		nr, err := srcFile.Read(buf)
		if nr > 0 {
			nw, err := dstFile.Write(buf[:nr])
			if err != nil {
				return err
			}
			written += int64(nw)
			op.UpdateProgress(op.Progress.ProcessedItems, op.Progress.ProcessedBytes+written, filepath.Base(src), size)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Update final progress for this file
	op.UpdateProgress(op.Progress.ProcessedItems+1, op.Progress.ProcessedBytes+written, "", 0)
	return nil
}

func (a *App) copyDirWithProgress(src, dst string, op *ActiveOperation) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
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

// MoveFileWithProgress moves with progress tracking
func (a *App) MoveFileWithProgress(source, destination string) (*OperationResult, error) {
	op := a.operations.StartOperation(OpMove, "Moving files", source, destination)

	// Try atomic rename first
	err := os.Rename(source, destination)
	if err != nil {
		// Cross-device: copy then delete
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

// DeleteFileWithProgress deletes with progress tracking
func (a *App) DeleteFileWithProgress(path string, recursive bool) (*OperationResult, error) {
	op := a.operations.StartOperation(OpDelete, "Deleting files", path, "")

	info, err := os.Stat(path)
	if err != nil {
		op.SetError(err)
		return nil, err
	}

	if info.IsDir() && recursive {
		// Calculate size for progress
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
	// Walk and delete files first, then directories
	var files []string
	var dirs []string

	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == path {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, p)
		}
		return nil
	})

	// Delete files with progress
	for _, f := range files {
		if op.IsCancelled() {
			return fmt.Errorf("operation cancelled")
		}

		info, _ := os.Stat(f)
		size := int64(0)
		if info != nil {
			size = info.Size()
		}

		os.Remove(f)
		op.UpdateProgress(op.Progress.ProcessedItems+1, op.Progress.ProcessedBytes+size, filepath.Base(f), size)
	}

	// Delete directories (reverse order)
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i])
	}

	return os.Remove(path)
}
