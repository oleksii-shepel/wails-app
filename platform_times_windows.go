//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

func checkHidden(name, path string) bool {
	// On Windows a file is hidden if the Hidden file attribute is set,
	// OR if the name starts with '.' (by convention some tools use this too).
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false
	}
	return attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}

func getCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return stat.CreationTime.Nanoseconds() / int64(time.Second)
	}
	return info.ModTime().Unix()
}

func getAccessedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return stat.LastAccessTime.Nanoseconds() / int64(time.Second)
	}
	return info.ModTime().Unix()
}
