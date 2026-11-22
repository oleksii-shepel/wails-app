package app

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddTarTreeExcludesItsStagingFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	staging := filepath.Join(root, "staging")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staging, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := addTarTree(writer, root, "folder", make(map[string]struct{}), staging); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	entries := readTarEntries(t, bytes.NewReader(buffer.Bytes()))
	if len(entries) != 2 {
		t.Fatalf("expected folder dir plus source entry, got %v", entryNames(entries))
	}
	if !entries[0].isDir || entries[0].name != "folder" {
		t.Fatalf("entry[0] = %v, want dir folder", entries[0])
	}
	if entries[1].name != "folder/source.txt" || entries[1].body != "fixture" {
		t.Fatalf("entry[1] = %v, want folder/source.txt with the payload", entries[1])
	}
}

type tarEntryRecord struct {
	name string
	isDir bool
	body string
}

func entryNames(entries []tarEntryRecord) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.name
	}
	return names
}

func readTarEntries(t *testing.T, r io.Reader) []tarEntryRecord {
	t.Helper()
	tr := tar.NewReader(r)
	var entries []tarEntryRecord
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		body := ""
		if header.Typeflag == tar.TypeReg {
			payload, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read %q: %v", header.Name, err)
			}
			body = string(payload)
		}
		entries = append(entries, tarEntryRecord{
			name:  strings.TrimSuffix(header.Name, "/"),
			isDir: header.Typeflag == tar.TypeDir,
			body:  body,
		})
	}
}
