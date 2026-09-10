package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWord97ProjectionUsesSingleBodyBuildInputBoundary(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"input := word97BinaryDocumentInput(snapshot, visible)",
		"buildBinaryWordDocument(filename, input)",
		"word97.ApplySnapshotDiagnostics(doc, snapshot)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("Word97 projection missing consolidated boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"word97DocumentBuildInputs(",
		"word97BuildInputs",
		"build.ListTables",
		"build.StyleSheet",
		"legacyBookmarkEndCPByName(",
		"binaryWordListCatalogFromCanonical(",
		"binaryWordStyleCatalogFromCatalog(",
		"for _, entry := range opened.CHPX",
		"for _, entry := range opened.PAPX",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 projection retained legacy/piecemeal build ownership %q", forbidden)
		}
	}
	if _, err := os.Stat("word97_styles_bridge.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("standalone Word97 style bridge must be removed: %v", err)
	}
}
