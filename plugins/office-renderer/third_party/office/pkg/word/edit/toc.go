package edit

import (
	"fmt"
	"strings"

	wordir "office/pkg/word/ir"
)

// TOC wraps a StoryField of kind "TOC" and provides helpers to generate or
// update the table of contents from document headings.
type TOC struct {
	doc *Document
	sf  *StoryField
}

// InsertTOC inserts a TOC field at the beginning of the document body and
// returns a TOC editor. levels controls how many heading levels to include
// (default 3). A "Table of Contents" heading paragraph is inserted before the
// field results.
func (d *Document) InsertTOC(levels int) *TOC {
	if d == nil || d.IR == nil {
		return nil
	}
	if levels <= 0 {
		levels = 3
	}
	instr := fmt.Sprintf(`TOC \o "1-%d" \h \z \u`, levels)
	sf := d.AddStoryField("TOC", instr)
	if sf == nil {
		return nil
	}
	// Prepend a title paragraph to the document body.
	title := wordir.Paragraph{Index: d.nextParaIndex}
	d.nextParaIndex++
	title.Runs = append(title.Runs, wordir.Run{
		Kind: "run",
		Text: "Table of Contents",
		Format: wordir.RunFormat{
			Bold: boolPtr(true),
		},
	})
	d.IR.Blocks = append([]wordir.Block{{
		Index:       0,
		Kind:        "paragraph",
		Paragraph:   &title,
		FieldKind:   "TOC",
		FieldIndex:  len(d.IR.Fields) - 1,
	}}, d.IR.Blocks...)
	d.rebuildFlatSlices()
	return &TOC{doc: d, sf: sf}
}

// StoryField returns the underlying StoryField editor.
func (t *TOC) StoryField() *StoryField {
	if t == nil {
		return nil
	}
	return t.sf
}

// UpdateFromHeadings scans all body paragraphs for heading levels and
// regenerates the TOC result entries. Existing result blocks are cleared.
// Each entry contains the heading text; page numbers are emitted as
// placeholders since the library has no layout engine.
func (t *TOC) UpdateFromHeadings() {
	if t == nil || t.doc == nil || t.doc.IR == nil || t.sf == nil {
		return
	}
	// Clear existing result blocks.
	t.sf.ir.Blocks = nil

	for i := range t.doc.IR.Blocks {
		b := &t.doc.IR.Blocks[i]
		if b.Paragraph == nil {
			continue
		}
		p := b.Paragraph
		level := headingLevel(p)
		if level < 0 {
			continue
		}
		text := paragraphText(p)
		if text == "" {
			continue
		}
		t.addEntry(level, text)
	}
}

// headingLevel returns the heading level (0-based) for a paragraph, or -1 if
// the paragraph is not a heading.
func headingLevel(p *wordir.Paragraph) int {
	if p == nil {
		return -1
	}
	// Check explicit outline level first.
	if p.Format.OutlineLevel != nil {
		lvl := *p.Format.OutlineLevel
		if lvl >= 0 && lvl <= 8 {
			return lvl
		}
		return -1
	}
	// Fall back to style ID naming heuristics.
	sid := strings.ToLower(p.StyleID)
	for i := 1; i <= 9; i++ {
		if sid == fmt.Sprintf("heading%d", i) {
			return i - 1
		}
	}
	return -1
}

// addEntry creates a TOC entry paragraph with indentation matching the level.
func (t *TOC) addEntry(level int, text string) {
	if t.sf == nil {
		return
	}
	rp := t.sf.AddResultParagraph()
	if rp == nil {
		return
	}
	// Indent: 220 twips per level.
	if level > 0 {
		indent := level * 220
		rp.ir.Format.IndentLeft = &indent
	}
	// Entry text run.
	r1 := rp.AddRun()
	r1.SetText(text)
	// Tab + page number placeholder.
	r2 := rp.AddRun()
	r2.SetText("\t1")
}

func paragraphText(p *wordir.Paragraph) string {
	if p == nil {
		return ""
	}
	var s string
	for i := range p.Runs {
		s += p.Runs[i].Text
	}
	return s
}

func boolPtr(v bool) *bool {
	return &v
}