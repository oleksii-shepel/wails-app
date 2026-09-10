package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestSharedBinaryWordAssemblerDoesNotUseLegacyDocRunCarrierName(t *testing.T) {
	for _, name := range []string{"binary_document_builder.go", "doc.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "docRun") || strings.Contains(string(data), "docField") {
			t.Fatalf("%s still exposes the old legacy run/field carrier names", name)
		}
	}
}

func TestWord97RunAdapterIsOnlyRemainingWord97AssemblyBoundary(t *testing.T) {
	runData, err := os.ReadFile("word97_run_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	runAdapter := string(runData)
	for _, required := range []string{
		"func adaptCanonicalWord97Runs(src []word97.Run) []binaryWordRun",
		"func adaptCanonicalWord97Run(r word97.Run) binaryWordRun",
		"func adaptCanonicalMaterializedFields(src []*docformat.MaterializedField[word97.Run]) []*binaryWordField",
	} {
		if !strings.Contains(runAdapter, required) {
			t.Fatalf("Word97 run boundary missing %q", required)
		}
	}
	if _, err := os.Stat("word97_field_adapter.go"); !os.IsNotExist(err) {
		t.Fatal("obsolete standalone Word97 field adapter must stay deleted")
	}
	for _, forbidden := range []string{
		"adaptCanonicalWord97Fields",
		"projectCanonicalWord97Runs",
		"projectCanonicalWord97Run",
		"projectCanonicalWord97Fields",
		"docRun",
		"docField",
	} {
		if strings.Contains(runAdapter, forbidden) {
			t.Fatalf("Word97 run boundary retained obsolete compatibility surface %q", forbidden)
		}
	}
}
