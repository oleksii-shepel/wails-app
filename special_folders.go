package main

import (
	"os"
	"path/filepath"
)

// SpecialFolder represents a special system folder
type SpecialFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Icon string `json:"icon"`
}

// SpecialFoldersResponse contains all special folders
type SpecialFoldersResponse struct {
	Folders []SpecialFolder `json:"folders"`
}

// GetSpecialFolders returns a list of special system folders
func (a *App) GetSpecialFolders() SpecialFoldersResponse {
	folders := getSpecialFoldersPlatform()
	return SpecialFoldersResponse{Folders: folders}
}

// getHomeDir returns the user's home directory
func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.ToSlash(home)
}
