package contracts

import (
	"encoding/json"
	"testing"
)

func TestFileInfoWireShape(t *testing.T) {
	raw, err := json.Marshal(FileInfo{Name: "example.txt", Path: "C:/example.txt", Type: FileTypeFile, Size: 42})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "example.txt" || got["type"] != string(FileTypeFile) {
		t.Fatalf("unexpected wire shape: %s", raw)
	}
}

func TestSystemSurfaceRecordKeepsOpenFields(t *testing.T) {
	record := SystemSurfaceRecord{ID: "process:42", Name: "demo", Fields: map[string]any{"pid": 42}}
	raw, err := json.Marshal(record)
	if err != nil || len(raw) == 0 {
		t.Fatalf("encode failed: %v", err)
	}
}
