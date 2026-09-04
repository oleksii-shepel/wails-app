package edit

import (
	wordir "office/pkg/word/ir"
)

// Table wraps an IR table for editing.
type Table struct {
	ir  *wordir.Table
	doc *Document
}

// ElementType returns the body element type for this table.
func (t *Table) ElementType() string { return "table" }

// ContentControl returns the block-level content control wrapping this table.
func (t *Table) ContentControl() *ContentControl {
	if t == nil || t.ir == nil || t.doc == nil {
		return nil
	}
	cc := blockContentControlForTableInBlocks(t.doc.IR.Blocks, t.ir)
	if cc == nil {
		cc = blockContentControlForTableInStories(t.doc.headerPtrs, t.ir)
	}
	if cc == nil {
		cc = blockContentControlForTableInStories(t.doc.footerPtrs, t.ir)
	}
	if cc == nil {
		cc = blockContentControlForTableInStories(t.doc.footnotePtrs, t.ir)
	}
	if cc == nil {
		cc = blockContentControlForTableInStories(t.doc.endnotePtrs, t.ir)
	}
	if cc == nil {
		cc = blockContentControlForTableInStories(t.doc.commentPtrs, t.ir)
	}
	if cc == nil {
		return nil
	}
	return &ContentControl{ir: cc}
}

// SetContentControl wraps this table in a block-level content control.
func (t *Table) SetContentControl(cc *wordir.ContentControl) bool {
	if t == nil || t.doc == nil {
		return false
	}
	return t.doc.WrapTableInContentControl(t, cc)
}

// ClearContentControl removes the block-level content control wrapping this table.
func (t *Table) ClearContentControl() bool {
	if t == nil || t.doc == nil {
		return false
	}
	return t.doc.UnwrapBlockContentControlFromTable(t)
}

// AddRow appends a new row to the table and returns it.
func (t *Table) AddRow() *TableRow {
	if t.ir == nil {
		return nil
	}
	r := wordir.TableRow{}
	t.ir.Rows = append(t.ir.Rows, r)
	return &TableRow{table: t, index: len(t.ir.Rows) - 1}
}

// Rows returns all rows in the table.
func (t *Table) Rows() []*TableRow {
	if t.ir == nil {
		return nil
	}
	out := make([]*TableRow, len(t.ir.Rows))
	for i := range t.ir.Rows {
		out[i] = &TableRow{table: t, index: i}
	}
	return out
}

// RemoveRow removes a row from the table.
func (t *Table) RemoveRow(r *TableRow) {
	if t.ir == nil || r == nil || r.table != t {
		return
	}
	idx := r.index
	if idx < 0 || idx >= len(t.ir.Rows) {
		return
	}
	t.ir.Rows = append(t.ir.Rows[:idx], t.ir.Rows[idx+1:]...)
}

