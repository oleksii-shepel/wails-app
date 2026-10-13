package edit

import (
	"office/pkg/ooxml/drawing"
	presir "office/pkg/presentation/ir"
)

// Table wraps an IR table for editing.
type Table struct {
	shape *Shape
	ir    *presir.Table
}

// RowCount returns the number of rows.
func (t *Table) RowCount() int {
	if t == nil || t.ir == nil {
		return 0
	}
	return len(t.ir.Rows)
}

// ColumnCount returns the number of columns.
func (t *Table) ColumnCount() int {
	if t == nil || t.ir == nil {
		return 0
	}
	return len(t.ir.ColumnWidthsEMU)
}

// ColumnWidth returns the width of the given column in EMU, or 0 if out of range.
func (t *Table) ColumnWidth(col int) int64 {
	if t == nil || t.ir == nil || col < 0 || col >= len(t.ir.ColumnWidthsEMU) {
		return 0
	}
	return t.ir.ColumnWidthsEMU[col]
}

// SetColumnWidth sets the width of the given column in EMU. Out-of-range
// indices are ignored.
func (t *Table) SetColumnWidth(col int, widthEMU int64) {
	if t == nil || t.ir == nil || col < 0 || col >= len(t.ir.ColumnWidthsEMU) {
		return
	}
	t.ir.ColumnWidthsEMU[col] = widthEMU
}

// Rows returns all rows in the table.
func (t *Table) Rows() []*TableRow {
	if t == nil || t.ir == nil {
		return nil
	}
	out := make([]*TableRow, len(t.ir.Rows))
	for i, r := range t.ir.Rows {
		out[i] = &TableRow{table: t, ir: r, index: i}
	}
	return out
}

// Row returns the row at the given index, or nil if out of range.
func (t *Table) Row(index int) *TableRow {
	if t == nil || t.ir == nil || index < 0 || index >= len(t.ir.Rows) {
		return nil
	}
	return &TableRow{table: t, ir: t.ir.Rows[index], index: index}
}

// Cell returns the cell at the given row and column, or nil if out of range.
func (t *Table) Cell(row, col int) *Cell {
	if t == nil || t.ir == nil || row < 0 || row >= len(t.ir.Rows) {
		return nil
	}
	r := t.ir.Rows[row]
	if r == nil || col < 0 || col >= len(r.Cells) {
		return nil
	}
	return &Cell{table: t, row: row, ir: r.Cells[col]}
}

// AddRow appends a new row at the end of the table and returns it.
func (t *Table) AddRow() *TableRow {
	if t == nil || t.ir == nil {
		return nil
	}
	return t.AddRowAt(len(t.ir.Rows))
}

// AddRowAt inserts a new row at the given index.
func (t *Table) AddRowAt(index int) *TableRow {
	if t == nil || t.ir == nil {
		return nil
	}
	if index < 0 {
		index = 0
	}
	if index > len(t.ir.Rows) {
		index = len(t.ir.Rows)
	}
	cols := len(t.ir.ColumnWidthsEMU)
	row := &presir.TableRow{
		HeightEMU: defaultRowHeight(t.ir),
		Cells:     make([]*presir.TableCell, cols),
	}
	for c := 0; c < cols; c++ {
		row.Cells[c] = &presir.TableCell{Paragraphs: []*presir.Paragraph{}}
	}
	t.ir.Rows = append(t.ir.Rows[:index], append([]*presir.TableRow{row}, t.ir.Rows[index:]...)...)
	return &TableRow{table: t, ir: row, index: index}
}

// RemoveRow removes the row at the given index. Returns true if a row was removed.
func (t *Table) RemoveRow(index int) bool {
	if t == nil || t.ir == nil || index < 0 || index >= len(t.ir.Rows) {
		return false
	}
	t.ir.Rows = append(t.ir.Rows[:index], t.ir.Rows[index+1:]...)
	return true
}

// AddColumn appends a new column to the table with the given width in EMU.
func (t *Table) AddColumn(widthEMU int64) {
	t.AddColumnAt(len(t.ir.ColumnWidthsEMU), widthEMU)
}

