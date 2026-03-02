//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func getOSTypeAndVersion() (OSType, string) {
	// Read Windows version from registry
	k, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return OSWindows, "unknown"
	}
	defer k.Close()

	major, _, _ := k.GetIntegerValue("CurrentMajorVersionNumber")
	minor, _, _ := k.GetIntegerValue("CurrentMinorVersionNumber")
	build, _, _ := k.GetStringValue("CurrentBuildNumber")
	ubr, _, _ := k.GetIntegerValue("UBR")

	version := ""
	if ubr > 0 {
		version = strings.Join([]string{
			itoa(int(major)), itoa(int(minor)), build, itoa(int(ubr)),
		}, ".")
	} else {
		version = strings.Join([]string{itoa(int(major)), itoa(int(minor)), build}, ".")
	}

	return OSWindows, version
}

func getSystemDrive() string {
	if sd := os.Getenv("SystemDrive"); sd != "" {
		return filepath.ToSlash(sd) + "/"
	}
	return "C:/"
}

func listDrivesPlatform() []DriveInfo {
	var drives []DriveInfo

	// GetLogicalDriveStrings returns a double-null-terminated list like
	// "C:\\\0D:\\\0\0"
	buf := make([]uint16, 256)
	n, err := windows.GetLogicalDriveStrings(uint32(len(buf)), &buf[0])
	if err != nil || n == 0 {
		return drives
	}

	// Split on null bytes
	raw := string(utf16ToString(buf[:n]))
	roots := strings.Split(raw, "\x00")

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}

		info := buildDriveInfo(root)
		drives = append(drives, info)
	}

	return drives
}

func buildDriveInfo(root string) DriveInfo {
	rootPtr, _ := syscall.UTF16PtrFromString(root)

	// Drive type
	driveTypeInt := windows.GetDriveType(rootPtr)
	dt := toDriveType(driveTypeInt)

	// Volume name = drive letter without backslash, e.g. "C:"
	name := strings.TrimSuffix(root, `\`)

	// Disk space
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	_ = windows.GetDiskFreeSpaceEx(rootPtr,
		(*uint64)(unsafe.Pointer(&freeBytesAvailable)),
		(*uint64)(unsafe.Pointer(&totalBytes)),
		(*uint64)(unsafe.Pointer(&totalFreeBytes)))

	// File system name (e.g. "NTFS")
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
		// Prefer volume label as display name if present
		vol := syscall.UTF16ToString(volNameBuf[:])
		if vol != "" {
			name = vol + " (" + name + ")"
		}
	}

	return DriveInfo{
		Name:       name,
		Path:       filepath.ToSlash(root),
		DriveType:  dt,
		TotalSpace: int64(totalBytes),
		FreeSpace:  int64(totalFreeBytes),
		FileSystem: fsName,
	}
}

func toDriveType(t uint32) DriveType {
	const (
		DRIVE_REMOVABLE = 2
		DRIVE_FIXED     = 3
		DRIVE_REMOTE    = 4
		DRIVE_CDROM     = 5
		DRIVE_RAMDISK   = 6
	)
	switch t {
	case DRIVE_REMOVABLE:
		return DriveRemovable
	case DRIVE_FIXED:
		return DriveFixed
	case DRIVE_REMOTE:
		return DriveNetwork
	case DRIVE_CDROM:
		return DriveCDROM
	case DRIVE_RAMDISK:
		return DriveRAMDisk
	default:
		return DriveUnknown
	}
}

func utf16ToString(s []uint16) string {
	return syscall.UTF16ToString(s)
}

func itoa(i int) string {
	return strings.TrimSpace(strings.Repeat(" ", 0) + intToStr(i))
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

func openFileWithDefaultApp(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}
