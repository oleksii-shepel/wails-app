package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97RunEnrichmentOccursBeforeLegacyProjection(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	enrich := strings.Index(source, "word97.EnrichMaterializedRuns(")
	project := strings.Index(source, "adaptCanonicalWord97Runs(canonicalRuns)")
	if enrich < 0 || project < 0 || enrich > project {
		t.Fatal("canonical Word97 runs must be enriched before crossing into binaryWordRun")
	}
	for _, forbidden := range []string{
		"applyCanonicalWord97Enrichment(",
		"injectDocNoteRefs(runs",
		"injectDocCommentRefs(runs",
		"applyResolvedWord97Drawings(runs",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Word97 projection retained post-conversion enrichment %q", forbidden)
		}
	}
	if _, err := os.Stat("word97_enrichment.go"); !os.IsNotExist(err) {
		t.Fatalf("legacy Word97 enrichment adapter must stay removed: %v", err)
	}
}
