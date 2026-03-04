//go:build windows

package main

import (
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Known folder GUIDs for Windows special folders
var (
	FOLDERID_Desktop   = windows.GUID{Data1: 0xB4BFCC3A, Data2: 0xDB2C, Data3: 0x424C, Data4: [8]byte{0xB0, 0x29, 0x7F, 0xE9, 0x9A, 0x87, 0xC6, 0x41}}
	FOLDERID_Documents = windows.GUID{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF, Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}
	FOLDERID_Downloads = windows.GUID{Data1: 0x374DE290, Data2: 0x123F, Data3: 0x4565, Data4: [8]byte{0x91, 0x64, 0x39, 0xC4, 0x92, 0x5E, 0x46, 0x7B}}
	FOLDERID_Music     = windows.GUID{Data1: 0x4BD8D571, Data2: 0x6D19, Data3: 0x48D3, Data4: [8]byte{0xBE, 0x97, 0x42, 0x22, 0x20, 0x08, 0x0E, 0x43}}
	FOLDERID_Pictures  = windows.GUID{Data1: 0x33E28130, Data2: 0x4E1E, Data3: 0x4676, Data4: [8]byte{0x83, 0x5A, 0x98, 0x39, 0x5C, 0x3B, 0xC3, 0xBB}}
	FOLDERID_Videos    = windows.GUID{Data1: 0x18989B1D, Data2: 0x99B5, Data3: 0x455B, Data4: [8]byte{0x84, 0x1C, 0xAB, 0x7C, 0x74, 0xE4, 0xDD, 0xFC}}
	FOLDERID_Profile   = windows.GUID{Data1: 0x5E6C858F, Data2: 0x0E22, Data3: 0x4760, Data4: [8]byte{0x9A, 0xFE, 0xEA, 0x33, 0x17, 0xB6, 0x71, 0x73}}
)

var (
	procSHGetKnownFolderPath = shell32.NewProc("SHGetKnownFolderPath")
)

// getKnownFolderPath retrieves the path of a known folder
func getKnownFolderPath(folderID *windows.GUID) string {
	var path *uint16
	ret, _, _ := procSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(folderID)),
		0, // dwFlags
		0, // hToken (current user)
		uintptr(unsafe.Pointer(&path)),
	)
	if ret != 0 {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(path))

	result := syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(path))[:])
	return filepath.ToSlash(result)
}

func getSpecialFoldersPlatform() []SpecialFolder {
	folders := []SpecialFolder{}

	// Home / User Profile
	if home := getHomeDir(); home != "" {
		folders = append(folders, SpecialFolder{
			Name: "Home",
			Path: home,
			Icon: "🏠",
		})
	}

	// Desktop
	if desktop := getKnownFolderPath(&FOLDERID_Desktop); desktop != "" {
		folders = append(folders, SpecialFolder{
			Name: "Desktop",
			Path: desktop,
			Icon: "🖥️",
		})
	}

	// Documents
	if documents := getKnownFolderPath(&FOLDERID_Documents); documents != "" {
		folders = append(folders, SpecialFolder{
			Name: "Documents",
			Path: documents,
			Icon: "📄",
		})
	}

	// Downloads
	if downloads := getKnownFolderPath(&FOLDERID_Downloads); downloads != "" {
		folders = append(folders, SpecialFolder{
			Name: "Downloads",
			Path: downloads,
			Icon: "📥",
		})
	}

	// Music
	if music := getKnownFolderPath(&FOLDERID_Music); music != "" {
		folders = append(folders, SpecialFolder{
			Name: "Music",
			Path: music,
			Icon: "🎵",
		})
	}

	// Pictures
	if pictures := getKnownFolderPath(&FOLDERID_Pictures); pictures != "" {
		folders = append(folders, SpecialFolder{
			Name: "Pictures",
			Path: pictures,
			Icon: "🖼️",
		})
	}

	// Videos
	if videos := getKnownFolderPath(&FOLDERID_Videos); videos != "" {
		folders = append(folders, SpecialFolder{
			Name: "Videos",
			Path: videos,
			Icon: "🎬",
		})
	}

	return folders
}
