package sax

import "testing"

func TestIRBuilderOwnsShapeActionAndAdvancedCellStructure(t *testing.T) {
	b := newIRBuilder()
	for _, ev := range []Event{
		{Type: EventStartPresentation},
		{Type: EventStartSlide, SlideIndex: 0},
		{Type: EventStartShape, SlideIndex: 0, ShapeID: 7, ShapeKind: "sp", ShapeName: "action"},
		{Type: EventShapeAction, SlideIndex: 0, ShapeID: 7, Action: "ppaction://hlinkPresentationJump?jump=nextSlide"},
		{Type: EventEndShape, SlideIndex: 0, ShapeID: 7},
		{Type: EventStartShape, SlideIndex: 0, ShapeID: 8, ShapeKind: "graphicFrame", ShapeName: "table"},
		{Type: EventStartTable, SlideIndex: 0, ShapeID: 8},
		{Type: EventStartTableRow, SlideIndex: 0, ShapeID: 8, TableValue: 100},
		{Type: EventStartTableCell, SlideIndex: 0, ShapeID: 8, TableGridSpan: 2, TableRowSpan: 3, TableHMerge: true},
		{Type: EventTableCellProps, SlideIndex: 0, ShapeID: 8, TableAnchor: "ctr", TableMarginLeft: 11, TableMarginRight: 12, TableMarginTop: 13, TableMarginBottom: 14},
		{Type: EventEndTableCell, SlideIndex: 0, ShapeID: 8},
		{Type: EventEndTableRow, SlideIndex: 0, ShapeID: 8},
		{Type: EventEndTable, SlideIndex: 0, ShapeID: 8},
		{Type: EventEndShape, SlideIndex: 0, ShapeID: 8},
		{Type: EventEndSlide, SlideIndex: 0},
		{Type: EventEndPresentation},
	} {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.slides) != 1 || len(b.slides[0].Shapes) != 2 {
		t.Fatalf("unexpected shape tree: %+v", b.slides)
	}
	if got := b.slides[0].Shapes[0].ActionLink; got != "nextSlide" {
		t.Fatalf("action = %q", got)
	}
	cell := b.slides[0].Shapes[1].Table.Rows[0].Cells[0]
	if cell.GridSpan != 2 || cell.RowSpan != 3 || !cell.HMerge {
		t.Fatalf("cell merge semantics lost: %+v", cell)
	}
	if cell.TextAnchor != "ctr" || cell.MarginLeftEMU != 11 || cell.MarginRightEMU != 12 || cell.MarginTopEMU != 13 || cell.MarginBottomEMU != 14 {
		t.Fatalf("cell properties lost: %+v", cell)
	}
}
