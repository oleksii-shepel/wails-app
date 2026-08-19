//go:build !windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"

	contracts "wails-app/internal/contracts"
)

func GetPathCapacity(path string) (contracts.PathCapacity, error) {
	target := filepath.Clean(path)

	// Statfs needs an existing filesystem location. If the caller passed a file,
	// querying the containing directory is equivalent for capacity purposes.
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		target = filepath.Dir(target)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(target, &stat); err != nil {
		return contracts.PathCapacity{}, err
	}

	blockSize := int64(stat.Bsize)
	return contracts.PathCapacity{
		TotalSpace: int64(stat.Blocks) * blockSize,
		FreeSpace:  int64(stat.Bavail) * blockSize,
	}, nil
}