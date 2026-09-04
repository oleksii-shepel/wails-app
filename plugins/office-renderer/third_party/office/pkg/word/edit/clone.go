package edit

import (
	wordir "office/pkg/word/ir"
)

// cloneRun returns a deep copy of a run.
func cloneRun(r wordir.Run) wordir.Run {
	out := r
	out.Format = cloneRunFormat(r.Format)
	if r.Image != nil {
		img := *r.Image
		if r.Image.Data != nil {
			img.Data = append([]byte(nil), r.Image.Data...)
		}
		out.Image = &img
	}
	if r.OLEData != nil {
		out.OLEData = append([]byte(nil), r.OLEData...)
	}
	if r.OLEPreview != nil {
		img := *r.OLEPreview
		if r.OLEPreview.Data != nil {
			img.Data = append([]byte(nil), r.OLEPreview.Data...)
		}
		out.OLEPreview = &img
	}
	if r.TextBox != nil {
		tb := *r.TextBox
		out.TextBox = &tb
	}
	if r.Shape != nil {
		s := *r.Shape
		out.Shape = &s
	}
	if r.Chart != nil {
		// Charts are reference-like; shallow copy is sufficient.
		c := *r.Chart
		out.Chart = &c
	}
	if r.PropertyRevision != nil {
		pr := *r.PropertyRevision
		out.PropertyRevision = &pr
	}
	if r.DisplayFieldRevision != nil {
		dr := *r.DisplayFieldRevision
		out.DisplayFieldRevision = &dr
	}
	if len(r.FieldInstrRuns) > 0 {
		out.FieldInstrRuns = make([]wordir.Run, len(r.FieldInstrRuns))
		for i := range r.FieldInstrRuns {
			out.FieldInstrRuns[i] = cloneRun(r.FieldInstrRuns[i])
		}
	}
	if len(r.Fields) > 0 {
		out.Fields = make([]wordir.Field, len(r.Fields))
		for i := range r.Fields {
			out.Fields[i] = cloneField(r.Fields[i])
		}
	}
	return out
}

func cloneRunFormat(rf wordir.RunFormat) wordir.RunFormat {
	out := rf
	cloneBoolPtr := func(p **bool) {
		if *p != nil {
			v := **p
			*p = &v
		}
	}
	cloneIntPtr := func(p **int) {
		if *p != nil {
			v := **p
			*p = &v
		}
	}
	cloneBoolPtr(&out.Bold)
	cloneBoolPtr(&out.Italic)
	cloneBoolPtr(&out.Strike)
	cloneBoolPtr(&out.DoubleStrike)
	cloneBoolPtr(&out.Caps)
	cloneBoolPtr(&out.SmallCaps)
	cloneBoolPtr(&out.BoldCS)
	cloneBoolPtr(&out.ItalicCS)
	cloneBoolPtr(&out.Vanish)
	cloneBoolPtr(&out.NoProof)
	cloneBoolPtr(&out.WebHidden)
	cloneBoolPtr(&out.ComplexScript)
	cloneBoolPtr(&out.UsePageSetup)
	cloneBoolPtr(&out.RTL)
	cloneIntPtr(&out.FontSize)
	cloneIntPtr(&out.FontSizeCS)
	cloneIntPtr(&out.CharacterSpacing)
	cloneIntPtr(&out.FitTextWidth)
	cloneIntPtr(&out.Kerning)
	cloneIntPtr(&out.CharScale)
	if len(out.StylisticSets) > 0 {
		out.StylisticSets = append([]int(nil), out.StylisticSets...)
	}
	if out.TextOutline != nil {
		v := *out.TextOutline
		out.TextOutline = &v
	}
	if out.TextGlow != nil {
		v := *out.TextGlow
		out.TextGlow = &v
	}
	if out.TextShadow != nil {
		v := *out.TextShadow
		out.TextShadow = &v
	}
	if out.TextSoftEdge != nil {
		v := *out.TextSoftEdge
		out.TextSoftEdge = &v
	}
	if out.TextReflection != nil {
		v := *out.TextReflection
		out.TextReflection = &v
	}
	return out
}

func cloneField(f wordir.Field) wordir.Field {
	out := f
	if len(f.Children) > 0 {
		out.Children = make([]wordir.Run, len(f.Children))
		for i := range f.Children {
			out.Children[i] = cloneRun(f.Children[i])
		}
	}
	if len(f.InstrRuns) > 0 {
		out.InstrRuns = make([]wordir.Run, len(f.InstrRuns))
		for i := range f.InstrRuns {
			out.InstrRuns[i] = cloneRun(f.InstrRuns[i])
		}
	}
	return out
}

