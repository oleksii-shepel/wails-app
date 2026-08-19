package archive

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/pathref"
	vfsarchive "wails-app/vfs/archive"
	"wails-app/vfs/archive/cache"
)

// makeTestZip creates a zip file with a single entry returning the native path.
func makeTestZip(t *testing.T, dir string, name string, entryName string, content string) string {
	t.Helper()
	zipPath := filepath.Join(dir, name)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create(entryName)
	if err != nil {
		f.Close()
		t.Fatalf("zip.Create entry: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("zip.Write: %v", err)
	}
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatalf("zip.Close: %v", err)
	}
	f.Close()
	return zipPath
}

func newNoopOperation() Operation {
	return &noopOp{}
}

type noopOp struct{}

func (n *noopOp) ID() string                                 { return "test-op" }
func (n *noopOp) Complete()                                  {}
func (n *noopOp) SetError(error)                             {}
func (n *noopOp) SetTotal(int64, int64)                      {}
func (n *noopOp) UpdateProgress(int64, int64, string, int64) {}
func (n *noopOp) IsCancelled() bool                          { return false }

func newTestService() *Service {
	// ExtractEntries/ExtractNestedArchive don't use the cache; provide a no-op opener.
	c := cache.New(func(_ context.Context, fsPath string, nestedChain []string, password string) (vfsarchive.Archive, error) {
		return nil, fmt.Errorf("cache not used in these tests")
	})
	return NewService(c, func(_ contracts.OperationType, _, _, _ string) Operation {
		return newNoopOperation()
	})
}

// TestExtractEntriesAcceptsPublicFsPath verifies that CopyEntries correctly
// converts a filesystem:// scheme path to a native OS path before accessing
// the archive on disk.
func TestExtractEntriesAcceptsPublicFsPath(t *testing.T) {
	tmp := t.TempDir()
	const entryName = "hello.txt"
	const content = "hello from archive"
	nativeZip := makeTestZip(t, tmp, "test.zip", entryName, content)

	// Build a public filesystem:// path as the frontend would send
	publicZipPath := pathref.FromNativePath(nativeZip)

	destDir := filepath.Join(tmp, "output")
	publicDestDir := pathref.FromNativePath(destDir)

	svc := newTestService()
	req := contracts.ArchiveEntriesRequest{
		FsPath:         publicZipPath,
		NestedChain:    nil,
		Entries:        []string{entryName},
		StripPrefix:    "",
		DestinationDir: publicDestDir,
	}

	succeeded, failed, errs, fatalErr := svc.ExtractEntries(req, newNoopOperation())
	if fatalErr != nil {
		t.Fatalf("ExtractEntries fatal error: %v", fatalErr)
	}
	if len(errs) > 0 {
		t.Fatalf("ExtractEntries errors: %v", errs)
	}
	if failed != 0 || succeeded != 1 {
		t.Fatalf("unexpected result: succeeded=%d failed=%d", succeeded, failed)
	}

	extractedPath := filepath.Join(destDir, entryName)
	data, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("content mismatch: want %q got %q", content, string(data))
	}
}

// TestCopyEntriesAcceptsPublicFsPath verifies the full CopyEntries path with
// public filesystem:// paths, matching what the frontend sends to ArchiveCopyEntries.
func TestCopyEntriesAcceptsPublicFsPath(t *testing.T) {
	tmp := t.TempDir()
	const entryName = "doc/readme.txt"
	const content = "copy entries test"
	nativeZip := makeTestZip(t, tmp, "archive.zip", entryName, content)

	publicZipPath := pathref.FromNativePath(nativeZip)
	destDir := filepath.Join(tmp, "dest")
	publicDestDir := pathref.FromNativePath(destDir)

	svc := newTestService()
	req := contracts.ArchiveEntriesRequest{
		FsPath:         publicZipPath,
		Entries:        []string{entryName},
		DestinationDir: publicDestDir,
	}

	result, err := svc.CopyEntries(req)
	if err != nil {
		t.Fatalf("CopyEntries error: %v", err)
	}
	if result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}

	data, err := os.ReadFile(filepath.Join(destDir, filepath.FromSlash(entryName)))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("content mismatch: want %q got %q", content, string(data))
	}
}

// TestExtractNestedArchiveAcceptsPublicFsPath verifies that ExtractNestedArchive
// converts the filesystem:// path before opening the outer archive.
func TestExtractNestedArchiveAcceptsPublicFsPath(t *testing.T) {
	tmp := t.TempDir()
	const innerEntry = "inner.txt"
	const content = "nested content"

	// Create inner zip (bytes)
	innerZipPath := makeTestZip(t, tmp, "inner.zip", innerEntry, content)
	innerZipBytes, err := os.ReadFile(innerZipPath)
	if err != nil {
		t.Fatalf("read inner zip: %v", err)
	}

	// Create outer zip containing inner.zip
	outerZipPath := filepath.Join(tmp, "outer.zip")
	f, err := os.Create(outerZipPath)
	if err != nil {
		t.Fatalf("create outer zip: %v", err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create("inner.zip")
	if err != nil {
		f.Close()
		t.Fatalf("create inner.zip in outer: %v", err)
	}
	if _, err := fw.Write(innerZipBytes); err != nil {
		f.Close()
		t.Fatalf("write inner.zip data: %v", err)
	}
	w.Close()
	f.Close()

	publicOuterPath := pathref.FromNativePath(outerZipPath)

	svc := newTestService()
	data, err := svc.ExtractNestedArchive(publicOuterPath, []string{"inner.zip"})
	if err != nil {
		t.Fatalf("ExtractNestedArchive error: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("ExtractNestedArchive returned empty data")
	}
	// Verify the returned bytes are a valid zip containing innerEntry
	r, err := zip.NewReader(readerAt(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parse returned data as zip: %v", err)
	}
	if len(r.File) == 0 || r.File[0].Name != innerEntry {
		t.Fatalf("unexpected inner zip contents: %v", r.File)
	}
}

// readerAt wraps a byte slice to implement io.ReaderAt.
type byteReaderAt []byte

func readerAt(b []byte) byteReaderAt { return byteReaderAt(b) }

func (b byteReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, nil
	}
	n := copy(p, b[off:])
	return n, nil
}