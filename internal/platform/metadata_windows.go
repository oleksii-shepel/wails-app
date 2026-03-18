//go:build windows

package platform

import (
	"os"
	"syscall"
	"time"
)

func CheckHidden(_ string, path string) bool {
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
