package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestObjectFamilyDoesNotClaimUnrelatedContinueRecord(t *testing.T) {
	state := newLegacyWorksheetParseState()
	result, _ := state.consumeObjectRecord(nil, 17, 29, biffContinue, nil, nil, legacyFormulaWorkbookContext{})
	if result != legacyWorksheetRecordUnhandled {
		t.Fatalf("idle CONTINUE result = %v, want unhandled so another record family may consume it", result)
	}
}

func TestObjectExtractionDoesNotStealObjProtectRecord(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeSemanticRecord")
	if start < 0 {
		t.Fatal("worksheet semantic family missing")
	}
	tail := source[start:]
	end := strings.Index(tail, "func parseBIFFWorksheet")
	if end < 0 {
		t.Fatal("worksheet orchestrator missing after semantic family")
	}
	semantic := tail[:end]
	if !strings.Contains(semantic, "case biffObjProtect:") {
		t.Fatal("OBJPROTECT must remain owned by worksheet protection semantics")
	}
}
