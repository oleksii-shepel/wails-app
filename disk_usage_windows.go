//go:build windows

package main

import (
	"os"
	"syscall"
)

// isLink returns true for any entry that is a symbolic link, junction, or
// other reparse point (FILE_ATTRIBUTE_REPARSE_POINT) on Windows.
func isLink(e os.DirEntry) bool {
	if e.Type()&os.ModeSymlink != 0 {
		return true
	}
	info, err := e.Info()
	if err != nil {
		return false
	}
	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	const FILE_ATTRIBUTE_REPARSE_POINT = 0x400
	return sys.FileAttributes&FILE_ATTRIBUTE_REPARSE_POINT != 0
}
