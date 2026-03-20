package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetMemoryResolverForTest(t *testing.T, root string) {
	t.Helper()

	prevOnce := memoryPathOnce
	prevPath := memoryPath
	prevErr := memoryPathErr
	prevRootFn := memoryRootFn

	configureMemoryRootProvider(func() string { return root })
	memoryPathOnce = sync.Once{}
	memoryPath = nil
	memoryPathErr = nil

	t.Cleanup(func() {
		memoryPathOnce = prevOnce
		memoryPath = prevPath
		memoryPathErr = prevErr
		memoryRootFn = prevRootFn
	})
}

func TestResolveMemoryAliasRoundTrip(t *testing.T) {
	root := t.TempDir()
	resetMemoryResolverForTest(t, root)

	resolved, err := resolveMemoryAlias("memory://folder/file.txt")
	if err != nil {
		t.Fatalf("resolveMemoryAlias returned error: %v", err)
	}

	wantResolved := filepath.ToSlash(filepath.Join(root, "folder", "file.txt"))
	if filepath.ToSlash(resolved) != wantResolved {
		t.Fatalf("resolved path mismatch\nwant: %s\n got: %s", wantResolved, filepath.ToSlash(resolved))
	}

	public := toPublicMemoryPath(resolved)
	if public != "memory://folder/file.txt" {
		t.Fatalf("public path mismatch\nwant: memory://folder/file.txt\n got: %s", public)
	}
}

func TestRawFilesystemCopyBetweenDiskAndMemory(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	source := filepath.Join(diskRoot, "source.txt")
	const content = "hello from disk"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	memoryDest, err := resolveMemoryAlias("memory://inbox/source.txt")
	if err != nil {
		t.Fatalf("resolve memory destination: %v", err)
	}
	if _, err := rawFilesystemOps.CopyFile(source, memoryDest, false); err != nil {
		t.Fatalf("copy disk -> memory: %v", err)
	}

	memoryBytes, err := os.ReadFile(memoryDest)
	if err != nil {
		t.Fatalf("read memory destination: %v", err)
	}
	if string(memoryBytes) != content {
		t.Fatalf("memory destination content mismatch\nwant: %q\n got: %q", content, string(memoryBytes))
	}

	diskDest := filepath.Join(diskRoot, "roundtrip.txt")
	if _, err := rawFilesystemOps.CopyFile(memoryDest, diskDest, false); err != nil {
		t.Fatalf("copy memory -> disk: %v", err)
	}

	roundtripBytes, err := os.ReadFile(diskDest)
	if err != nil {
		t.Fatalf("read roundtrip destination: %v", err)
	}
	if string(roundtripBytes) != content {
		t.Fatalf("roundtrip content mismatch\nwant: %q\n got: %q", content, string(roundtripBytes))
	}
}
