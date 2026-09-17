package layout

import (
	"os"
	"strings"
	"testing"
)

func TestLayoutDoesNotRetainCompetingOOXMLResolver(t *testing.T) {
	if _, err := os.Stat("resolve_adapters.go"); !os.IsNotExist(err) {
		t.Fatalf("superseded resolver adapter still exists: %v", err)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{"func resolveShape(", "func resolvePicture(", "func resolveGroupShape(", "func resolveConnector(", "func resolveParagraph(", "func resolveRun(", "func findLayoutPlaceholderShape(", "func findMasterPlaceholderShape("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s retained competing resolver surface %q", entry.Name(), forbidden)
			}
		}
	}
}
