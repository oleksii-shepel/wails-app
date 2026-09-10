package legacy

import (
	"office/pkg/sheet/ir"
	"os"
	"strings"
	"testing"
)

func TestWorksheetSemanticFamilyLeavesOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseBIFFWorksheet")
	tail := source[start:]
	if end := strings.Index(tail[1:], "\nfunc "); end >= 0 {
		tail = tail[:end+1]
	}
	if !strings.Contains(tail, "state.consumeSemanticRecord(") {
		t.Fatal("worksheet orchestrator must dispatch semantic record family")
	}
	for _, id := range []string{
		"biffHLink", "biffCondFmt", "biffCFHeader12", "biffCF", "biffCFRule12",
		"biffDval", "biffDV", "biffAutoFilterInfo", "biffAutoFilter",
		"biffProtect", "biffObjProtect", "biffScenProtect", "biffPassword",
		"biffSetup", "biffLeftMargin", "biffRightMargin", "biffTopMargin",
		"biffBottomMargin", "biffHCenter", "biffVCenter", "biffPrintHeaders",
		"biffPrintGridlines", "biffHeader", "biffFooter", "biffHeaderFooterExt",
		"biffHorizPageBreak", "biffVertPageBreak",
	} {
		if strings.Contains(tail, "case "+id+":") {
			t.Fatalf("semantic record implementation leaked into worksheet orchestrator: %s", id)
		}
	}
}

func TestSemanticFamilyOwnsInteractiveProtectionAndPrintSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeSemanticRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("semantic family boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"parseHLINKRecord", "parseCONDFMTSqRef", "parseCONDFMT12SqRef",
		"parseCFRecordWithPalette", "parseCF12Record", "parseDVRecord",
		"parseAutoFilterRecord", "sheetObjProtected", "sheetScenProtected",
		"applySetupRecord", "applyMarginRecord", "applyPrintFlagRecord",
		"applyHeaderFooterRecord", "applyHeaderFooterExtRecord", "parsePageBreakRecord",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("semantic family missing ownership %q", required)
		}
	}
}

func TestObjProtectBelongsToSemanticProtectionFamily(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeSemanticRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	block := source[start : start+end]
	if !strings.Contains(block, "case biffObjProtect:") {
		t.Fatal("OBJPROTECT must be owned by worksheet semantic/protection family")
	}
}

func TestFilterModeIsClassifiedAsWorksheetSemanticRecord(t *testing.T) {
	if got := classifyLegacyWorksheetRecord(biffFilterMode); got != legacyWorksheetRecordSemantic {
		t.Fatalf("FILTERMODE family = %v, want worksheet semantic", got)
	}
}

func TestWSBoolIsClassifiedAsWorksheetSemanticRecord(t *testing.T) {
	if got := classifyLegacyWorksheetRecord(biffWSBool); got != legacyWorksheetRecordSemantic {
		t.Fatalf("WSBOOL family = %v, want worksheet semantic", got)
	}
}

func TestSetupRetainsLegacyFitToPageFallback(t *testing.T) {
	sheet := &ir.Sheet{}
	payload := make([]byte, 12)
	applySetupRecord(sheet, payload)
	if sheet.PageSetup == nil || !sheet.PageSetup.FitToPage {
		t.Fatal("SETUP scale=0 must retain legacy fit-to-page fallback")
	}
}
