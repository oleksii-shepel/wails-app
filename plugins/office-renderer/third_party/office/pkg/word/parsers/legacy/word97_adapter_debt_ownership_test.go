package legacy

import (
	"os"
	"strings"
	"testing"
)

// This gate names the remaining semantic conversion. Both Word97 and pre-97
// enter the shared binary Word assembler with canonical table and field shapes;
// only the run payload still crosses a Word97-to-assembler carrier boundary.
func TestWord97RunAdapterHasOnlyKnownSemanticShapeBridges(t *testing.T) {
	data, err := os.ReadFile("word97_run_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	for _, required := range []string{
		"TableProps:",
		"r.TableProps.Clone()",
		"adaptCanonicalMaterializedFields(r.Fields)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("Word97 semantic adapter missing canonical assembly bridge %q", required)
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
			t.Fatalf("Word97 semantic adapter regained forbidden dependency %q", forbidden)
		}
	}
}
