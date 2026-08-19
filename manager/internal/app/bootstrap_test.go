package app

import (
	"os"
	"path/filepath"
	"testing"

	internalbatch "wails-app/internal/batch"
	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
	"wails-app/internal/pathref"
)

func resetMemoryResolverForTest(t *testing.T, root string) {
	t.Helper()

	oldProviders := defaultProviders
	defaultProviders = newBootstrapProviders()
	defaultProviders.configureMemoryRoot(func() string { return root })

	t.Cleanup(func() {
		defaultProviders = oldProviders
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

func TestJoinPathsPreservesAliasRoot(t *testing.T) {
	app := &App{}

	joined := app.JoinPaths([]string{"memory://", "file.txt"})
	if joined != "memory://file.txt" {
		t.Fatalf("memory alias join mismatch\nwant: memory://file.txt\n got: %s", joined)
	}

	joined = app.JoinPaths([]string{"memory://folder", "child.txt"})
	if joined != "memory://folder/child.txt" {
		t.Fatalf("memory alias nested join mismatch\nwant: memory://folder/child.txt\n got: %s", joined)
	}
}

func TestSplitPathDecodesWebAliasBasename(t *testing.T) {
	app := &App{}

	split := app.SplitPath("https://samples.ffmpeg.org/MPEG4%20by%20philips.mp4")
	if split["base"] != "MPEG4 by philips.mp4" {
		t.Fatalf("web alias base mismatch\nwant: %q\n got: %q", "MPEG4 by philips.mp4", split["base"])
	}
	if split["name"] != "MPEG4 by philips" {
		t.Fatalf("web alias name mismatch\nwant: %q\n got: %q", "MPEG4 by philips", split["name"])
	}
	if split["ext"] != ".mp4" {
		t.Fatalf("web alias ext mismatch\nwant: %q\n got: %q", ".mp4", split["ext"])
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

func TestBatchCopyBetweenDiskAndMemory(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	source := filepath.Join(diskRoot, "batch-source.txt")
	const content = "hello from batch copy"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("write batch source file: %v", err)
	}

	manager := internalops.NewManager()
	service := internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	copyToMemory, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      source,
			Destination: "memory://batch/incoming.txt",
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy disk -> memory returned error: %v", err)
	}
	if copyToMemory.Failed != 0 || copyToMemory.Succeeded != 1 {
		t.Fatalf("unexpected disk -> memory batch result: %+v", copyToMemory)
	}

	memoryPath, err := resolveMemoryAlias("memory://batch/incoming.txt")
	if err != nil {
		t.Fatalf("resolve batch memory path: %v", err)
	}
	memoryBytes, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("read batch memory file: %v", err)
	}
	if string(memoryBytes) != content {
		t.Fatalf("batch memory content mismatch\nwant: %q\n got: %q", content, string(memoryBytes))
	}

	roundtripDest := filepath.Join(diskRoot, "batch-roundtrip.txt")
	copyToDisk, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      "memory://batch/incoming.txt",
			Destination: roundtripDest,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy memory -> disk returned error: %v", err)
	}
	if copyToDisk.Failed != 0 || copyToDisk.Succeeded != 1 {
		t.Fatalf("unexpected memory -> disk batch result: %+v", copyToDisk)
	}

	roundtripBytes, err := os.ReadFile(roundtripDest)
	if err != nil {
		t.Fatalf("read batch roundtrip file: %v", err)
	}
	if string(roundtripBytes) != content {
		t.Fatalf("batch roundtrip content mismatch\nwant: %q\n got: %q", content, string(roundtripBytes))
	}
}

func TestBatchCopyBetweenDriveAndMemorySchemes(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	sourceNative := filepath.Join(diskRoot, "scheme-source.txt")
	const content = "hello from scheme batch copy"
	if err := os.WriteFile(sourceNative, []byte(content), 0o644); err != nil {
		t.Fatalf("write scheme source file: %v", err)
	}

	sourceScheme := pathref.FromNativePath(sourceNative)
	diskDestNative := filepath.Join(diskRoot, "scheme-roundtrip.txt")
	diskDestScheme := pathref.FromNativePath(diskDestNative)

	manager := internalops.NewManager()
	service := internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	copyToMemory, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourceScheme,
			Destination: "memory://scheme/incoming.txt",
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy drive scheme -> memory returned error: %v", err)
	}
	if copyToMemory.Failed != 0 || copyToMemory.Succeeded != 1 {
		t.Fatalf("unexpected drive scheme -> memory batch result: %+v", copyToMemory)
	}

	memoryPath, err := resolveMemoryAlias("memory://scheme/incoming.txt")
	if err != nil {
		t.Fatalf("resolve scheme memory path: %v", err)
	}
	memoryBytes, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("read scheme memory file: %v", err)
	}
	if string(memoryBytes) != content {
		t.Fatalf("scheme memory content mismatch\nwant: %q\n got: %q", content, string(memoryBytes))
	}

	copyToDisk, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      "memory://scheme/incoming.txt",
			Destination: diskDestScheme,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy memory -> drive scheme returned error: %v", err)
	}
	if copyToDisk.Failed != 0 || copyToDisk.Succeeded != 1 {
		t.Fatalf("unexpected memory -> drive scheme batch result: %+v", copyToDisk)
	}

	roundtripBytes, err := os.ReadFile(diskDestNative)
	if err != nil {
		t.Fatalf("read scheme roundtrip file: %v", err)
	}
	if string(roundtripBytes) != content {
		t.Fatalf("scheme roundtrip content mismatch\nwant: %q\n got: %q", content, string(roundtripBytes))
	}
}

