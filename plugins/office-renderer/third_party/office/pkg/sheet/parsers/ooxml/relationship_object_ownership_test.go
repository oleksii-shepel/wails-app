package ooxml

import (
	"os"
	"strings"
	"testing"
)

func TestProductionOwnsRelationshipObjectsExplicitly(t *testing.T) {
	data, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	loadAt := strings.Index(source, "parts.LoadRelationshipObjects(pkg, parsed)")
	resolveAt := strings.Index(source, "resolve.ResolveMetadataPackage(parsed)")
	if loadAt < 0 || resolveAt < 0 || loadAt > resolveAt {
		t.Fatal("relationship object graph must load explicitly before canonical metadata projection")
	}
}

func TestRelationshipObjectLoaderOwnsWholeObjectFamily(t *testing.T) {
	data, err := os.ReadFile("parts/load.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func LoadRelationshipObjects")
	if start < 0 {
		t.Fatal("relationship object loader missing")
	}
	block := source[start:]
	for _, required := range []string{
		"loadWorksheetDrawing(", "loadWorksheetLegacyDrawingRaw(",
		"loadWorksheetFormControls(", "loadWorksheetLegacyHFImages(",
		"loadWorksheetPivotTables(", "loadWorksheetTables(",
		"loadWorksheetOLEObjects(",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("relationship object loader does not own %q", required)
		}
	}
}
