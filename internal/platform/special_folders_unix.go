//go:build !windows

package platform

import (
	"os"
	"path/filepath"

	contracts "wails-app/internal/contracts"
)

func getSpecialFoldersPlatform() []contracts.SpecialFolder {
	home := getHomeDir()
	if home == "" {
		return nil
	}

	folders := []contracts.SpecialFolder{
		{Name: "Home", Path: home, Icon: "home"},
	}

	xdgDirs := []struct {
		name string
		icon string
		env  string
		def  string
	}{
		{name: "Desktop", icon: "desktop", env: "XDG_DESKTOP_DIR", def: "Desktop"},
		{name: "Documents", icon: "documents", env: "XDG_DOCUMENTS_DIR", def: "Documents"},
		{name: "Downloads", icon: "downloads", env: "XDG_DOWNLOAD_DIR", def: "Downloads"},
		{name: "Music", icon: "music", env: "XDG_MUSIC_DIR", def: "Music"},
		{name: "Pictures", icon: "pictures", env: "XDG_PICTURES_DIR", def: "Pictures"},
		{name: "Videos", icon: "videos", env: "XDG_VIDEOS_DIR", def: "Videos"},
	}

	for _, dir := range xdgDirs {
		path := os.Getenv(dir.env)
		if path == "" {
			path = filepath.Join(home, dir.def)
		}
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			folders = append(folders, contracts.SpecialFolder{
				Name: dir.name,
				Path: filepath.ToSlash(path),
				Icon: dir.icon,
			})
		}
	}

	return folders
}
