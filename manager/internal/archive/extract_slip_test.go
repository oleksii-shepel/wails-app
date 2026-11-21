package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeExtractEntryName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "plain file", path: "docs/readme.md", want: true},
		{name: "leading slash stripped", path: "/etc/hosts", want: true},
		{name: "windows slashes", path: `docs\readme.md`, want: true},
		{name: "parent segment", path: "../evil.txt", want: false},
		{name: "parent inside path", path: "docs/../../evil.txt", want: false},
		{name: "bare parent", path: "..", want: false},
		{name: "windows drive", path: "C:/evil.txt", want: false},
		{name: "unc style", path: "//server/share/file", want: false},
		{name: "dot segment ok", path: "docs/./readme.md", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeExtractEntryName(tc.path); got != tc.want {
				t.Fatalf("SafeExtractEntryName(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestExtractZipSkipsEntriesEscapingDestination(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "slip.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	w := zip.NewWriter(f)
	for _, name := range []string{"ok.txt", "../evil.txt"} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := fw.Write([]byte("data")); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	destDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	succeeded, failed, errs, fatalErr := (&Service{}).extractZipSync(archivePath, []string{""}, "", destDir, false, rewriteStubOp{})
	if fatalErr != nil {
		t.Fatalf("extractZipSync: %v", fatalErr)
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("succeeded=%d failed=%d, want 1 and 1 (errs: %+v)", succeeded, failed, errs)
	}
	if len(errs) != 1 {
		t.Fatalf("expected one per-entry error, got %+v", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err == nil {
		t.Fatal("entry escaped the destination directory")
	}
	if _, err := os.Stat(filepath.Join(destDir, "ok.txt")); err != nil {
		t.Fatalf("legitimate entry missing: %v", err)
	}
}
