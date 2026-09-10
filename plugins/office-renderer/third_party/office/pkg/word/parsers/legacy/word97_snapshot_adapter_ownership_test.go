package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97ProjectionBuildsDocumentDirectlyFromCanonicalSnapshot(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"func loadCanonicalWord97Document(",
		"word97.OpenSnapshotReaderAt(",
		"canonicalRuns := word97.MaterializeRunRange(",
		"buildBinaryWordDocument(filename, input)",
		"adaptCanonicalWord97Runs(canonicalRuns)",
		"input := word97BinaryDocumentInput(snapshot, visible)",
		"word97.ApplySnapshotDocumentState(doc, snapshot)",
		"word97.ApplyDocumentPostProjection(doc, opened.Metadata.BookmarkStarts)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("direct canonical Word97 projection missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"legacyDocParseData{",
		"loadLegacyWord97ParseData(",
		"legacyWord97FullBufferState",
		"legacyWord97StructureFromContext(",
		"applyLegacyWord97Structure(",
		"applyCanonicalWord97Context(",
		"legacyComplexFileTableFromSource(",
		"FullBuffer",
		"opened.RawCLX",
		"appendSequentialCHPX",
		"appendSequentialPAPX",
		"rememberLegacyWord97Structure",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Word97 projection retained compatibility carrier/state %q", forbidden)
		}
	}
}
