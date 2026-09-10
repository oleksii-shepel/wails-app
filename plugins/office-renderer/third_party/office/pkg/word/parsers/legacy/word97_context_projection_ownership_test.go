package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCanonicalWord97ContextProjectionLivesInWord97(t *testing.T) {
	if _, err := os.Stat("word97_structure_projection.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy Word97 structure projection must stay removed: %v", err)
	}
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "word97.ApplySnapshotDocumentState(doc, snapshot)") {
		t.Fatal("legacy projection must delegate snapshot structure to canonical word97")
	}
}