// InsertRowAt inserts a new row at the given index.
func (t *Table) InsertRowAt(idx int) *TableRow {
	if t.ir == nil {
		return nil
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(t.ir.Rows) {
		idx = len(t.ir.Rows)
	}
	r := wordir.TableRow{}
	t.ir.Rows = append(t.ir.Rows, wordir.TableRow{})
	copy(t.ir.Rows[idx+1:], t.ir.Rows[idx:])
	t.ir.Rows[idx] = r
	return &TableRow{table: t, index: idx}
}

// MergeCellsHorizontally merges cells from fromCol through toCol in the given row.
// It sets the grid span on the first cell and removes the merged cells.
func (t *Table) MergeCellsHorizontally(rowIdx, fromCol, toCol int) {
	rows := t.Rows()
	if rowIdx < 0 || rowIdx >= len(rows) {
		return
	}
	rows[rowIdx].MergeCells(fromCol, toCol)
}

// MergeCellsVertically marks the cell at (fromRow, col) as the start of a
// vertical merge and the cells in the same column from fromRow+1 through toRow
// as continuation cells. The continued cells have their content cleared.
func (t *Table) MergeCellsVertically(col, fromRow, toRow int) {
	rows := t.Rows()
	if fromRow < 0 || toRow >= len(rows) || fromRow >= toRow {
		return
	}
	startCell := rows[fromRow].Cells()
	if col < 0 || col >= len(startCell) {
		return
	}
	startCell[col].SetVMerge("restart")
	for r := fromRow + 1; r <= toRow; r++ {
		cells := rows[r].Cells()
		if col < 0 || col >= len(cells) {
			return
		}
		cells[col].SetVMerge("continue")
		cells[col].SetText("")
	}
}

// MergeCells merges a rectangular range of cells from (startRow, startCol) to
// (endRow, endCol) inclusive. It performs horizontal merges on each row and
// then a vertical merge on the first column of the range.
func (t *Table) MergeCells(startRow, startCol, endRow, endCol int) {
	rows := t.Rows()
	if startRow < 0 || endRow >= len(rows) || startRow > endRow {
		return
	}
	if startCol < 0 || startCol > endCol {
		return
	}
	// Horizontal merge on each row.
	for r := startRow; r <= endRow; r++ {
		rowCells := rows[r].Cells()
		if startCol >= len(rowCells) {
			continue
		}
		end := endCol
		if end >= len(rowCells) {
			end = len(rowCells) - 1
		}
		if startCol < end {
			rows[r].MergeCells(startCol, end)
		}
	}
	// Vertical merge on the first column of the range.
	if startRow < endRow {
		t.MergeCellsVertically(startCol, startRow, endRow)
	}
}

// SetWidth sets the table width in twips.
func (t *Table) SetWidth(twips int) {
	if t.ir == nil {
		return
	}
	t.ir.Format.Width = &twips
}

// SetAlignment sets the table alignment.
func (t *Table) SetAlignment(v string) {
	if t.ir == nil {
		return
	}
	t.ir.Format.Alignment = v
}

// TableRow wraps an IR table row for editing.
type TableRow struct {
	table *Table
	index int
	irPtr *wordir.TableRow // non-nil when detached (cloned)
}

func (r *TableRow) ir() *wordir.TableRow {
	if r.irPtr != nil {
		return r.irPtr
	}
	if r.table == nil || r.table.ir == nil || r.index < 0 || r.index >= len(r.table.ir.Rows) {
		return nil
	}
	return &r.table.ir.Rows[r.index]
}

// AddCell appends a new cell to the row and returns it.
func (r *TableRow) AddCell() *TableCell {
	ir := r.ir()
	if ir == nil {
		return nil
	}
	c := wordir.TableCell{}
	ir.Cells = append(ir.Cells, c)
	return &TableCell{row: r, index: len(ir.Cells) - 1}
}

// Cells returns all cells in the row.
func (r *TableRow) Cells() []*TableCell {
	ir := r.ir()
	if ir == nil {
		return nil
	}
	out := make([]*TableCell, len(ir.Cells))
	for i := range ir.Cells {
		out[i] = &TableCell{row: r, index: i}
	}
	return out
}

// RemoveCell removes a cell from the row.
func (r *TableRow) RemoveCell(c *TableCell) {
	ir := r.ir()
	if ir == nil || c == nil || c.row != r {
		return
	}
	idx := c.index
	if idx < 0 || idx >= len(ir.Cells) {
		return
	}
	ir.Cells = append(ir.Cells[:idx], ir.Cells[idx+1:]...)
}

// InsertCellAt inserts a new cell at the given index.
func (r *TableRow) InsertCellAt(idx int) *TableCell {
	ir := r.ir()
	if ir == nil {
		return nil
	}
	if idx < 0 {
		idx = 0
	}
	if idx > len(ir.Cells) {
		idx = len(ir.Cells)
	}
	c := wordir.TableCell{}
	ir.Cells = append(ir.Cells, wordir.TableCell{})
	copy(ir.Cells[idx+1:], ir.Cells[idx:])
	ir.Cells[idx] = c
	return &TableCell{row: r, index: idx}
}

// MergeCells merges cells from startIdx through endIdx (inclusive) into a
// single cell by setting the gridSpan on the first cell and removing the rest.
func (r *TableRow) MergeCells(startIdx, endIdx int) {
	ir := r.ir()
	if ir == nil || startIdx < 0 || endIdx >= len(ir.Cells) || startIdx >= endIdx {
		return
	}
	span := endIdx - startIdx + 1
	ir.Cells[startIdx].Format.GridSpan = span
	ir.Cells = append(ir.Cells[:startIdx+1], ir.Cells[endIdx+1:]...)
}

// SetHeight sets the row height in twips.
func (r *TableRow) SetHeight(twips int) {
	if ir := r.ir(); ir != nil {
		ir.Format.Height = &twips
	}
}

// SetHeaderRow marks this row as a header row (repeats on new pages).
func (r *TableRow) SetHeaderRow(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.IsHeader = v
	}
}

