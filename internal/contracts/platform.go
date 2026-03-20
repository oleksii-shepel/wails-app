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

type DriveType string

const (
	DriveFixed     DriveType = "fixed"
	DriveRemovable DriveType = "removable"
	DriveNetwork   DriveType = "network"
	DriveCDROM     DriveType = "cdrom"
	DriveRAMDisk   DriveType = "ramdisk"
	DriveMemory    DriveType = "memory"
	DriveRegistry  DriveType = "registry"
	DriveSearch    DriveType = "search"
	DriveSMB       DriveType = "smb"
	DriveFTP       DriveType = "ftp"
	DriveSFTP      DriveType = "sftp"
	DriveWebDAV    DriveType = "webdav"
	DriveUnknown   DriveType = "unknown"
)

type DriveInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	DriveType  DriveType `json:"driveType"`
	TotalSpace int64     `json:"totalSpace"`
	FreeSpace  int64     `json:"freeSpace"`
	FileSystem *string   `json:"fileSystem"`
}

type DrivesList struct {
	Drives []DriveInfo `json:"drives"`
}

type SpecialFolder struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Icon string `json:"icon"`
}

type SpecialFoldersResponse struct {
	Folders []SpecialFolder `json:"folders"`
}
