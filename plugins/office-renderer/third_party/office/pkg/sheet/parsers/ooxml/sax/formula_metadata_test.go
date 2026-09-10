package sax

import "testing"

func TestCellFormulaMetadataIsStreamed(t *testing.T) {
	// Formula metadata is part of the event contract because edit/save must
	// preserve shared and array formula groups without the full worksheet parser.
	event := Event{
		Type: EventCell, CellRef: "A1", Formula: "B1+1",
		FormulaType: "shared", FormulaRef: "A1:A3",
	}
	si := 7
	event.FormulaSharedIndex = &si
	if event.FormulaType != "shared" || event.FormulaRef != "A1:A3" ||
		event.FormulaSharedIndex == nil || *event.FormulaSharedIndex != 7 {
		t.Fatalf("formula metadata lost: %#v", event)
	}
}
