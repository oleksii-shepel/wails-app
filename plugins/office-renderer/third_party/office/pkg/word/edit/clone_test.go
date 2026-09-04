package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestRunClone(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("hello")
	r.SetBold(true)
	r.SetItalic(true)
	r.SetFontSize(24)
	r.SetUnderline("single")
	r.SetColor("FF0000")
	r.SetHighlight("yellow")
	r.SetHyperlink("https://example.com")

	clone := r.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.Text() != "hello" {
		t.Fatalf("text = %q, want hello", clone.Text())
	}
	if clone.ir().Format.Bold == nil || !*clone.ir().Format.Bold {
		t.Fatal("bold not cloned")
	}
	if clone.ir().Format.Italic == nil || !*clone.ir().Format.Italic {
		t.Fatal("italic not cloned")
	}
	if clone.ir().Format.FontSize == nil || *clone.ir().Format.FontSize != 24 {
		t.Fatal("font size not cloned")
	}
	if clone.ir().Format.Underline != "single" {
		t.Fatal("underline not cloned")
	}
	if clone.ir().Format.Color != "FF0000" {
		t.Fatal("color not cloned")
	}
	if clone.ir().Format.Highlight != "yellow" {
		t.Fatal("highlight not cloned")
	}
	if clone.ir().Target != "https://example.com" {
		t.Fatal("hyperlink not cloned")
	}

	// Mutation of clone should not affect original.
	clone.SetText("world")
	if r.Text() != "hello" {
		t.Fatal("original run mutated by clone")
	}
}

func TestRunCloneImage(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.AddImage([]byte("data"), "image/png", 100, 200)

	clone := r.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.ir().Image == nil {
		t.Fatal("image not cloned")
	}
	if string(clone.ir().Image.Data) != "data" {
		t.Fatal("image data not cloned")
	}

	// Deep copy: mutate clone image data.
	clone.ir().Image.Data[0] = 'X'
	if r.ir().Image.Data[0] != 'd' {
		t.Fatal("original image data mutated by clone")
	}
}

func TestRunCloneField(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetPageNumber()

	clone := r.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.FieldInstr() != "PAGE" {
		t.Fatalf("field instr = %q", clone.FieldInstr())
	}
}

func TestParagraphClone(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetText("hello world")
	p.SetAlignment("center")
	p.SetSpacingBefore(120)

	clone := p.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.Text() != "hello world" {
		t.Fatalf("text = %q", clone.Text())
	}
	if clone.ir.Format.Alignment != "center" {
		t.Fatal("alignment not cloned")
	}
	if clone.ir.Format.SpacingBefore == nil || *clone.ir.Format.SpacingBefore != 120 {
		t.Fatal("spacing before not cloned")
	}

	// Detached clone runs should work.
	runs := clone.Runs()
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	runs[0].SetText("mutated")
	if p.Text() != "hello world" {
		t.Fatal("original paragraph mutated by clone")
	}
}

func TestParagraphCloneWithNumbering(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetText("item")
	p.ir.Numbering = &wordir.Numbering{NumID: 1, Level: 2}

	clone := p.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.ir.Numbering == nil {
		t.Fatal("numbering not cloned")
	}
	if clone.ir.Numbering.NumID != 1 || clone.ir.Numbering.Level != 2 {
		t.Fatal("numbering values not cloned")
	}
}

func TestTableClone(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetText("A1")
	cell.SetWidth(1000)

	clone := tbl.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}

	rows := clone.Rows()
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	cells := rows[0].Cells()
	if len(cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(cells))
	}
	if cells[0].ir().Paragraphs[0].Runs[0].Text != "A1" {
		t.Fatal("cell text not cloned")
	}
	if cells[0].ir().Format.Width == nil || *cells[0].ir().Format.Width != 1000 {
		t.Fatal("cell width not cloned")
	}

	// Mutation of clone should not affect original.
	cells[0].SetText("mutated")
	if cell.Text() != "A1" {
		t.Fatal("original cell mutated by clone")
	}
}

func TestTableCloneNestedTable(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	// Use raw IR to add nested table since TableCell has no AddTable wrapper.
	inner := wordir.Table{}
	innerRow := wordir.TableRow{}
	innerCell := wordir.TableCell{}
	innerCell.Paragraphs = append(innerCell.Paragraphs, wordir.Paragraph{
		Runs: []wordir.Run{{Text: "nested"}},
	})
	innerRow.Cells = append(innerRow.Cells, innerCell)
	inner.Rows = append(inner.Rows, innerRow)
	cell.ir().Tables = append(cell.ir().Tables, inner)

	clone := tbl.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	outerCell := clone.Rows()[0].Cells()[0]
	if len(outerCell.ir().Tables) != 1 {
		t.Fatalf("nested tables = %d, want 1", len(outerCell.ir().Tables))
	}
	if outerCell.ir().Tables[0].Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "nested" {
		t.Fatal("nested table text not cloned")
	}
}

func TestTableRowClone(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetText("cell1")
	row.SetHeight(500)

	clone := row.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	cells := clone.Cells()
	if len(cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(cells))
	}
	if cells[0].Text() != "cell1" {
		t.Fatal("cell text not cloned")
	}
	if clone.ir().Format.Height == nil || *clone.ir().Format.Height != 500 {
		t.Fatal("row height not cloned")
	}

	cells[0].SetText("mutated")
	if cell.Text() != "cell1" {
		t.Fatal("original cell mutated by clone")
	}
}

func TestTableCellClone(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetText("hello")
	cell.SetWidth(2000)
	cell.SetGridSpan(2)

	clone := cell.Clone()
	if clone == nil {
		t.Fatal("nil clone")
	}
	if clone.Text() != "hello" {
		t.Fatalf("text = %q", clone.Text())
	}
	if clone.ir().Format.Width == nil || *clone.ir().Format.Width != 2000 {
		t.Fatal("width not cloned")
	}
	if clone.ir().Format.GridSpan != 2 {
		t.Fatal("grid span not cloned")
	}

	clone.SetText("mutated")
	if cell.Text() != "hello" {
		t.Fatal("original cell mutated by clone")
	}
}

func TestCloneNilSafety(t *testing.T) {
	var r *Run
	if r.Clone() != nil {
		t.Fatal("nil run clone should return nil")
	}
	var p *Paragraph
	if p.Clone() != nil {
		t.Fatal("nil paragraph clone should return nil")
	}
	var tbl *Table
	if tbl.Clone() != nil {
		t.Fatal("nil table clone should return nil")
	}
	var row *TableRow
	if row.Clone() != nil {
		t.Fatal("nil row clone should return nil")
	}
	var cell *TableCell
	if cell.Clone() != nil {
		t.Fatal("nil cell clone should return nil")
	}
}