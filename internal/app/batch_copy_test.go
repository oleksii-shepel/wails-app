package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	internalfilesystem "wails-app/internal/filesystem"
	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
	vfslib "wails-app/vfs"
)

func TestStateCopyBatchPathFromMountedSMBSource(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	memoryFS, err := getMemoryFS()
	if err != nil {
		t.Fatalf("getMemoryFS returned error: %v", err)
	}

	vfs, err := internalruntime.InitVFS(memoryFS)
	if err != nil {
		t.Fatalf("InitVFS returned error: %v", err)
	}
	if _, err := vfs.Mount("/remote/smb/example.com:445", memoryFS, false, "SMB Example", vfslib.VolumeRemote); err != nil {
		t.Fatalf("mount smb test fs: %v", err)
	}

	sourcePath, err := resolveMemoryAlias("memory://pub/readme.txt")
	if err != nil {
		t.Fatalf("resolve memory source path: %v", err)
	}
	const content = "copied from mounted smb"
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	state := &State{vfs: vfs}
	destNative := filepath.Join(t.TempDir(), "downloaded.txt")
	destPublic := pathref.FromNativePath(destNative)
	streamToFile := func(reader io.Reader, destination string, _ int64, _ string) (int64, error) {
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return 0, err
		}
		file, err := os.Create(destination)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		return io.Copy(file, reader)
	}

	if err := state.copyBatchPath("smb://example.com/pub/readme.txt", destPublic, false, streamToFile); err != nil {
		t.Fatalf("copyBatchPath returned error: %v", err)
	}

	data, err := os.ReadFile(destNative)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != content {
		t.Fatalf("destination content mismatch\nwant: %q\n got: %q", content, string(data))
	}
}

func TestStateCopyBatchPathFromHTTPSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/MPEG4%20by%20philips.mp4" && r.URL.Path != "/MPEG4 by philips.mp4" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("http tab sample"))
	}))
	defer server.Close()

	filesystem := internalfilesystem.NewService(internalfilesystem.ServiceConfig{
		Raw:      rawFilesystemOps,
		Metadata: rawFilesystemMetadataCallbacks(),
	})

	state := &State{Filesystem: filesystem}
	destNative := filepath.Join(t.TempDir(), "downloaded.mp4")
	destPublic := pathref.FromNativePath(destNative)
	streamToFile := func(reader io.Reader, destination string, _ int64, _ string) (int64, error) {
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return 0, err
		}
		file, err := os.Create(destination)
		if err != nil {
			return 0, err
		}
		defer file.Close()
		return io.Copy(file, reader)
	}

	source := server.URL + "/MPEG4%20by%20philips.mp4"
	if err := state.copyBatchPath(source, destPublic, false, streamToFile); err != nil {
		t.Fatalf("copyBatchPath returned error: %v", err)
	}

	data, err := os.ReadFile(destNative)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "http tab sample" {
		t.Fatalf("destination content mismatch\nwant: %q\n got: %q", "http tab sample", string(data))
	}
}
