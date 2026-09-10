package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestObjectCommentFamilyLeavesWorksheetOrchestrator(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "func (state *legacyWorksheetParseState) consumeObjectRecord") {
		t.Fatal("object/comment BIFF family handler missing")
	}
	start := strings.Index(source, "func parseBIFFWorksheet")
	tail := source[start:]
	end := strings.Index(tail[1:], "\nfunc ")
	if end >= 0 {
		tail = tail[:end+1]
	}
	if !strings.Contains(tail, "state.consumeObjectRecord(") {
		t.Fatal("worksheet orchestrator must dispatch the object/comment family")
	}
	for _, forbidden := range []string{
		"case biffNote:",
		"case biffObj:",
		"case biffMsoDrawing:",
		"case biffTxO:",
		"case biffContinue:",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("object/comment implementation leaked into worksheet orchestrator: %q", forbidden)
		}
	}
}

func TestObjectFamilyOwnsRecordSpanningObjectSemantics(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeObjectRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("object/comment family boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{
		"parseNoteComment",
		"parseObjSubRecords",
		"legacyObjTypeToFormControlType",
		"parseLegacyObjLinkFormula",
		"parseLegacySbsControlState",
		"parseLegacyLbsDataControlState",
		"gatherBiffPayload",
		"parseTxOCommentCharCount",
		"parseLegacyTxOControlLink",
		"parseCommentTextContinueLimited",
		"parseTxOFormattingRuns",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("object/comment family missing ownership %q", required)
		}
	}
}

func TestObjectFamilyKeepsContinueConsumptionExplicit(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func (state *legacyWorksheetParseState) consumeObjectRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	block := source[start : start+end]
	if strings.Count(block, "legacyWorksheetRecordHandledWithPosition") < 4 {
		t.Fatal("object/comment family must explicitly return stream advancement for drawing and TXO continuations")
	}
}
