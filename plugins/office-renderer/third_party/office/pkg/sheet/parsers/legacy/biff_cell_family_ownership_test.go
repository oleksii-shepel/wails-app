package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCellFormulaFamilyLeavesWorksheetOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "func (state *legacyWorksheetParseState) consumeCellRecord") {
		t.Fatal("cell/formula BIFF family handler missing")
	}
	start := strings.Index(source, "func parseBIFFWorksheet")
	tail := source[start:]
	end := strings.Index(tail[1:], "\nfunc ")
	if end >= 0 {
		tail = tail[:end+1]
	}
	if !strings.Contains(tail, "state.consumeCellRecord(") {
		t.Fatal("worksheet orchestrator must dispatch the cell/formula family")
	}
	for _, forbidden := range []string{
		"case biffLabelSst:",
		"case biffNumber,",
		"case biffRK:",
		"case biffFormula,",
		"case biffStringOld,",
		"case biffArray:",
		"case biffShrFmla:",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("cell/formula implementation leaked into worksheet orchestrator: %q", forbidden)
		}
	}
}

func TestCellFormulaFamilyOwnsFormulaContinuationSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeCellRecord")
	end := strings.Index(source[start:], "func (state *legacyWorksheetParseState) clearLastFormula")
	if start < 0 || end < 0 {
		t.Fatal("cell/formula family handler boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"parseFormulaCell",
		"parseOldFormulaCell",
		"parseFormulaExpRef",
		"gatherBiffPayload",
		"parseArrayFormulaRecord",
		"parseSharedFormulaRecord",
		"canonicalizeBIFFNumericCell",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("cell/formula family missing ownership %q", required)
		}
	}
}

func TestCellFamilyCanAdvanceSequentialStreamForContinuedString(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "legacyWorksheetRecordHandledWithPosition") ||
		!strings.Contains(source, "return legacyWorksheetRecordHandledWithPosition, fullNext") {
		t.Fatal("cell family must explicitly report consumed CONTINUE records to the sequential orchestrator")
	}
}
