package pluginpackage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	ManifestFileName        = "plugin.json"
	MaxManifestBytes        = 1 << 20
	MaxPluginArchiveBytes   = 256 << 20
	MaxPluginArchiveEntry   = 128 << 20
)

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
var semverPattern = regexp.MustCompile(`^\d+(?:\.\d+){0,2}(?:-[0-9A-Za-z.-]+)?$`)

type Target struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type FrontendEntry struct {
	Entry string `json:"entry"`
}

type BackendEntry struct {
	Entry    string `json:"entry"`
	Protocol int    `json:"protocol"`
}

type Manifest struct {
	SchemaVersion    int                    `json:"schemaVersion"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description,omitempty"`
	Author           string                 `json:"author,omitempty"`
	Homepage         string                 `json:"homepage,omitempty"`
	License          string                 `json:"license,omitempty"`
	APIVersion       string                 `json:"apiVersion,omitempty"`
	Target           *Target                `json:"target,omitempty"`
	Frontend         *FrontendEntry         `json:"frontend,omitempty"`
	Backend          *BackendEntry          `json:"backend,omitempty"`
	Permissions      []string               `json:"permissions,omitempty"`
	ActivationEvents []string               `json:"activationEvents,omitempty"`
	Contributes      map[string]interface{} `json:"contributes,omitempty"`
}

type DiscoveredPackage struct {
	Manifest     Manifest `json:"manifest"`
	Kind         string   `json:"kind"`
	Path         string   `json:"path"`
	ManifestPath string   `json:"manifestPath"`
	Valid        bool     `json:"valid"`
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

func DefaultDirectory(appName string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		base, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, appName, "plugins"), nil
}

func EnsureDirectory(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("plugin directory is empty")
	}
	return os.MkdirAll(root, 0o755)
}

// Discover is metadata-only. Directory packages are read directly; ZIP archives
// are inspected in-place but are not extracted and no plugin code is executed.
func Discover(root string) ([]DiscoveredPackage, error) {
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

	out := make([]DiscoveredPackage, 0)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		if entry.IsDir() {
			pkgDir := filepath.Join(rootAbs, entry.Name())
			manifestPath := filepath.Join(pkgDir, ManifestFileName)
			info, err := os.Lstat(manifestPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				out = append(out, invalidPackage("directory", pkgDir, manifestPath, err))
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				out = append(out, invalidPackage("directory", pkgDir, manifestPath, fmt.Errorf("manifest must be a regular file")))
				continue
			}
			if !withinRoot(rootAbs, manifestPath) {
				out = append(out, invalidPackage("directory", pkgDir, manifestPath, fmt.Errorf("manifest escapes plugin directory")))
				continue
			}
			out = append(out, readDirectoryPackage(pkgDir, manifestPath))
			continue
		}

		if strings.EqualFold(filepath.Ext(entry.Name()), ".zip") {
			archivePath := filepath.Join(rootAbs, entry.Name())
			out = append(out, readArchivePackage(archivePath))
		}
	}

	markDuplicatePluginIDs(out)
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Manifest.ID
		if left == "" {
			left = filepath.Base(out[i].Path)
		}
		right := out[j].Manifest.ID
		if right == "" {
			right = filepath.Base(out[j].Path)
		}
		return left < right
	})
	return out, nil
}

func readDirectoryPackage(pkgDir, manifestPath string) DiscoveredPackage {
	result := DiscoveredPackage{Kind: "directory", Path: pkgDir, ManifestPath: manifestPath}
	file, err := os.Open(manifestPath)
	if err != nil {
		result.Errors = []string{err.Error()}
		return result
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxManifestBytes+1))
	if err != nil {
		result.Errors = []string{err.Error()}
		return result
	}
	if len(data) > MaxManifestBytes {
		result.Errors = []string{fmt.Sprintf("manifest exceeds %d bytes", MaxManifestBytes)}
		return result
	}
	if err := json.Unmarshal(data, &result.Manifest); err != nil {
		result.Errors = []string{fmt.Sprintf("invalid JSON: %v", err)}
		return result
	}

	result.Errors, result.Warnings = validateManifest(result.Manifest, filepath.Base(pkgDir), "directory")
	result.Valid = len(result.Errors) == 0
	return result
}

