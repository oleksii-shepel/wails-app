package zip

import (
	stdzip "archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"wails-app/vfs"
)

func TestReadDirSynthesizesNavigableImplicitDirectories(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "implicit.zip")
	writeZipWithoutDirectoryEntries(t, zipPath, map[string]string{
		"frontend/src/main.ts":       "main",
		"frontend/src/app/app.ts":    "app",
		"frontend/src/app/routes.ts": "routes",
		"README.md":                  "readme",
	})

	archive, err := openDisk(zipPath, true)
	if err != nil {
		t.Fatalf("openDisk: %v", err)
	}
	defer archive.Close()

	rootEntries, err := archive.ReadDir(context.Background(), "")
	if err != nil {
		t.Fatalf("ReadDir(root): %v", err)
	}

	frontend := findEntry(t, rootEntries, "frontend")
	if !frontend.IsDir() {
		t.Fatalf("frontend should be a synthesized directory")
	}
	if got := frontend.Path(); got != "frontend" {
		t.Fatalf("frontend Path() = %q, want %q", got, "frontend")
	}

	frontendEntries, err := archive.ReadDir(context.Background(), frontend.Path())
	if err != nil {
		t.Fatalf("ReadDir(frontend): %v", err)
	}

	src := findEntry(t, frontendEntries, "src")
	if !src.IsDir() {
		t.Fatalf("src should be a synthesized directory")
	}
	if got := src.Path(); got != "frontend/src" {
		t.Fatalf("src Path() = %q, want %q", got, "frontend/src")
	}

	srcEntries, err := archive.ReadDir(context.Background(), src.Path())
	if err != nil {
		t.Fatalf("ReadDir(frontend/src): %v", err)
	}

	app := findEntry(t, srcEntries, "app")
	if !app.IsDir() {
		t.Fatalf("app should be a synthesized directory")
	}
	if got := app.Path(); got != "frontend/src/app" {
		t.Fatalf(
			"app Path() = %q, want %q",
			got,
			"frontend/src/app",
		)
	}

	main := findEntry(t, srcEntries, "main.ts")
	if main.IsDir() {
		t.Fatalf("main.ts should be a file")
	}
	if got := main.Path(); got != "frontend/src/main.ts" {
		t.Fatalf(
			"main.ts Path() = %q, want %q",
			got,
			"frontend/src/main.ts",
		)
	}
}

func TestStatImplicitDirectoryReturnsNavigablePath(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "implicit-stat.zip")
	writeZipWithoutDirectoryEntries(t, zipPath, map[string]string{
		"frontend/src/main.ts": "main",
	})

	archive, err := openDisk(zipPath, true)
	if err != nil {
		t.Fatalf("openDisk: %v", err)
	}
	defer archive.Close()

	info, err := archive.Stat(context.Background(), "frontend/src")
	if err != nil {
		t.Fatalf("Stat(frontend/src): %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("frontend/src should be a directory")
	}
	if got := info.Path(); got != "frontend/src" {
		t.Fatalf(
			"Stat Path() = %q, want %q",
			got,
			"frontend/src",
		)
	}
}

func TestExplicitAndImplicitDirectoryEntriesExposeSamePath(t *testing.T) {
	dir := t.TempDir()

	explicitPath := filepath.Join(dir, "explicit.zip")
	writeZipEntries(t, explicitPath, []zipTestEntry{
		{name: "frontend/", body: ""},
		{name: "frontend/src/", body: ""},
		{name: "frontend/src/main.ts", body: "main"},
	})

	implicitPath := filepath.Join(dir, "implicit.zip")
	writeZipWithoutDirectoryEntries(t, implicitPath, map[string]string{
		"frontend/src/main.ts": "main",
	})

	for _, zipPath := range []string{explicitPath, implicitPath} {
		archive, err := openDisk(zipPath, true)
		if err != nil {
			t.Fatalf("openDisk(%s): %v", zipPath, err)
		}

		entries, err := archive.ReadDir(context.Background(), "")
		if err != nil {
			archive.Close()
			t.Fatalf("ReadDir(root): %v", err)
		}

		frontend := findEntry(t, entries, "frontend")
		if got := frontend.Path(); got != "frontend" {
			archive.Close()
			t.Fatalf(
				"%s frontend Path() = %q, want %q",
				filepath.Base(zipPath),
				got,
				"frontend",
			)
		}

		archive.Close()
	}
}

type zipTestEntry struct {
	name string
	body string
}

func writeZipWithoutDirectoryEntries(
	t *testing.T,
	path string,
	files map[string]string,
) {
	t.Helper()

	entries := make([]zipTestEntry, 0, len(files))
	for name, body := range files {
		entries = append(entries, zipTestEntry{name: name, body: body})
	}
	writeZipEntries(t, path, entries)
}

func writeZipEntries(
	t *testing.T,
	path string,
	entries []zipTestEntry,
) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	writer := stdzip.NewWriter(file)
	for _, entry := range entries {
		w, err := writer.Create(entry.name)
		if err != nil {
			writer.Close()
			file.Close()
			t.Fatalf("create zip entry %q: %v", entry.name, err)
		}
		if entry.body != "" {
			if _, err := w.Write([]byte(entry.body)); err != nil {
				writer.Close()
				file.Close()
				t.Fatalf("write zip entry %q: %v", entry.name, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		file.Close()
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func findEntry(
	t *testing.T,
	entries []vfs.FileInfo,
	name string,
) vfs.FileInfo {
	t.Helper()

	for _, entry := range entries {
		if entry.Name() == name {
			return entry
		}
	}

	t.Fatalf("entry %q not found", name)
	return nil
}