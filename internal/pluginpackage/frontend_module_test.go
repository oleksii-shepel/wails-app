package pluginpackage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFrontendModule(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "demo.plugin")
	if err := os.Mkdir(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(pkgDir, FrontendModuleFileName)
	data := `{"schemaVersion":1,"commands":[{"id":"refresh","targetCommandId":"pane.nav.refresh"}],"toolbar":[{"id":"refresh","commandId":"refresh","when":"always"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := DiscoveredPackage{Manifest: Manifest{ID: "demo.plugin"}, Path: pkgDir, Valid: true}
	module, err := ReadFrontendModule(root, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Commands) != 1 || module.Commands[0].TargetCommandID != "pane.nav.refresh" {
		t.Fatalf("unexpected module: %#v", module)
	}
}

func TestReadFrontendModuleRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "demo.plugin")
	if err := os.Mkdir(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, FrontendModuleFileName), []byte(`{"schemaVersion":1,"execute":"evil"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := DiscoveredPackage{Manifest: Manifest{ID: "demo.plugin"}, Path: pkgDir, Valid: true}
	if _, err := ReadFrontendModule(root, pkg); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func TestReadFrontendModuleRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "demo.plugin")
	if err := os.Mkdir(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside.json")
	if err := os.WriteFile(target, []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(pkgDir, FrontendModuleFileName)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	pkg := DiscoveredPackage{Manifest: Manifest{ID: "demo.plugin"}, Path: pkgDir, Valid: true}
	if _, err := ReadFrontendModule(root, pkg); err == nil {
		t.Fatal("expected symlink rejection")
	}
}
