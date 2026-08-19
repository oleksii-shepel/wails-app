package pluginpackage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRejectsDuplicatePluginIDs(t *testing.T) {
	root := t.TempDir()
	manifest := []byte(`{"schemaVersion":1,"id":"duplicate.plugin","name":"Duplicate","version":"1.0.0"}`)
	for _, dir := range []string{"one", "two"} {
		pkgDir := filepath.Join(root, dir)
		if err := os.Mkdir(pkgDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, ManifestFileName), manifest, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	packages, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("got %d packages", len(packages))
	}
	for _, pkg := range packages {
		if pkg.Valid {
			t.Fatalf("duplicate package unexpectedly valid: %#v", pkg)
		}
	}
}