// AddColumnAt inserts a new column at the given index with the given width in EMU.
func (t *Table) AddColumnAt(index int, widthEMU int64) {
	if t == nil || t.ir == nil {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > len(t.ir.ColumnWidthsEMU) {
		index = len(t.ir.ColumnWidthsEMU)
	}
	t.ir.ColumnWidthsEMU = append(t.ir.ColumnWidthsEMU[:index], append([]int64{widthEMU}, t.ir.ColumnWidthsEMU[index:]...)...)
	for _, row := range t.ir.Rows {
		if row == nil {
			continue
		}
		cell := &presir.TableCell{Paragraphs: []*presir.Paragraph{}}
		row.Cells = append(row.Cells[:index], append([]*presir.TableCell{cell}, row.Cells[index:]...)...)
	}
}

// RemoveColumn removes the column at the given index. Returns true if removed.
func (t *Table) RemoveColumn(index int) bool {
	if t == nil || t.ir == nil || index < 0 || index >= len(t.ir.ColumnWidthsEMU) {
		return false
	}
	t.ir.ColumnWidthsEMU = append(t.ir.ColumnWidthsEMU[:index], t.ir.ColumnWidthsEMU[index+1:]...)
	for _, row := range t.ir.Rows {
		if row == nil || index >= len(row.Cells) {
			continue
		}
		row.Cells = append(row.Cells[:index], row.Cells[index+1:]...)
	}
	return true
}

// MergeCells merges the rectangular cell range from (firstRow, firstCol) to
// (lastRow, lastCol) inclusive. It follows POI semantics: the top-left cell
// spans the region, and other cells are marked as merged continuations.
func (t *Table) MergeCells(firstRow, firstCol, lastRow, lastCol int) bool {
	if t == nil || t.ir == nil {
		return false
	}
	if firstRow < 0 || firstCol < 0 || lastRow < firstRow || lastCol < firstCol {
		return false
	}
	if lastRow >= len(t.ir.Rows) || lastCol >= len(t.ir.ColumnWidthsEMU) {
		return false
	}
	rowSpan := lastRow - firstRow + 1
	colSpan := lastCol - firstCol + 1
	if rowSpan == 1 && colSpan == 1 {
		return true
	}

	anchor := t.ir.Rows[firstRow].Cells[firstCol]
	anchor.GridSpan = colSpan
	anchor.RowSpan = rowSpan
	for r := firstRow; r <= lastRow; r++ {
		for c := firstCol; c <= lastCol; c++ {
			if r == firstRow && c == firstCol {
				continue
			}
			cell := t.ir.Rows[r].Cells[c]
			cell.HMerge = true
			cell.VMerge = true
			cell.GridSpan = 1
			cell.RowSpan = 1
		}
	}
	return true
}

// TableRow wraps an IR table row for editing.
type TableRow struct {
	table *Table
	ir    *presir.TableRow
	index int
}

// Index returns the row's 0-based position in the table.
func (r *TableRow) Index() int {
	if r == nil {
		return -1
	}
	return r.index
}

// Height returns the row height in EMU.
func (r *TableRow) Height() int64 {
	if r == nil || r.ir == nil {
		return 0
	}
	return r.ir.HeightEMU
}

// SetHeight sets the row height in EMU.
func (r *TableRow) SetHeight(heightEMU int64) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.HeightEMU = heightEMU
}

// Cells returns all cells in the row.
func (r *TableRow) Cells() []*Cell {
	if r == nil || r.ir == nil {
		return nil
	}
	out := make([]*Cell, len(r.ir.Cells))
	for i, c := range r.ir.Cells {
		out[i] = &Cell{table: r.table, row: r.index, ir: c}
	}
	return out
}

// Cell returns the cell at the given column index, or nil if out of range.
func (r *TableRow) Cell(col int) *Cell {
	if r == nil || r.ir == nil || col < 0 || col >= len(r.ir.Cells) {
		return nil
	}
	return &Cell{table: r.table, row: r.index, ir: r.ir.Cells[col]}
}

// Cell wraps an IR table cell for editing.
type Cell struct {
	table *Table
	row   int
	ir    *presir.TableCell
}

