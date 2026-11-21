package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkDirOrderedPropagatesMissingRoot(t *testing.T) {
	err := walkDirOrdered(filepath.Join(t.TempDir(), "missing"), func(string, bool, int64) error {
		t.Fatal("visitor must not run for a missing root")
		return nil
	})
	if err == nil {
		t.Fatal("expected an error for a missing root")
	}
}

func TestWalkDirOrderedPropagatesVisitError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	wantErr := os.ErrPermission
	err := walkDirOrdered(root, func(string, bool, int64) error {
		return wantErr
	})
	if err == nil {
		t.Fatal("expected the visit error to propagate")
	}
}

func TestDeleteDirReportsMissingPath(t *testing.T) {
	operator := NewFileOperator(nil, NewProgressTracker(nil))
	if err := operator.DeleteDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected deleting a missing path to fail")
	}
}

func TestDeleteDirRemovesNestedTree(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "tree")
	nested := filepath.Join(target, "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		if err := os.WriteFile(filepath.Join(target, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	operator := NewFileOperator(nil, NewProgressTracker(nil))
	if err := operator.DeleteDir(target); err != nil {
		t.Fatalf("DeleteDir: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("target still exists after DeleteDir")
	}
}
