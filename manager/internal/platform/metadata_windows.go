//go:build windows

package platform

import (
	"os"
	"strings"
	"syscall"
	"time"

	"wails-app/internal/pathref"
)

func CheckHidden(_ string, path string) bool {
	if !isNativeMetadataPath(path) {
		return false
	}
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

func GetCreatedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return stat.CreationTime.Nanoseconds() / int64(time.Second)
	}
	return info.ModTime().Unix()
}

func GetAccessedTime(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return stat.LastAccessTime.Nanoseconds() / int64(time.Second)
	}
	return info.ModTime().Unix()
}

func isNativeMetadataPath(path string) bool {
	trimmed := ToSlash(path)
	if trimmed == "" {
		return false
	}
	if pathref.IsRegistry(trimmed) {
		return false
	}
	if pathref.IsAlias(trimmed) {
		return false
	}
	return pathref.IsDrive(trimmed) || strings.HasPrefix(trimmed, "//")
}