func readArchivePackage(archivePath string) DiscoveredPackage {
	result := DiscoveredPackage{
		Kind: "archive",
		Path: archivePath,
		ManifestPath: archivePath + "!/" + ManifestFileName,
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		result.Errors = []string{err.Error()}
		return result
	}
	if info.Size() > MaxPluginArchiveBytes {
		result.Errors = []string{fmt.Sprintf("plugin archive exceeds %d bytes", MaxPluginArchiveBytes)}
		return result
	}

	reader, err := zip.OpenReader(archivePath)
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

	errors, warnings := validateManifest(result.Manifest, filepath.Base(archivePath), "archive")
	result.Errors = append(result.Errors, errors...)
	result.Warnings = append(result.Warnings, warnings...)
	result.Valid = len(result.Errors) == 0
	return result
}

func validateManifest(m Manifest, packageName, kind string) ([]string, []string) {
	var errors []string
	var warnings []string

	switch kind {
	case "directory":
		if m.SchemaVersion != 1 {
			errors = append(errors, fmt.Sprintf("directory packages require manifest schema 1, got %d", m.SchemaVersion))
		}
	case "archive":
		if m.SchemaVersion != 2 {
			errors = append(errors, fmt.Sprintf("archive packages require manifest schema 2, got %d", m.SchemaVersion))
		}
	default:
		errors = append(errors, fmt.Sprintf("unknown plugin package kind %q", kind))
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

	if kind == "directory" && m.ID != "" && packageName != m.ID {
		warnings = append(warnings, fmt.Sprintf("directory %q does not match plugin id %q", packageName, m.ID))
	}

	if kind == "archive" {
		errors = append(errors, validateArchiveManifest(m)...)
	}

	if hasDuplicates(m.Permissions) {
		warnings = append(warnings, "duplicate permissions")
	}
	if hasDuplicates(m.ActivationEvents) {
		warnings = append(warnings, "duplicate activation events")
	}
	return errors, warnings
}

func validateArchiveManifest(m Manifest) []string {
	var errors []string
	if m.Target == nil {
		errors = append(errors, "target is required")
	} else {
		host := hostTarget()
		if m.Target.OS != host.OS || m.Target.Arch != host.Arch {
			errors = append(errors, fmt.Sprintf(
				"plugin target %s-%s does not match host %s-%s",
				m.Target.OS, m.Target.Arch, host.OS, host.Arch,
			))
		}
	}
	if m.Frontend == nil {
		errors = append(errors, "frontend entry is required")
	} else if err := validateRelativeEntry(m.Frontend.Entry); err != nil {
		errors = append(errors, "frontend entry: "+err.Error())
	}
	if m.Backend == nil {
		errors = append(errors, "backend entry is required")
	} else {
		if err := validateRelativeEntry(m.Backend.Entry); err != nil {
			errors = append(errors, "backend entry: "+err.Error())
		}
		if m.Backend.Protocol <= 0 {
			errors = append(errors, "backend protocol must be positive")
		}
	}
	return errors
}

func hostTarget() Target {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	}
	return Target{OS: runtime.GOOS, Arch: arch}
}

func validateRelativeEntry(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("entry is required")
	}
	clean, err := safeArchiveName(name)
	if err != nil {
		return err
	}
	if clean != filepath.ToSlash(strings.ReplaceAll(name, "\\", "/")) {
		return fmt.Errorf("entry must be normalized")
	}
	return nil
}

func safeArchiveName(name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	clean := filepath.ToSlash(filepath.Clean(normalized))
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, "../") ||
		filepath.IsAbs(normalized) || strings.Contains(clean, ":") {
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

func markDuplicatePluginIDs(packages []DiscoveredPackage) {
	counts := make(map[string]int)
	for _, pkg := range packages {
		if pkg.Manifest.ID != "" {
			counts[pkg.Manifest.ID]++
		}
	}
	for i := range packages {
		id := packages[i].Manifest.ID
		if id == "" || counts[id] < 2 {
			continue
		}
		packages[i].Errors = append(packages[i].Errors, fmt.Sprintf("duplicate plugin id %q", id))
		packages[i].Valid = false
	}
}

func hasDuplicates(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func invalidPackage(kind, pkgPath, manifestPath string, err error) DiscoveredPackage {
	return DiscoveredPackage{
		Kind: kind, Path: pkgPath, ManifestPath: manifestPath, Errors: []string{err.Error()},
	}
}
