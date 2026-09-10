package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97PostProjectionCompatibilityIsCanonical(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	if !strings.Contains(src, "word97.ApplyDocumentPostProjection(doc, opened.Metadata.BookmarkStarts)") {
		t.Fatal("Word97 projection must delegate document-level semantics to canonical word97")
	}

	for _, forbidden := range []string{
		"resolveDocCrossReferenceFields(",
		"applyLegacyPreservationMeta(",
		"applyPreWord97PreservationMeta(",
		"SnapToGrid = true",
		"legacyPreservationMetaStream",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 projection retained legacy post-projection compatibility %q", forbidden)
		}
	}

	metaData, err := os.ReadFile("legacy_metadata.go")
	if err != nil {
		t.Fatal(err)
	}
	meta := string(metaData)
	if strings.Contains(meta, "func applyLegacyPreservationMeta(") {
		t.Fatal("mixed-era preservation reader name must stay removed")
	}
	if !strings.Contains(meta, "func applyPreWord97PreservationMeta(") {
		t.Fatal("genuine pre-97 preservation compatibility must remain explicit")
	}
}
