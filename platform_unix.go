//go:build !windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func getOSTypeAndVersion() (OSType, string) {
	switch runtime.GOOS {
	case "linux":
		version := readLinuxVersion()
		return OSLinux, version
	case "darwin":
		version := readDarwinVersion()
		return OSDarwin, version
	default:
		return OSUnknown, "unknown"
	}
}

func readLinuxVersion() string {
	// Try /etc/os-release first
	f, err := os.Open("/etc/os-release")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VERSION_ID=") {
				v := strings.TrimPrefix(line, "VERSION_ID=")
				v = strings.Trim(v, `"`)
				return v
			}
		}
	}
	// Fallback: uname -r
	out, err := exec.Command("uname", "-r").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "unknown"
}

func readDarwinVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return "unknown"
}

func getSystemDrive() string {
	return "/" // Not used on non-Windows, but satisfies interface
}

func listDrivesPlatform() []DriveInfo {
	if runtime.GOOS == "darwin" {
		return listDrivesDarwin()
	}
	return listDrivesLinux()
}

// ---- Linux ----------------------------------------------------------------

func listDrivesLinux() []DriveInfo {
	var drives []DriveInfo

	f, err := os.Open("/proc/mounts")
	if err != nil {
		return drives
	}
	defer f.Close()

	// Filter to real filesystems only
	realFS := map[string]bool{
		"ext2": true, "ext3": true, "ext4": true,
		"xfs": true, "btrfs": true, "zfs": true,
		"vfat": true, "ntfs": true, "exfat": true,
		"fuseblk": true, "fuse": true,
	}

	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mount, fsType := fields[0], fields[1], fields[2]
		if !realFS[strings.ToLower(fsType)] {
			continue
		}
		if seen[mount] {
			continue
		}
		seen[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)

		name := device
		if idx := strings.LastIndex(device, "/"); idx >= 0 {
			name = device[idx+1:]
		}

		fs := fsType
		dt := guessDriveTypeLinux(device, mount)

		drives = append(drives, DriveInfo{
			Name:       name,
			Path:       mount,
			DriveType:  dt,
			TotalSpace: total,
			FreeSpace:  free,
			FileSystem: &fs,
		})
	}

	return drives
}

func guessDriveTypeLinux(device, mount string) DriveType {
	// Network mounts often have ':' or '//' in device name
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return DriveNetwork
	}
	// Optical drives
	if strings.Contains(device, "sr") || strings.Contains(device, "cdrom") {
		return DriveCDROM
	}
	// RAM disk
	if strings.HasPrefix(device, "tmpfs") || strings.HasPrefix(device, "ramfs") {
		return DriveRAMDisk
	}
	// Removable: check /sys/block/<dev>/removable
	name := device
	if idx := strings.LastIndex(device, "/"); idx >= 0 {
		name = device[idx+1:]
	}
	// Strip partition number to get block device name
	base := strings.TrimRight(name, "0123456789")
	removablePath := fmt.Sprintf("/sys/block/%s/removable", base)
	data, err := os.ReadFile(removablePath)
	if err == nil && strings.TrimSpace(string(data)) == "1" {
		return DriveRemovable
	}
	return DriveFixed
}

// ---- macOS ----------------------------------------------------------------

func listDrivesDarwin() []DriveInfo {
	var drives []DriveInfo

	out, err := exec.Command("df", "-k").Output()
	if err != nil {
		return drives
	}

	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		device := fields[0]
		mount := fields[8]

		// Skip devfs, autofs, etc.
		if !strings.HasPrefix(device, "/dev/") && !strings.HasPrefix(device, "//") {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)

		name := device
		if idx := strings.LastIndex(device, "/"); idx >= 0 {
			name = device[idx+1:]
		}

		fsType := getFSTypeDarwin(mount)
		dt := guessDriveTypeDarwin(device)

		drives = append(drives, DriveInfo{
			Name:       name,
			Path:       mount,
			DriveType:  dt,
			TotalSpace: total,
			FreeSpace:  free,
			FileSystem: &fsType,
		})
	}

	return drives
}

func getFSTypeDarwin(mount string) string {
	out, err := exec.Command("stat", "-f", "%T", mount).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func guessDriveTypeDarwin(device string) DriveType {
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return DriveNetwork
	}
	// macOS removable: diskutil info would be ideal, but keep it simple
	if strings.Contains(strings.ToLower(device), "disk") {
		return DriveFixed
	}
	return DriveUnknown
}

func openFileWithDefaultApp(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	default: // linux and others
		return exec.Command("xdg-open", path).Start()
	}
}

// symlinkPointsToDir returns true if following the symlink leads to a directory.
func symlinkPointsToDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// isUntrackedDirectoryJunction is a no-op on Unix: Go's Lstat always correctly
// identifies symlinks and directories, so no last-resort check is needed.
func isUntrackedDirectoryJunction(_ string) bool { return false }

// resolveLinkPath follows a Unix symlink and returns the real target path
// together with whether that target is a directory.
func resolveLinkPath(symlinkPath string) (target string, isDir bool, err error) {
	resolved, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false, err
	}
	return resolved, info.IsDir(), nil
}