// TableCell wraps an IR table cell for editing.
type TableCell struct {
	row   *TableRow
	index int
	irPtr *wordir.TableCell // non-nil when detached (cloned)
}

func (c *TableCell) ir() *wordir.TableCell {
	if c.irPtr != nil {
		return c.irPtr
	}
	if c.row == nil || c.row.ir() == nil || c.index < 0 || c.index >= len(c.row.ir().Cells) {
		return nil
	}
	return &c.row.ir().Cells[c.index]
}

// SetContentControl attaches a content control (SDT) to this cell.
// Pass nil to remove.
func (c *TableCell) SetContentControl(cc *wordir.ContentControl) {
	if ir := c.ir(); ir != nil {
		ir.ContentControl = cc
	}
}

// ContentControl returns the content control attached to this cell, or nil.
func (c *TableCell) ContentControl() *wordir.ContentControl {
	if ir := c.ir(); ir != nil {
		return ir.ContentControl
	}
	return nil
}

// AddParagraph appends a new paragraph to the cell and returns it.
func (c *TableCell) AddParagraph() *Paragraph {
	ir := c.ir()
	if ir == nil {
		return nil
	}
	p := wordir.Paragraph{}
	ir.Blocks = append(ir.Blocks, wordir.Block{
		Index:     len(ir.Blocks),
		Kind:      "paragraph",
		Paragraph: &p,
	})
	c.rebuildFlatSlices()
	return &Paragraph{ir: ir.Blocks[len(ir.Blocks)-1].Paragraph, doc: c.row.table.doc}
}

// RemoveParagraph removes a paragraph from the cell by identity.
func (c *TableCell) RemoveParagraph(p *Paragraph) {
	if p == nil || p.ir == nil {
		return
	}
	ir := c.ir()
	if ir == nil {
		return
	}
	for i := range ir.Blocks {
		if ir.Blocks[i].Paragraph == p.ir {
			ir.Blocks = append(ir.Blocks[:i], ir.Blocks[i+1:]...)
			for j := i; j < len(ir.Blocks); j++ {
				ir.Blocks[j].Index = j
			}
			c.rebuildFlatSlices()
			return
		}
	}
}

// Paragraphs returns all paragraphs in the cell.
func (c *TableCell) Paragraphs() []*Paragraph {
	ir := c.ir()
	if ir == nil {
		return nil
	}
	var out []*Paragraph
	for i := range ir.Blocks {
		if ir.Blocks[i].Kind == "paragraph" && ir.Blocks[i].Paragraph != nil {
			out = append(out, &Paragraph{ir: ir.Blocks[i].Paragraph, doc: c.row.table.doc})
		}
	}
	return out
}

// SetText replaces all cell content with a single paragraph containing v.
func (c *TableCell) SetText(v string) {
	ir := c.ir()
	if ir == nil {
		return
	}
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: v}}}
	ir.Blocks = []wordir.Block{{
		Index:     0,
		Kind:      "paragraph",
		Paragraph: &p,
	}}
	c.rebuildFlatSlices()
}

// Text aggregates the text of all paragraphs in the cell.
func (c *TableCell) Text() string {
	ir := c.ir()
	if ir == nil {
		return ""
	}
	var s string
	for i := range ir.Blocks {
		if ir.Blocks[i].Kind != "paragraph" || ir.Blocks[i].Paragraph == nil {
			continue
		}
		para := ir.Blocks[i].Paragraph
		for j := range para.Runs {
			s += para.Runs[j].Text
		}
		if i < len(ir.Blocks)-1 {
			s += "\n"
		}
	}
	return s
}

// SetWidth sets the preferred cell width in twips.
func (c *TableCell) SetWidth(twips int) {
	if ir := c.ir(); ir != nil {
		ir.Format.Width = &twips
	}
}

