package platform

import (
	"os"
	"path/filepath"
	"runtime"

	contracts "wails-app/internal/contracts"
)

func GetOSInfo() contracts.OSInfoResponse {
	hostname, _ := os.Hostname()
	arch := runtime.GOARCH

	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	osType, version := getOSTypeAndVersion()
	var systemDrive *string
	if osType == contracts.OSWindows {
		drive := getSystemDrive()
		systemDrive = &drive
	}

	return contracts.OSInfoResponse{
		OS:          osType,
		Version:     version,
		Arch:        arch,
		Hostname:    hostname,
		SystemDrive: systemDrive,
	}
}

func ListDrives() contracts.DrivesList {
	return contracts.DrivesList{Drives: listDrivesPlatform()}
}

func GetSpecialFolders() contracts.SpecialFoldersResponse {
	return contracts.SpecialFoldersResponse{Folders: getSpecialFoldersPlatform()}
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.ToSlash(home)
}
