package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyWord97AdapterConsumesCanonicalSnapshotDirectly(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"word97.OpenSnapshotReaderAt(",
		"word97.MaterializeRunRange(",
		"word97.EnrichMaterializedRuns(canonicalRuns, opened.Metadata, snapshot.Context, f)",
		"input := word97BinaryDocumentInput(snapshot, visible)",
		"opened.Metadata",
		"word97.ApplySnapshotDocumentState(doc, snapshot)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("legacy Word97 projection must consume %q", want)
		}
	}
	for _, forbidden := range []string{
		"word97.OpenCoreReaderAt(",
		"word97.LoadDocumentContextReaderAt(",
		"word97.LoadRevisionAuthorsReaderAt(",
		"opened.RawCLX",
		"opened.RevisionAuthors",
		"opened.Settings",
		"opened.FontFamilies",
		"opened.EmbeddedFonts",
		"opened.RawDOP",
		"opened.Source",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy Word97 projection regained piecemeal/intermediate ownership %q", forbidden)
		}
	}
}
