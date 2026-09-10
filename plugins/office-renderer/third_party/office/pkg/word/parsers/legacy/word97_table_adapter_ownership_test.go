package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97TablePropsUseCanonicalSharedCarrier(t *testing.T) {
	projectionData, err := os.ReadFile("word97_run_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(projectionData)

	for _, required := range []string{
		"TableProps:",
		"r.TableProps.Clone()",
	} {
		if !strings.Contains(projection, required) {
			t.Fatalf("Word97 table projection missing canonical carrier %q", required)
		}
	}
	for _, forbidden := range []string{
		"CanonicalTableProps",
		"binaryWordTableRowPropsFromCanonical(",
		"legacyTableRowPropsFromCanonical(",
		"tableRowPropsStorageFromCanonical(",
		"legacyTableRowProps",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Word97 table projection regained legacy storage dependency %q", forbidden)
		}
	}

	docData, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(docData)
	if !strings.Contains(doc, "TableProps *docformat.TableRowProps") {
		t.Fatal("shared binary Word run must carry canonical table props")
	}
	for _, forbidden := range []string{
		"CanonicalTableProps",
		"binaryWordRunTableProps(",
		"TableProps          *legacyTableRowProps",
	} {
		if strings.Contains(doc, forbidden) {
			t.Fatalf("shared binary Word carrier retained transitional table dependency %q", forbidden)
		}
	}
}
