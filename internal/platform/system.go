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
	return contracts.DrivesList{Schemes: append(listDrivesPlatform(), syntheticDrives()...)}
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
	drives := []contracts.DriveInfo{
		{
			Name:       "Memory",
			Path:       pathref.MemoryScheme,
			SchemeType: contracts.SchemeMemory,
		},
		{
			Name:       "Registry",
			Path:       pathref.RegistryScheme,
			SchemeType: contracts.SchemeRegistry,
		},
		{
			Name:       "Search",
			Path:       pathref.SearchScheme,
			SchemeType: contracts.SchemeSearch,
		},
		{
			Name:       "HTTP",
			Path:       pathref.HTTPAliasScheme,
			SchemeType: contracts.SchemeHTTP,
		},
		{
			Name:       "HTTPS",
			Path:       pathref.HTTPSAliasScheme,
			SchemeType: contracts.SchemeHTTPS,
		},
		{
			Name:       "SMB",
			Path:       pathref.SMBScheme,
			SchemeType: contracts.SchemeSMB,
		},
		{
			Name:       "S3",
			Path:       pathref.S3Scheme,
			SchemeType: contracts.SchemeS3,
		},
	}

	for _, provider := range pathref.AvailableLocalCloudProviders() {
		drives = append(drives, contracts.DriveInfo{
			Name:       provider.Name,
			Path:       provider.Scheme,
			SchemeType: cloudSchemeType(provider.Scheme),
		})
	}

	return drives
}

func cloudSchemeType(scheme string) contracts.SchemeType {
	switch scheme {
	case pathref.OneDriveScheme:
		return contracts.SchemeCloudOneDrive
	case pathref.GDriveScheme:
		return contracts.SchemeCloudGoogle
	case pathref.DropboxScheme:
		return contracts.SchemeCloudDropbox
	default:
		return contracts.SchemeUnknown
	}
}
