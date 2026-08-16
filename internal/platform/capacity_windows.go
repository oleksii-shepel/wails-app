//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	contracts "wails-app/internal/contracts"
)

func GetPathCapacity(path string) (contracts.PathCapacity, error) {
	target := filepath.Clean(path)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		target = filepath.Dir(target)
	}

	volume := filepath.VolumeName(target)
	root := volume + `\`
	if volume == "" {
		root = target
	}

	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return contracts.PathCapacity{}, err
	}

	var freeAvailable uint64
	var total uint64
	var totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(
		rootPtr,
		(*uint64)(&freeAvailable),
		(*uint64)(&total),
		(*uint64)(&totalFree),
	); err != nil {
		return contracts.PathCapacity{}, err
	}

	var fsNameBuf [256]uint16
	var fsName *string
	if err := windows.GetVolumeInformation(
		rootPtr,
		nil,
		0,
		nil,
		nil,
		nil,
		&fsNameBuf[0],
		uint32(len(fsNameBuf)),
	); err == nil {
		fs := syscall.UTF16ToString(fsNameBuf[:])
		if fs != "" {
			fsName = &fs
		}
	}

	return contracts.PathCapacity{
		TotalSpace: int64(total),
		FreeSpace:  int64(freeAvailable),
		FileSystem: fsName,
	}, nil
}