func cloneParagraph(p wordir.Paragraph) wordir.Paragraph {
	out := p
	out.Runs = make([]wordir.Run, len(p.Runs))
	for i := range p.Runs {
		out.Runs[i] = cloneRun(p.Runs[i])
	}
	out.Format = cloneParagraphFormat(p.Format)
	if p.Numbering != nil {
		n := *p.Numbering
		out.Numbering = &n
	}
	if p.Frame != nil {
		f := *p.Frame
		out.Frame = &f
	}
	if p.PropertyRevision != nil {
		pr := *p.PropertyRevision
		out.PropertyRevision = &pr
	}
	if p.Source != nil {
		s := *p.Source
		if p.Source.PapxSprms != nil {
			s.PapxSprms = append([]byte(nil), p.Source.PapxSprms...)
		}
		if p.Source.StyleIndex != nil {
			v := *p.Source.StyleIndex
			s.StyleIndex = &v
		}
		if p.Source.PropertyRevision != nil {
			pr := *p.Source.PropertyRevision
			s.PropertyRevision = &pr
		}
		out.Source = &s
	}
	return out
}

func cloneParagraphFormat(pf wordir.ParagraphFormat) wordir.ParagraphFormat {
	out := pf
	if pf.SpacingBefore != nil {
		v := *pf.SpacingBefore
		out.SpacingBefore = &v
	}
	if pf.SpacingAfter != nil {
		v := *pf.SpacingAfter
		out.SpacingAfter = &v
	}
	if pf.SpacingBeforeLines != nil {
		v := *pf.SpacingBeforeLines
		out.SpacingBeforeLines = &v
	}
	if pf.SpacingAfterLines != nil {
		v := *pf.SpacingAfterLines
		out.SpacingAfterLines = &v
	}
	if pf.LineSpacing != nil {
		v := *pf.LineSpacing
		out.LineSpacing = &v
	}
	if pf.IndentLeft != nil {
		v := *pf.IndentLeft
		out.IndentLeft = &v
	}
	if pf.IndentRight != nil {
		v := *pf.IndentRight
		out.IndentRight = &v
	}
	if pf.IndentFirstLine != nil {
		v := *pf.IndentFirstLine
		out.IndentFirstLine = &v
	}
	if pf.IndentHanging != nil {
		v := *pf.IndentHanging
		out.IndentHanging = &v
	}
	if pf.OutlineLevel != nil {
		v := *pf.OutlineLevel
		out.OutlineLevel = &v
	}
	if pf.BinaryStyleIndex != nil {
		v := *pf.BinaryStyleIndex
		out.BinaryStyleIndex = &v
	}
	if pf.BinaryListLevel != nil {
		v := *pf.BinaryListLevel
		out.BinaryListLevel = &v
	}
	if pf.BinaryListOverride != nil {
		v := *pf.BinaryListOverride
		out.BinaryListOverride = &v
	}
	if pf.BinaryListFormat != nil {
		v := *pf.BinaryListFormat
		out.BinaryListFormat = &v
	}
	if pf.BinaryFrame != nil {
		f := *pf.BinaryFrame
		out.BinaryFrame = &f
	}
	if pf.Borders != nil {
		out.Borders = cloneParagraphBorders(pf.Borders)
	}
	if len(pf.TabStops) > 0 {
		out.TabStops = make([]wordir.TabStop, len(pf.TabStops))
		copy(out.TabStops, pf.TabStops)
	}
	return out
}

func cloneParagraphBorders(b *wordir.ParagraphBorders) *wordir.ParagraphBorders {
	if b == nil {
		return nil
	}
	out := &wordir.ParagraphBorders{}
	if b.Top != nil {
		v := *b.Top
		out.Top = &v
	}
	if b.Left != nil {
		v := *b.Left
		out.Left = &v
	}
	if b.Bottom != nil {
		v := *b.Bottom
		out.Bottom = &v
	}
	if b.Right != nil {
		v := *b.Right
		out.Right = &v
	}
	if b.Between != nil {
		v := *b.Between
		out.Between = &v
	}
	if b.Bar != nil {
		v := *b.Bar
		out.Bar = &v
	}
	return out
}

func cloneTable(t wordir.Table) wordir.Table {
	out := t
	out.Rows = make([]wordir.TableRow, len(t.Rows))
	for i := range t.Rows {
		out.Rows[i] = cloneTableRow(t.Rows[i])
	}
	if t.Format.Width != nil {
		v := *t.Format.Width
		out.Format.Width = &v
	}
	if t.PropertyRevision != nil {
		pr := *t.PropertyRevision
		out.PropertyRevision = &pr
	}
	return out
}

