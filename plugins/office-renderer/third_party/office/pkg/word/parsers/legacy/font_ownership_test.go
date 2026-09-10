package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsEmbeddedFontDecoding(t *testing.T) {
	canonicalPath := filepath.Join("..", "doc", "word97", "open_readerat.go")
	data, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "EmbeddedFonts") {
		t.Fatal("canonical Word97 OpenCore must own embedded-font state")
	}
	projectionData, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(projectionData)
	if !strings.Contains(projection, "word97.ApplySnapshotDocumentState(doc, snapshot)") {
		t.Fatal("legacy Word97 projection must consume canonical snapshot-owned fonts")
	}
	for _, forbidden := range []string{
		"doc.EmbeddedFonts = opened.EmbeddedFonts",
		"loadLegacyEmbeddedFonts(",
		"parseLegacyEmbeddedFonts(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("legacy Word97 projection retained font ownership %q", forbidden)
		}
	}
}
