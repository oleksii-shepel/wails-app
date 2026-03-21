package batch

import (
	"os"
	"path/filepath"
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
		ResolvePath: func(path string) (string, error) { return path, nil },
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
		ResolvePath: func(path string) (string, error) { return path, nil },
		ToPublicPath: func(path string) string { return path },
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