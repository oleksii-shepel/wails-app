package pluginpackage

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeArchiveForTest(t *testing.T, path string, manifest Manifest, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil { t.Fatal(err) }
	z := zip.NewWriter(f)
	data, err := json.Marshal(manifest)
	if err != nil { t.Fatal(err) }
	w, err := z.Create("plugin.json")
	if err != nil { t.Fatal(err) }
	_, _ = w.Write(data)
	for name, body := range files {
		w, err := z.Create(name)
		if err != nil { t.Fatal(err) }
		_, _ = w.Write([]byte(body))
	}
	if err := z.Close(); err != nil { t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
}

func TestDiscoverIncludesUniversalArchive(t *testing.T) {
	root := t.TempDir()
	target := hostTarget()
	manifest := Manifest{
		SchemaVersion: 2,
		ID: "reassembly.registry",
		Name: "Windows Registry",
		Version: "1.2.0",
		APIVersion: "^2.0.0",
		Target: &target,
		Frontend: &FrontendEntry{Entry: "frontend/index.js"},
		Backend: &BackendEntry{Entry: "backend/plugin.exe", Protocol: 1},
	}
	path := filepath.Join(root, "reassembly.registry-1.2.0-windows-x64.zip")
	writeArchiveForTest(t, path, manifest, map[string]string{
		"frontend/index.js": "export function activate() {}",
		"backend/plugin.exe": "test",
	})

	packages, err := Discover(root)
	if err != nil { t.Fatal(err) }
	if len(packages) != 1 { t.Fatalf("got %d packages, want 1", len(packages)) }
	pkg := packages[0]
	if !pkg.Valid { t.Fatalf("archive invalid: %v", pkg.Errors) }
	if pkg.Kind != "archive" { t.Fatalf("kind = %q", pkg.Kind) }
	if pkg.Manifest.ID != "reassembly.registry" { t.Fatalf("id = %q", pkg.Manifest.ID) }
}

func TestDiscoverDoesNotExtractArchive(t *testing.T) {
	root := t.TempDir()
	target := hostTarget()
	manifest := Manifest{
		SchemaVersion: 2, ID: "demo.plugin", Name: "Demo", Version: "1.0.0",
		Target: &target,
		Frontend: &FrontendEntry{Entry: "frontend/index.js"},
		Backend: &BackendEntry{Entry: "backend/plugin.exe", Protocol: 1},
	}
	path := filepath.Join(root, "demo.zip")
	writeArchiveForTest(t, path, manifest, map[string]string{
		"frontend/index.js": "export function activate() {}",
		"backend/plugin.exe": "x",
	})
	if _, err := Discover(root); err != nil { t.Fatal(err) }
	entries, err := os.ReadDir(root)
	if err != nil { t.Fatal(err) }
	if len(entries) != 1 || entries[0].Name() != "demo.zip" {
		t.Fatalf("discovery extracted files: %#v", entries)
	}
}
