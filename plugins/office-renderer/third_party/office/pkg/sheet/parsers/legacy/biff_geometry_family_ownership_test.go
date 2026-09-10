package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestGeometryViewFamilyLeavesWorksheetOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "func (state *legacyWorksheetParseState) consumeGeometryRecord") {
		t.Fatal("geometry/view BIFF family handler missing")
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
	if !strings.Contains(tail, "state.consumeGeometryRecord(") {
		t.Fatal("worksheet orchestrator must dispatch the geometry/view family")
	}
	for _, forbidden := range []string{
		"case biffWindow2:",
		"case biffPane:",
		"case biffDimensions:",
		"case biffRow:",
		"case biffColInfo:",
		"case biffMergeCells:",
		"case biffDefRowHeight:",
		"case biffDefColWidth:",
		"case biffSelection:",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("geometry/view implementation leaked into worksheet orchestrator: %q", forbidden)
		}
	}
}

func TestGeometryFamilyOwnsViewAndPhysicalGridSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeGeometryRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("geometry/view family boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"applyPane",
		"applyDimensionsRecord",
		"applyGutsRecord",
		"applySCLRecord",
		"applyRowRecord",
		"applyColInfo",
		"gatherContiguousMergeCells",
		"skipCustomViewSettingsRecordAggregate",
		"applyDefaultRowHeight",
		"applyDefaultColWidth",
		"parseSelectionRecord",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("geometry/view family missing ownership %q", required)
		}
	}
}

func TestGeometryFamilyReportsAggregateStreamConsumption(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		"return legacyWorksheetRecordHandledWithPosition, nextPos",
		"return legacyWorksheetRecordHandledWithPosition, skipCustomViewSettingsRecordAggregate(data, pos)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("geometry family must return consumed aggregate position: %q", required)
		}
	}
}
