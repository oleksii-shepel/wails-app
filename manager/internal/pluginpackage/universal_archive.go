package pluginpackage

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

const preparedMarkerFile = ".prepared"

func PrepareUniversalArchive(appName string, pkg DiscoveredPackage) (PreparedPackage, error) {
	prepared := PreparedPackage{
		Manifest:    pkg.Manifest,
		Kind:        pkg.Kind,
		ArchivePath: pkg.Path,
		Valid:       pkg.Valid,
		Errors:      pkg.Errors,
		Warnings:    pkg.Warnings,
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

	revision, err := archiveRevision(pkg.Path)
	if err != nil {
		return prepared, err
	}

	// Immutable cache layout:
	//
	//   <cache>/<app>/plugins/<id>/<version>/<sha256>/
	//
	// A running Windows executable can lock its own file. Therefore this
	// function never removes or overwrites an already-prepared revision.
	cache := filepath.Join(
		base,
		appName,
		"plugins",
		pkg.Manifest.ID,
		pkg.Manifest.Version,
		revision,
	)

	if preparedRevisionReady(cache, pkg.Manifest) {
		prepared.CachePath = cache
		return prepared, nil
	}

	// Never attempt to repair an existing revision in-place. It may contain a
	// running executable from another activation. An incomplete directory means
	// a previous extraction did not finish cleanly and should be handled
	// explicitly rather than unlinking potentially locked files.
	if _, err := os.Stat(cache); err == nil {
		return prepared, fmt.Errorf(
			"plugin cache revision %s exists but is incomplete; stop the plugin and remove that cache revision before retrying",
			revision,
		)
	} else if !os.IsNotExist(err) {
		return prepared, err
	}

	parent := filepath.Dir(cache)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return prepared, err
	}

	tmp, err := os.MkdirTemp(parent, "."+revision+".tmp-")
	if err != nil {
		return prepared, err
	}
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = os.RemoveAll(tmp)
		}
	}()

	if err := extractUniversalArchive(tmp, pkg); err != nil {
		return prepared, err
	}

	// Write the readiness marker only after every declared entry has been
	// validated in the temporary directory.
	if err := os.WriteFile(
		filepath.Join(tmp, preparedMarkerFile),
		[]byte(revision+"\n"),
		0o644,
	); err != nil {
		return prepared, err
	}

	// The revision directory is immutable. Rename promotes the fully prepared
	// temp directory without modifying an existing running revision.
	if err := os.Rename(tmp, cache); err != nil {
		// Another preparation may have won the race. Reuse it if complete.
		if preparedRevisionReady(cache, pkg.Manifest) {
			prepared.CachePath = cache
			return prepared, nil
		}
		return prepared, fmt.Errorf("promote plugin cache revision: %w", err)
	}
	keepTemp = false

	prepared.CachePath = cache
	return prepared, nil
}

func archiveRevision(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func preparedRevisionReady(cache string, manifest Manifest) bool {
	if manifest.Frontend == nil || manifest.Backend == nil {
		return false
	}

	marker := filepath.Join(cache, preparedMarkerFile)
	if !regularFile(marker) {
		return false
	}

	frontend := filepath.Join(
		cache,
		filepath.FromSlash(manifest.Frontend.Entry),
	)
	backend := filepath.Join(
		cache,
		filepath.FromSlash(manifest.Backend.Entry),
	)

	return regularFile(frontend) && regularFile(backend)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func extractUniversalArchive(cache string, pkg DiscoveredPackage) error {
	reader, err := zip.OpenReader(pkg.Path)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name, err := safeArchiveName(file.Name)
		if err != nil {
			return err
		}

		dst := filepath.Join(cache, filepath.FromSlash(name))
		if !withinRoot(cache, dst) {
			return fmt.Errorf("archive entry escapes cache: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}

		if file.UncompressedSize64 > MaxPluginArchiveEntry {
			return fmt.Errorf("archive entry %s exceeds limit", file.Name)
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}

		data, err := readZipFile(file, MaxPluginArchiveEntry)
		if err != nil {
			return err
		}

		mode := os.FileMode(0o644)
		if filepath.ToSlash(name) == filepath.ToSlash(pkg.Manifest.Backend.Entry) {
			mode = 0o755
		}

		if err := os.WriteFile(dst, data, mode); err != nil {
			return err
		}
	}

	for _, entry := range []string{
		pkg.Manifest.Frontend.Entry,
		pkg.Manifest.Backend.Entry,
	} {
		path := filepath.Join(cache, filepath.FromSlash(entry))
		if !withinRoot(cache, path) {
			return fmt.Errorf("declared entry escapes cache")
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("declared entry %s is missing", entry)
		}
	}

	return nil
}
