package archive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishNewArchivePreservesExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "staged.zip")
	destination := filepath.Join(root, "existing.zip")
	for path, contents := range map[string]string{source: "new data", destination: "original data"} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := PublishNewArchive(source, destination); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected destination collision, got %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "original data" {
		t.Fatalf("destination = %q, error = %v", got, err)
	}
}

func TestPublishNewArchiveCopiesCompletedStagingFile(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "staged.zip")
	destination := filepath.Join(root, "new.zip")
	if err := os.WriteFile(source, []byte("archive data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PublishNewArchive(source, destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "archive data" {
		t.Fatalf("destination = %q, error = %v", got, err)
	}
}
