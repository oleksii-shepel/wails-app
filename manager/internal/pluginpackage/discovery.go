package pluginpackage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ManifestFileName = "plugin.json"
	MaxManifestBytes = 1 << 20
)

var pluginIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
var semverPattern = regexp.MustCompile(`^\d+(?:\.\d+){0,2}(?:-[0-9A-Za-z.-]+)?$`)

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
	Permissions      []string               `json:"permissions,omitempty"`
	ActivationEvents []string               `json:"activationEvents,omitempty"`
	Contributes      map[string]interface{} `json:"contributes,omitempty"`
}

type DiscoveredPackage struct {
	Manifest     Manifest `json:"manifest"`
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
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		pkgDir := filepath.Join(rootAbs, entry.Name())
		manifestPath := filepath.Join(pkgDir, ManifestFileName)
		info, err := os.Lstat(manifestPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			out = append(out, invalidPackage(pkgDir, manifestPath, err))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			out = append(out, invalidPackage(pkgDir, manifestPath, fmt.Errorf("manifest must be a regular file")))
			continue
		}
		if !withinRoot(rootAbs, manifestPath) {
			out = append(out, invalidPackage(pkgDir, manifestPath, fmt.Errorf("manifest escapes plugin directory")))
			continue
		}
		out = append(out, readPackage(pkgDir, manifestPath))
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

func readPackage(pkgDir, manifestPath string) DiscoveredPackage {
	result := DiscoveredPackage{Path: pkgDir, ManifestPath: manifestPath}
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

	result.Errors, result.Warnings = validateManifest(result.Manifest, filepath.Base(pkgDir))
	result.Valid = len(result.Errors) == 0
	return result
}

func validateManifest(m Manifest, directoryName string) ([]string, []string) {
	var errors []string
	var warnings []string
	if m.SchemaVersion != 1 {
		errors = append(errors, fmt.Sprintf("unsupported manifest schema %d", m.SchemaVersion))
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
	if m.ID != "" && directoryName != m.ID {
		warnings = append(warnings, fmt.Sprintf("directory %q does not match plugin id %q", directoryName, m.ID))
	}
	if hasDuplicates(m.Permissions) {
		warnings = append(warnings, "duplicate permissions")
	}
	if hasDuplicates(m.ActivationEvents) {
		warnings = append(warnings, "duplicate activation events")
	}
	return errors, warnings
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

func invalidPackage(pkgDir, manifestPath string, err error) DiscoveredPackage {
	return DiscoveredPackage{Path: pkgDir, ManifestPath: manifestPath, Errors: []string{err.Error()}}
}