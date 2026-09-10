package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestProductionWorkbookContextUsesSingleSemanticGlobalScan(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func buildLegacyWorkbookParseContext")
	end := strings.Index(source[start:], "func (ctx legacyWorkbookParseContext) formulaContext")
	if start < 0 || end < 0 {
		t.Fatal("canonical BIFF workbook context builder missing")
	}
	block := source[start : start+end]

	if !strings.Contains(block, "parseWorkbookGlobalsInto(data, &ctx)") {
		t.Fatal("production BIFF context must use the single workbook-global collector")
	}
	for _, forbidden := range []string{
		"parseLegacyPivotCacheSources(data)",
		"parseLegacyWorkbookMetadata(data)",
		"parseLegacyExternalLinks(data)",
	} {
		if strings.Contains(block, forbidden) {
			t.Fatalf("production context retained repeated workbook-global scan %q", forbidden)
		}
	}
	if !strings.Contains(block, "findSheetBOFOffsets(data)") {
		t.Fatal("corrupt BoundSheet recovery must remain a separate complete-stream structural scan")
	}
}

func TestSingleGlobalCollectorOwnsAuxiliaryWorkbookSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseWorkbookGlobalsInto")
	end := strings.Index(source[start:], "func parseLegacyExternalLinks")
	if start < 0 || end < 0 {
		t.Fatal("single workbook-global collector missing")
	}
	block := source[start : start+end]

	for _, required := range []string{
		"ctx.externalLinks = buildWorkbookExternalLinks(supBooks)",
		"ctx.metadata.ActiveSheetIndex",
		"ctx.metadata.WriteAccessUser",
		"ctx.metadata.BackupFile",
		"ctx.metadata.SaveExternalLinkValues",
		"ctx.metadata.Calc",
		"ctx.pivotCaches = append",
		"parseLegacyPivotDConRef",
		"parseLegacyPivotDConName",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("single BIFF global scan missing semantic ownership %q", required)
		}
	}
}

func TestFocusedLegacyGlobalHelpersRemainOracleOnly(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseWorkbook(data []byte")
	end := strings.Index(source[start:], "// findSheetBOFOffsets")
	if start < 0 || end < 0 {
		t.Fatal("production BIFF workbook projection missing")
	}
	production := source[start : start+end]

	for _, oracle := range []string{
		"parseLegacyExternalLinks(",
		"parseLegacyPivotCacheSources(",
		"parseLegacyWorkbookMetadata(",
	} {
		if strings.Contains(production, oracle) {
			t.Fatalf("focused legacy decoder %q returned to production projection", oracle)
		}
	}
}
