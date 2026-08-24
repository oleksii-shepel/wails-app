package batch

import (
	"os"
	"path/filepath"
	"testing"

	contracts "wails-app/internal/contracts"
	internalops "wails-app/internal/ops"
)

func trashBool(value bool) *bool { return &value }

func trashTestService(
	t *testing.T,
	start func(kind contracts.OperationType, title, source, dest string) *internalops.ActiveOperation,
) *Service {
	t.Helper()
	return NewService(ServiceConfig{
		StartOperation: start,
		BatchErr: func(source, destination string, err error) contracts.BatchItemError {
			return contracts.BatchItemError{
				Source: source,
				Destination: destination,
				Error: err.Error(),
				Code: "unknown",
			}
		},
		CalculateSize: func(path string) (int64, int64) {
			info, err := os.Stat(path)
			if err != nil {
				return 0, 0
			}
			return 1, info.Size()
		},
		ResolvePath: func(path string) (string, error) { return path, nil },
		ToPublicPath: func(path string) string { return path },
		MoveToTrash: func(path string) error { return os.Remove(path) },
		IsPermissionDenied: func(error) bool { return false },
		ShouldRetryDeleteWithElevation: func(error) bool { return false },
		RunElevatedCopy: func(src, dst string, recursive bool) (bool, error) {
			return false, nil
		},
		RunElevatedMove: func(src, dst string) (bool, error) {
			return false, nil
		},
		RunElevatedDelete: func(path string, recursive bool, useTrash *bool) (bool, error) {
			return false, nil
		},
	})
}

func TestBatchDeleteToTrashStartsMoveOperation(t *testing.T) {
	manager := internalops.NewManager()

	var gotKind contracts.OperationType
	var gotTitle, gotSource, gotDestination string
	start := func(
		kind contracts.OperationType,
		title, source, destination string,
	) *internalops.ActiveOperation {
		gotKind = kind
		gotTitle = title
		gotSource = source
		gotDestination = destination
		return manager.StartOperation(kind, title, source, destination)
	}

	source := filepath.Join(t.TempDir(), "move-me.txt")
	if err := os.WriteFile(source, []byte("trash progress"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := trashTestService(t, start)
	result, err := service.BatchDelete(contracts.BatchDeleteRequest{
		Paths: []string{source},
		Recursive: true,
		UseTrash: trashBool(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 {
		t.Fatalf("expected one trashed item, got %+v", result)
	}
	if gotKind != contracts.OpMove {
		t.Fatalf("expected MOVE operation, got %q", gotKind)
	}
	if gotTitle != "Move to Recycle Bin" {
		t.Fatalf("unexpected title %q", gotTitle)
	}
	if gotSource != source {
		t.Fatalf("expected source %q, got %q", source, gotSource)
	}
	if gotDestination != "bin://" {
		t.Fatalf("expected bin:// destination, got %q", gotDestination)
	}
}

func TestPermanentBatchDeleteRemainsDeleteOperation(t *testing.T) {
	manager := internalops.NewManager()

	var gotKind contracts.OperationType
	var gotTitle, gotDestination string
	start := func(
		kind contracts.OperationType,
		title, source, destination string,
	) *internalops.ActiveOperation {
		gotKind = kind
		gotTitle = title
		gotDestination = destination
		return manager.StartOperation(kind, title, source, destination)
	}

	source := filepath.Join(t.TempDir(), "delete-me.txt")
	if err := os.WriteFile(source, []byte("delete permanently"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := trashTestService(t, start)
	result, err := service.BatchDelete(contracts.BatchDeleteRequest{
		Paths: []string{source},
		Recursive: true,
		UseTrash: trashBool(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Succeeded != 1 {
		t.Fatalf("expected one deleted item, got %+v", result)
	}
	if gotKind != contracts.OpDelete {
		t.Fatalf("expected DELETE operation, got %q", gotKind)
	}
	if gotTitle != "Batch delete" {
		t.Fatalf("unexpected title %q", gotTitle)
	}
	if gotDestination != "" {
		t.Fatalf("permanent delete should not have a destination, got %q", gotDestination)
	}
}
