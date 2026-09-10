package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWorksheetOrchestratorIsDispatcherNotSemanticParser(t *testing.T) {
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
	for _, dispatch := range []string{
		"state.consumePivotRecord(", "state.consumeCellRecord(", "state.consumeGeometryRecord(",
		"state.consumeObjectRecord(", "state.consumeSemanticRecord(", "state.consumeTableRecord(",
	} {
		if !strings.Contains(tail, dispatch) {
			t.Fatalf("worksheet orchestrator missing family dispatch %q", dispatch)
		}
	}
	for _, semantic := range []string{
		"parseDVRecord(", "parseCFRecordWithPalette(", "parseObjSubRecords(", "applyRowRecord(",
		"parseLegacyFeature11Table(", "parseLegacyPivotView(",
	} {
		if strings.Contains(tail, semantic) {
			t.Fatalf("worksheet orchestrator regained semantic parsing %q", semantic)
		}
	}
}
