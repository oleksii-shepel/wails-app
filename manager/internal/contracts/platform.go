package contracts

type OSType string

const (
	OSWindows OSType = "windows"
	OSLinux   OSType = "linux"
	OSDarwin  OSType = "macos"
	OSUnknown OSType = "unknown"
)

type OSInfoResponse struct {
	OS          OSType  `json:"os"`
	Version     string  `json:"version"`
	Arch        string  `json:"arch"`
	Hostname    string  `json:"hostname"`
	SystemDrive *string `json:"systemDrive"`
}

type SchemeType string

const (
	SchemeFixed         SchemeType = "fixed"
	SchemeRemovable     SchemeType = "removable"
	SchemeNetwork       SchemeType = "network"
	SchemeCDROM         SchemeType = "cdrom"
	SchemeRAMDisk       SchemeType = "ramdisk"
	SchemeRegistry      SchemeType = "registry"
	SchemeSearch        SchemeType = "search"
	SchemeHTTP          SchemeType = "http"
	SchemeHTTPS         SchemeType = "https"
	SchemeSMB           SchemeType = "smb"
	SchemeS3            SchemeType = "s3"
	SchemeCloudOneDrive SchemeType = "cloud-onedrive"
	SchemeCloudGoogle   SchemeType = "cloud-google"
	SchemeCloudDropbox  SchemeType = "cloud-dropbox"
	SchemeUnknown       SchemeType = "unknown"
)

type DriveInfo struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	SchemeType SchemeType `json:"schemeType"`
	TotalSpace int64      `json:"totalSpace"`
	FreeSpace  int64      `json:"freeSpace"`
	FileSystem *string    `json:"fileSystem"`
}

type DrivesList struct {
	Schemes []DriveInfo `json:"schemes"`
}

type SpecialFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Icon string `json:"icon"`
}

type SpecialFoldersResponse struct {
	Folders []SpecialFolder `json:"folders"`
}
