package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWorksheetRecordsHaveSemanticFamilyBoundary(t *testing.T) {
	cases := []struct {
		id   uint16
		want legacyWorksheetRecordFamily
	}{
		{biffFormula, legacyWorksheetRecordCell},
		{biffRow, legacyWorksheetRecordGeometry},
		{biffNote, legacyWorksheetRecordObject},
		{biffCF, legacyWorksheetRecordSemantic},
		{biffSXView, legacyWorksheetRecordPivot},
	}
	for _, tc := range cases {
		if got := classifyLegacyWorksheetRecord(tc.id); got != tc.want {
			t.Fatalf("record %#04x family = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestPivotFamilyLeavesWorksheetOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "func (state *legacyWorksheetParseState) consumePivotRecord") {
		t.Fatal("pivot BIFF family handler missing")
	}
	start := strings.Index(source, "func parseBIFFWorksheet")
	if start < 0 {
		t.Fatal("worksheet orchestrator missing")
	}
	tail := source[start:]
	end := strings.Index(tail[1:], "\nfunc ")
	if end >= 0 {
		tail = tail[:end+1]
	}
	if !strings.Contains(tail, "classifyLegacyWorksheetRecord(id)") ||
		!strings.Contains(tail, "state.consumePivotRecord(id, payload)") {
		t.Fatal("worksheet orchestrator must dispatch pivot records through the family boundary")
	}
	for _, forbidden := range []string{
		"case biffSXView:",
		"case biffSXVD:",
		"case biffSXVI:",
		"case biffSXIvd:",
		"case biffSXPI:",
		"case biffSXDI:",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("pivot record implementation leaked back into worksheet orchestrator: %q", forbidden)
		}
	}
}

func TestWorksheetFamilyClassifierGroupsCrossRecordSemantics(t *testing.T) {
	for _, id := range []uint16{biffFormula, biffString, biffArray, biffShrFmla} {
		if classifyLegacyWorksheetRecord(id) != legacyWorksheetRecordCell {
			t.Fatalf("formula record %#04x must belong to cell/formula family", id)
		}
	}
	for _, id := range []uint16{biffNote, biffObj, biffTxO, biffContinue, biffMsoDrawing} {
		if classifyLegacyWorksheetRecord(id) != legacyWorksheetRecordObject {
			t.Fatalf("object continuation record %#04x must belong to object family", id)
		}
	}
}