// SetVerticalAlignment sets the cell vertical alignment (e.g. "top", "center", "bottom").
func (c *TableCell) SetVerticalAlignment(v string) {
	if ir := c.ir(); ir != nil {
		ir.Format.VertAlign = v
	}
}

// SetBackground sets the cell background color as a hex string (e.g., "FFFF00").
func (c *TableCell) SetBackground(hex string) {
	if ir := c.ir(); ir != nil {
		ir.Format.Shading = hex
	}
}

// SetBorderTop sets the top border style, size in points, and color.
func (c *TableCell) SetBorderTop(style string, sizePt float64, color string) {
	if ir := c.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.CellBorders{}
		}
		ir.Format.Borders.Top = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetBorderBottom sets the bottom border style, size in points, and color.
func (c *TableCell) SetBorderBottom(style string, sizePt float64, color string) {
	if ir := c.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.CellBorders{}
		}
		ir.Format.Borders.Bottom = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetBorderLeft sets the left border style, size in points, and color.
func (c *TableCell) SetBorderLeft(style string, sizePt float64, color string) {
	if ir := c.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.CellBorders{}
		}
		ir.Format.Borders.Left = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetBorderRight sets the right border style, size in points, and color.
func (c *TableCell) SetBorderRight(style string, sizePt float64, color string) {
	if ir := c.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.CellBorders{}
		}
		ir.Format.Borders.Right = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetVMerge sets the vertical merge state ("restart", "continue", or "").
func (c *TableCell) SetVMerge(v string) {
	if ir := c.ir(); ir != nil {
		ir.Format.VMerge = v
	}
}

// SetGridSpan sets how many grid columns this cell spans.
func (c *TableCell) SetGridSpan(n int) {
	if ir := c.ir(); ir != nil {
		ir.Format.GridSpan = n
	}
}

// GridSpan returns how many grid columns this cell spans (1 = no merge).
func (c *TableCell) GridSpan() int {
	if ir := c.ir(); ir != nil {
		if ir.Format.GridSpan <= 0 {
			return 1
		}
		return ir.Format.GridSpan
	}
	return 1
}

// VMerge returns the vertical merge state ("restart", "continue", or "").
func (c *TableCell) VMerge() string {
	if ir := c.ir(); ir != nil {
		return ir.Format.VMerge
	}
	return ""
}

// IsMerged reports whether the cell is part of a horizontal or vertical merge.
func (c *TableCell) IsMerged() bool {
	if ir := c.ir(); ir != nil {
		return ir.Format.GridSpan > 1 || ir.Format.VMerge != ""
	}
	return false
}

// SplitHorizontally splits a horizontally merged cell into individual cells.
// It resets GridSpan to 1 and inserts (oldGridSpan-1) new cells after this one.
// If the cell is not horizontally merged (GridSpan <= 1), this is a no-op.
func (c *TableCell) SplitHorizontally() {
	ir := c.ir()
	if ir == nil || ir.Format.GridSpan <= 1 {
		return
	}
	span := ir.Format.GridSpan
	ir.Format.GridSpan = 1
	// Insert new cells after this one in the same row.
	row := c.row.ir()
	if row == nil {
		return
	}
	idx := c.index
	if idx < 0 || idx >= len(row.Cells) {
		return
	}
	for i := 1; i < span; i++ {
		newCell := wordir.TableCell{}
		row.Cells = append(row.Cells, wordir.TableCell{})
		copy(row.Cells[idx+i+1:], row.Cells[idx+i:])
		row.Cells[idx+i] = newCell
	}
}

// SplitVertically splits a vertically merged cell by clearing the VMerge
// property on this cell and all continuation cells below it in the same column.
func (c *TableCell) SplitVertically() {
	ir := c.ir()
	if ir == nil {
		return
	}
	if ir.Format.VMerge == "" {
		return
	}
	// If this is a continuation cell, walk up to find the restart cell.
	startRow := c.row.index
	colIdx := c.index
	if ir.Format.VMerge == "continue" {
		for r := startRow - 1; r >= 0; r-- {
			rows := c.row.table.Rows()
			if r >= len(rows) {
				continue
			}
			cells := rows[r].Cells()
			if colIdx >= len(cells) {
				continue
			}
			if cells[colIdx].ir().Format.VMerge == "restart" {
				startRow = r
				break
			}
			if cells[colIdx].ir().Format.VMerge != "continue" {
				break
			}
		}
	}
	// Clear VMerge from the restart cell and all continuations below.
	rows := c.row.table.Rows()
	for r := startRow; r < len(rows); r++ {
		cells := rows[r].Cells()
		if colIdx >= len(cells) {
			break
		}
		cellIR := cells[colIdx].ir()
		if cellIR == nil {
			break
		}
		if r == startRow {
			cellIR.Format.VMerge = ""
		} else if cellIR.Format.VMerge == "continue" {
			cellIR.Format.VMerge = ""
		} else {
			break
		}
	}
}

