package edit

import wordir "office/pkg/word/ir"

// NumberingDef wraps an IR numbering definition for editing.
type NumberingDef struct {
	doc   *Document
	index int
}

func (n *NumberingDef) ir() *wordir.NumberingDefinition {
	if n.doc == nil || n.index < 0 || n.index >= len(n.doc.IR.NumberingDefinitions) {
		return nil
	}
	return &n.doc.IR.NumberingDefinitions[n.index]
}

// NumID returns the numbering instance ID.
func (n *NumberingDef) NumID() int {
	if ir := n.ir(); ir != nil {
		return ir.NumID
	}
	return 0
}

// AbstractNumID returns the abstract numbering ID.
func (n *NumberingDef) AbstractNumID() int {
	if ir := n.ir(); ir != nil {
		return ir.AbstractNumID
	}
	return 0
}

// SetLevel defines or updates a level in this numbering definition.
// It returns the level so that run formatting can be applied to the marker.
func (n *NumberingDef) SetLevel(level int, format, text string) *wordir.NumberingLevel {
	ir := n.ir()
	if ir == nil {
		return nil
	}
	for i := range ir.Levels {
		if ir.Levels[i].Level == level {
			ir.Levels[i].Format = format
			ir.Levels[i].Text = text
			return &ir.Levels[i]
		}
	}
	ir.Levels = append(ir.Levels, wordir.NumberingLevel{
		Level:  level,
		Format: format,
		Text:   text,
	})
	return &ir.Levels[len(ir.Levels)-1]
}

// Level returns the level definition for the given level, or nil if not defined.
func (n *NumberingDef) Level(level int) *wordir.NumberingLevel {
	ir := n.ir()
	if ir == nil {
		return nil
	}
	for i := range ir.Levels {
		if ir.Levels[i].Level == level {
			return &ir.Levels[i]
		}
	}
	return nil
}

// Levels returns all defined levels.
func (n *NumberingDef) Levels() []wordir.NumberingLevel {
	if ir := n.ir(); ir != nil {
		return append([]wordir.NumberingLevel(nil), ir.Levels...)
	}
	return nil
}

// SetStart sets the starting value for the given level.
func (n *NumberingDef) SetStart(level, start int) {
	ir := n.ir()
	if ir == nil {
		return
	}
	for i := range ir.Levels {
		if ir.Levels[i].Level == level {
			ir.Levels[i].Start = start
			return
		}
	}
	ir.Levels = append(ir.Levels, wordir.NumberingLevel{
		Level: level,
		Start: start,
	})
}

// SetLeftIndent sets the left indent (in twips) for the given level.
func (n *NumberingDef) SetLeftIndent(level, twips int) {
	ir := n.ir()
	if ir == nil {
		return
	}
	for i := range ir.Levels {
		if ir.Levels[i].Level == level {
			ir.Levels[i].IndentLeft = twips
			return
		}
	}
	ir.Levels = append(ir.Levels, wordir.NumberingLevel{
		Level:      level,
		IndentLeft: twips,
	})
}

// SetParagraphStyle links the numbering level to a paragraph style ID.
func (n *NumberingDef) SetParagraphStyle(level int, styleID string) {
	ir := n.ir()
	if ir == nil {
		return
	}
	for i := range ir.Levels {
		if ir.Levels[i].Level == level {
			ir.Levels[i].StyleID = styleID
			return
		}
	}
	ir.Levels = append(ir.Levels, wordir.NumberingLevel{
		Level:   level,
		StyleID: styleID,
	})
}