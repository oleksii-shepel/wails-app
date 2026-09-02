package batch

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

func TestNormalizeItemDestinationKeepsExactDestination(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "New Folder")
	destPath := filepath.Join(tempDir, "Existing")

	if err := os.Mkdir(sourcePath, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.Mkdir(destPath, 0o755); err != nil {
		t.Fatalf("create destination dir: %v", err)
	}

	runner := NewTransferRunner(TransferCallbacks{
		ResolvePath:  func(path string) (string, error) { return path, nil },
		ToPublicPath: func(path string) string { return path },
	})

	normalized, err := runner.normalizeItemDestination(contracts.BatchItem{
		Source:           sourcePath,
		Destination:      destPath,
		ExactDestination: true,
	}, sourcePath)
	if err != nil {
		t.Fatalf("normalizeItemDestination returned error: %v", err)
	}
	if normalized != destPath {
		t.Fatalf("exact destination mismatch\nwant: %q\n got: %q", destPath, normalized)
	}
}

func TestBatchMoveExactDestinationReportsConflict(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "New Folder")
	destPath := filepath.Join(tempDir, "Existing")

	if err := os.Mkdir(sourcePath, 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	if err := os.Mkdir(destPath, 0o755); err != nil {
		t.Fatalf("create destination dir: %v", err)
	}

	manager := internalops.NewManager()
	var service *Service
	service = NewService(ServiceConfig{
		StartOperation: func(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation {
			return manager.StartOperation(kind, title, source, dest)
		},
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{Source: source, Destination: destination, Error: err.Error(), Code: "unknown"}
		},
		CalculateSize: func(path string) (int64, int64) { return 1, 0 },
		ResolvePath:   func(path string) (string, error) { return path, nil },
		ToPublicPath:  func(path string) string { return path },
		ConflictPrompter: ConflictPrompterFunc(func(event contracts.ConflictEvent) {
			service.ResolveConflict(event.ConflictID, contracts.ConflictSkip, false)
		}),
	})

	result, err := service.BatchMove(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:           sourcePath,
			Destination:      destPath,
			ExactDestination: true,
		}},
		Recursive: true,
	})
	if err != nil {
		t.Fatalf("BatchMove returned error: %v", err)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected conflict skip result, got %+v", result)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Fatalf("source should remain after skipped conflict: %v", err)
	}
}

func TestBatchCopyUsesCallbackForVirtualSource(t *testing.T) {
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "downloaded.txt")
	const sourcePath = "smb://example.com/share/readme.txt"
	const content = "hello from smb"

	manager := internalops.NewManager()
	service := NewService(ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{Source: source, Destination: destination, Error: err.Error(), Code: "unknown"}
		},
		CalculateSize:                  func(path string) (int64, int64) { return 0, 0 },
		ResolvePath:                    func(path string) (string, error) { return path, nil },
		ToPublicPath:                   func(path string) string { return path },
		IsPermissionDenied:             func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy:                func(src, dst string, recursive bool) (bool, error) { return false, nil },
		RunElevatedMove:                func(src, dst string) (bool, error) { return false, nil },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
		MoveToTrash:                    func(path string) error { return nil },
		GetFileInfo: func(path string) (*contracts.FileInfo, error) {
			if path != sourcePath {
				t.Fatalf("unexpected getFileInfo path: %s", path)
			}
			return &contracts.FileInfo{
				Name: "readme.txt",
				Path: path,
				Type: contracts.FileTypeFile,
				Size: int64(len(content)),
			}, nil
		},
		CopyPath: func(source, destination string, recursive bool, streamToFile func(io.Reader, string, int64, string) (int64, error)) error {
			if source != sourcePath {
				t.Fatalf("unexpected source: %s", source)
			}
			if destination != destPath {
				t.Fatalf("unexpected destination: %s", destination)
			}
			if recursive {
				t.Fatalf("recursive should be false for file copy")
			}
			_, err := streamToFile(strings.NewReader(content), destination, int64(len(content)), "readme.txt")
			return err
		},
	})

	result, err := service.BatchCopy(contracts.BatchRequest{
		Items: []contracts.BatchItem{{
			Source:      sourcePath,
			Destination: destPath,
		}},
	})
	if err != nil {
		t.Fatalf("BatchCopy returned error: %v", err)
	}
	if result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != content {
		t.Fatalf("destination content mismatch\nwant: %q\n got: %q", content, string(data))
	}
}
func TestBatchDeleteReportsAffectedSources(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first.txt")
	second := filepath.Join(tempDir, "second.txt")
	if err := os.WriteFile(first, []byte("one"), 0o644); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if err := os.WriteFile(second, []byte("two"), 0o644); err != nil {
		t.Fatalf("write second: %v", err)
	}

	manager := internalops.NewManager()
	service := NewService(ServiceConfig{
		StartOperation: manager.StartOperation,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{Source: source, Destination: destination, Error: err.Error(), Code: "unknown"}
		},
		CalculateSize:                  func(path string) (int64, int64) { return 1, 3 },
		ResolvePath:                    func(path string) (string, error) { return path, nil },
		ToPublicPath:                   func(path string) string { return path },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedDelete:              func(path string, recursive bool, useTrash *bool) (bool, error) { return false, nil },
	})

	useTrash := false
	result, err := service.BatchDelete(contracts.BatchDeleteRequest{
		Paths:     []string{first, second},
		Recursive: true,
		UseTrash:  &useTrash,
	})
	if err != nil {
		t.Fatalf("BatchDelete returned error: %v", err)
	}
	if result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
	if len(result.AffectedPaths) != 2 {
		t.Fatalf("affected paths = %d, want 2", len(result.AffectedPaths))
	}
	if result.AffectedPaths[0].Source != first || result.AffectedPaths[1].Source != second {
		t.Fatalf("unexpected affected paths: %+v", result.AffectedPaths)
	}
}
