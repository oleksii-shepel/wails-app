package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"

	"wails-app/vfs/archive"
)

func writeGzipTar(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:   name,
			Mode:   0o644,
			Size:   int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("write header %q: %v", name, err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatalf("write body %q: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
}

func TestExtractSelectedWritesSelectedEntries(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "sample.tar.gz")
	writeGzipTar(t, archivePath, map[string]string{
		"docs/a.txt":        "alpha",
		"docs/sub/b.txt":    "beta",
		"other/c.txt":       "gamma",
		"docs/keep/d.txt":   "delta",
	})

	a, err := openDisk(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}

	result, err := a.ExtractSelected(t.Context(), []string{"docs"}, outDir, archive.ExtractOptions{})
	if err != nil {
		t.Fatalf("ExtractSelected: %v", err)
	}
	if result.Extracted != 3 {
		t.Fatalf("extracted=%d, want 3 (errors: %+v)", result.Extracted, result.Errors)
	}
	for name, want := range map[string]string{
		"docs/a.txt":        "alpha",
		filepath.Join("docs", "sub", "b.txt"):  "beta",
		filepath.Join("docs", "keep", "d.txt"): "delta",
	} {
		body, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(body) != want {
			t.Fatalf("%s = %q, want %q", name, body, want)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "other", "c.txt")); !os.IsNotExist(err) {
		t.Fatal("unselected entry was extracted")
	}
}

func TestExtractSelectedHonorsConflictSkip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "sample.tar.gz")
	writeGzipTar(t, archivePath, map[string]string{"a.txt": "new-body"})

	a, err := openDisk(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}

	outDir := filepath.Join(dir, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("mkdir out: %v", err)
	}
	existing := filepath.Join(outDir, "a.txt")
	if err := os.WriteFile(existing, []byte("old-body"), 0o644); err != nil {
		t.Fatalf("seed conflict: %v", err)
	}

	result, err := a.ExtractSelected(t.Context(), []string{"a.txt"}, outDir, archive.ExtractOptions{
		OnConflict: func(srcEntry, dstPath string) (string, bool, error) {
			return "", true, nil // skip
		},
	})
	if err != nil {
		t.Fatalf("ExtractSelected: %v", err)
	}
	if result.Extracted != 0 {
		t.Fatalf("skipped entry was extracted (%d)", result.Extracted)
	}
	body, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing: %v", err)
	}
	if string(body) != "old-body" {
		t.Fatalf("skipped entry was overwritten: %q", body)
	}
}

func TestDecompressReaderHandlesXz(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	body := "xz payload"
	if err := tw.WriteHeader(&tar.Header{Name: "in.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("header: %v", err)
	}
	if _, err := io.WriteString(tw, body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	var xzBuf bytes.Buffer
	xzw, err := xz.NewWriter(&xzBuf)
	if err != nil {
		t.Fatalf("xz writer: %v", err)
	}
	if _, err := xzw.Write(raw.Bytes()); err != nil {
		t.Fatalf("xz write: %v", err)
	}
	if err := xzw.Close(); err != nil {
		t.Fatalf("xz close: %v", err)
	}

	dc, err := DecompressReader(bytes.NewReader(xzBuf.Bytes()), "sample.tar.xz")
	if err != nil {
		t.Fatalf("DecompressReader: %v", err)
	}
	defer dc.Close()
	tr := tar.NewReader(dc)
	header, err := tr.Next()
	if err != nil {
		t.Fatalf("tar read: %v", err)
	}
	if header.Name != "in.txt" {
		t.Fatalf("entry = %q, want in.txt", header.Name)
	}
	payload, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	if string(payload) != body {
		t.Fatalf("payload = %q, want %q", payload, body)
	}
}
