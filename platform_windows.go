//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
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

// symlinkPointsToDir returns true if the path (a symlink/junction/reparse point)
// points to a directory. Uses GetFileAttributes so it works even on protected
// system junctions (e.g. "Documents and Settings") that deny traversal.
func symlinkPointsToDir(path string) bool {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false
	}
	return attrs&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
}

// resolveLinkPath parses a Windows Shell Link (.lnk) file (MS-SHLLINK binary
// format) in pure Go — no COM, no PowerShell, no extra dependencies.
// Returns the target path and whether the target is a directory.
func resolveLinkPath(lnkPath string) (target string, isDir bool, err error) {
	data, err := os.ReadFile(lnkPath)
	if err != nil {
		return "", false, err
	}
	if len(data) < 76 {
		return "", false, fmt.Errorf(".lnk file too small")
	}
	// HeaderMagic must be 0x4C000000
	if data[0] != 0x4C || data[1] != 0 || data[2] != 0 || data[3] != 0 {
		return "", false, fmt.Errorf("not a valid .lnk file")
	}

	linkFlags := binary.LittleEndian.Uint32(data[20:24])
	fileAttrs := binary.LittleEndian.Uint32(data[24:28])
	isDir = (fileAttrs & 0x10) != 0 // FILE_ATTRIBUTE_DIRECTORY

	hasIDList := (linkFlags & 0x01) != 0
	hasLinkInfo := (linkFlags & 0x02) != 0
	isUnicode := (linkFlags & 0x80) != 0

	offset := 76 // skip fixed-size header

	// Skip optional LinkTargetIDList
	if hasIDList {
		if offset+2 > len(data) {
			return "", isDir, fmt.Errorf("unexpected EOF in IDList")
		}
		idListSize := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2 + idListSize
	}

	if !hasLinkInfo {
		return "", isDir, fmt.Errorf("no LinkInfo section")
	}
	if offset+32 > len(data) {
		return "", isDir, fmt.Errorf("unexpected EOF in LinkInfo")
	}

	liStart := offset
	liHeaderSize := int(binary.LittleEndian.Uint32(data[liStart+4 : liStart+8]))
	liFlags := binary.LittleEndian.Uint32(data[liStart+8 : liStart+12])

	if liFlags&0x01 == 0 {
		return "", isDir, fmt.Errorf("no local volume path in .lnk")
	}

	var path string
	// Prefer Unicode path when available (LinkInfoHeaderSize >= 0x24)
	if isUnicode && liHeaderSize >= 0x24 && liStart+32 <= len(data) {
		uOff := int(binary.LittleEndian.Uint32(data[liStart+28 : liStart+32]))
		pathStart := liStart + uOff
		if pathStart < len(data) {
			path = readUTF16LEString(data[pathStart:])
		}
	}
	// Fall back to ANSI path
	if path == "" {
		ansiOff := int(binary.LittleEndian.Uint32(data[liStart+16 : liStart+20]))
		pathStart := liStart + ansiOff
		if pathStart < len(data) {
			end := pathStart
			for end < len(data) && data[end] != 0 {
				end++
			}
			path = string(data[pathStart:end])
		}
	}

	if path == "" {
		return "", isDir, fmt.Errorf("could not extract target path from .lnk")
	}
	return filepath.ToSlash(path), isDir, nil
}

func readUTF16LEString(data []byte) string {
	var u16 []uint16
	for i := 0; i+1 < len(data); i += 2 {
		ch := binary.LittleEndian.Uint16(data[i : i+2])
		if ch == 0 {
			break
		}
		u16 = append(u16, ch)
	}
	return syscall.UTF16ToString(u16)
}
