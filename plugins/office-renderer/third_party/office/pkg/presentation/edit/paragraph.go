package edit

import (
	presir "office/pkg/presentation/ir"
)

// Paragraph wraps an IR paragraph for editing.
type Paragraph struct {
	shape *Shape
	cell  *Cell
	ir    *presir.Paragraph
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	if p == nil || p.ir == nil {
		return nil
	}
	out := make([]*Run, len(p.ir.Runs))
	for i, r := range p.ir.Runs {
		out[i] = &Run{paragraph: p, ir: r}
	}
	return out
}

// AddRun adds a new text run and returns it.
func (p *Paragraph) AddRun(text string) *Run {
	if p == nil || p.ir == nil {
		return nil
	}
	r := &presir.Run{Text: text}
	p.ir.Runs = append(p.ir.Runs, r)
	return &Run{paragraph: p, ir: r}
}

// SetText replaces all runs with a single run containing the given text.
func (p *Paragraph) SetText(text string) {
	if p == nil || p.ir == nil {
		return
	}
	p.ir.Runs = []*presir.Run{{Text: text}}
}

// Text returns the concatenated plain text of all runs.
func (p *Paragraph) Text() string {
	if p == nil || p.ir == nil {
		return ""
	}
	var out string
	for _, r := range p.ir.Runs {
		out += r.Text
	}
	return out
}

// SetAlignment sets the paragraph alignment.
// Values: "l" (left), "ctr" (center), "r" (right), "just" (justify).
func (p *Paragraph) SetAlignment(align string) {
	if p == nil || p.ir == nil {
		return
	}
	p.ir.Align = align
}

// Alignment returns the paragraph alignment.
func (p *Paragraph) Alignment() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.Align
}

// SetLevel sets the indentation level (0–8).
func (p *Paragraph) SetLevel(level int) {
	if p == nil || p.ir == nil {
		return
	}
	p.ir.Level = level
}

// Level returns the indentation level.
func (p *Paragraph) Level() int {
	if p == nil || p.ir == nil {
		return 0
	}
	return p.ir.Level
}

// SetFontSize sets the default font size for the paragraph in hundredths of a point.
func (p *Paragraph) SetFontSize(size int) {
	if p == nil || p.ir == nil {
		return
	}
	for _, r := range p.ir.Runs {
		r.FontSize = size
	}
}
