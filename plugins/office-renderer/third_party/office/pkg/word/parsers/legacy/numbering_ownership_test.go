package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsNumberingBinaryLayout(t *testing.T) {
	canonicalPath := filepath.Join("..", "doc", "word97", "open_readerat.go")
	canonicalData, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(canonicalData), "Numbering") {
		t.Fatal("canonical Word97 OpenCore must own numbering state")
	}

	projectionData, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(projectionData)
	if !strings.Contains(projection, "input := word97BinaryDocumentInput(snapshot, runs)") {
		t.Fatal("legacy Word97 projection must consume canonical numbering through consolidated build inputs")
	}
	for _, forbidden := range []string{
		"parseLegacyListTables(",
		"loadLegacyListTables(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("legacy Word97 projection must not decode numbering through %q", forbidden)
		}
	}
}
