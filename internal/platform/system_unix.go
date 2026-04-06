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
			SchemeType: guessSchemeTypeLinux(device),
			TotalSpace: total,
			FreeSpace:  free,
			FileSystem: &fs,
		})
	}

	return drives
}

func guessSchemeTypeLinux(device string) contracts.SchemeType {
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return contracts.SchemeNetwork
	}
	if strings.Contains(device, "sr") || strings.Contains(device, "cdrom") {
		return contracts.SchemeCDROM
	}
	if strings.HasPrefix(device, "tmpfs") || strings.HasPrefix(device, "ramfs") {
		return contracts.SchemeRAMDisk
	}

	name := filepath.Base(device)
	for _, candidate := range linuxBlockDeviceCandidates(name) {
		removablePath := fmt.Sprintf("/sys/block/%s/removable", candidate)
		data, err := os.ReadFile(removablePath)
		if err == nil && strings.TrimSpace(string(data)) == "1" {
			return contracts.SchemeRemovable
		}
	}
	return contracts.SchemeFixed
}

func linuxBlockDeviceCandidates(name string) []string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}

	candidates := []string{trimmed}
	if prefix, ok := trimLinuxPSuffix(trimmed); ok {
		candidates = append(candidates, prefix)
	}
	if prefix := strings.TrimRight(trimmed, "0123456789"); prefix != "" && prefix != trimmed {
		candidates = append(candidates, prefix)
	}

	unique := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func trimLinuxPSuffix(name string) (string, bool) {
	idx := strings.LastIndex(name, "p")
	if idx <= 0 || idx >= len(name)-1 {
		return "", false
	}
	suffix := name[idx+1:]
	if strings.Trim(suffix, "0123456789") != "" {
		return "", false
	}
	prefix := name[:idx]
	if !strings.ContainsAny(prefix, "0123456789") {
		return "", false
	}
	return prefix, true
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
		mount := strings.Join(fields[8:], " ")
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
			SchemeType: guessSchemeTypeDarwin(device),
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

func guessSchemeTypeDarwin(device string) contracts.SchemeType {
	if strings.Contains(device, ":") || strings.HasPrefix(device, "//") {
		return contracts.SchemeNetwork
	}
	if strings.Contains(strings.ToLower(device), "disk") {
		return contracts.SchemeFixed
	}
	return contracts.SchemeUnknown
}
