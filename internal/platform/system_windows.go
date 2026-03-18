//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	contracts "wails-app/internal/contracts"
)

func getOSTypeAndVersion() (contracts.OSType, string) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return contracts.OSWindows, "unknown"
	}
	defer key.Close()

	major, _, _ := key.GetIntegerValue("CurrentMajorVersionNumber")
	minor, _, _ := key.GetIntegerValue("CurrentMinorVersionNumber")
	build, _, _ := key.GetStringValue("CurrentBuildNumber")
	ubr, _, _ := key.GetIntegerValue("UBR")

	version := ""
	if ubr > 0 {
		version = strings.Join([]string{
			intToStr(int(major)), intToStr(int(minor)), build, intToStr(int(ubr)),
		}, ".")
	} else {
		version = strings.Join([]string{intToStr(int(major)), intToStr(int(minor)), build}, ".")
	}

	return contracts.OSWindows, version
}

func getSystemDrive() string {
	if drive := os.Getenv("SystemDrive"); drive != "" {
		return filepath.ToSlash(drive) + "/"
	}
	return "C:/"
}

func listDrivesPlatform() []contracts.DriveInfo {
	var drives []contracts.DriveInfo

	buf := make([]uint16, 256)
	n, err := windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0])
	if err != nil || n == 0 {
		return drives
	}

	roots := strings.Split(syscall.UTF16ToString(buf[:n]), "\x00")
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		drives = append(drives, buildDriveInfo(root))
	}

	return drives
}

func buildDriveInfo(root string) contracts.DriveInfo {
	rootPtr, _ := syscall.UTF16PtrFromString(root)
	driveType := toDriveType(windows.GetDriveType(rootPtr))
	name := strings.TrimSuffix(root, `\`)

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	_ = windows.GetDiskFreeSpaceEx(rootPtr,
		(*uint64)(unsafe.Pointer(&freeBytesAvailable)),
		(*uint64)(unsafe.Pointer(&totalBytes)),
		(*uint64)(unsafe.Pointer(&totalFreeBytes)))

	var fsNameBuf [256]uint16
	var volNameBuf [256]uint16
	err := windows.GetVolumeInformation(rootPtr,
		&volNameBuf[0], uint32(len(volNameBuf)),
		nil, nil, nil,
		&fsNameBuf[0], uint32(len(fsNameBuf)))

	var fsName *string
	if err == nil {
		fs := syscall.UTF16ToString(fsNameBuf[:])
		if fs != "" {
			fsName = &fs
		}
		vol := syscall.UTF16ToString(volNameBuf[:])
		if vol != "" {
			name = vol + " (" + name + ")"
		}
	}

	return contracts.DriveInfo{
		Name:       name,
		Path:       filepath.ToSlash(root),
		DriveType:  driveType,
		TotalSpace: int64(totalBytes),
		FreeSpace:  int64(totalFreeBytes),
		FileSystem: fsName,
	}
}

func toDriveType(kind uint32) contracts.DriveType {
	const (
		driveRemovable = 2
		driveFixed     = 3
		driveRemote    = 4
		driveCDROM     = 5
		driveRAMDisk   = 6
	)
	switch kind {
	case driveRemovable:
		return contracts.DriveRemovable
	case driveFixed:
		return contracts.DriveFixed
	case driveRemote:
		return contracts.DriveNetwork
	case driveCDROM:
		return contracts.DriveCDROM
	case driveRAMDisk:
		return contracts.DriveRAMDisk
	default:
		return contracts.DriveUnknown
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
