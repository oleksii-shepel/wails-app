//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32         = syscall.NewLazyDLL("shell32.dll")
	shFileOperation = shell32.NewProc("SHFileOperationW")
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW.
// Field layout on 64-bit Windows (Go follows the same ABI as MSVC):
//
//	offset  0 : hwnd                    (8 bytes)
//	offset  8 : wFunc                   (4 bytes)
//	offset 12 : [4 bytes padding]
//	offset 16 : pFrom                   (8 bytes)
//	offset 24 : pTo                     (8 bytes)
//	offset 32 : fFlags                  (2 bytes)
//	offset 34 : [2 bytes padding]
//	offset 36 : fAnyOperationsAborted   (4 bytes)
//	offset 40 : hNameMappings           (8 bytes)
//	offset 48 : lpszProgressTitle       (8 bytes)
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	_                     [4]byte // padding to align pFrom to 8 bytes
	pFrom                 uintptr
	pTo                   uintptr
	fFlags                uint16
	_                     [2]byte // padding to align fAnyOperationsAborted to 4 bytes
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     uintptr
}

const (
	foDelete        uint32 = 0x0003
	fofSilent       uint16 = 0x0004
	fofNoConfirm    uint16 = 0x0010
	fofAllowUndo    uint16 = 0x0040
	fofNoErrorUI    uint16 = 0x0400
	fofWantNukeWarn uint16 = 0x4000 // warn before permanently deleting (too large for bin)
)

// moveToTrash moves path to the Windows Recycle Bin using SHFileOperationW.
// It works for both files and directories.
func moveToTrash(path string) error {
	path = normalizeShellTrashPath(path)
	if path == "" {
		return fmt.Errorf("trash: empty path")
	}

	// pFrom must be double-null-terminated UTF-16.
	from, err := syscall.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("trash: invalid path %q: %w", path, err)
	}
	// syscall.UTF16FromString already appends one null; append a second.
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  uintptr(unsafe.Pointer(&from[0])),
		fFlags: fofAllowUndo | fofNoConfirm | fofSilent | fofNoErrorUI | fofWantNukeWarn,
	}

	ret, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("trash: SHFileOperationW returned error code 0x%X (path=%q)", ret, path)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("trash: operation was aborted by the user or system")
	}
	return nil
}

func normalizeShellTrashPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	// Shell APIs are picky about slash direction.
	p = filepath.FromSlash(p)
	p = filepath.Clean(p)
	// SHFileOperationW does not support extended-length path prefixes.
	p = strings.TrimPrefix(p, `\\?\`)
	return p
}