func TestBatchCopyDirectoryBetweenDriveAndMemorySchemes(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	sourceDirNative := filepath.Join(diskRoot, "scheme-dir")
	nestedDirNative := filepath.Join(sourceDirNative, "nested")
	if err := os.MkdirAll(nestedDirNative, 0o755); err != nil {
		t.Fatalf("create source dir tree: %v", err)
	}
	const content = "hello from scheme dir copy"
	if err := os.WriteFile(filepath.Join(nestedDirNative, "child.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write source dir child: %v", err)
	}

	sourceDirScheme := pathref.FromNativePath(sourceDirNative)
	diskDestDirNative := filepath.Join(diskRoot, "scheme-dir-roundtrip")
	diskDestDirScheme := pathref.FromNativePath(diskDestDirNative)

	manager := internalops.NewManager()
	service := internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	copyToMemory, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourceDirScheme,
			Destination: "memory://scheme-dir-copy",
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy drive scheme dir -> memory returned error: %v", err)
	}
	if copyToMemory.Failed != 0 || copyToMemory.Succeeded != 1 {
		t.Fatalf("unexpected drive scheme dir -> memory batch result: %+v", copyToMemory)
	}

	memoryChildPath, err := resolveMemoryAlias("memory://scheme-dir-copy/nested/child.txt")
	if err != nil {
		t.Fatalf("resolve scheme memory child path: %v", err)
	}
	memoryBytes, err := os.ReadFile(memoryChildPath)
	if err != nil {
		t.Fatalf("read copied memory child: %v", err)
	}
	if string(memoryBytes) != content {
		t.Fatalf("scheme dir memory content mismatch\nwant: %q\n got: %q", content, string(memoryBytes))
	}

	copyToDisk, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      "memory://scheme-dir-copy",
			Destination: diskDestDirScheme,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy memory dir -> drive scheme returned error: %v", err)
	}
	if copyToDisk.Failed != 0 || copyToDisk.Succeeded != 1 {
		t.Fatalf("unexpected memory dir -> drive scheme batch result: %+v", copyToDisk)
	}

	roundtripBytes, err := os.ReadFile(filepath.Join(diskDestDirNative, "nested", "child.txt"))
	if err != nil {
		t.Fatalf("read roundtrip copied child: %v", err)
	}
	if string(roundtripBytes) != content {
		t.Fatalf("scheme dir roundtrip content mismatch\nwant: %q\n got: %q", content, string(roundtripBytes))
	}
}

func TestBatchCopyIntoExistingDirectoryUsesSourceBasename(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	sourceNative := filepath.Join(diskRoot, "existing-dir-source.txt")
	const content = "copy into existing directory"
	if err := os.WriteFile(sourceNative, []byte(content), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	targetDirNative := filepath.Join(diskRoot, "target-dir")
	if err := os.MkdirAll(targetDirNative, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}

	sourceScheme := pathref.FromNativePath(sourceNative)
	targetDirScheme := pathref.FromNativePath(targetDirNative)

	manager := internalops.NewManager()
	service := internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	result, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourceScheme,
			Destination: targetDirScheme,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy into existing directory returned error: %v", err)
	}
	if result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected copy into existing directory result: %+v", result)
	}

	copiedBytes, err := os.ReadFile(filepath.Join(targetDirNative, filepath.Base(sourceNative)))
	if err != nil {
		t.Fatalf("read copied file from target directory: %v", err)
	}
	if string(copiedBytes) != content {
		t.Fatalf("copied file content mismatch\nwant: %q\n got: %q", content, string(copiedBytes))
	}
}

