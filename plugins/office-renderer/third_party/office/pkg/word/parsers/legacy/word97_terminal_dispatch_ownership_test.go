package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalWord97ProjectionHasExplicitFailureBoundary(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		"func loadCanonicalWord97Document(filename string, wordStream []byte, f *cfb.File) (*wordir.Document, error)",
		`fmt.Errorf("legacy doc: canonical Word97 snapshot: %w", err)`,
		`fmt.Errorf("legacy doc: canonical Word97 snapshot is incomplete")`,
		"return doc, nil",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("canonical Word97 projection missing explicit result contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"if len(canonicalRuns) == 0 {",
		"scoreLegacyText(joinDocRunsText(visible)) == 0",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("valid empty/zero-visible-text Word97 documents must not be rejected by %q", forbidden)
		}
	}
}
