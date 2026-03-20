//go:build !windows

package platform

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	contracts "wails-app/internal/contracts"
)

func getOSTypeAndVersion() (contracts.OSType, string) {
	switch runtime.GOOS {
	case "linux":
		return contracts.OSLinux, readLinuxVersion()
	case "darwin":
		return contracts.OSDarwin, readDarwinVersion()
	default:
		return contracts.OSUnknown, "unknown"
	}
}

func readLinuxVersion() string {
	file, err := os.Open("/etc/os-release")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "VERSION_ID=") {
				return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			}
		}
	}

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
	return "/"
}

func listDrivesPlatform() []contracts.DriveInfo {
	if runtime.GOOS == "darwin" {
		return listDrivesDarwin()
	}
	return listDrivesLinux()
}

func listDrivesLinux() []contracts.DriveInfo {
	var drives []contracts.DriveInfo

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return drives
	}
	defer file.Close()

	realFS := map[string]bool{
		"ext2": true, "ext3": true, "ext4": true,
		"xfs": true, "btrfs": true, "zfs": true,
		"vfat": true, "ntfs": true, "exfat": true,
		"fuseblk": true, "fuse": true,
	}

	seen := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		device, mount, fsType := fields[0], fields[1], fields[2]
		if !realFS[strings.ToLower(fsType)] || seen[mount] {
			continue
		}
		seen[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)
		name := filepath.Base(device)
		if name == "." || name == string(filepath.Separator) {
			name = device
		}
		fs := fsType

		drives = append(drives, contracts.DriveInfo{
			Name:       name,
			Path:       mount,
			DriveType:  guessDriveTypeLinux(device),
			TotalSpace: total,
			FreeSpace:  free,
			FileSystem: &fs,
		})
	}

	return drives
}

func guessDriveTypeLinux(device string) contracts.DriveType {
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return contracts.DriveNetwork
	}
	if strings.Contains(device, "sr") || strings.Contains(device, "cdrom") {
		return contracts.DriveCDROM
	}
	if strings.HasPrefix(device, "tmpfs") || strings.HasPrefix(device, "ramfs") {
		return contracts.DriveRAMDisk
	}

	name := filepath.Base(device)
	base := strings.TrimRight(name, "0123456789")
	removablePath := fmt.Sprintf("/sys/block/%s/removable", base)
	data, err := os.ReadFile(removablePath)
	if err == nil && strings.TrimSpace(string(data)) == "1" {
		return contracts.DriveRemovable
	}
	return contracts.DriveFixed
}

func listDrivesDarwin() []contracts.DriveInfo {
	var drives []contracts.DriveInfo

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
		if !strings.HasPrefix(device, "/dev/") && !strings.HasPrefix(device, "//") {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		total := int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)
		name := filepath.Base(device)
		fsType := getFSTypeDarwin(mount)

		drives = append(drives, contracts.DriveInfo{
			Name:       name,
			Path:       mount,
			DriveType:  guessDriveTypeDarwin(device),
			TotalSpace: total,
			FreeSpace:  free,
			FileSystem: &fsType,
		})
	}

	return drives
}

// JoinMountNamespace makes the calling process join the specified mount namespace.
// The nsPath should be a path like "/proc/<pid>/ns/mnt".
func JoinMountNamespace(nsPath string) error {
	f, err := os.Open(nsPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := unix.Setns(int(f.Fd()), unix.CLONE_NEWNS); err != nil {
		return err
	}
	return nil
}

func getFSTypeDarwin(mount string) string {
	out, err := exec.Command("stat", "-f", "%T", mount).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func guessDriveTypeDarwin(device string) contracts.DriveType {
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return contracts.DriveNetwork
	}
	if strings.Contains(strings.ToLower(device), "disk") {
		return contracts.DriveFixed
	}
	return contracts.DriveUnknown
}
