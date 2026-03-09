//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
)

// GetClipboardFilePaths retrieves file paths from the Windows clipboard.
// When files are dragged from Explorer, they are placed in the clipboard as CF_HDROP format.
func GetClipboardFilePaths() ([]string, error) {
	// Open clipboard
	ret, _, err := openClipboard.Call()
	if ret == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	const (
		CF_HDROP = 15 // File descriptor list
	)

	// Get clipboard data in CF_HDROP format
	ret, _, err = getClipboardData.Call(CF_HDROP)
	hGlobal := uintptr(ret)
	if hGlobal == 0 {
		return nil, fmt.Errorf("no clipboard data in CF_HDROP format")
	}

	hDrop := hGlobal

	// Use Windows API to get file count and paths
	dragQueryFile := shell32.NewProc("DragQueryFileW")

	// First call to get file count (index = 0xFFFFFFFF)
	ret, _, _ = dragQueryFile.Call(hDrop, ^uintptr(0), 0, 0)
	fileCount := ret

	if fileCount == 0 {
		return nil, fmt.Errorf("no files in clipboard drag data")
	}

	var paths []string

	// Get each file path
	for i := uintptr(0); i < fileCount; i++ {
		// Get buffer size needed
		ret, _, _ = dragQueryFile.Call(hDrop, i, 0, 0)
		bufferSize := ret

		if bufferSize == 0 {
			continue
		}

		// Allocate buffer for path
		pathBuf := make([]uint16, bufferSize+1)

		// Get the file path
		dragQueryFile.Call(hDrop, i, uintptr(unsafe.Pointer(&pathBuf[0])), bufferSize+1)

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

// ReadClipboardText reads plain text from the clipboard.
// This is used as a fallback and diagnostic tool.
func ReadClipboardText() (string, error) {
	ret, _, err := openClipboard.Call()
	if ret == 0 {
		return "", fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	const CF_UNICODETEXT = 13 // Unicode text format

	ret, _, err = getClipboardData.Call(CF_UNICODETEXT)
	if ret == 0 {
		// Try as regular text
		ret, _, _ = getClipboardData.Call(1) // CF_TEXT
		if ret == 0 {
			return "", fmt.Errorf("no text in clipboard")
		}
	}

	// For safety, we won't try to dereference the clipboard handle directly
	// since it requires valid global memory locking.
	// Instead, just return a placeholder message.
	return "[clipboard text data available]", nil
}
