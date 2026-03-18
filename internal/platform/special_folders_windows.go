//go:build windows

package platform

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	contracts "wails-app/internal/contracts"
)

var (
	folderDesktop   = windows.GUID{Data1: 0xB4BFCC3A, Data2: 0xDB2C, Data3: 0x424C, Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}}
	folderDocuments = windows.GUID{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	folderDownloads = windows.GUID{Data1: 0x374DE290, Data2: 0x123F, Data3: 0x4565, Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}}
	folderMusic     = windows.GUID{Data1: 0x4BD8D571, Data2: 0x6D19, Data3: 0x48D3, Data4: [8]byte{0xBE, 0x97, 0x42, 0x22, 0x20, 0x08, 0x0E, 0x43}}
	folderPictures  = windows.GUID{Data1: 0x33E28130, Data2: 0x4E1E, Data3: 0x4676, Data4: [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}}
	folderVideos    = windows.GUID{Data1: 0x18989B1D, Data2: 0x99B5, Data3: 0x455B, Data4: [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}}
)

var procSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")

func getSpecialFoldersPlatform() []contracts.SpecialFolder {
	folders := make([]contracts.SpecialFolder, 0, 7)

	appendIfPresent := func(name, path, icon string) {
		if path != "" {
			folders = append(folders, contracts.SpecialFolder{Name: name, Path: path, Icon: icon})
		}
	}

	appendIfPresent("Home", getHomeDir(), "home")
	appendIfPresent("Desktop", getKnownFolderPath(&folderDesktop), "desktop")
	appendIfPresent("Documents", getKnownFolderPath(&folderDocuments), "documents")
	appendIfPresent("Downloads", getKnownFolderPath(&folderDownloads), "downloads")
	appendIfPresent("Music", getKnownFolderPath(&folderMusic), "music")
	appendIfPresent("Pictures", getKnownFolderPath(&folderPictures), "pictures")
	appendIfPresent("Videos", getKnownFolderPath(&folderVideos), "videos")

	return folders
}

func getKnownFolderPath(folderID *windows.GUID) string {
	var path *uint16
	ret, _, _ := procSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(folderID)),
		0,
		0,
		uintptr(unsafe.Pointer(&path)),
	)
	if ret != 0 {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(path))

	result := syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(path))[:])
	return filepath.ToSlash(result)
}
