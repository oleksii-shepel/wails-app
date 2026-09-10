package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWord97SnapshotDocumentStateProjectionIsCanonical(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "word97.ApplySnapshotDocumentState(doc, snapshot)") {
		t.Fatal("Word97 projection must consume canonical snapshot document state")
	}
	for _, forbidden := range []string{
		"applyCanonicalWord97Settings(",
		"applyCanonicalWord97Context(",
		"legacyComplexFileTableFromSource(",
		"doc.FontFamilies = opened.FontFamilies",
		"doc.EmbeddedFonts = opened.EmbeddedFonts",
		"doc.Source = &wordir.DocumentSource{",
		"doc.Compatibility = opened.Settings.Compatibility",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 projection retained piecemeal snapshot projection %q", forbidden)
		}
	}
	if _, err := os.Stat("word97_structure_projection.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired legacy Word97 structure projector must stay removed: %v", err)
	}
}
