package edit

import wordir "office/pkg/word/ir"

// StoryField edits a block-spanning complex field (e.g. TOC, INDEX) on a
// document or sub-story.  It wraps the IR StoryField and provides helpers to
// manage the field's instruction runs and result blocks.
type StoryField struct {
	ir    *wordir.StoryField
	doc   *Document
	story *wordir.Story
}

// Kind returns the field kind (e.g. "TOC", "INDEX").
func (sf *StoryField) Kind() string {
	if sf == nil || sf.ir == nil {
		return ""
	}
	return sf.ir.Kind
}

// Instr returns the raw field instruction text.
func (sf *StoryField) Instr() string {
	if sf == nil || sf.ir == nil {
		return ""
	}
	return sf.ir.Instr
}

// SetInstr sets the raw field instruction text.
func (sf *StoryField) SetInstr(v string) {
	if sf == nil || sf.ir == nil {
		return
	}
	sf.ir.Instr = v
}

// Arg returns the field argument (e.g. the TOC options string).
func (sf *StoryField) Arg() string {
	if sf == nil || sf.ir == nil {
		return ""
	}
	return sf.ir.Arg
}

// SetArg sets the field argument.
func (sf *StoryField) SetArg(v string) {
	if sf == nil || sf.ir == nil {
		return
	}
	sf.ir.Arg = v
}

// Anchor returns the field anchor (e.g. bookmark name for REF fields).
func (sf *StoryField) Anchor() string {
	if sf == nil || sf.ir == nil {
		return ""
	}
	return sf.ir.Anchor
}

// SetAnchor sets the field anchor.
func (sf *StoryField) SetAnchor(v string) {
	if sf == nil || sf.ir == nil {
		return
	}
	sf.ir.Anchor = v
}

// Target returns the field target (e.g. URL for HYPERLINK fields).
func (sf *StoryField) Target() string {
	if sf == nil || sf.ir == nil {
		return ""
	}
	return sf.ir.Target
}

// SetTarget sets the field target.
func (sf *StoryField) SetTarget(v string) {
	if sf == nil || sf.ir == nil {
		return
	}
	sf.ir.Target = v
}

// fieldIndex returns the index of this field in its parent document or story.
func (sf *StoryField) fieldIndex() int {
	if sf.story != nil {
		for i := range sf.story.Fields {
			if sf.story.Fields[i].Instr == sf.ir.Instr &&
				sf.story.Fields[i].Kind == sf.ir.Kind &&
				sf.story.Fields[i].Arg == sf.ir.Arg {
				return i
			}
		}
	}
	if sf.doc != nil {
		for i := range sf.doc.IR.Fields {
			if sf.doc.IR.Fields[i].Instr == sf.ir.Instr &&
				sf.doc.IR.Fields[i].Kind == sf.ir.Kind &&
				sf.doc.IR.Fields[i].Arg == sf.ir.Arg {
				return i
			}
		}
	}
	return -1
}

// AddInstrRun appends an instruction run to the field and returns it.
// Instruction runs are used for complex field operands (e.g. styled TOC
// headings).
func (sf *StoryField) AddInstrRun() *Run {
	if sf == nil || sf.ir == nil {
		return nil
	}
	sf.ir.InstrRuns = append(sf.ir.InstrRuns, wordir.Run{Kind: "run"})
	return &Run{para: nil, index: -1, irPtr: &sf.ir.InstrRuns[len(sf.ir.InstrRuns)-1]}
}

// InstrRuns returns the instruction runs of the field.
func (sf *StoryField) InstrRuns() []*Run {
	if sf == nil || sf.ir == nil {
		return nil
	}
	out := make([]*Run, len(sf.ir.InstrRuns))
	for i := range sf.ir.InstrRuns {
		out[i] = &Run{para: nil, index: -1, irPtr: &sf.ir.InstrRuns[i]}
	}
	return out
}

// AddResultParagraph appends a new paragraph to the field's result blocks
// (and to the parent document/story blocks) and returns it.
func (sf *StoryField) AddResultParagraph() *Paragraph {
	if sf == nil || sf.ir == nil {
		return nil
	}
	p := wordir.Paragraph{}
	idx := sf.fieldIndex()
	b := wordir.Block{
		Kind:       "paragraph",
		Paragraph:  &p,
		FieldKind:  sf.ir.Kind,
		FieldIndex: idx,
	}
	if sf.story != nil {
		sf.story.Blocks = append(sf.story.Blocks, b)
		sf.story.Paragraphs = append(sf.story.Paragraphs, p)
		sf.ir.Blocks = append(sf.ir.Blocks, sf.story.Blocks[len(sf.story.Blocks)-1])
		return &Paragraph{ir: sf.story.Blocks[len(sf.story.Blocks)-1].Paragraph, doc: sf.doc}
	}
	if sf.doc != nil {
		p.Index = sf.doc.nextParaIndex
		sf.doc.nextParaIndex++
		sf.doc.IR.Blocks = append(sf.doc.IR.Blocks, b)
		sf.doc.rebuildFlatSlices()
		// Store a copy of the block in the field so it tracks the same pointer.
		sf.ir.Blocks = append(sf.ir.Blocks, sf.doc.IR.Blocks[len(sf.doc.IR.Blocks)-1])
		return &Paragraph{ir: sf.doc.IR.Blocks[len(sf.doc.IR.Blocks)-1].Paragraph, doc: sf.doc}
	}
	return nil
}

// AddResultTable appends a new table to the field's result blocks (and to the
// parent document/story blocks) and returns it.
func (sf *StoryField) AddResultTable() *Table {
	if sf == nil || sf.ir == nil {
		return nil
	}
	t := wordir.Table{}
	idx := sf.fieldIndex()
	b := wordir.Block{
		Kind:       "table",
		Table:      &t,
		FieldKind:  sf.ir.Kind,
		FieldIndex: idx,
	}
	if sf.story != nil {
		sf.story.Blocks = append(sf.story.Blocks, b)
		sf.story.Tables = append(sf.story.Tables, t)
		sf.ir.Blocks = append(sf.ir.Blocks, sf.story.Blocks[len(sf.story.Blocks)-1])
		return &Table{ir: sf.story.Blocks[len(sf.story.Blocks)-1].Table, doc: sf.doc}
	}
	if sf.doc != nil {
		sf.doc.IR.Blocks = append(sf.doc.IR.Blocks, b)
		sf.doc.rebuildFlatSlices()
		sf.ir.Blocks = append(sf.ir.Blocks, sf.doc.IR.Blocks[len(sf.doc.IR.Blocks)-1])
		return &Table{ir: sf.doc.IR.Blocks[len(sf.doc.IR.Blocks)-1].Table, doc: sf.doc}
	}
	return nil
}

// ResultBlockCount returns the number of result blocks (paragraphs/tables)
// currently in the field.
func (sf *StoryField) ResultBlockCount() int {
	if sf == nil || sf.ir == nil {
		return 0
	}
	return len(sf.ir.Blocks)
}