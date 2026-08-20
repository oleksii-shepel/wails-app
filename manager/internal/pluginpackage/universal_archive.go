package pluginpackage

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

type PreparedPackage struct {
	Manifest    Manifest `json:"manifest"`
	Kind        string   `json:"kind"`
	ArchivePath string   `json:"archivePath"`
	CachePath   string   `json:"cachePath"`
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

func PrepareUniversalArchive(appName string, pkg DiscoveredPackage) (PreparedPackage, error) {
	prepared := PreparedPackage{
		Manifest: pkg.Manifest,
		Kind: pkg.Kind,
		ArchivePath: pkg.Path,
		Valid: pkg.Valid,
		Errors: pkg.Errors,
		Warnings: pkg.Warnings,
	}
	if pkg.Kind != "archive" {
		return prepared, fmt.Errorf("plugin %s is not an archive package", pkg.Manifest.ID)
	}
	if !pkg.Valid {
		return prepared, fmt.Errorf("plugin %s is not a valid universal package", pkg.Manifest.ID)
	}
	if pkg.Manifest.Frontend == nil || pkg.Manifest.Backend == nil {
		return prepared, fmt.Errorf("plugin %s has incomplete universal entries", pkg.Manifest.ID)
	}

	base, err := os.UserCacheDir()
	if err != nil {
		return prepared, err
	}
	cache := filepath.Join(base, appName, "plugins", pkg.Manifest.ID, pkg.Manifest.Version)
	if err := os.RemoveAll(cache); err != nil {
		return prepared, err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return prepared, err
	}

	reader, err := zip.OpenReader(pkg.Path)
	if err != nil {
		return prepared, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name, err := safeArchiveName(file.Name)
		if err != nil {
			return prepared, err
		}
		dst := filepath.Join(cache, filepath.FromSlash(name))
		if !withinRoot(cache, dst) {
			return prepared, fmt.Errorf("archive entry escapes cache: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return prepared, err
			}
			continue
		}
		if file.UncompressedSize64 > MaxPluginArchiveEntry {
			return prepared, fmt.Errorf("archive entry %s exceeds limit", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return prepared, err
		}
		data, err := readZipFile(file, MaxPluginArchiveEntry)
		if err != nil {
			return prepared, err
		}
		mode := os.FileMode(0o644)
		if filepath.ToSlash(name) == filepath.ToSlash(pkg.Manifest.Backend.Entry) {
			mode = 0o755
		}
		if err := os.WriteFile(dst, data, mode); err != nil {
			return prepared, err
		}
	}

	for _, entry := range []string{
		pkg.Manifest.Frontend.Entry,
		pkg.Manifest.Backend.Entry,
	} {
		path := filepath.Join(cache, filepath.FromSlash(entry))
		if !withinRoot(cache, path) {
			return prepared, fmt.Errorf("declared entry escapes cache")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return prepared, fmt.Errorf("declared entry %s is missing", entry)
		}
	}

	prepared.CachePath = cache
	return prepared, nil
}
