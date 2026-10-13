package edit

import (
	"bytes"
	"testing"

	presir "office/pkg/presentation/ir"
)

func TestTableColumnWidthAndRowHeight(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 3, 0, 0, 6000, 4000)

	tbl.SetColumnWidth(1, 999999)
	if tbl.ColumnWidth(1) != 999999 {
		t.Fatalf("column width = %d", tbl.ColumnWidth(1))
	}

	row := tbl.Row(0)
	row.SetHeight(1234567)
	if row.Height() != 1234567 {
		t.Fatalf("row height = %d", row.Height())
	}
}

func TestTableAddRemoveRow(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 2, 0, 0, 4000, 2000)

	newRow := tbl.AddRow()
	if tbl.RowCount() != 3 {
		t.Fatalf("expected 3 rows, got %d", tbl.RowCount())
	}
	if newRow.Index() != 2 {
		t.Fatalf("new row index = %d", newRow.Index())
	}
	if len(newRow.Cells()) != 2 {
		t.Fatalf("expected 2 cells in new row, got %d", len(newRow.Cells()))
	}

	if !tbl.RemoveRow(0) {
		t.Fatal("expected row removal")
	}
	if tbl.RowCount() != 2 {
		t.Fatalf("expected 2 rows after remove, got %d", tbl.RowCount())
	}
	if tbl.RemoveRow(10) {
		t.Fatal("expected no removal for out-of-range")
	}
}

func TestTableAddRemoveColumn(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 2, 0, 0, 4000, 2000)

	tbl.AddColumnAt(1, 12345)
	if tbl.ColumnCount() != 3 {
		t.Fatalf("expected 3 columns, got %d", tbl.ColumnCount())
	}
	for i, row := range tbl.Rows() {
		if len(row.Cells()) != 3 {
			t.Fatalf("row %d expected 3 cells, got %d", i, len(row.Cells()))
		}
	}

	if !tbl.RemoveColumn(0) {
		t.Fatal("expected column removal")
	}
	if tbl.ColumnCount() != 2 {
		t.Fatalf("expected 2 columns, got %d", tbl.ColumnCount())
	}
}

func TestCellSpansAndMerge(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(3, 3, 0, 0, 9000, 3000)

	cell := tbl.Cell(0, 0)
	cell.SetGridSpan(2)
	cell.SetRowSpan(2)
	if cell.GridSpan() != 2 || cell.RowSpan() != 2 {
		t.Fatalf("spans = %dx%d", cell.GridSpan(), cell.RowSpan())
	}

	cell.SetHMerge(true)
	cell.SetVMerge(true)
	if !cell.HMerge() || !cell.VMerge() {
		t.Fatal("merge flags not set")
	}

	if !tbl.MergeCells(1, 1, 2, 2) {
		t.Fatal("expected merge to succeed")
	}
	anchor := tbl.Cell(1, 1)
	if anchor.GridSpan() != 2 || anchor.RowSpan() != 2 {
		t.Fatalf("merge anchor spans = %dx%d", anchor.GridSpan(), anchor.RowSpan())
	}
	if !tbl.Cell(1, 2).HMerge() || !tbl.Cell(2, 1).VMerge() {
		t.Fatal("expected continuation cells to be marked merged")
	}
}

func TestCellFillAndAnchor(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(1, 1, 0, 0, 1000, 1000)
	cell := tbl.Cell(0, 0)

	cell.SetFillSolid("FF0000")
	if cell.FillColor() != "FF0000" {
		t.Fatalf("fill color = %q", cell.FillColor())
	}

	cell.SetTextAnchor("ctr")
	if cell.TextAnchor() != "ctr" {
		t.Fatalf("anchor = %q", cell.TextAnchor())
	}
}

func TestCellMargins(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(1, 1, 0, 0, 1000, 1000)
	cell := tbl.Cell(0, 0)

	cell.SetMarginsEMU(100000)
	if cell.MarginLeftEMU() != 100000 || cell.MarginRightEMU() != 100000 ||
		cell.MarginTopEMU() != 100000 || cell.MarginBottomEMU() != 100000 {
		t.Fatal("margins not set uniformly")
	}

	cell.SetMarginLeftEMU(111)
	cell.SetMarginRightEMU(222)
	cell.SetMarginTopEMU(333)
	cell.SetMarginBottomEMU(444)
	if cell.MarginLeftEMU() != 111 || cell.MarginRightEMU() != 222 ||
		cell.MarginTopEMU() != 333 || cell.MarginBottomEMU() != 444 {
		t.Fatal("individual margins not set")
	}
}

