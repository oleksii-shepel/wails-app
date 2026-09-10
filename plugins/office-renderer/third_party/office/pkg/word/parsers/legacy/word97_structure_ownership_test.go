package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWord97StructureHandoffStaysRemoved(t *testing.T) {
	if _, err := os.Stat("word97_structure_handoff.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Word97 structure handoff must stay removed: %v", err)
	}
	if _, err := os.Stat("word97_structure_projection.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy Word97 structure projector must stay removed: %v", err)
	}
}

func TestLegacySectionAdapterIsPre97Only(t *testing.T) {
	data, err := os.ReadFile("sepx.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"takeLegacyWord97Sections(", "word97.OpenSnapshotReaderAt("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("section adapter retained Word97 path %q", forbidden)
		}
	}
}

func TestLegacyHeaderFooterAdapterIsPre97Only(t *testing.T) {
	data, err := os.ReadFile("hdrftr.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{"takeLegacyWord97HeaderFooterData(", "word97.OpenSnapshotReaderAt("} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("header/footer adapter retained Word97 path %q", forbidden)
		}
	}
}

func TestWord97ProjectionProjectsCanonicalStructureThroughSnapshotBoundary(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "word97.ApplySnapshotDocumentState(doc, snapshot)") {
		t.Fatal("direct Word97 projection must delegate canonical structure to word97")
	}
	for _, forbidden := range []string{
		"applyCanonicalWord97Context(",
		"data.Structure",
		"legacyDocParseData{",
		"legacyWord97StructureFromContext(",
		"applyLegacyWord97Structure(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("direct Word97 structure projection retained compatibility state %q", forbidden)
		}
	}
}
