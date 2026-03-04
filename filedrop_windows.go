package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// SetupFileDropHandler registers the app window to accept file drops.
// This uses Windows OLE drag & drop API to intercept native file drops.
func (a *App) SetupFileDropHandler(hwnd uintptr) error {
	// Register window class to accept drops on Windows
	// This is a Windows-specific implementation using DragAcceptFiles

	const (
		USER32DLL = "user32.dll"
	)

	user32 := windows.NewLazyDLL(USER32DLL)
	dragAcceptFiles := user32.NewProc("DragAcceptFiles")

	if dragAcceptFiles.Find() != nil {
		return fmt.Errorf("DragAcceptFiles not found")
	}

	// Enable drop for this window
	dragAcceptFiles.Call(hwnd, 1)

	fmt.Println("[SetupFileDropHandler] File drop handler registered for window")
	return nil
}

// RegisterDropListener sets up the native drop event listener.
// When files are dropped in Windows, WM_DROPFILES message is sent.
// We currently rely on browser drop events + clipboard fallback.
func (a *App) RegisterDropListener() {
	// Browser drop event handling is implemented in the frontend
	// with Windows clipboard as fallback for full file paths
}