// Paragraphs returns all paragraphs in the cell.
func (c *Cell) Paragraphs() []*Paragraph {
	if c == nil || c.ir == nil {
		return nil
	}
	out := make([]*Paragraph, len(c.ir.Paragraphs))
	for i, p := range c.ir.Paragraphs {
		out[i] = &Paragraph{cell: c, ir: p}
	}
	return out
}

// AddParagraph adds a new paragraph to the cell and returns it.
func (c *Cell) AddParagraph() *Paragraph {
	if c == nil || c.ir == nil {
		return nil
	}
	p := &presir.Paragraph{Runs: []*presir.Run{}}
	c.ir.Paragraphs = append(c.ir.Paragraphs, p)
	return &Paragraph{cell: c, ir: p}
}

// SetText replaces all cell paragraphs with a single paragraph containing one run.
func (c *Cell) SetText(text string) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.Paragraphs = []*presir.Paragraph{{
		Runs: []*presir.Run{{Text: text}},
	}}
}

// Text returns the concatenated plain text of all cell paragraphs.
func (c *Cell) Text() string {
	if c == nil || c.ir == nil {
		return ""
	}
	var out string
	for _, p := range c.ir.Paragraphs {
		for _, r := range p.Runs {
			out += r.Text
		}
	}
	return out
}

// GridSpan returns the number of columns this cell spans.
func (c *Cell) GridSpan() int {
	if c == nil || c.ir == nil {
		return 1
	}
	return c.ir.GridSpan
}

// SetGridSpan sets the number of columns this cell spans.
func (c *Cell) SetGridSpan(span int) {
	if c == nil || c.ir == nil || span < 1 {
		return
	}
	c.ir.GridSpan = span
}

// RowSpan returns the number of rows this cell spans.
func (c *Cell) RowSpan() int {
	if c == nil || c.ir == nil {
		return 1
	}
	return c.ir.RowSpan
}

// SetRowSpan sets the number of rows this cell spans.
func (c *Cell) SetRowSpan(span int) {
	if c == nil || c.ir == nil || span < 1 {
		return
	}
	c.ir.RowSpan = span
}

// HMerge reports whether this cell is a horizontal merge continuation.
func (c *Cell) HMerge() bool {
	if c == nil || c.ir == nil {
		return false
	}
	return c.ir.HMerge
}

// SetHMerge marks this cell as a horizontal merge continuation.
func (c *Cell) SetHMerge(merge bool) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.HMerge = merge
}

// VMerge reports whether this cell is a vertical merge continuation.
func (c *Cell) VMerge() bool {
	if c == nil || c.ir == nil {
		return false
	}
	return c.ir.VMerge
}

// SetVMerge marks this cell as a vertical merge continuation.
func (c *Cell) SetVMerge(merge bool) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.VMerge = merge
}

// FillColor returns the cell's solid fill color hex string, or "" for none.
func (c *Cell) FillColor() string {
	if c == nil || c.ir == nil {
		return ""
	}
	return c.ir.FillColor
}

// SetFillSolid sets the cell's solid fill color as a 6-digit hex string.
func (c *Cell) SetFillSolid(color string) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.FillColor = color
}

// TextAnchor returns the cell's vertical text anchoring ("t", "ctr", "b").
func (c *Cell) TextAnchor() string {
	if c == nil || c.ir == nil {
		return ""
	}
	return c.ir.TextAnchor
}

// SetTextAnchor sets the cell's vertical text anchoring.
func (c *Cell) SetTextAnchor(anchor string) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.TextAnchor = anchor
}

// MarginLeftEMU returns the cell's left margin in EMU.
func (c *Cell) MarginLeftEMU() int64 {
	if c == nil || c.ir == nil {
		return 0
	}
	return c.ir.MarginLeftEMU
}

// SetMarginLeftEMU sets the cell's left margin in EMU.
func (c *Cell) SetMarginLeftEMU(emu int64) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.MarginLeftEMU = emu
}

// MarginRightEMU returns the cell's right margin in EMU.
func (c *Cell) MarginRightEMU() int64 {
	if c == nil || c.ir == nil {
		return 0
	}
	return c.ir.MarginRightEMU
}

