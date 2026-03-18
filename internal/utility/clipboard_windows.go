//go:build windows

package utility

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	dragQueryFile    = shell32.NewProc("DragQueryFileW")
)

func GetFilePaths() ([]string, error) {
	ret, _, err := openClipboard.Call()
	if ret == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	const cfHdrop = 15
	ret, _, err = getClipboardData.Call(cfHdrop)
	hDrop := uintptr(ret)
	if hDrop == 0 {
		return nil, fmt.Errorf("no clipboard data in CF_HDROP format")
	}

	ret, _, _ = dragQueryFile.Call(hDrop, ^uintptr(0), 0, 0)
	fileCount := ret
	if fileCount == 0 {
		return nil, fmt.Errorf("no files in clipboard drag data")
	}

	paths := make([]string, 0, fileCount)
	for index := uintptr(0); index < fileCount; index++ {
		ret, _, _ = dragQueryFile.Call(hDrop, index, 0, 0)
		bufferSize := ret
		if bufferSize == 0 {
			continue
		}

		pathBuf := make([]uint16, bufferSize+1)
		dragQueryFile.Call(hDrop, index, uintptr(unsafe.Pointer(&pathBuf[0])), bufferSize+1)

		path := syscall.UTF16ToString(pathBuf)
		if path != "" {
			paths = append(paths, path)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("could not extract file paths from clipboard")
	}
	return paths, nil
}

func ReadText() (string, error) {
	ret, _, err := openClipboard.Call()
	if ret == 0 {
		return "", fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	const cfUnicodeText = 13
	ret, _, err = getClipboardData.Call(cfUnicodeText)
	if ret == 0 {
		ret, _, _ = getClipboardData.Call(1)
		if ret == 0 {
			return "", fmt.Errorf("no text in clipboard")
		}
	}
	return "[clipboard text data available]", nil
}
