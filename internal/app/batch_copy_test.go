package app

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"wails-app/internal/pathref"
	internalruntime "wails-app/internal/runtime"
	vfslib "wails-app/vfs"
)

func TestStateCopyBatchPathFromMountedFTPSource(t *testing.T) {
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
	if _, err := vfs.Mount("/remote/ftp/example.com:21", memoryFS, false, "FTP Example", vfslib.VolumeRemote); err != nil {
		t.Fatalf("mount ftp test fs: %v", err)
	}

	sourcePath, err := resolveMemoryAlias("memory://pub/readme.txt")
	if err != nil {
		t.Fatalf("resolve memory source path: %v", err)
	}
	const content = "copied from mounted ftp"
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

	if err := state.copyBatchPath("ftp://example.com/pub/readme.txt", destPublic, false, streamToFile); err != nil {
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
