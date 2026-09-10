package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionOOXMLReadUsesExplicitMetadataBoundary(t *testing.T) {
	data, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"parts.LoadPackage(", "parts.LoadCompatibilityPackage(",
		"resolve.ResolvePackage(", "resolve.ResolveCompatibilityPackage(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("production OOXML read path regained obsolete parser bridge %q", forbidden)
		}
	}
	if !strings.Contains(source, "parts.LoadMetadataPackage(pkg)") ||
		!strings.Contains(source, "resolve.ResolveMetadataPackage(parsed)") {
		t.Fatal("production OOXML read path must use explicit package metadata ownership")
	}
}

func TestMetadataLoaderDoesNotOwnWorksheetDataOrObjects(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("parts", "load.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func loadWorksheetSkeletons")
	endRel := strings.Index(source, "func LoadRelationshipObjects")
	if start < 0 || endRel < 0 || endRel <= start {
		t.Fatal("worksheet metadata skeleton / relationship-object ownership boundary missing")
	}
	block := source[start:endRel]
	for _, forbidden := range []string{
		`xml:"sheetData>row"`, `xml:"cols>col"`, `xml:"mergeCells>mergeCell"`,
		`xml:"sheetViews>sheetView"`, `xml:"hyperlinks>hyperlink"`,
		"loadWorksheetPivotTables(", "loadWorksheetDrawing(", "loadWorksheetTables(",
	} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("metadata skeleton regained worksheet/object ownership %q", forbidden)
		}
	}
}
