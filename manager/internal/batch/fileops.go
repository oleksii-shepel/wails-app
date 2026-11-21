package batch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	contracts "wails-app/internal/contracts"
)

type FileOperator struct {
	isCancelled func() bool
	progress    *ProgressTracker
}

func NewFileOperator(isCancelled func() bool, progress *ProgressTracker) *FileOperator {
	return &FileOperator{isCancelled: isCancelled, progress: progress}
}

func SamePath(a, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanA, cleanB)
	}
	return cleanA == cleanB
}

func ApplyResolution(res contracts.ConflictResolution, dst string) (string, bool, error) {
	switch res {
	case contracts.ConflictSkip:
		return dst, true, nil
	case contracts.ConflictOverwrite:
		return dst, false, nil
	case contracts.ConflictRename:
		renamed, err := uniqueDestination(dst)
		return renamed, false, err
	case contracts.ConflictCancel:
		return "", true, fmt.Errorf("operation cancelled by user")
	default:
		return dst, true, nil
	}
}

func (f *FileOperator) StreamToFile(r io.Reader, dst string, size int64, label string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	baseItems, baseBytes := f.progress.Snapshot()
	f.progress.Set(baseItems, baseBytes, label, size, false)

	buf := make([]byte, 32*1024)
	var written int64
	for {
		if f.isCancelled != nil && f.isCancelled() {
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
			f.progress.Set(baseItems, baseBytes+written, label, size, false)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, readErr
		}
	}

	f.progress.Set(baseItems+1, baseBytes+written, "", 0, false)
	return written, nil
}

func (f *FileOperator) CopyPath(src, dst string, recursive bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("source is a directory; use recursive=true")
		}
		return walkDirOrdered(src, func(path string, isDir bool, size int64) error {
			if f.isCancelled != nil && f.isCancelled() {
				return fmt.Errorf("operation cancelled")
			}
			rel, _ := filepath.Rel(src, path)
			target := filepath.Join(dst, rel)
			if isDir {
				return os.MkdirAll(target, 0755)
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = f.StreamToFile(file, target, size, filepath.Base(path))
			return err
		})
	}
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = f.StreamToFile(file, dst, info.Size(), filepath.Base(src))
	return err
}

func (f *FileOperator) DeleteDir(path string) error {
	var files, dirs []string
	walkDirOrdered(path, func(current string, isDir bool, _ int64) error {
		if current == path {
			return nil
		}
		if isDir {
			dirs = append(dirs, current)
		} else {
			files = append(files, current)
		}
		return nil
	})

	for _, file := range files {
		if f.isCancelled != nil && f.isCancelled() {
			return fmt.Errorf("operation cancelled")
		}
		var size int64
		if info, err := os.Stat(file); err == nil {
			size = info.Size()
		}
		os.Remove(file)
		f.progress.Advance(1, size, filepath.Base(file), size, false)
	}
	for index := len(dirs) - 1; index >= 0; index-- {
		os.Remove(dirs[index])
	}
	return os.Remove(path)
}

func uniqueDestination(dst string) (string, error) {
	ext := filepath.Ext(dst)
	base := dst[:len(dst)-len(ext)]
	if candidate := fmt.Sprintf("%s_copy%s", base, ext); pathAvailable(candidate) {
		return candidate, nil
	}
	for index := 2; index <= 999; index++ {
		candidate := fmt.Sprintf("%s_copy%d%s", base, index, ext)
		if pathAvailable(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a free name for %q", dst)
}

func pathAvailable(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

func walkDirOrdered(root string, visit func(path string, isDir bool, size int64) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		return visit(path, info.IsDir(), info.Size())
	})
}