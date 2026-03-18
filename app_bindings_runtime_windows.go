//go:build windows

package main

import internalruntime "wails-app/internal/runtime"

func (a *App) SetupFileDropHandler(hwnd uintptr) error {
	return internalruntime.SetupFileDropHandler(hwnd)
}

func (a *App) RegisterDropListener() {
	internalruntime.RegisterDropListener()
}
