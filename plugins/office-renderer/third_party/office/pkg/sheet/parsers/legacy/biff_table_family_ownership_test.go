package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestTableFeatureFamilyLeavesWorksheetOrchestrator(t *testing.T) {
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
	if !strings.Contains(tail, "state.consumeTableRecord(") {
		t.Fatal("worksheet orchestrator must dispatch table family")
	}
	for _, id := range []string{"biffFeatHdr", "biffFeatHdr11", "biffFeat", "biffFeature11", "biffFeature12", "biffList12"} {
		if strings.Contains(tail, "case "+id+":") {
			t.Fatalf("table implementation leaked into worksheet orchestrator: %s", id)
		}
	}
}

func TestTableFamilyOwnsLegacySharedFeatureProjection(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeTableRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	block := source[start : start+end]
	for _, required := range []string{"buildLegacyTableColumns", "gatherFutureRecordPayload", "parseLegacyFeature11Table", "parseLegacyList12TableStyle", "parseLegacyList12DisplayName", "tableFeaturesByID"} {
		if !strings.Contains(block, required) {
			t.Fatalf("table family missing ownership %q", required)
		}
	}
}

func TestTableFamilyOwnsFutureRecordAggregateConsumption(t *testing.T) {
	if got := classifyLegacyWorksheetRecord(biffFeature11); got != legacyWorksheetRecordTable {
		t.Fatalf("FEATURE11 family = %v, want table", got)
	}
	if got := classifyLegacyWorksheetRecord(biffList12); got != legacyWorksheetRecordTable {
		t.Fatalf("LIST12 family = %v, want table", got)
	}
}
