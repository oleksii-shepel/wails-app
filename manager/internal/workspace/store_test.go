package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStateLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	store := &Store{statePath: filepath.Join(dir, "state.json")}

	if err := store.WriteState([]byte(`{"first":true}`)); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := store.WriteState([]byte(`{"second":true}`)); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, err := os.ReadFile(store.statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if string(data) != `{"second":true}` {
		t.Fatalf("state content = %q, want the latest write", data)
	}
	if _, err := os.Stat(store.statePath + ".tmp"); err == nil {
		t.Fatal("temp file was left behind after the rename")
	}
}

func TestWriteStateFailsWhenDestinationIsUndesirable(t *testing.T) {
	dir := t.TempDir()
	// A directory in place of the state file makes the rename fail; the temp
	// file must be cleaned up instead of leaking.
	statePath := filepath.Join(dir, "state.json")
	if err := os.Mkdir(statePath, 0o755); err != nil {
		t.Fatalf("mkdir state path: %v", err)
	}
	store := &Store{statePath: statePath}

	if err := store.WriteState([]byte("{}")); err == nil {
		t.Fatal("expected the write to fail against a directory")
	}
	if _, err := os.Stat(statePath + ".tmp"); err == nil {
		t.Fatal("temp file was left behind after the failed rename")
	}
}
