//go:build windows

package platform

import "syscall"

func EnsureHiddenPath(path string) error {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return nil
	}
	return syscall.SetFileAttributes(ptr, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}