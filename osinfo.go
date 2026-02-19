package main

import (
	"os"
	"runtime"
)

// GetOSInfo returns information about the current operating system.
func (a *App) GetOSInfo() OSInfoResponse {
	hostname, _ := os.Hostname()
	arch := runtime.GOARCH

	// Normalize GOARCH to match protocol expectations
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	osType, version := getOSTypeAndVersion()

	var systemDrive *string
	if osType == OSWindows {
		drive := getSystemDrive()
		systemDrive = &drive
	}

	return OSInfoResponse{
		OS:          osType,
		Version:     version,
		Arch:        arch,
		Hostname:    hostname,
		SystemDrive: systemDrive,
	}
}

// ListDrives returns all mounted drives/volumes on the system.
func (a *App) ListDrives() DrivesList {
	drives := listDrivesPlatform()
	return DrivesList{Drives: drives}
}
