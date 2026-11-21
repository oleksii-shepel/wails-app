package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAddZipTreeExcludesItsStagingFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	tempArchive := filepath.Join(root, "staging.zip")
	for _, path := range []string{source, tempArchive} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	if err := addZipTree(writer, root, "folder", make(map[string]struct{}), tempArchive); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[1].Name != "folder/source.txt" {
		t.Fatalf("expected only the folder and source entry, got %v", len(reader.File))
	}
}
