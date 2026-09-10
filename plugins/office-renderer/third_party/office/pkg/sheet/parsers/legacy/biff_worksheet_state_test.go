package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWorksheetParserCentralizesCrossRecordState(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "type legacyWorksheetParseState struct")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("canonical BIFF worksheet parse state missing")
	}
	block := source[start : start+end]

	for _, required := range []string{
		"pendingFormula",
		"pendingCommentText",
		"pendingControlText",
		"pendingDrawingTextObjID",
		"pendingNotesByObjID",
		"controlByObjID",
		"freezePanes",
		"pendingCFSqRef",
		"sheetProtected",
		"arrayFormulas",
		"sharedFormulas",
		"officeArtPayload",
		"pivotViews",
		"tableFeaturesByID",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("worksheet parse state missing cross-record ownership %q", required)
		}
	}
}

func TestWorksheetParserHasOneTransientStateOwner(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseBIFFWorksheet")
	if start < 0 {
		t.Fatal("worksheet parser missing")
	}
	tail := source[start:]
	end := strings.Index(tail[1:], "\nfunc ")
	if end >= 0 {
		tail = tail[:end+1]
	}

	if !strings.Contains(tail, "state := newLegacyWorksheetParseState()") {
		t.Fatal("worksheet parser must create one explicit transient state owner")
	}
	for _, forbidden := range []string{
		"var pendingFormula ",
		"pendingCFSqRef :=",
		"sharedFormulas :=",
		"officeArtPayload :=",
		"var currentPivotView ",
		"tableFeaturesByID :=",
	} {
		if strings.Contains(tail, forbidden) {
			t.Fatalf("worksheet parser retained loose cross-record state %q", forbidden)
		}
	}
}

func TestCrossRecordFlushesBelongToWorksheetState(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, method := range []string{
		"func (state *legacyWorksheetParseState) flushPendingCommentText",
		"func (state *legacyWorksheetParseState) flushPendingControlText",
		"func (state *legacyWorksheetParseState) flushPendingDrawingText",
		"func (state *legacyWorksheetParseState) flushPendingFormulaText",
		"func (state *legacyWorksheetParseState) flushPendingPivotView",
	} {
		if !strings.Contains(source, method) {
			t.Fatalf("cross-record finalization must belong to worksheet state: %q", method)
		}
	}
}
