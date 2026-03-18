//go:build windows

package platform

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

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	_                     [4]byte
	pFrom                 uintptr
	pTo                   uintptr
	fFlags                uint16
	_                     [2]byte
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
	fofWantNukeWarn uint16 = 0x4000
)

func MoveToTrash(path string) error {
	path = normalizeShellTrashPath(path)
	if path == "" {
		return fmt.Errorf("trash: empty path")
	}

	from, err := syscall.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("trash: invalid path %q: %w", path, err)
	}
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
	p = filepath.FromSlash(p)
	p = filepath.Clean(p)
	return strings.TrimPrefix(p, `\\?\`)
}
