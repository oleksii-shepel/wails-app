package pluginpackage

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeUniversalTestArchive(t *testing.T, path string, manifest UniversalManifest, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil { t.Fatal(err) }
	z := zip.NewWriter(f)
	data, _ := json.Marshal(manifest)
	w, _ := z.Create("plugin.json")
	_, _ = w.Write(data)
	for name, body := range files {
		w, err := z.Create(name)
		if err != nil { t.Fatal(err) }
		_, _ = w.Write([]byte(body))
	}
	if err := z.Close(); err != nil { t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }
}

func TestUniversalArchiveRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, _ := os.Create(path)
	z := zip.NewWriter(f)
	w, _ := z.Create("../escape")
	_, _ = w.Write([]byte("x"))
	_ = z.Close()
	_ = f.Close()
	pkg := inspectUniversalArchive(path)
	if pkg.Valid || len(pkg.Errors) == 0 {
		t.Fatal("expected traversal archive to be invalid")
	}
}

func TestUniversalArchiveRequiresDeclaredEntries(t *testing.T) {
	target := hostTarget()
	path := filepath.Join(t.TempDir(), "demo.zip")
	manifest := UniversalManifest{
		SchemaVersion: 2, ID: "demo.plugin", Name: "Demo", Version: "1.0.0",
		Target: target,
		Frontend: UniversalEntry{Entry: "frontend/index.js"},
		Backend: UniversalBackendEntry{Entry: "backend/plugin.exe", Protocol: 1},
	}
	writeUniversalTestArchive(t, path, manifest, map[string]string{
		"frontend/index.js": "export function activate(){}",
	})
	pkg := inspectUniversalArchive(path)
	if !pkg.Valid {
		t.Fatalf("manifest inspection should be valid before extraction: %v", pkg.Errors)
	}
	if _, err := PrepareUniversalArchive("test-app", pkg); err == nil {
		t.Fatal("expected missing backend entry to fail preparation")
	}
}
