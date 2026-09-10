package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalAndEditableReadsShareOneProductionParseBoundary(t *testing.T) {
	parseData, err := os.ReadFile("parse.go")
	if err != nil {
		t.Fatal(err)
	}
	parse := string(parseData)
	if !strings.Contains(parse, "wb, _, err := ParseReaderAtWithPackage(r, size)") {
		t.Fatal("canonical ParseReaderAt must delegate to the single package-returning production boundary")
	}

	editData, err := os.ReadFile(filepath.Join("..", "..", "edit", "workbook.go"))
	if err != nil {
		t.Fatal(err)
	}
	edit := string(editData)
	if !strings.Contains(edit, "sheetooxml.ParseReaderAtWithPackage(r, size)") {
		t.Fatal("editable XLSX reads must reuse the canonical production parse package")
	}
}

func TestMetadataLoaderDoesNotEagerlyOwnWorksheetMediaOrCharts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("parts", "load.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func loadPackage(")
	end := strings.Index(source, "func collectPackageDiagnostics")
	if start < 0 || end <= start {
		t.Fatal("package loader boundary missing")
	}
	block := source[start:end]
	required := []string{
		"if !metadataOnly {",
		"loadMediaParts(",
		"loadChartParts(",
	}
	for _, text := range required {
		if !strings.Contains(block, text) {
			t.Fatalf("full-parser oracle preservation gate missing %q", text)
		}
	}
}

func TestRelationshipObjectLoadRefreshesObjectDiagnostics(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("parts", "load.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func LoadRelationshipObjects")
	end := strings.Index(source[start:], "func loadWorksheets")
	if start < 0 || end < 0 {
		t.Fatal("relationship object loader boundary missing")
	}
	block := source[start : start+end]
	if !strings.Contains(block, "out.Diagnostics = collectPackageDiagnostics(out)") {
		t.Fatal("relationship object ownership must refresh diagnostics after object graph loading")
	}
}
