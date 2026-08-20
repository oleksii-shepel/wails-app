package pluginpackage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	MaxUniversalArchiveBytes = 256 << 20
	MaxUniversalEntryBytes   = 128 << 20
)

type UniversalTarget struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type UniversalEntry struct {
	Entry string `json:"entry"`
}

type UniversalBackendEntry struct {
	Entry    string `json:"entry"`
	Protocol int    `json:"protocol"`
}

type UniversalManifest struct {
	SchemaVersion    int                    `json:"schemaVersion"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description,omitempty"`
	Author           string                 `json:"author,omitempty"`
	Homepage         string                 `json:"homepage,omitempty"`
	License          string                 `json:"license,omitempty"`
	APIVersion       string                 `json:"apiVersion,omitempty"`
	Target           UniversalTarget        `json:"target"`
	Frontend         UniversalEntry         `json:"frontend"`
	Backend          UniversalBackendEntry  `json:"backend"`
	Permissions      []string               `json:"permissions,omitempty"`
	ActivationEvents []string               `json:"activationEvents,omitempty"`
	Contributes      map[string]interface{} `json:"contributes,omitempty"`
}

type UniversalPackage struct {
	Manifest    UniversalManifest `json:"manifest"`
	ArchivePath string            `json:"archivePath"`
	CachePath   string            `json:"cachePath"`
	Valid       bool              `json:"valid"`
	Errors      []string          `json:"errors,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
}

func hostTarget() UniversalTarget {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	} else if arch == "arm64" {
		arch = "arm64"
	}
	return UniversalTarget{OS: runtime.GOOS, Arch: arch}
}

func DiscoverUniversalArchives(root string) ([]UniversalPackage, error) {
	if err := EnsureDirectory(root); err != nil {
		return nil, err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		return nil, err
	}

	var out []UniversalPackage
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".zip") {
			continue
		}
		path := filepath.Join(rootAbs, entry.Name())
		out = append(out, inspectUniversalArchive(path))
	}
	return out, nil
}

func inspectUniversalArchive(path string) UniversalPackage {
	result := UniversalPackage{ArchivePath: path}
	info, err := os.Stat(path)
	if err != nil {
		result.Errors = []string{err.Error()}
		return result
	}
	if info.Size() > MaxUniversalArchiveBytes {
		result.Errors = []string{fmt.Sprintf("plugin archive exceeds %d bytes", MaxUniversalArchiveBytes)}
		return result
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		result.Errors = []string{fmt.Sprintf("invalid zip archive: %v", err)}
		return result
	}
	defer reader.Close()

	var manifestFile *zip.File
	for _, file := range reader.File {
		clean, err := safeArchiveName(file.Name)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		if clean == ManifestFileName {
			if manifestFile != nil {
				result.Errors = append(result.Errors, "duplicate plugin.json")
			} else {
				manifestFile = file
			}
		}
	}
	if manifestFile == nil {
		result.Errors = append(result.Errors, "plugin.json is missing from archive root")
		return result
	}

	data, err := readZipFile(manifestFile, MaxManifestBytes)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	if err := json.Unmarshal(data, &result.Manifest); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid plugin.json: %v", err))
		return result
	}

	result.Errors = append(result.Errors, validateUniversalManifest(result.Manifest)...)
	result.Valid = len(result.Errors) == 0
	return result
}

func validateUniversalManifest(m UniversalManifest) []string {
	var errors []string
	if m.SchemaVersion != 2 {
		errors = append(errors, fmt.Sprintf("unsupported universal manifest schema %d", m.SchemaVersion))
	}
	if !pluginIDPattern.MatchString(m.ID) {
		errors = append(errors, "invalid plugin id")
	}
	if strings.TrimSpace(m.Name) == "" {
		errors = append(errors, "plugin name is required")
	}
	if !semverPattern.MatchString(m.Version) {
		errors = append(errors, "invalid plugin version")
	}
	target := hostTarget()
	if m.Target.OS != target.OS || m.Target.Arch != target.Arch {
		errors = append(errors, fmt.Sprintf(
			"plugin target %s-%s does not match host %s-%s",
			m.Target.OS, m.Target.Arch, target.OS, target.Arch,
		))
	}
	if err := validateRelativeEntry(m.Frontend.Entry); err != nil {
		errors = append(errors, "frontend entry: "+err.Error())
	}
	if err := validateRelativeEntry(m.Backend.Entry); err != nil {
		errors = append(errors, "backend entry: "+err.Error())
	}
	if m.Backend.Protocol <= 0 {
		errors = append(errors, "backend protocol must be positive")
	}
	return errors
}

func validateRelativeEntry(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("entry is required")
	}
	clean, err := safeArchiveName(name)
	if err != nil {
		return err
	}
	if clean != filepath.ToSlash(name) {
		return fmt.Errorf("entry must be normalized")
	}
	return nil
}

func safeArchiveName(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." ||
		filepath.IsAbs(name) || strings.Contains(clean, ":") {
		return "", fmt.Errorf("unsafe archive entry %q", name)
	}
	return clean, nil
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("%s exceeds %d bytes", file.Name, limit)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", file.Name, limit)
	}
	return data, nil
}

func PrepareUniversalArchive(appName string, pkg UniversalPackage) (UniversalPackage, error) {
	if !pkg.Valid {
		return pkg, fmt.Errorf("plugin %s is not a valid universal package", pkg.Manifest.ID)
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return pkg, err
	}
	cache := filepath.Join(base, appName, "plugins", pkg.Manifest.ID, pkg.Manifest.Version)
	if err := os.RemoveAll(cache); err != nil {
		return pkg, err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return pkg, err
	}

	reader, err := zip.OpenReader(pkg.ArchivePath)
	if err != nil {
		return pkg, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name, err := safeArchiveName(file.Name)
		if err != nil {
			return pkg, err
		}
		dst := filepath.Join(cache, filepath.FromSlash(name))
		if !withinRoot(cache, dst) {
			return pkg, fmt.Errorf("archive entry escapes cache: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return pkg, err
			}
			continue
		}
		if file.UncompressedSize64 > MaxUniversalEntryBytes {
			return pkg, fmt.Errorf("archive entry %s exceeds limit", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return pkg, err
		}
		data, err := readZipFile(file, MaxUniversalEntryBytes)
		if err != nil {
			return pkg, err
		}
		mode := os.FileMode(0o644)
		if filepath.ToSlash(name) == filepath.ToSlash(pkg.Manifest.Backend.Entry) {
			mode = 0o755
		}
		if err := os.WriteFile(dst, data, mode); err != nil {
			return pkg, err
		}
	}

	// Both declared entries are mandatory after extraction.
	for _, entry := range []string{pkg.Manifest.Frontend.Entry, pkg.Manifest.Backend.Entry} {
		path := filepath.Join(cache, filepath.FromSlash(entry))
		if !withinRoot(cache, path) {
			return pkg, fmt.Errorf("declared entry escapes cache")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return pkg, fmt.Errorf("declared entry %s is missing", entry)
		}
	}

	pkg.CachePath = cache
	return pkg, nil
}
