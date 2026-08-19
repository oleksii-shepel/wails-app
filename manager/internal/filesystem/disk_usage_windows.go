//go:build windows

package filesystem

import (
	"os"
	"syscall"
)

func isLink(entry os.DirEntry) bool {
	if entry.Type()&os.ModeSymlink != 0 {
		return true
	}
	info, err := entry.Info()
	if err != nil {
		return false
	}
	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	const fileAttributeReparsePoint = 0x400
	return sys.FileAttributes&fileAttributeReparsePoint != 0
}