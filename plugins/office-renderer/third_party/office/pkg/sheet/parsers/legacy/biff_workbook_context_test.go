package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestProductionBIFFWorkbookUsesOneGlobalContext(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseWorkbook(data []byte")
	end := strings.Index(source[start:], "// findSheetBOFOffsets")
	if start < 0 || end < 0 {
		t.Fatal("production BIFF workbook projection boundary missing")
	}
	block := source[start : start+end]
	if !strings.Contains(block, "buildLegacyWorkbookParseContext(data)") {
		t.Fatal("production BIFF workbook must consume the canonical workbook-global context")
	}
	for _, forbidden := range []string{
		"parseWorkbookGlobals(data)",
		"parseLegacyPivotCacheSources(data)",
		"parseLegacyWorkbookMetadata(data)",
		"parseLegacyExternalLinks(data)",
		"findSheetBOFOffsets(data)",
	} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("production workbook projection retained independent global scan %q", forbidden)
		}
	}
}

func TestLegacyWorkbookContextCarriesCrossSheetDependencies(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "type legacyWorkbookParseContext struct")
	end := strings.Index(source[start:], "func parseWorkbook(data []byte")
	if start < 0 || end < 0 {
		t.Fatal("canonical legacy workbook context missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"sheets",
		"sst",
		"xfs",
		"definedNames",
		"externSheets",
		"drawingStore",
		"date1904",
		"styleCatalog",
		"protection",
		"pivotCaches",
		"metadata",
		"externalLinks",
		"sheetOffsets",
		"func (ctx legacyWorkbookParseContext) formulaContext()",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("canonical legacy workbook context missing ownership %q", required)
		}
	}
}

func TestWorksheetProjectionConsumesOnlyCanonicalGlobalContext(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseWorkbook(data []byte")
	end := strings.Index(source[start:], "// findSheetBOFOffsets")
	if start < 0 || end < 0 {
		t.Fatal("production workbook projection missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"ctx.sst",
		"ctx.xfs",
		"ctx.drawingStore",
		"ctx.styleCatalog",
		"ctx.externSheets",
		"ctx.sheetOffsets",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("worksheet projection bypasses canonical global context %q", required)
		}
	}
}

func TestProductionRawBIFFFallbackUsesCanonicalWorksheetDispatcher(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseRawBIFFWorkbookStream(")
	end := strings.Index(source, "func rawWorksheetHasContent(")
	if start < 0 || end <= start {
		t.Fatal("raw BIFF production boundary not found")
	}
	body := source[start:end]
	if strings.Contains(body, "parseWorksheet(") {
		t.Fatal("raw BIFF fallback must not route through the historical worksheet wrapper")
	}
	if !strings.Contains(body, "parseBIFFWorksheet(") {
		t.Fatal("raw BIFF fallback must use the canonical worksheet dispatcher")
	}
}

func TestXLSCompletionInventoryNamesUnsupportedWorkbookSurfaces(t *testing.T) {
	data, err := os.ReadFile("XLS_COMPLETION.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"Connections", "EmbeddedFonts", "Diagnostics", "parseBIFFWorksheet", "parseWorksheet"} {
		if !strings.Contains(text, required) {
			t.Fatalf("XLS completion inventory must name %q", required)
		}
	}
}