func TestBatchCopyDirectoryIntoExistingDirectoryUsesSourceBasename(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	diskRoot := t.TempDir()
	sourceDirNative := filepath.Join(diskRoot, "source-dir")
	if err := os.MkdirAll(filepath.Join(sourceDirNative, "nested"), 0o755); err != nil {
		t.Fatalf("create source directory tree: %v", err)
	}
	const content = "directory copy into existing directory"
	if err := os.WriteFile(filepath.Join(sourceDirNative, "nested", "child.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("write source directory child: %v", err)
	}

	targetDirNative := filepath.Join(diskRoot, "target-dir")
	if err := os.MkdirAll(targetDirNative, 0o755); err != nil {
		t.Fatalf("create target directory: %v", err)
	}

	sourceDirScheme := pathref.FromNativePath(sourceDirNative)
	targetDirScheme := pathref.FromNativePath(targetDirNative)

	manager := internalops.NewManager()
	service := internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	result, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourceDirScheme,
			Destination: targetDirScheme,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch copy directory into existing directory returned error: %v", err)
	}
	if result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected directory copy into existing directory result: %+v", result)
	}

	copiedBytes, err := os.ReadFile(filepath.Join(targetDirNative, filepath.Base(sourceDirNative), "nested", "child.txt"))
	if err != nil {
		t.Fatalf("read copied nested file from target directory: %v", err)
	}
	if string(copiedBytes) != content {
		t.Fatalf("copied directory content mismatch\nwant: %q\n got: %q", content, string(copiedBytes))
	}
}

func TestBatchMoveFromMemoryToDriveOverwriteUsesResolvedPath(t *testing.T) {
	memoryRoot := t.TempDir()
	resetMemoryResolverForTest(t, memoryRoot)

	sourcePublicPath := "memory://move-source.txt"
	sourceNativePath, err := resolveMemoryAlias(sourcePublicPath)
	if err != nil {
		t.Fatalf("resolve memory source path: %v", err)
	}
	const sourceContent = "updated-from-memory"
	if err := os.WriteFile(sourceNativePath, []byte(sourceContent), 0o644); err != nil {
		t.Fatalf("write memory source file: %v", err)
	}

	diskRoot := t.TempDir()
	destinationNativePath := filepath.Join(diskRoot, "move-source.txt")
	const oldContent = "existing-destination-content"
	if err := os.WriteFile(destinationNativePath, []byte(oldContent), 0o644); err != nil {
		t.Fatalf("write existing destination file: %v", err)
	}
	destinationPublicPath := pathref.FromNativePath(destinationNativePath)

	manager := internalops.NewManager()
	var service *internalbatch.Service
	service = internalbatch.NewService(internalbatch.ServiceConfig{
		StartOperation: manager.StartOperation,
		ConflictPrompter: internalbatch.ConflictPrompterFunc(func(event contracts.ConflictEvent) {
			service.ResolveConflict(event.ConflictID, contracts.ConflictOverwrite, false)
		}),
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source:      source,
				Destination: destination,
				Error:       err.Error(),
				Code:        contracts.ClassifyError(err),
			}
		},
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		CalculateSize:                  internalops.CalculateSize,
		ResolvePath:                    resolveMemoryAlias,
		ToPublicPath:                   toPublicMemoryPath,
	})

	result, err := service.BatchMove(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourcePublicPath,
			Destination: destinationPublicPath,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("batch move memory -> drive overwrite returned error: %v", err)
	}
	if result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected memory -> drive overwrite move result: %+v", result)
	}

	destinationBytes, err := os.ReadFile(destinationNativePath)
	if err != nil {
		t.Fatalf("read overwritten destination file: %v", err)
	}
	if string(destinationBytes) != sourceContent {
		t.Fatalf("overwritten destination content mismatch\nwant: %q\n got: %q", sourceContent, string(destinationBytes))
	}

	if _, err := os.Stat(sourceNativePath); !os.IsNotExist(err) {
		t.Fatalf("expected memory source to be removed after move, stat err=%v", err)
	}
}