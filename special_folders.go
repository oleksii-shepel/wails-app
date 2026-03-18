package main

import internalplatform "wails-app/internal/platform"

// GetSpecialFolders returns a list of special system folders
func (a *App) GetSpecialFolders() SpecialFoldersResponse {
	return internalplatform.GetSpecialFolders()
}