func TestCellBorders(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(1, 1, 0, 0, 1000, 1000)
	cell := tbl.Cell(0, 0)

	cell.SetBorder("000000", 1.5)
	for _, border := range []*Border{cell.BorderLeft(), cell.BorderRight(), cell.BorderTop(), cell.BorderBottom()} {
		if border == nil {
			t.Fatal("expected border")
		}
		if border.Color != "000000" || border.WidthPt != 1.5 {
			t.Fatalf("border = %+v", border)
		}
	}

	cell.SetBorderLeft("FF0000", 2)
	if cell.BorderLeft().Color != "FF0000" {
		t.Fatalf("left border color = %q", cell.BorderLeft().Color)
	}

	cell.ClearBorders()
	if cell.BorderLeft() != nil {
		t.Fatal("expected borders cleared")
	}
}

func TestRoundTripTableCellProperties(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 2, 0, 0, 4000, 2000)
	tbl.SetColumnWidth(0, 1234567)
	tbl.Row(0).SetHeight(765432)

	cell := tbl.Cell(0, 0)
	cell.SetGridSpan(2)
	cell.SetRowSpan(1)
	cell.SetFillSolid("00FF00")
	cell.SetTextAnchor("ctr")
	cell.SetMarginsEMU(100000)
	cell.SetBorder("0000FF", 1.25)
	cell.SetText("hello")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedTbl := loaded.Slide(0).Shapes()[0].Table()
	if loadedTbl == nil {
		t.Fatal("expected table after load")
	}
	if loadedTbl.ColumnWidth(0) != 1234567 {
		t.Fatalf("column width = %d", loadedTbl.ColumnWidth(0))
	}
	if loadedTbl.Row(0).Height() != 765432 {
		t.Fatalf("row height = %d", loadedTbl.Row(0).Height())
	}

	loadedCell := loadedTbl.Cell(0, 0)
	if loadedCell.GridSpan() != 2 {
		t.Fatalf("grid span = %d", loadedCell.GridSpan())
	}
	if loadedCell.FillColor() != "00FF00" {
		t.Fatalf("fill color = %q", loadedCell.FillColor())
	}
	if loadedCell.TextAnchor() != "ctr" {
		t.Fatalf("anchor = %q", loadedCell.TextAnchor())
	}
	if loadedCell.MarginLeftEMU() != 100000 {
		t.Fatalf("margin left = %d", loadedCell.MarginLeftEMU())
	}
	if loadedCell.BorderLeft() == nil || loadedCell.BorderLeft().Color != "0000FF" || loadedCell.BorderLeft().WidthPt != 1.25 {
		t.Fatalf("border left = %+v", loadedCell.BorderLeft())
	}
	if loadedCell.Text() != "hello" {
		t.Fatalf("cell text = %q", loadedCell.Text())
	}
}

func TestRoundTripMergeCells(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(3, 3, 0, 0, 9000, 3000)
	tbl.MergeCells(0, 0, 1, 1)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	loadedTbl := loaded.Slide(0).Shapes()[0].Table()
	anchor := loadedTbl.Cell(0, 0)
	if anchor.GridSpan() != 2 || anchor.RowSpan() != 2 {
		t.Fatalf("anchor spans = %dx%d", anchor.GridSpan(), anchor.RowSpan())
	}
}

func TestTableRowAccessorBounds(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(1, 1, 0, 0, 1000, 1000)
	if tbl.Row(-1) != nil || tbl.Row(5) != nil {
		t.Fatal("expected nil for out-of-range row")
	}
	if tbl.Cell(-1, 0) != nil || tbl.Cell(0, 5) != nil {
		t.Fatal("expected nil for out-of-range cell")
	}
}

func TestTableCellKind(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(1, 1, 0, 0, 1000, 1000)
	if tbl.shape.ir.Kind != presir.ShapeTable {
		t.Fatalf("expected table shape, got %v", tbl.shape.ir.Kind)
	}
}
