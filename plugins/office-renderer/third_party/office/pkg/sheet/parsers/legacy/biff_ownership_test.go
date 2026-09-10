package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestXLSContainerAndRawBIFFConvergeOnCanonicalBIFFBoundary(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	start := strings.Index(source, "func ParseXlsBytes")
	end := strings.Index(source[start:], "func isRawBIFFStream")
	if start < 0 || end < 0 {
		t.Fatal("legacy XLS public parser boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"openBIFFWorkbookStream(data)",
		"ParseBIFFStream(stream, filename)",
		"parseRawBIFFWorkbookStream(stream, filename)",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("legacy XLS input paths must converge through canonical BIFF ownership %q", required)
		}
	}
}

func TestCanonicalBIFFBoundaryOwnsWorkbookProjection(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func ParseBIFFStream")
	end := strings.Index(source[start:], "func normalizeLegacyFilename")
	if start < 0 || end < 0 {
		t.Fatal("canonical BIFF stream boundary missing")
	}
	block := source[start : start+end]
	if !strings.Contains(block, "parseWorkbook(stream, normalizeLegacyFilename(filename))") {
		t.Fatal("canonical BIFF stream boundary must project directly into canonical workbook IR")
	}
	if strings.Contains(block, "cfb.Open") {
		t.Fatal("canonical BIFF parser must not own CFB container extraction")
	}
}

func TestRawBIFFFallbackPreservesCanonicalWorksheetSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func rawWorksheetHasContent")
	end := strings.Index(source[start:], "func parseWorkbook(")
	if start < 0 || end < 0 {
		t.Fatal("raw BIFF canonical worksheet fallback boundary missing")
	}
	block := source[start : start+end]
	for _, semantic := range []string{
		"sheet.Rows",
		"sheet.MergedCells",
		"sheet.Drawings",
		"sheet.FormControls",
		"sheet.Notes",
		"sheet.DataValidations",
		"sheet.CFRules",
		"sheet.PageSetup",
		"sheet.Protection",
		"sheet.RowPageBreaks",
		"sheet.ColumnWidths",
	} {
		if !strings.Contains(block, semantic) {
			t.Fatalf("raw BIFF fallback lost canonical worksheet semantic %q", semantic)
		}
	}
}
