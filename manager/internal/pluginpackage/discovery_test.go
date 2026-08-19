package pluginpackage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverValidAndInvalidPackages(t *testing.T) {
	root := t.TempDir()
	validDir := filepath.Join(root, "epikode.hash-tools")
	invalidDir := filepath.Join(root, "broken")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, ManifestFileName), []byte(`{"schemaVersion":1,"id":"epikode.hash-tools","name":"Hash Tools","version":"1.0.0","apiVersion":"^1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, ManifestFileName), []byte(`{"schemaVersion":1,"id":"BAD","name":"","version":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	packages, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(packages))
	}
	var valid, invalid *DiscoveredPackage
	for i := range packages {
		if packages[i].Manifest.ID == "epikode.hash-tools" {
			valid = &packages[i]
		} else {
			invalid = &packages[i]
		}
	}
	if valid == nil || !valid.Valid {
		t.Fatalf("valid package not discovered correctly: %#v", valid)
	}
	if invalid == nil || invalid.Valid || len(invalid.Errors) == 0 {
		t.Fatalf("invalid package not reported: %#v", invalid)
	}
}

func TestDiscoverIgnoresSymlinkPackageDirectories(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, ManifestFileName), []byte(`{"schemaVersion":1,"id":"outside","name":"Outside","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	packages, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 0 {
		t.Fatalf("symlink package should be ignored: %#v", packages)
	}
}

func TestDiscoverRejectsOversizedManifest(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "large")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, MaxManifestBytes+1)
	for i := range data {
		data[i] = ' '
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	packages, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Valid || len(packages[0].Errors) == 0 {
		t.Fatalf("oversized manifest was not rejected: %#v", packages)
	}
}