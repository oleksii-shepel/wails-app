package main

import internalruntime "wails-app/internal/runtime"

// SetupFileDropHandler registers the app window to accept file drops.
// This uses Windows OLE drag & drop API to intercept native file drops.
func (a *App) SetupFileDropHandler(hwnd uintptr) error {
	return internalruntime.SetupFileDropHandler(hwnd)
}

// RegisterDropListener sets up the native drop event listener.
// When files are dropped in Windows, WM_DROPFILES message is sent.
// We currently rely on browser drop events + clipboard fallback.
func (a *App) RegisterDropListener() {
	internalruntime.RegisterDropListener()
}
