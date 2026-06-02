//go:build windows

package osfs

import (
	"strings"
	"syscall"
)

func isHiddenName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	// Check Windows FILE_ATTRIBUTE_HIDDEN. This is a best-effort check
	// for the filename alone; the full attribute check happens in
	// internal/platform when browsing through RawService.
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false
	}
	return attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}
