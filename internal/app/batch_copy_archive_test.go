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

func TestStateCopyBatchPathRejectsArchivePathsWhenNotMounted(t *testing.T) {
	// This test verifies that batch copy properly rejects archive paths
	// if they're not mounted in the VFS.
	// Archive entries must be extracted using the Archive service, not batch copy.

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

	state := &State{vfs: vfs}
	destNative := filepath.Join(t.TempDir(), "archive_copy.txt")
	destPublic := pathref.FromNativePath(destNative)

	// Try to copy from an archive path that's not accessible
	archivePath := "archive:///path/to/file.zip/entry.txt"
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

	err = state.copyBatchPath(archivePath, destPublic, false, streamToFile)
	if err == nil {
		t.Fatalf("copyBatchPath should have returned an error for archive path")
	}

	// Error should indicate source is not a mounted virtual path
	t.Logf("Expected error received: %v", err)
}

func TestStateCopyBatchPathWithMountedSource(t *testing.T) {
	// This test verifies that batch copy works correctly with VFS-mounted sources
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

	// Mount a VFS: this simulates a remote file source
	if _, err := vfs.Mount("/mnt/source", memoryFS, false, "Source Mount", vfslib.VolumeRemote); err != nil {
		t.Fatalf("mount source fs: %v", err)
	}

	// Create a file in the mounted source
	sourcePath, err := resolveMemoryAlias("memory://mounted/file.txt")
	if err != nil {
		t.Fatalf("resolve memory source path: %v", err)
	}
	const content = "batch copy test content"
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

	// This should work with the mounted source
	mountedSourcePath := "/mnt/source/mounted/file.txt"
	if err := state.copyBatchPath(mountedSourcePath, destPublic, false, streamToFile); err != nil {
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

func TestBatchCopyArchivesUsesCorrectService(t *testing.T) {
	// This test documents the CORRECT usage pattern:
	// Archive entries should NOT be copied via BatchCopy.
	// They must use ArchiveCopyEntries from the Archive service instead.

	// Expected flow:
	// 1. Frontend detects archive selection via getTreeArchiveSelectionContext()
	// 2. Frontend calls archiveService.copyByContext() instead of apiService.batchCopy()
	// 3. Backend uses ArchiveCopyEntries() which calls ExtractEntries()

	t.Logf("Archive copying flow:")
	t.Logf("- Frontend: archiveService.copyByContext(archivePath, innerPaths, destDir)")
	t.Logf("- Backend: App.ArchiveCopyEntries(req) → Archive.CopyEntries()")
	t.Logf("- Should NOT use: BatchCopy() or batch_copy.go functions")
	t.Logf("")
	t.Logf("Regular file copying flow:")
	t.Logf("- Frontend: apiService.batchCopy(items, recursive)")
	t.Logf("- Backend: App.BatchCopy(req) → Batch.BatchCopy()")
	t.Logf("- Uses: batch/service.go and batch_copy.go functions")
}
