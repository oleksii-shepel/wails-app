//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
	contracts "wails-app/internal/contracts"
)

const autorunDisabledSuffix = ".DisabledByReassembly"

type autorunRegistryLocation struct {
	scope, origin, path string
	root                registry.Key
}

var autorunRegistryLocations = []autorunRegistryLocation{
	{"user", "registry-run", `Software\Microsoft\Windows\CurrentVersion\Run`, registry.CURRENT_USER},
	{"user", "registry-run-once", `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.CURRENT_USER},
	{"machine", "registry-run", `Software\Microsoft\Windows\CurrentVersion\Run`, registry.LOCAL_MACHINE},
	{"machine", "registry-run-once", `Software\Microsoft\Windows\CurrentVersion\RunOnce`, registry.LOCAL_MACHINE},
}

func (a *App) AutorunList() ([]contracts.AutorunEntry, error) {
	entries := make([]contracts.AutorunEntry, 0, 32)
	for _, loc := range autorunRegistryLocations {
		enabled, err := listAutorunRegistry(loc, loc.path, true)
		if err == nil {
			entries = append(entries, enabled...)
		}
		disabled, err := listAutorunRegistry(loc, loc.path+autorunDisabledSuffix, false)
		if err == nil {
			entries = append(entries, disabled...)
		}
	}
	entries = append(entries, listStartupFolder("user", userStartupFolder())...)
	entries = append(entries, listStartupFolder("machine", machineStartupFolder())...)
	return entries, nil
}

func (a *App) AutorunSetEnabled(id string, enabled bool) error {
	parts := strings.SplitN(id, "|", 4)
	if len(parts) != 4 {
		return fmt.Errorf("invalid autorun id")
	}
	kind, scope, origin, name := parts[0], parts[1], parts[2], parts[3]
	if kind == "registry" {
		return setRegistryAutorunEnabled(scope, origin, name, enabled)
	}
	if kind == "startup" {
		return setStartupAutorunEnabled(scope, name, enabled)
	}
	return fmt.Errorf("unsupported autorun kind %q", kind)
}

func (a *App) AutorunDelete(id string) error {
	parts := strings.SplitN(id, "|", 4)
	if len(parts) != 4 {
		return fmt.Errorf("invalid autorun id")
	}
	kind, scope, origin, name := parts[0], parts[1], parts[2], parts[3]
	if kind == "registry" {
		loc, ok := findAutorunRegistry(scope, origin)
		if !ok {
			return fmt.Errorf("unknown autorun registry location")
		}
		for _, path := range []string{loc.path, loc.path + autorunDisabledSuffix} {
			if key, err := registry.OpenKey(loc.root, path, registry.SET_VALUE); err == nil {
				_ = key.DeleteValue(name)
				key.Close()
			}
		}
		return nil
	}
	if kind == "startup" {
		folder := startupFolder(scope)
		if folder == "" {
			return fmt.Errorf("startup folder unavailable")
		}
		for _, candidate := range []string{name, name + autorunDisabledSuffix} {
			if err := os.Remove(filepath.Join(folder, candidate)); err == nil || os.IsNotExist(err) {
				continue
			} else {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unsupported autorun kind %q", kind)
}

func listAutorunRegistry(loc autorunRegistryLocation, path string, enabled bool) ([]contracts.AutorunEntry, error) {
	key, err := registry.OpenKey(loc.root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()
	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}
	result := make([]contracts.AutorunEntry, 0, len(names))
	for _, name := range names {
		command, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		result = append(result, contracts.AutorunEntry{ID: "registry|" + loc.scope + "|" + loc.origin + "|" + name, Name: name, Command: command, Executable: commandExecutable(command), SourcePath: registryPublicPath(loc, path, name), Scope: loc.scope, Origin: loc.origin, Enabled: enabled})
	}
	return result, nil
}

func setRegistryAutorunEnabled(scope, origin, name string, enabled bool) error {
	loc, ok := findAutorunRegistry(scope, origin)
	if !ok {
		return fmt.Errorf("unknown autorun registry location")
	}
	srcPath, dstPath := loc.path, loc.path+autorunDisabledSuffix
	if enabled {
		srcPath, dstPath = dstPath, loc.path
	}
	src, err := registry.OpenKey(loc.root, srcPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer src.Close()
	value, valueType, err := src.GetStringValue(name)
	if err != nil {
		return err
	}
	dst, _, err := registry.CreateKey(loc.root, dstPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer dst.Close()
	if valueType == registry.EXPAND_SZ {
		err = dst.SetExpandStringValue(name, value)
	} else {
		err = dst.SetStringValue(name, value)
	}
	if err != nil {
		return err
	}
	return src.DeleteValue(name)
}

func listStartupFolder(scope, folder string) []contracts.AutorunEntry {
	if folder == "" {
		return nil
	}
	items, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	result := make([]contracts.AutorunEntry, 0, len(items))
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		name := item.Name()
		enabled := true
		display := name
		if strings.HasSuffix(name, autorunDisabledSuffix) {
			enabled = false
			display = strings.TrimSuffix(name, autorunDisabledSuffix)
		}
		path := filepath.Join(folder, name)
		result = append(result, contracts.AutorunEntry{ID: "startup|" + scope + "|startup-folder|" + display, Name: display, Command: path, Executable: path, SourcePath: filepath.ToSlash(path), Scope: scope, Origin: "startup-folder", Enabled: enabled})
	}
	return result
}

func setStartupAutorunEnabled(scope, name string, enabled bool) error {
	folder := startupFolder(scope)
	if folder == "" {
		return fmt.Errorf("startup folder unavailable")
	}
	from, to := filepath.Join(folder, name), filepath.Join(folder, name+autorunDisabledSuffix)
	if enabled {
		from, to = to, filepath.Join(folder, name)
	}
	return os.Rename(from, to)
}
func startupFolder(scope string) string {
	if scope == "machine" {
		return machineStartupFolder()
	}
	return userStartupFolder()
}
func userStartupFolder() string {
	if p := os.Getenv("APPDATA"); p != "" {
		return filepath.Join(p, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	}
	return ""
}
func machineStartupFolder() string {
	if p := os.Getenv("ProgramData"); p != "" {
		return filepath.Join(p, "Microsoft", "Windows", "Start Menu", "Programs", "StartUp")
	}
	return ""
}
func findAutorunRegistry(scope, origin string) (autorunRegistryLocation, bool) {
	for _, l := range autorunRegistryLocations {
		if l.scope == scope && l.origin == origin {
			return l, true
		}
	}
	return autorunRegistryLocation{}, false
}
func registryPublicPath(loc autorunRegistryLocation, path, name string) string {
	hive := "HKCU"
	if loc.root == registry.LOCAL_MACHINE {
		hive = "HKLM"
	}
	return "registry://" + hive + "/" + strings.ReplaceAll(path, "\\", "/") + "/" + name
}
func commandExecutable(command string) string {
	s := strings.TrimSpace(command)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "\"") {
		if i := strings.Index(s[1:], "\""); i >= 0 {
			return s[1 : i+1]
		}
	}
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
