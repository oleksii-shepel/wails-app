package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97UsesSharedBinaryDocumentAssemblerNotLegacyContextBuilder(t *testing.T) {
	projectionData, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(projectionData)
	if !strings.Contains(projection, "buildBinaryWordDocument(filename, input)") {
		t.Fatal("Word97 projection must enter the decoded binary Word assembler directly")
	}
	if strings.Contains(projection, "buildLegacyDocFromRunsWithContext(") {
		t.Fatal("Word97 projection must not depend on the legacy context-builder shim")
	}
	builderData, err := os.ReadFile("binary_document_builder.go")
	if err != nil {
		t.Fatal(err)
	}
	builder := string(builderData)
	for _, required := range []string{
		"type binaryWordDocumentInput struct",
		"ListCatalog     *binaryWordListCatalog",
		"StyleCatalog    *binaryWordStyleCatalog",
		"func buildBinaryWordDocument(",
		"lineSlices := splitDocRunsIntoLines(runs)",
		"rebindLegacyTopLevelBlocks(doc)",
	} {
		if !strings.Contains(builder, required) {
			t.Fatalf("shared binary Word builder missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"ListTables      *binaryWordListCatalog",
		"StyleSheet      *binaryWordStyleCatalog",
	} {
		if strings.Contains(builder, forbidden) {
			t.Fatalf("shared binary Word builder retained legacy catalog contract %q", forbidden)
		}
	}
}

func TestPreWord97ProducesSharedBinaryDocumentInput(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"Assembly  binaryWordDocumentInput",
		"data.Assembly.Runs = runs",
		"buildBinaryWordDocument(filename, data.Assembly)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("pre-Word97 path missing shared assembly input %q", required)
		}
	}
	for _, forbidden := range []string{
		"ListTables      *binaryWordListCatalog",
		"StyleSheet      *binaryWordStyleCatalog",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("pre-Word97 carrier retained duplicate assembly field %q", forbidden)
		}
	}
}
