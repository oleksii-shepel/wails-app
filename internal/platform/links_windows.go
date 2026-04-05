//go:build windows

package platform

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"wails-app/internal/pathref"
)

const (
	swShownormal = 1
	seErrNoAssoc = 31
)

func OpenFileWithDefaultApp(path string) error {
	ret, callErr, err := shellExecute("open", path)
	if err != nil {
		return err
	}
	if ret > 32 {
		return nil
	}
	if ret == seErrNoAssoc {
		fallbackRet, fallbackCallErr, fallbackErr := shellExecute("openas", path)
		if fallbackErr != nil {
			return fallbackErr
		}
		if fallbackRet > 32 {
			return nil
		}
		if fallbackCallErr != nil && fallbackCallErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecuteW failed: %w", fallbackCallErr)
		}
		return fmt.Errorf("ShellExecuteW failed with code %d", fallbackRet)
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("ShellExecuteW failed: %w", callErr)
	}
	return fmt.Errorf("ShellExecuteW failed with code %d", ret)
}

func shellExecute(verbValue string, path string) (uintptr, error, error) {
	verb, err := syscall.UTF16PtrFromString(verbValue)
	if err != nil {
		return 0, nil, err
	}
	target, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, nil, err
	}

	shell32 := windows.NewLazySystemDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	ret, _, callErr := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		swShownormal,
	)
	if callErr != nil && callErr != syscall.Errno(0) {
		return ret, callErr, nil
	}
	return ret, nil, nil
}

func SymlinkPointsToDir(path string) bool {
	if !isNativeLinkPath(path) {
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
	return attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
}

func IsUntrackedDirectoryJunction(path string) bool {
	if !isNativeLinkPath(path) {
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
	const reparsePoint = uint32(0x400)
	return attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 && attrs&reparsePoint != 0
}

func ResolveLinkPath(lnkPath string) (target string, isDir bool, err error) {
	data, err := os.ReadFile(lnkPath)
	if err != nil {
		return "", false, err
	}
	if len(data) < 76 {
		return "", false, fmt.Errorf(".lnk file too small")
	}
	if data[0] != 0x4C || data[1] != 0 || data[2] != 0 || data[3] != 0 {
		return "", false, fmt.Errorf("not a valid .lnk file")
	}

	linkFlags := binary.LittleEndian.Uint32(data[20:24])
	fileAttrs := binary.LittleEndian.Uint32(data[24:28])
	isDir = (fileAttrs & 0x10) != 0

	hasIDList := (linkFlags & 0x01) != 0
	hasLinkInfo := (linkFlags & 0x02) != 0
	isUnicode := (linkFlags & 0x80) != 0

	offset := 76
	if hasIDList {
		if offset+2 > len(data) {
			return "", isDir, fmt.Errorf("unexpected EOF in IDList")
		}
		idListSize := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2 + idListSize
	}

	if !hasLinkInfo {
		return "", isDir, fmt.Errorf("no LinkInfo section")
	}
	if offset+32 > len(data) {
		return "", isDir, fmt.Errorf("unexpected EOF in LinkInfo")
	}

	liStart := offset
	liHeaderSize := int(binary.LittleEndian.Uint32(data[liStart+4 : liStart+8]))
	liFlags := binary.LittleEndian.Uint32(data[liStart+8 : liStart+12])
	if liFlags&0x01 == 0 {
		return "", isDir, fmt.Errorf("no local volume path in .lnk")
	}

	var path string
	if isUnicode && liHeaderSize >= 0x24 && liStart+32 <= len(data) {
		unicodeOffset := int(binary.LittleEndian.Uint32(data[liStart+28 : liStart+32]))
		pathStart := liStart + unicodeOffset
		if pathStart < len(data) {
			path = readUTF16LEString(data[pathStart:])
		}
	}
	if path == "" {
		ansiOffset := int(binary.LittleEndian.Uint32(data[liStart+16 : liStart+20]))
		pathStart := liStart + ansiOffset
		if pathStart < len(data) {
			end := pathStart
			for end < len(data) && data[end] != 0 {
				end++
			}
			path = string(data[pathStart:end])
		}
	}

	if path == "" {
		return "", isDir, fmt.Errorf("could not extract target path from .lnk")
	}
	return filepath.ToSlash(path), isDir, nil
}

func readUTF16LEString(data []byte) string {
	var chars []uint16
	for i := 0; i+1 < len(data); i += 2 {
		ch := binary.LittleEndian.Uint16(data[i : i+2])
		if ch == 0 {
			break
		}
		chars = append(chars, ch)
	}
	return syscall.UTF16ToString(chars)
}

func isNativeLinkPath(path string) bool {
	trimmed := ToSlash(path)
	if trimmed == "" {
		return false
	}
	if pathref.IsRegistry(trimmed) {
		return false
	}
	if pathref.IsAlias(trimmed) && !pathref.IsMemory(trimmed) {
		return false
	}
	return pathref.IsDrive(trimmed) || strings.HasPrefix(trimmed, "//")
}
