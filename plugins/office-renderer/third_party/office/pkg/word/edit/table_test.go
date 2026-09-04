package edit

import "testing"

func TestTableCellGridSpanAndVMerge(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	c1 := row.AddCell()
	c2 := row.AddCell()

	if c1.GridSpan() != 1 {
		t.Fatalf("GridSpan = %d, want 1", c1.GridSpan())
	}
	if c1.VMerge() != "" {
		t.Fatalf("VMerge = %q, want empty", c1.VMerge())
	}
	if c1.IsMerged() {
		t.Fatal("expected not merged")
	}

	c1.SetGridSpan(3)
	if c1.GridSpan() != 3 {
		t.Fatalf("GridSpan = %d, want 3", c1.GridSpan())
	}
	if !c1.IsMerged() {
		t.Fatal("expected merged")
	}

	c2.SetVMerge("restart")
	if c2.VMerge() != "restart" {
		t.Fatalf("VMerge = %q, want restart", c2.VMerge())
	}
	if !c2.IsMerged() {
		t.Fatal("expected merged")
	}
}

func TestTableCellSplitHorizontally(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	for i := 0; i < 4; i++ {
		row.AddCell()
	}
	row.MergeCells(0, 2) // span 3 cells
	if len(row.Cells()) != 2 {
		t.Fatalf("cells = %d, want 2", len(row.Cells()))
	}
	if row.Cells()[0].GridSpan() != 3 {
		t.Fatalf("gridSpan = %d, want 3", row.Cells()[0].GridSpan())
	}

	row.Cells()[0].SplitHorizontally()
	if len(row.Cells()) != 4 {
		t.Fatalf("cells after split = %d, want 4", len(row.Cells()))
	}
	for i, c := range row.Cells() {
		if c.GridSpan() != 1 {
			t.Fatalf("cell[%d] gridSpan = %d, want 1", i, c.GridSpan())
		}
	}
}

func TestTableCellSplitHorizontallyNoOp(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	c := row.AddCell()
	c.SetText("x")
	c.SplitHorizontally() // no-op because GridSpan == 1
	if len(row.Cells()) != 1 {
		t.Fatalf("cells = %d, want 1", len(row.Cells()))
	}
}

func TestTableCellSplitVertically(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	for i := 0; i < 3; i++ {
		r := tbl.AddRow()
		for j := 0; j < 2; j++ {
			r.AddCell()
		}
	}
	tbl.MergeCellsVertically(0, 0, 2)
	for r := 0; r < 3; r++ {
		cell := tbl.Rows()[r].Cells()[0]
		if r == 0 && cell.VMerge() != "restart" {
			t.Fatalf("row %d vMerge = %q, want restart", r, cell.VMerge())
		}
		if r > 0 && cell.VMerge() != "continue" {
			t.Fatalf("row %d vMerge = %q, want continue", r, cell.VMerge())
		}
	}

	// Split from the restart cell.
	tbl.Rows()[0].Cells()[0].SplitVertically()
	for r := 0; r < 3; r++ {
		cell := tbl.Rows()[r].Cells()[0]
		if cell.VMerge() != "" {
			t.Fatalf("row %d vMerge = %q, want empty after split", r, cell.VMerge())
		}
	}
}

func TestTableCellSplitVerticallyFromContinue(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	for i := 0; i < 3; i++ {
		r := tbl.AddRow()
		for j := 0; j < 2; j++ {
			r.AddCell()
		}
	}
	tbl.MergeCellsVertically(0, 0, 2)
	// Split from a continuation cell.
	tbl.Rows()[1].Cells()[0].SplitVertically()
	for r := 0; r < 3; r++ {
		cell := tbl.Rows()[r].Cells()[0]
		if cell.VMerge() != "" {
			t.Fatalf("row %d vMerge = %q, want empty after split", r, cell.VMerge())
		}
	}
}

