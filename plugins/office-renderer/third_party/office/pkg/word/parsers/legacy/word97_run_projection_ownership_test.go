package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalWord97RunProjectionIsIsolated(t *testing.T) {
	data, err := os.ReadFile("word97_run_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	for _, required := range []string{
		"adaptCanonicalWord97Runs(",
		"adaptCanonicalWord97Run(",
		"TableProps:",
		"r.TableProps.Clone()",
		"adaptCanonicalMaterializedFields(r.Fields)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("canonical run projection missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"adaptCanonicalWord97Fields(",
		"CanonicalTableProps",
		"binaryWordTableRowPropsFromCanonical(",
		"legacyTableRowPropsFromCanonical(",
		"legacyTableRowProps",
		"DecodeDocumentSettings(",
		"loadLegacyDocCompatibility(",
		"loadLegacyDopRecord(",
		"parsePapxGrpprl(",
		"parseChpxGrpprl(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("canonical run projection regained forbidden dependency %q", forbidden)
		}
	}
}
