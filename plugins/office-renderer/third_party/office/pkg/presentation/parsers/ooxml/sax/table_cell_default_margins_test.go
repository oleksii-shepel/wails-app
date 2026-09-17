package sax

import "testing"

func TestIRBuilderUsesDrawingMLTableCellDefaultInsetsAndPreservesAuthoredZero(t *testing.T) {
	b := newIRBuilder()
	feed := func(ev Event) {
		t.Helper()
		if err := b.HandleEvent(ev); err != nil {
			t.Fatalf("HandleEvent(%v): %v", ev.Type, err)
		}
	}
	feed(Event{Type: EventStartSlide, SlideIndex: 0})
	feed(Event{Type: EventStartShape, ShapeID: 1})
	feed(Event{Type: EventStartTable})
	feed(Event{Type: EventStartTableRow, TableValue: 685800})
	feed(Event{Type: EventStartTableCell, TableGridSpan: 1})
	cell := b.tableCell
	if cell == nil {
		t.Fatal("table cell was not materialized")
	}
	if cell.MarginLeftEMU != 91440 || cell.MarginRightEMU != 91440 || cell.MarginTopEMU != 45720 || cell.MarginBottomEMU != 45720 {
		t.Fatalf("default margins = L%d R%d T%d B%d", cell.MarginLeftEMU, cell.MarginRightEMU, cell.MarginTopEMU, cell.MarginBottomEMU)
	}
	feed(Event{Type: EventTableCellProps, TableMarginLeftSet: true, TableMarginLeft: 0})
	if cell.MarginLeftEMU != 0 {
		t.Fatalf("authored marL=0 became %d", cell.MarginLeftEMU)
	}
	if cell.MarginRightEMU != 91440 {
		t.Fatalf("omitted marR lost default: %d", cell.MarginRightEMU)
	}
}