func TestTableMergeCells2D(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	for i := 0; i < 3; i++ {
		r := tbl.AddRow()
		for j := 0; j < 4; j++ {
			c := r.AddCell()
			c.SetText(string('a' + byte(i*4+j)))
		}
	}
	// Merge a 2x3 block: rows 0-1, cols 1-3
	tbl.MergeCells(0, 1, 1, 3)

	// Row 0 should have 2 cells: col0 (unmerged), col1-3 (merged, span=3)
	row0 := tbl.Rows()[0]
	if len(row0.Cells()) != 2 {
		t.Fatalf("row0 cells = %d, want 2", len(row0.Cells()))
	}
	if row0.Cells()[0].GridSpan() != 1 {
		t.Fatalf("row0 cell0 gridSpan = %d, want 1", row0.Cells()[0].GridSpan())
	}
	if row0.Cells()[1].GridSpan() != 3 {
		t.Fatalf("row0 cell1 gridSpan = %d, want 3", row0.Cells()[1].GridSpan())
	}
	if row0.Cells()[1].VMerge() != "restart" {
		t.Fatalf("row0 cell1 vMerge = %q, want restart", row0.Cells()[1].VMerge())
	}

	// Row 1 should have 2 cells: col0 (unmerged), col1-3 (merged, span=3, continue)
	row1 := tbl.Rows()[1]
	if len(row1.Cells()) != 2 {
		t.Fatalf("row1 cells = %d, want 2", len(row1.Cells()))
	}
	if row1.Cells()[1].GridSpan() != 3 {
		t.Fatalf("row1 cell1 gridSpan = %d, want 3", row1.Cells()[1].GridSpan())
	}
	if row1.Cells()[1].VMerge() != "continue" {
		t.Fatalf("row1 cell1 vMerge = %q, want continue", row1.Cells()[1].VMerge())
	}

	// Row 2 should be unchanged (4 cells)
	row2 := tbl.Rows()[2]
	if len(row2.Cells()) != 4 {
		t.Fatalf("row2 cells = %d, want 4", len(row2.Cells()))
	}
}

func TestTableMergeCells2DVerticalOnly(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	for i := 0; i < 3; i++ {
		r := tbl.AddRow()
		for j := 0; j < 2; j++ {
			r.AddCell()
		}
	}
	// Merge a single column vertically: rows 0-2, col 0
	tbl.MergeCells(0, 0, 2, 0)
	for r := 0; r < 3; r++ {
		cell := tbl.Rows()[r].Cells()[0]
		if r == 0 {
			if cell.VMerge() != "restart" {
				t.Fatalf("row %d vMerge = %q, want restart", r, cell.VMerge())
			}
		} else {
			if cell.VMerge() != "continue" {
				t.Fatalf("row %d vMerge = %q, want continue", r, cell.VMerge())
			}
		}
		if cell.GridSpan() != 1 {
			t.Fatalf("row %d gridSpan = %d, want 1", r, cell.GridSpan())
		}
	}
}

func TestTableMergeCells2DHorizontalOnly(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	r := tbl.AddRow()
	for j := 0; j < 4; j++ {
		r.AddCell()
	}
	// Merge a single row horizontally: row 0, cols 0-2
	tbl.MergeCells(0, 0, 0, 2)
	if len(r.Cells()) != 2 {
		t.Fatalf("cells = %d, want 2", len(r.Cells()))
	}
	if r.Cells()[0].GridSpan() != 3 {
		t.Fatalf("cell0 gridSpan = %d, want 3", r.Cells()[0].GridSpan())
	}
	if r.Cells()[0].VMerge() != "" {
		t.Fatalf("cell0 vMerge = %q, want empty", r.Cells()[0].VMerge())
	}
}

func TestTableMergeCellsBounds(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	r := tbl.AddRow()
	r.AddCell()
	// Out of bounds should be no-op.
	tbl.MergeCells(-1, 0, 0, 0)
	tbl.MergeCells(0, 0, 5, 0)
	tbl.MergeCells(0, 0, 0, 5)
	if len(r.Cells()) != 1 {
		t.Fatalf("cells = %d, want 1", len(r.Cells()))
	}
}