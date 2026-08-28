//go:build windows

package platform

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	seFileObject             = 1
	ownerSecurityInformation = 0x00000001
)

var (
	advapi32Owner             = syscall.NewLazyDLL("advapi32.dll")
	procGetNamedSecurityInfoW = advapi32Owner.NewProc("GetNamedSecurityInfoW")
)

func GetFileOwner(path string, _ os.FileInfo) string {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var owner *syscall.SID
	var descriptor uintptr
	result, _, _ := procGetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(seFileObject),
		uintptr(ownerSecurityInformation),
		uintptr(unsafe.Pointer(&owner)),
		0, 0, 0,
		uintptr(unsafe.Pointer(&descriptor)),
	)
	if descriptor != 0 {
		defer syscall.LocalFree(syscall.Handle(descriptor))
	}
	if result != 0 || owner == nil {
		return ""
	}
	account, domain, _, err := owner.LookupAccount("")
	if err != nil {
		sid, sidErr := owner.String()
		if sidErr == nil {
			return sid
		}
		return ""
	}
	if domain != "" {
		return domain + "\\" + account
	}
	return account
}

func FormatFileAttributes(path string, _ os.FileInfo) string {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return ""
	}
	flags := make([]byte, 0, 4)
	if attrs&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		flags = append(flags, 'R')
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		flags = append(flags, 'H')
	}
	if attrs&syscall.FILE_ATTRIBUTE_SYSTEM != 0 {
		flags = append(flags, 'S')
	}
	if attrs&syscall.FILE_ATTRIBUTE_ARCHIVE != 0 {
		flags = append(flags, 'A')
	}
	return string(flags)
}
