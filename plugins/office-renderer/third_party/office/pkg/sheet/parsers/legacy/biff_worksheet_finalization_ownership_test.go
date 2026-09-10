package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWorksheetFinalizationLeavesSequentialOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseBIFFWorksheet")
	if start < 0 {
		t.Fatal("worksheet orchestrator missing")
	}
	tail := source[start:]
	if end := strings.Index(tail[1:], "\nfunc "); end >= 0 {
		tail = tail[:end+1]
	}

	if !strings.Contains(tail, "return state.finalizeWorksheet(sheet, rows, drawingStore, formulaCtx)") {
		t.Fatal("worksheet orchestrator must delegate canonical finalization")
	}
	for _, forbidden := range []string{
		"applyArrayFormulaMetadata(", "applySharedFormulaMetadata(",
		"parseLegacyWorksheetImageDrawings(", "parseLegacyWorksheetNonPictureShapes(",
		"sheet.FormControls = append(", "sheet.PivotTables = append(",
		"sheet.AutoFilterRef =", "sheet.Protection =", "sheet.Hyperlinks = append(",
		"sheet.OutlineRows[", "sheet.ColumnWidths = nil",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("worksheet orchestrator retained finalization ownership %q", forbidden)
		}
	}
}

func TestWorksheetFinalizerOwnsCanonicalAssembly(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) finalizeWorksheet")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("worksheet finalizer boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"flushPendingCommentText(", "flushPendingControlText(", "flushPendingDrawingText(",
		"flushPendingPivotView(", "applyArrayFormulaMetadata(", "applyExpFormulaMetadata(",
		"applySharedFormulaMetadata(", "sortedSheetRows(", "parseLegacyWorksheetImageDrawings(",
		"parseLegacyWorksheetNonPictureShapes(", "sheet.FormControls = append(",
		"sheet.PivotTables = append(", "sheet.AutoFilterRef =", "sheet.Protection =",
		"sheet.Hyperlinks = append(", "sheet.OutlineRows[", "sheet.ColumnWidths = nil",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("worksheet finalizer missing canonical assembly ownership %q", required)
		}
	}
}
