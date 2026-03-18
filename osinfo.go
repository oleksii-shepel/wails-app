package main

import internalplatform "wails-app/internal/platform"

// GetOSInfo returns information about the current operating system.
func (a *App) GetOSInfo() OSInfoResponse {
	return internalplatform.GetOSInfo()
}

// ListDrives returns all mounted drives/volumes on the system.
func (a *App) ListDrives() DrivesList {
	return internalplatform.ListDrives()
}
