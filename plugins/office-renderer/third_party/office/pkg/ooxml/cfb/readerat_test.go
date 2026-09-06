package cfb_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"office/pkg/ooxml/cfb"
)

func TestOpenReaderAtReadsRegularStreamWithoutWholeFileBuffer(t *testing.T) {
	payload := []byte("reader-at-cfb")
	data := buildMinimalCFB("WordDocument", payload)
	f, err := cfb.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReaderAt: %v", err)
	}
	got, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("WordDocument stream missing")
	}
	if len(got) < len(payload) || !bytes.Equal(got[:len(payload)], payload) {
		t.Fatalf("stream prefix = %q, want %q", got, payload)
	}
}

func TestOpenFileBackedReadsAndCloses(t *testing.T) {
	payload := []byte("file-backed-cfb")
	data := buildMinimalCFB("WordDocument", payload)
	path := filepath.Join(t.TempDir(), "sample.doc")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	f, err := cfb.OpenFileBacked(path)
	if err != nil {
		t.Fatalf("OpenFileBacked: %v", err)
	}
	got, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("WordDocument stream missing")
	}
	if len(got) < len(payload) || !bytes.Equal(got[:len(payload)], payload) {
		t.Fatalf("stream = %q, want prefix %q", got, payload)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
