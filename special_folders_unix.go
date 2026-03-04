//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func getSpecialFoldersPlatform() []SpecialFolder {
	folders := []SpecialFolder{}
	home := getHomeDir()
	if home == "" {
		return folders
	}

	// Home directory
	folders = append(folders, SpecialFolder{
		Name: "Home",
		Path: home,
		Icon: "🏠",
	})

	// Try XDG user directories
	xdgDirs := map[string]struct {
		name string
		icon string
		env  string
		def  string
	}{
		"Desktop": {
			name: "Desktop",
			icon: "🖥️",
			env:  "XDG_DESKTOP_DIR",
			def:  "Desktop",
		},
		"Documents": {
			name: "Documents",
			icon: "📄",
			env:  "XDG_DOCUMENTS_DIR",
			def:  "Documents",
		},
		"Downloads": {
			name: "Downloads",
			icon: "📥",
			env:  "XDG_DOWNLOAD_DIR",
			def:  "Downloads",
		},
		"Music": {
			name: "Music",
			icon: "🎵",
			env:  "XDG_MUSIC_DIR",
			def:  "Music",
		},
		"Pictures": {
			name: "Pictures",
			icon: "🖼️",
			env:  "XDG_PICTURES_DIR",
			def:  "Pictures",
		},
		"Videos": {
			name: "Videos",
			icon: "🎬",
			env:  "XDG_VIDEOS_DIR",
			def:  "Videos",
		},
	}

	for _, dir := range []string{"Desktop", "Documents", "Downloads", "Music", "Pictures", "Videos"} {
		info := xdgDirs[dir]

		// Try XDG environment variable first
		path := os.Getenv(info.env)
		if path == "" {
			// Fall back to default location
			path = filepath.Join(home, info.def)
		}

		// Check if directory exists
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			folders = append(folders, SpecialFolder{
				Name: info.name,
				Path: filepath.ToSlash(path),
				Icon: info.icon,
			})
		}
	}

	return folders
}