func cloneTableRow(r wordir.TableRow) wordir.TableRow {
	out := r
	out.Cells = make([]wordir.TableCell, len(r.Cells))
	for i := range r.Cells {
		out.Cells[i] = cloneTableCell(r.Cells[i])
	}
	if r.Format.Height != nil {
		v := *r.Format.Height
		out.Format.Height = &v
	}
	if r.PropertyRevision != nil {
		pr := *r.PropertyRevision
		out.PropertyRevision = &pr
	}
	return out
}

func cloneTableCell(c wordir.TableCell) wordir.TableCell {
	out := c
	out.Paragraphs = make([]wordir.Paragraph, len(c.Paragraphs))
	for i := range c.Paragraphs {
		out.Paragraphs[i] = cloneParagraph(c.Paragraphs[i])
	}
	out.Tables = make([]wordir.Table, len(c.Tables))
	for i := range c.Tables {
		out.Tables[i] = cloneTable(c.Tables[i])
	}
	out.Blocks = make([]wordir.Block, len(c.Blocks))
	for i := range c.Blocks {
		out.Blocks[i] = cloneBlock(c.Blocks[i])
	}
	if c.Format.Width != nil {
		v := *c.Format.Width
		out.Format.Width = &v
	}
	if c.Format.GridSpan != 0 {
		out.Format.GridSpan = c.Format.GridSpan
	}
	if c.Format.Borders != nil {
		out.Format.Borders = cloneCellBorders(c.Format.Borders)
	}
	if c.PropertyRevision != nil {
		pr := *c.PropertyRevision
		out.PropertyRevision = &pr
	}
	if c.CellIns != nil {
		ci := *c.CellIns
		out.CellIns = &ci
	}
	if c.CellDel != nil {
		cd := *c.CellDel
		out.CellDel = &cd
	}
	if c.CellMerge != nil {
		cm := *c.CellMerge
		out.CellMerge = &cm
	}
	return out
}

func cloneCellBorders(b *wordir.CellBorders) *wordir.CellBorders {
	if b == nil {
		return nil
	}
	out := &wordir.CellBorders{}
	cloneBorderSide := func(s *wordir.BorderSide) *wordir.BorderSide {
		if s == nil {
			return nil
		}
		v := *s
		return &v
	}
	out.Top = cloneBorderSide(b.Top)
	out.Left = cloneBorderSide(b.Left)
	out.Start = cloneBorderSide(b.Start)
	out.Bottom = cloneBorderSide(b.Bottom)
	out.Right = cloneBorderSide(b.Right)
	out.End = cloneBorderSide(b.End)
	out.TL2BR = cloneBorderSide(b.TL2BR)
	out.TR2BL = cloneBorderSide(b.TR2BL)
	return out
}

func cloneBlock(b wordir.Block) wordir.Block {
	out := b
	if b.Paragraph != nil {
		p := cloneParagraph(*b.Paragraph)
		out.Paragraph = &p
	}
	if b.Table != nil {
		t := cloneTable(*b.Table)
		out.Table = &t
	}
	return out
}

// Clone returns a deep copy of the run. The returned run is not attached to
// any paragraph; use Paragraph.AppendRun to insert it.
func (r *Run) Clone() *Run {
	if r == nil {
		return nil
	}
	ir := r.ir()
	if ir == nil {
		return nil
	}
	cloned := cloneRun(*ir)
	return &Run{para: nil, index: -1, irPtr: &cloned}
}

// Clone returns a deep copy of the paragraph. The returned paragraph is not
// attached to any document; use Document.AppendParagraph to insert it.
func (p *Paragraph) Clone() *Paragraph {
	if p == nil || p.ir == nil {
		return nil
	}
	cloned := cloneParagraph(*p.ir)
	return &Paragraph{ir: &cloned}
}

// Clone returns a deep copy of the table. The returned table is not attached
// to any document; use Document.AppendTable to insert it.
func (t *Table) Clone() *Table {
	if t == nil || t.ir == nil {
		return nil
	}
	cloned := cloneTable(*t.ir)
	return &Table{ir: &cloned, doc: t.doc}
}

// Clone returns a deep copy of the table row. The returned row is not
// attached to any table; use Table.AppendRow to insert it.
func (r *TableRow) Clone() *TableRow {
	if r == nil {
		return nil
	}
	ir := r.ir()
	if ir == nil {
		return nil
	}
	cloned := cloneTableRow(*ir)
	return &TableRow{table: nil, index: -1, irPtr: &cloned}
}

// Clone returns a deep copy of the table cell. The returned cell is not
// attached to any row; use TableRow.AppendCell to insert it.
func (c *TableCell) Clone() *TableCell {
	if c == nil {
		return nil
	}
	ir := c.ir()
	if ir == nil {
		return nil
	}
	cloned := cloneTableCell(*ir)
	return &TableCell{row: nil, index: -1, irPtr: &cloned}
}