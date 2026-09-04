package edit

import wordir "office/pkg/word/ir"

// List wraps a numbering definition and provides a high-level API for creating
// and managing bulleted or numbered list paragraphs.
type List struct {
	doc   *Document
	numID int
}

// CreateList creates a new list in the document. It generates a numbering
// definition with a fresh NumID and returns a List handle. The list starts
// empty; use AddItem / AddItemLevel to append paragraphs.
func (d *Document) CreateList() *List {
	if d == nil || d.IR == nil {
		return nil
	}
	numID := d.nextListNumID()
	abstractNumID := numID
	d.IR.NumberingDefinitions = append(d.IR.NumberingDefinitions, wordir.NumberingDefinition{
		NumID:         numID,
		AbstractNumID: abstractNumID,
	})
	return &List{doc: d, numID: numID}
}

// nextListNumID returns a NumID that is not currently used by any numbering
// definition in the document.
func (d *Document) nextListNumID() int {
	maxID := 1
	for _, nd := range d.IR.NumberingDefinitions {
		if nd.NumID > maxID {
			maxID = nd.NumID
		}
	}
	return maxID + 1
}

// NumID returns the numbering ID associated with this list.
func (l *List) NumID() int {
	if l == nil {
		return 0
	}
	return l.numID
}

// AddItem appends a new paragraph to the document body and marks it as a list
// item at level 0 for this list.
func (l *List) AddItem() *Paragraph {
	return l.AddItemLevel(0)
}

// AddItemLevel appends a new paragraph and marks it as a list item at the
// specified level for this list.
func (l *List) AddItemLevel(level int) *Paragraph {
	if l == nil || l.doc == nil || l.doc.IR == nil {
		return nil
	}
	p := l.doc.AddParagraph()
	if p == nil {
		return nil
	}
	p.ir.Numbering = &wordir.Numbering{NumID: l.numID, Level: level}
	return p
}

// Paragraphs returns all body paragraphs that belong to this list, in document
// order. It matches paragraphs by NumID.
func (l *List) Paragraphs() []*Paragraph {
	if l == nil || l.doc == nil || l.doc.IR == nil {
		return nil
	}
	var out []*Paragraph
	for i := range l.doc.IR.Blocks {
		if l.doc.IR.Blocks[i].Paragraph == nil {
			continue
		}
		p := l.doc.IR.Blocks[i].Paragraph
		if p.Numbering != nil && p.Numbering.NumID == l.numID {
			out = append(out, &Paragraph{ir: p, doc: l.doc})
		}
	}
	return out
}

// Count returns the number of body paragraphs currently assigned to this list.
func (l *List) Count() int {
	return len(l.Paragraphs())
}

// SetLevelStyle configures the appearance of a level in this list.
// format is the numbering format such as "decimal", "bullet", "lowerLetter",
// "upperRoman", etc. text is the level text (e.g. "%1." or "\u2022").
func (l *List) SetLevelStyle(level int, format, text string) {
	if l == nil || l.doc == nil || l.doc.IR == nil {
		return
	}
	for i := range l.doc.IR.NumberingDefinitions {
		if l.doc.IR.NumberingDefinitions[i].NumID == l.numID {
			nd := &l.doc.IR.NumberingDefinitions[i]
			for j := range nd.Levels {
				if nd.Levels[j].Level == level {
					nd.Levels[j].Format = format
					nd.Levels[j].Text = text
					return
				}
			}
			nd.Levels = append(nd.Levels, wordir.NumberingLevel{
				Level:  level,
				Format: format,
				Text:   text,
			})
			return
		}
	}
}

// SetList assigns this paragraph to the given list at the specified level.
// A level of -1 removes list membership.
func (p *Paragraph) SetList(list *List, level int) {
	if p.ir == nil {
		return
	}
	if list == nil || level < 0 {
		p.ir.Numbering = nil
		return
	}
	p.ir.Numbering = &wordir.Numbering{NumID: list.NumID(), Level: level}
}

// NumID returns the numbering ID for this paragraph, or 0 if none.
func (p *Paragraph) NumID() int {
	if p.ir == nil || p.ir.Numbering == nil {
		return 0
	}
	return p.ir.Numbering.NumID
}

// ListLevel returns the list level for this paragraph, or -1 if the paragraph
// is not a list item.
func (p *Paragraph) ListLevel() int {
	if p.ir == nil || p.ir.Numbering == nil {
		return -1
	}
	return p.ir.Numbering.Level
}

// IsListItem reports whether the paragraph belongs to a numbered/bulleted list.
func (p *Paragraph) IsListItem() bool {
	return p.ir != nil && p.ir.Numbering != nil
}