// SetMarginRightEMU sets the cell's right margin in EMU.
func (c *Cell) SetMarginRightEMU(emu int64) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.MarginRightEMU = emu
}

// MarginTopEMU returns the cell's top margin in EMU.
func (c *Cell) MarginTopEMU() int64 {
	if c == nil || c.ir == nil {
		return 0
	}
	return c.ir.MarginTopEMU
}

// SetMarginTopEMU sets the cell's top margin in EMU.
func (c *Cell) SetMarginTopEMU(emu int64) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.MarginTopEMU = emu
}

// MarginBottomEMU returns the cell's bottom margin in EMU.
func (c *Cell) MarginBottomEMU() int64 {
	if c == nil || c.ir == nil {
		return 0
	}
	return c.ir.MarginBottomEMU
}

// SetMarginBottomEMU sets the cell's bottom margin in EMU.
func (c *Cell) SetMarginBottomEMU(emu int64) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.MarginBottomEMU = emu
}

// SetMarginsEMU sets all cell margins to the same value in EMU.
func (c *Cell) SetMarginsEMU(emu int64) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.MarginLeftEMU = emu
	c.ir.MarginRightEMU = emu
	c.ir.MarginTopEMU = emu
	c.ir.MarginBottomEMU = emu
}

// Border describes a table cell border in a stable, public form.
type Border struct {
	Color   string
	WidthPt float64
}

// BorderLeft returns the cell's left border, or nil if none.
func (c *Cell) BorderLeft() *Border {
	return cellBorder(c.ir.BorderLeft)
}

// BorderRight returns the cell's right border, or nil if none.
func (c *Cell) BorderRight() *Border {
	return cellBorder(c.ir.BorderRight)
}

// BorderTop returns the cell's top border, or nil if none.
func (c *Cell) BorderTop() *Border {
	return cellBorder(c.ir.BorderTop)
}

// BorderBottom returns the cell's bottom border, or nil if none.
func (c *Cell) BorderBottom() *Border {
	return cellBorder(c.ir.BorderBottom)
}

// SetBorderLeft sets the cell's left border color (hex) and width in points.
func (c *Cell) SetBorderLeft(color string, widthPt float64) {
	c.setBorder(&c.ir.BorderLeft, color, widthPt)
}

// SetBorderRight sets the cell's right border color and width in points.
func (c *Cell) SetBorderRight(color string, widthPt float64) {
	c.setBorder(&c.ir.BorderRight, color, widthPt)
}

// SetBorderTop sets the cell's top border color and width in points.
func (c *Cell) SetBorderTop(color string, widthPt float64) {
	c.setBorder(&c.ir.BorderTop, color, widthPt)
}

// SetBorderBottom sets the cell's bottom border color and width in points.
func (c *Cell) SetBorderBottom(color string, widthPt float64) {
	c.setBorder(&c.ir.BorderBottom, color, widthPt)
}

// SetBorder sets all four cell borders to the same color and width.
func (c *Cell) SetBorder(color string, widthPt float64) {
	c.SetBorderLeft(color, widthPt)
	c.SetBorderRight(color, widthPt)
	c.SetBorderTop(color, widthPt)
	c.SetBorderBottom(color, widthPt)
}

// ClearBorders removes all cell borders.
func (c *Cell) ClearBorders() {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.BorderLeft = nil
	c.ir.BorderRight = nil
	c.ir.BorderTop = nil
	c.ir.BorderBottom = nil
}

func (c *Cell) setBorder(field **drawing.LineStyle, color string, widthPt float64) {
	if c == nil || c.ir == nil {
		return
	}
	if color == "" && widthPt <= 0 {
		*field = nil
		return
	}
	*field = &drawing.LineStyle{
		Color: color,
		Width: widthPt,
	}
}

func cellBorder(ls *drawing.LineStyle) *Border {
	if ls == nil {
		return nil
	}
	return &Border{
		Color:   ls.Color,
		WidthPt: ls.Width,
	}
}

func defaultRowHeight(table *presir.Table) int64 {
	if table == nil || len(table.Rows) == 0 {
		return 370000 // ~0.25 inch default
	}
	return table.Rows[0].HeightEMU
}