// rebuildFlatSlices rebuilds the Paragraphs and Tables convenience slices
// from the canonical Blocks slice.
func (c *TableCell) rebuildFlatSlices() {
	ir := c.ir()
	if ir == nil {
		return
	}
	ir.Paragraphs = ir.Paragraphs[:0]
	ir.Tables = ir.Tables[:0]
	for i := range ir.Blocks {
		b := &ir.Blocks[i]
		switch b.Kind {
		case "paragraph":
			if b.Paragraph != nil {
				ir.Paragraphs = append(ir.Paragraphs, *b.Paragraph)
			}
		case "table":
			if b.Table != nil {
				ir.Tables = append(ir.Tables, *b.Table)
			}
		}
	}
}

// SetStyle sets the table style ID.
func (t *Table) SetStyle(id string) {
	if t.ir != nil {
		t.ir.StyleID = id
	}
}

// SetBorders sets table-level borders. Any nil side is left unchanged.
func (t *Table) SetBorders(top, left, bottom, right *wordir.BorderSide) {
	if t.ir == nil {
		return
	}
	if t.ir.Format.Borders == nil {
		t.ir.Format.Borders = &wordir.TableBorders{}
	}
	if top != nil {
		t.ir.Format.Borders.Top = top
	}
	if left != nil {
		t.ir.Format.Borders.Left = left
	}
	if bottom != nil {
		t.ir.Format.Borders.Bottom = bottom
	}
	if right != nil {
		t.ir.Format.Borders.Right = right
	}
}

// SetCellSpacing sets the spacing between cells in twips.
func (t *Table) SetCellSpacing(twips int) {
	if t.ir != nil {
		t.ir.Format.CellSpacing = twips
	}
}

// SetLook sets conditional table-style formatting flags.
func (t *Table) SetLook(firstRow, lastRow, firstCol, lastCol, noHBand, noVBand bool) {
	if t.ir != nil {
		t.ir.Format.Look = &wordir.TableLook{
			FirstRow:    firstRow,
			LastRow:     lastRow,
			FirstColumn: firstCol,
			LastColumn:  lastCol,
			NoHBand:     noHBand,
			NoVBand:     noVBand,
		}
	}
}

// SetLayout sets the table layout type: "fixed" or "autofit".
func (t *Table) SetLayout(v string) {
	if t.ir != nil {
		t.ir.Format.Layout = v
	}
}

// SetIndent sets the table indent from the left margin in twips.
func (t *Table) SetIndent(twips int) {
	if t.ir != nil {
		t.ir.Format.Indent = twips
	}
}

// SetCaption sets the table caption.
func (t *Table) SetCaption(text string) {
	if t.ir != nil {
		t.ir.Format.Caption = text
	}
}

// SetDescription sets the table description (alt text).
func (t *Table) SetDescription(text string) {
	if t.ir != nil {
		t.ir.Format.Description = text
	}
}

// SetBidiVisual marks the table as using visual right-to-left ordering.
func (t *Table) SetBidiVisual(v bool) {
	if t.ir != nil {
		t.ir.Format.BidiVisual = v
	}
}

// SetStyleRowBandSize sets the number of rows in each row band.
func (t *Table) SetStyleRowBandSize(n int) {
	if t.ir != nil {
		t.ir.Format.StyleRowBandSize = n
	}
}

// SetStyleColBandSize sets the number of columns in each column band.
func (t *Table) SetStyleColBandSize(n int) {
	if t.ir != nil {
		t.ir.Format.StyleColBandSize = n
	}
}

// SetShading sets the table background color as a 6-digit hex string.
func (t *Table) SetShading(hex string) {
	if t.ir != nil {
		t.ir.Format.Shading = hex
	}
}

