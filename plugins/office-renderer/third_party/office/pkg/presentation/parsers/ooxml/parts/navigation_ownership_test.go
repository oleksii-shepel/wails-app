package parts

import (
	"os"
	"strings"
	"testing"
)

func TestPresentationMLNavigationHasSingleOPCOwnershipSurface(t *testing.T) {
	data, err := os.ReadFile("navigation.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"type packageNavigator struct", "decodeRelatedXML", "rels.ResolveTarget", "xml.Unmarshal"} {
		if !strings.Contains(text, required) {
			t.Fatalf("navigation.go must own %q", required)
		}
	}
}

func TestPresentationMLSemanticAssemblyDoesNotDecodeOPCDirectly(t *testing.T) {
	for _, name := range []string{"assembly.go", "load.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"pkg.Part(", "pkg.Relationships(", "xml.Unmarshal("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s bypasses packageNavigator with %q", name, forbidden)
			}
		}
	}
}
