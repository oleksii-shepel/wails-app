package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97BuildInputsExposeOnlyNeutralAssemblyCatalogs(t *testing.T) {
	data, err := os.ReadFile("word97_build_inputs.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"func word97BinaryDocumentInput(",
		") binaryWordDocumentInput",
		"ListCatalog:    binaryWordListCatalogFromWord97(core.Numbering)",
		"StyleCatalog:   binaryWordStyleCatalogFromWord97(core.Styles)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("Word97 direct assembly-input boundary missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"type word97BuildInputs struct",
		"ListTables",
		"StyleSheet",
		"binaryWordListCatalogFromCanonical(",
		"binaryWordStyleCatalogFromCatalog(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 direct assembly-input boundary retained obsolete ownership %q", forbidden)
		}
	}
}

func TestSharedAssemblyCatalogsAreConcreteNeutralTypes(t *testing.T) {
	for _, tc := range []struct {
		file     string
		required string
	}{
		{"lists.go", "type binaryWordListCatalog struct"},
		{"styles.go", "type binaryWordStyleCatalog struct"},
	} {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), tc.required) {
			t.Fatalf("%s missing concrete neutral catalog %q", tc.file, tc.required)
		}
	}
	for _, file := range []string{"binary_assembly_catalogs.go", "lists.go", "styles.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		for _, forbidden := range []string{"type binaryWordListCatalog =", "type binaryWordStyleCatalog =", "legacyListTables", "legacyStyleSheet"} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s retained obsolete catalog ownership %q", file, forbidden)
			}
		}
	}
}