// SetCellPadding sets the default cell padding for all cells in the table.
func (t *Table) SetCellPadding(top, bottom, left, right int) {
	if t.ir != nil {
		t.ir.Format.CellPadTop = top
		t.ir.Format.CellPadBottom = bottom
		t.ir.Format.CellPadLeft = left
		t.ir.Format.CellPadRight = right
	}
}

// SetColumnWidths sets explicit column widths in twips.
func (t *Table) SetColumnWidths(widths []int) {
	if t.ir != nil {
		t.ir.Format.ColumnWidths = append([]int(nil), widths...)
	}
}

// SetFloatProps sets floating table positioning properties.
// nil removes floating behavior.
func (t *Table) SetFloatProps(props *wordir.TableFloatProps) {
	if t.ir != nil {
		if props == nil {
			t.ir.Format.FloatProps = nil
			return
		}
		copyProps := *props
		t.ir.Format.FloatProps = &copyProps
	}
}

// SetHeightRule sets the row height rule: "atLeast", "exact", or "auto".
func (r *TableRow) SetHeightRule(rule string) {
	if ir := r.ir(); ir != nil {
		ir.Format.HeightRule = rule
	}
}

// SetCantSplit prevents the row from splitting across pages.
func (r *TableRow) SetCantSplit(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.CantSplit = v
	}
}

// SetGridBefore sets the number of grid columns before the first cell.
func (r *TableRow) SetGridBefore(n int) {
	if ir := r.ir(); ir != nil {
		ir.Format.GridBefore = n
	}
}

// SetGridAfter sets the number of grid columns after the last cell.
func (r *TableRow) SetGridAfter(n int) {
	if ir := r.ir(); ir != nil {
		ir.Format.GridAfter = n
	}
}

// SetJustification sets the row justification (e.g., "left", "center", "right").
func (r *TableRow) SetJustification(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Justification = v
	}
}

// SetConditionalFormat sets the conditional formatting override for this row.
func (r *TableRow) SetConditionalFormat(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Conditional = v
	}
}

// SetHidden hides the row.
func (r *TableRow) SetHidden(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Hidden = v
	}
}

// SetMargins sets the cell margins (padding) in twips for all four sides.
func (c *TableCell) SetMargins(top, left, bottom, right int) {
	if ir := c.ir(); ir != nil {
		ir.Format.CellPadTop = top
		ir.Format.CellPadLeft = left
		ir.Format.CellPadBottom = bottom
		ir.Format.CellPadRight = right
	}
}

// SetCellPadding sets the cell padding in twips for all four sides.
// This is an alias for SetMargins for API consistency with Table.SetCellPadding.
func (c *TableCell) SetCellPadding(top, bottom, left, right int) {
	c.SetMargins(top, left, bottom, right)
}

// SetTextDirection sets the cell text direction (e.g., "lrTb", "tbRl").
func (c *TableCell) SetTextDirection(dir string) {
	if ir := c.ir(); ir != nil {
		ir.Format.TextDirection = dir
	}
}

// SetNoWrap prevents text from wrapping in the cell.
func (c *TableCell) SetNoWrap(v bool) {
	if ir := c.ir(); ir != nil {
		ir.Format.NoWrap = v
	}
}

// SetHideMark hides the end-of-cell mark.
func (c *TableCell) SetHideMark(v bool) {
	if ir := c.ir(); ir != nil {
		ir.Format.HideMark = v
	}
}

// SetBorder sets a cell border by position.
// position is "top", "left", "bottom", or "right".
// style is "single", "double", "dashed", "dotted", etc.
// sizePt is the border width in points.
// color is a 6-digit hex string or "auto".
func (c *TableCell) SetBorder(position, style string, sizePt float64, color string) {
	if ir := c.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.CellBorders{}
		}
		b := &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
		switch position {
		case "top":
			ir.Format.Borders.Top = b
		case "left":
			ir.Format.Borders.Left = b
		case "bottom":
			ir.Format.Borders.Bottom = b
		case "right":
			ir.Format.Borders.Right = b
		}
	}
}

// ClearBorders removes all borders from the cell.
func (c *TableCell) ClearBorders() {
	if ir := c.ir(); ir != nil {
		ir.Format.Borders = nil
	}
}