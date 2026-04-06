package platform

import (
	"os"
	"path/filepath"
	"runtime"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
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
	return contracts.DrivesList{Drives: append(listDrivesPlatform(), syntheticDrives()...)}
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

func syntheticDrives() []contracts.DriveInfo {
	return []contracts.DriveInfo{
		{
			Name:      "Memory",
			Path:      pathref.MemoryScheme,
			DriveType: contracts.DriveMemory,
		},
		{
			Name:      "Registry",
			Path:      pathref.RegistryScheme,
			DriveType: contracts.DriveRegistry,
		},
		{
			Name:      "Search",
			Path:      pathref.SearchScheme,
			DriveType: contracts.DriveSearch,
		},
		{
			Name:      "HTTP",
			Path:      pathref.HTTPAliasScheme,
			DriveType: contracts.DriveHTTP,
		},
		{
			Name:      "HTTPS",
			Path:      pathref.HTTPSAliasScheme,
			DriveType: contracts.DriveHTTPS,
		},
		{
			Name:      "SMB",
			Path:      pathref.SMBScheme,
			DriveType: contracts.DriveSMB,
		},
		{
			Name:      "FTP",
			Path:      pathref.FTPScheme,
			DriveType: contracts.DriveFTP,
		},
		{
			Name:      "SFTP",
			Path:      pathref.SFTPScheme,
			DriveType: contracts.DriveSFTP,
		},
		{
			Name:      "WebDAV",
			Path:      pathref.WebDAVScheme,
			DriveType: contracts.DriveWebDAV,
		},
	}
}
