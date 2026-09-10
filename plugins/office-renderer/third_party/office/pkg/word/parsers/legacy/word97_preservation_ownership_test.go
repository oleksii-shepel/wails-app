package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97ProjectionDoesNotReloadPreservationOutsideCanonicalSnapshot(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"word97.OpenSnapshotReaderAt(",
		"word97.ApplySnapshotDocumentState(doc, snapshot)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("Word97 projection must consume canonical preservation through %q", required)
		}
	}
	for _, forbidden := range []string{
		"opened.Source",
		"legacyComplexFileTableFromSource(",
		"mergeLegacyStreamPreservationMeta(",
		"word97.LoadPreservation(",
		"applyLegacyPreservationMeta(",
		"applyPreWord97PreservationMeta(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 projection retained/reloaded preservation through %q", forbidden)
		}
	}
}
