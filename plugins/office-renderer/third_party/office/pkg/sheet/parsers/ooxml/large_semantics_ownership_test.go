package ooxml

import (
	"os"
	"strings"
	"testing"
)

func TestCompatibilityWorksheetDoesNotOwnStreamedSemantics(t *testing.T) {
	data, err := os.ReadFile("parts/load.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if strings.Contains(source, "type compatibilityWorksheet struct") {
		t.Fatal("compatibility worksheet parser must not exist after relationship-object cutover")
	}
	start := strings.Index(source, "func loadWorksheetSkeletons")
	end := strings.Index(source[start:], "func LoadRelationshipObjects")
	if start < 0 || end < 0 {
		t.Fatal("compatibility/object ownership boundaries missing")
	}
	block := source[start : start+end]
	for _, forbidden := range []string{"DrawingRef", "LegacyDrawing", "OLEObjects", "HeaderFooter", "TableParts", "PivotTables", "loadWorksheetDrawing", "loadWorksheetTables", "loadWorksheetPivotTables"} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("worksheet metadata skeleton loader regained relationship-object ownership %q", forbidden)
		}
	}
}

func TestProductionStreamingEnablesCommentOwnership(t *testing.T) {
	data, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "IncludeComments: true") {
		t.Fatal("production streaming path must own comments")
	}
}
