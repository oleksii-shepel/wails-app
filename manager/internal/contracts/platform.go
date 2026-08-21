package contracts

import core "github.com/epikodelabs/reassembly/core/contracts"

type OSType = core.OSType

const (
	OSWindows = core.OSWindows
	OSLinux   = core.OSLinux
	OSDarwin  = core.OSDarwin
	OSUnknown = core.OSUnknown
)

type OSInfoResponse = core.OSInfoResponse
type SchemeType = core.SchemeType

const (
	SchemeFixed         = core.SchemeFixed
	SchemeRemovable     = core.SchemeRemovable
	SchemeNetwork       = core.SchemeNetwork
	SchemeCDROM         = core.SchemeCDROM
	SchemeRAMDisk       = core.SchemeRAMDisk
	SchemeRegistry      = core.SchemeRegistry
	SchemeSearch        = core.SchemeSearch
	SchemeHTTP          = core.SchemeHTTP
	SchemeHTTPS         = core.SchemeHTTPS
	SchemeSMB           = core.SchemeSMB
	SchemeS3            = core.SchemeS3
	SchemeCloudOneDrive = core.SchemeCloudOneDrive
	SchemeCloudGoogle   = core.SchemeCloudGoogle
	SchemeCloudDropbox  = core.SchemeCloudDropbox
	SchemeUnknown       = core.SchemeUnknown
)

type DriveInfo = core.DriveInfo
type DrivesList = core.DrivesList
type SpecialFolder = core.SpecialFolder
type SpecialFoldersResponse = core.SpecialFoldersResponse
