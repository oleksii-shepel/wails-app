package batch

import (
	"archive/tar"
	"compress/gzip"
	"bytes"
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholt/archiver/v3"

	contracts "wails-app/internal/contracts"
)

func buildTarGzBytes(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %q: %v", name, err)
		}
		if _, err := io.WriteString(tw, body); err != nil {
			t.Fatalf("tar body %q: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func buildZipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip entry %q: %v", name, err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatalf("zip write %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractNestedFromBytesHandlesCompressedTar(t *testing.T) {
	inner := buildTarGzBytes(t, map[string]string{
		"content/report.txt": "quarterly numbers",
		"content/data.csv":   "a,b\n1,2",
	})
	outer := buildZipBytes(t, map[string][]byte{"inner.tar.gz": inner})

	destRoot := t.TempDir()
	ec := &extractCtx{
		destRoot: destRoot,
		callbacks: ExtractCallbacks{
			Normalize:        func(name string) string { return name },
			IsArchive:        func(name string) bool { return name == "inner.tar.gz" },
			ArchiveEntryPath: func(f archiver.File) string { return f.Name() },
			IsCancelled:      func() bool { return false },
			CheckConflict: func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error) {
				return dst, false, nil
			},
			StreamToFile: func(r io.Reader, dst string, size int64, label string) (int64, error) {
				out, err := os.Create(dst)
				if err != nil {
					return 0, err
				}
				defer out.Close()
				return io.Copy(out, r)
			},
		},
		req: &contracts.BatchExtractRequest{ExpandNestedArchives: true, MaxNestDepth: 3},
	}

	files, bytesWritten, err := ec.extractNestedFromBytes(outer, "outer.zip")
	if err != nil {
		t.Fatalf("extractNestedFromBytes: %v", err)
	}
	if files != 2 || bytesWritten != int64(len("quarterly numbers")+len("a,b\n1,2")) {
		t.Fatalf("files=%d bytes=%d", files, bytesWritten)
	}
	for _, rel := range []string{"inner/content/report.txt", "inner/content/data.csv"} {
		body, err := os.ReadFile(filepath.Join(destRoot, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if len(body) == 0 {
			t.Fatalf("%s is empty", rel)
		}
	}
}

func TestDebugNestedListing(t *testing.T) {
	inner := buildTarGzBytes(t, map[string]string{"content/report.txt": "q"})
	outer := buildZipBytes(t, map[string][]byte{"inner.tar.gz": inner})
	destRoot := t.TempDir()
	ec := &extractCtx{
		destRoot: destRoot,
		callbacks: ExtractCallbacks{
			Normalize:        func(name string) string { return name },
			IsArchive:        func(name string) bool { return name == "inner.tar.gz" },
			ArchiveEntryPath: func(f archiver.File) string { return f.Name() },
			IsCancelled:      func() bool { return false },
			CheckConflict: func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error) {
				return dst, false, nil
			},
			StreamToFile: func(r io.Reader, dst string, size int64, label string) (int64, error) {
				out, err := os.Create(dst)
				if err != nil {
					return 0, err
				}
				defer out.Close()
				return io.Copy(out, r)
			},
		},
		req: &contracts.BatchExtractRequest{ExpandNestedArchives: true, MaxNestDepth: 3},
	}
	files, bytesWritten, err := ec.extractNestedFromBytes(outer, "outer.zip")
	t.Logf("files=%d bytes=%d err=%v", files, bytesWritten, err)
	filepath.Walk(destRoot, func(p string, info os.FileInfo, err error) error {
		if err == nil {
			t.Logf("walk: %s (dir=%v)", p, info.IsDir())
		}
		return nil
	})
}
