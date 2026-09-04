package edit

import (
	wordir "office/pkg/word/ir"
)

// Paragraph wraps an IR paragraph for editing.
type Paragraph struct {
	ir  *wordir.Paragraph
	doc *Document
}

// ElementType returns the body element type for this paragraph.
func (p *Paragraph) ElementType() string { return "paragraph" }

// AddRun appends a new run to the paragraph and returns it.
func (p *Paragraph) AddRun() *Run {
	if p.ir == nil {
		return nil
	}
	r := wordir.Run{Kind: "run"}
	p.ir.Runs = append(p.ir.Runs, r)
	return &Run{para: p, index: len(p.ir.Runs) - 1}
}

// InsertRunAt inserts a new run at the given position within the paragraph.
// If pos is negative, the run is inserted at the beginning. If pos exceeds
// the number of runs, it is appended.
func (p *Paragraph) InsertRunAt(pos int) *Run {
	if p.ir == nil {
		return nil
	}
	if pos < 0 {
		pos = 0
	}
	if pos > len(p.ir.Runs) {
		pos = len(p.ir.Runs)
	}
	r := wordir.Run{Kind: "run"}
	p.ir.Runs = append(p.ir.Runs, wordir.Run{})
	copy(p.ir.Runs[pos+1:], p.ir.Runs[pos:])
	p.ir.Runs[pos] = r
	return &Run{para: p, index: pos}
}

// InsertRunFrom inserts a deep copy of another run at the given position.
// If pos is negative, the run is inserted at the beginning. If pos exceeds
// the number of runs, it is appended. Returns nil if either paragraph or
// the source run is nil.
func (p *Paragraph) InsertRunFrom(pos int, other *Run) *Run {
	if p.ir == nil || other == nil {
		return nil
	}
	cloned := other.Clone()
	if cloned == nil {
		return nil
	}
	if pos < 0 {
		pos = 0
	}
	if pos > len(p.ir.Runs) {
		pos = len(p.ir.Runs)
	}
	p.ir.Runs = append(p.ir.Runs, wordir.Run{})
	copy(p.ir.Runs[pos+1:], p.ir.Runs[pos:])
	p.ir.Runs[pos] = *cloned.ir()
	return &Run{para: p, index: pos}
}

// AddRunFrom appends a deep copy of another run to the paragraph.
func (p *Paragraph) AddRunFrom(other *Run) *Run {
	if p.ir == nil || other == nil {
		return nil
	}
	cloned := other.Clone()
	if cloned == nil {
		return nil
	}
	p.ir.Runs = append(p.ir.Runs, *cloned.ir())
	return &Run{para: p, index: len(p.ir.Runs) - 1}
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	if p.ir == nil {
		return nil
	}
	out := make([]*Run, len(p.ir.Runs))
	for i := range p.ir.Runs {
		out[i] = &Run{para: p, index: i}
	}
	return out
}

// RunCount returns the number of runs in the paragraph.
func (p *Paragraph) RunCount() int {
	if p == nil || p.ir == nil {
		return 0
	}
	return len(p.ir.Runs)
}

// RunAt returns the run at index, or nil when index is out of range.
func (p *Paragraph) RunAt(index int) *Run {
	if p == nil || p.ir == nil || index < 0 || index >= len(p.ir.Runs) {
		return nil
	}
	return &Run{para: p, index: index}
}

// SetText replaces all runs with a single text run containing v.
func (p *Paragraph) SetText(v string) {
	if p.ir == nil {
		return
	}
	p.ir.Runs = []wordir.Run{{Kind: "run", Text: v}}
	p.ir.Text = v
}

// Text aggregates the text of all runs.
func (p *Paragraph) Text() string {
	if p.ir == nil {
		return ""
	}
	var s string
	for i := range p.ir.Runs {
		s += p.ir.Runs[i].Text
	}
	return s
}

// ContentControl returns the block-level content control wrapping this paragraph.
func (p *Paragraph) ContentControl() *ContentControl {
	if p == nil || p.ir == nil || p.doc == nil {
		return nil
	}
	cc := blockContentControlForParagraphInBlocks(p.doc.IR.Blocks, p.ir)
	if cc == nil {
		cc = blockContentControlForParagraphInStories(p.doc.headerPtrs, p.ir)
	}
	if cc == nil {
		cc = blockContentControlForParagraphInStories(p.doc.footerPtrs, p.ir)
	}
	if cc == nil {
		cc = blockContentControlForParagraphInStories(p.doc.footnotePtrs, p.ir)
	}
	if cc == nil {
		cc = blockContentControlForParagraphInStories(p.doc.endnotePtrs, p.ir)
	}
	if cc == nil {
		cc = blockContentControlForParagraphInStories(p.doc.commentPtrs, p.ir)
	}
	if cc == nil {
		return nil
	}
	return &ContentControl{ir: cc}
}

// SetContentControl wraps this paragraph in a block-level content control.
func (p *Paragraph) SetContentControl(cc *wordir.ContentControl) bool {
	if p == nil || p.doc == nil {
		return false
	}
	return p.doc.WrapParagraphInContentControl(p, cc)
}

// ClearContentControl removes the block-level content control wrapping this paragraph.
func (p *Paragraph) ClearContentControl() bool {
	if p == nil || p.doc == nil {
		return false
	}
	return p.doc.UnwrapBlockContentControlFromParagraph(p)
}

// AddBookmarkStart adds a bookmark start with the given name to this paragraph.
// The bookmark end will be co-located in the same paragraph on write-back.
func (p *Paragraph) AddBookmarkStart(name string) {
	if p.ir == nil || name == "" {
		return
	}
	p.ir.Bookmarks = append(p.ir.Bookmarks, name)
}

// RemoveBookmarkStart removes the first occurrence of the given bookmark name
// from this paragraph's bookmark starts.
func (p *Paragraph) RemoveBookmarkStart(name string) {
	if p.ir == nil || name == "" {
		return
	}
	for i, bm := range p.ir.Bookmarks {
		if bm == name {
			p.ir.Bookmarks = append(p.ir.Bookmarks[:i], p.ir.Bookmarks[i+1:]...)
			return
		}
	}
}

// BookmarkStarts returns the bookmark start names on this paragraph.
func (p *Paragraph) BookmarkStarts() []string {
	if p.ir == nil {
		return nil
	}
	out := make([]string, len(p.ir.Bookmarks))
	copy(out, p.ir.Bookmarks)
	return out
}

// SetStyle sets the paragraph style ID.
func (p *Paragraph) SetStyle(id string) {
	if p.ir == nil {
		return
	}
	p.ir.StyleID = id
}

// SetAlignment sets the paragraph alignment.
func (p *Paragraph) SetAlignment(v string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.Alignment = v
}

// SetSpacingBefore sets the spacing before the paragraph in twips.
func (p *Paragraph) SetSpacingBefore(twips int) {
	if p.ir == nil {
		return
	}
	p.ir.Format.SpacingBefore = &twips
}

// SetSpacingAfter sets the spacing after the paragraph in twips.
func (p *Paragraph) SetSpacingAfter(twips int) {
	if p.ir == nil {
		return
	}
	p.ir.Format.SpacingAfter = &twips
}

// SetIndentFirstLine sets the first-line indent in twips.
func (p *Paragraph) SetIndentFirstLine(twips int) {
	if p.ir == nil {
		return
	}
	p.ir.Format.IndentFirstLine = &twips
}

// SetIndentLeft sets the left indent in twips.
func (p *Paragraph) SetIndentLeft(twips int) {
	if p.ir == nil {
		return
	}
	p.ir.Format.IndentLeft = &twips
}

// SetIndentRight sets the right indent in twips.
func (p *Paragraph) SetIndentRight(twips int) {
	if p.ir == nil {
		return
	}
	p.ir.Format.IndentRight = &twips
}

// SetNumbering sets the numbering ID and level for the paragraph.
func (p *Paragraph) SetNumbering(id, level int) {
	if p.ir == nil {
		return
	}
	p.ir.Numbering = &wordir.Numbering{NumID: id, Level: level}
}

// ClearNumbering removes numbering from the paragraph.
func (p *Paragraph) ClearNumbering() {
	if p.ir == nil {
		return
	}
	p.ir.Numbering = nil
}

// RemoveRun removes a run from the paragraph by identity.
func (p *Paragraph) RemoveRun(r *Run) {
	if p.ir == nil || r == nil || r.para != p {
		return
	}
	idx := r.index
	if idx < 0 || idx >= len(p.ir.Runs) {
		return
	}
	p.ir.Runs = append(p.ir.Runs[:idx], p.ir.Runs[idx+1:]...)
}

// RemoveRunAt removes the run at the given index from the paragraph.
// If pos is out of range, this is a no-op.
func (p *Paragraph) RemoveRunAt(pos int) {
	if p.ir == nil || pos < 0 || pos >= len(p.ir.Runs) {
		return
	}
	p.ir.Runs = append(p.ir.Runs[:pos], p.ir.Runs[pos+1:]...)
}

// AddTab appends a tab-character run to the paragraph.
func (p *Paragraph) AddTab() *Run {
	if p.ir == nil {
		return nil
	}
	r := wordir.Run{Kind: "tab", Text: "\t"}
	p.ir.Runs = append(p.ir.Runs, r)
	return &Run{para: p, index: len(p.ir.Runs) - 1}
}

// AddPageBreak appends a page-break run to the paragraph.
func (p *Paragraph) AddPageBreak() *Run {
	if p.ir == nil {
		return nil
	}
	r := wordir.Run{Kind: "pageBreak", Text: ""}
	p.ir.Runs = append(p.ir.Runs, r)
	return &Run{para: p, index: len(p.ir.Runs) - 1}
}

// SetSpacing sets the paragraph spacing.
// before and after are in twips; line is line height (twips for exact/atLeast, 240ths for auto);
// lineRule is "auto", "exact", or "atLeast".
func (p *Paragraph) SetSpacing(before, after, line int, lineRule string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.SpacingBefore = &before
	p.ir.Format.SpacingAfter = &after
	p.ir.Format.LineSpacing = &line
	p.ir.Format.LineRule = lineRule
}

// SetLineSpacing sets the line spacing independently of before/after spacing.
// line is line height (twips for exact/atLeast, 240ths for auto);
// rule is "auto", "exact", or "atLeast".
func (p *Paragraph) SetLineSpacing(line int, rule string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.LineSpacing = &line
	p.ir.Format.LineRule = rule
}

// SetBidi sets the paragraph text direction to right-to-left.
func (p *Paragraph) SetBidi(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.Bidi = v
}

// SetIndentation sets all paragraph indentation values in twips.
func (p *Paragraph) SetIndentation(left, right, firstLine, hanging int) {
	if p.ir == nil {
		return
	}
	if left != 0 {
		p.ir.Format.IndentLeft = &left
	} else {
		p.ir.Format.IndentLeft = nil
	}
	if right != 0 {
		p.ir.Format.IndentRight = &right
	} else {
		p.ir.Format.IndentRight = nil
	}
	if firstLine != 0 {
		p.ir.Format.IndentFirstLine = &firstLine
		p.ir.Format.IndentHanging = nil
	} else if hanging != 0 {
		p.ir.Format.IndentHanging = &hanging
		p.ir.Format.IndentFirstLine = nil
	} else {
		p.ir.Format.IndentFirstLine = nil
		p.ir.Format.IndentHanging = nil
	}
}

// SetOutlineLevel sets the heading outline level (0-8).
func (p *Paragraph) SetOutlineLevel(level int) {
	if p.ir == nil {
		return
	}
	if level < 0 || level > 8 {
		p.ir.Format.OutlineLevel = nil
		return
	}
	p.ir.Format.OutlineLevel = &level
}

// SetContextualSpacing toggles whether space between same-style paragraphs is ignored.
func (p *Paragraph) SetContextualSpacing(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.ContextualSpacing = v
}

// SetKeepLines keeps all lines of the paragraph on the same page.
func (p *Paragraph) SetKeepLines(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.KeepLines = v
}

// SetKeepNext keeps this paragraph on the same page as the next one.
func (p *Paragraph) SetKeepNext(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.KeepNext = v
}

// SetPageBreakBefore forces a page break before this paragraph.
func (p *Paragraph) SetPageBreakBefore(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.PageBreakBefore = v
}

// SetWidowControl keeps at least two lines on each side of a page break.
func (p *Paragraph) SetWidowControl(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.WidowControl = v
}

// SetShading sets the paragraph background color as a 6-digit hex string.
func (p *Paragraph) SetShading(hex string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.Shading = hex
}

// SetBorder sets a paragraph border.
// position is "top", "left", "bottom", or "right".
// style is "single", "double", "dashed", "dotted", etc.
// sizePt is the border width in points.
// color is a 6-digit hex string or "auto".
func (p *Paragraph) SetBorder(position, style string, sizePt float64, color string) {
	if p.ir == nil {
		return
	}
	if p.ir.Format.Borders == nil {
		p.ir.Format.Borders = &wordir.ParagraphBorders{}
	}
	b := &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	switch position {
	case "top":
		p.ir.Format.Borders.Top = b
	case "left":
		p.ir.Format.Borders.Left = b
	case "bottom":
		p.ir.Format.Borders.Bottom = b
	case "right":
		p.ir.Format.Borders.Right = b
	}
}

// ClearBorders removes all paragraph borders.
func (p *Paragraph) ClearBorders() {
	if p.ir != nil {
		p.ir.Format.Borders = nil
	}
}

// Style returns the paragraph style ID.
func (p *Paragraph) Style() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.StyleID
}

// Alignment returns the paragraph alignment.
func (p *Paragraph) Alignment() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.Format.Alignment
}

// SpacingBefore returns the spacing before in twips, or 0 if not set.
func (p *Paragraph) SpacingBefore() int {
	if p == nil || p.ir == nil || p.ir.Format.SpacingBefore == nil {
		return 0
	}
	return *p.ir.Format.SpacingBefore
}

// SpacingAfter returns the spacing after in twips, or 0 if not set.
func (p *Paragraph) SpacingAfter() int {
	if p == nil || p.ir == nil || p.ir.Format.SpacingAfter == nil {
		return 0
	}
	return *p.ir.Format.SpacingAfter
}

// LineSpacing returns the line spacing in twips, or 0 if not set.
func (p *Paragraph) LineSpacing() int {
	if p == nil || p.ir == nil || p.ir.Format.LineSpacing == nil {
		return 0
	}
	return *p.ir.Format.LineSpacing
}

// LineRule returns the line rule ("auto", "exact", "atLeast", or "").
func (p *Paragraph) LineRule() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.Format.LineRule
}

// IndentFirstLine returns the first-line indent in twips, or 0 if not set.
func (p *Paragraph) IndentFirstLine() int {
	if p == nil || p.ir == nil || p.ir.Format.IndentFirstLine == nil {
		return 0
	}
	return *p.ir.Format.IndentFirstLine
}

// IndentLeft returns the left indent in twips, or 0 if not set.
func (p *Paragraph) IndentLeft() int {
	if p == nil || p.ir == nil || p.ir.Format.IndentLeft == nil {
		return 0
	}
	return *p.ir.Format.IndentLeft
}

// IndentRight returns the right indent in twips, or 0 if not set.
func (p *Paragraph) IndentRight() int {
	if p == nil || p.ir == nil || p.ir.Format.IndentRight == nil {
		return 0
	}
	return *p.ir.Format.IndentRight
}

// Numbering returns the paragraph numbering info, or nil if none is set.
func (p *Paragraph) Numbering() *wordir.Numbering {
	if p == nil || p.ir == nil {
		return nil
	}
	return p.ir.Numbering
}

// OutlineLevel returns the outline level, or -1 if not set.
func (p *Paragraph) OutlineLevel() int {
	if p == nil || p.ir == nil || p.ir.Format.OutlineLevel == nil {
		return -1
	}
	return *p.ir.Format.OutlineLevel
}

// ContextualSpacing reports whether contextual spacing is enabled.
func (p *Paragraph) ContextualSpacing() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.ContextualSpacing
}

// KeepLines reports whether all lines are kept together.
func (p *Paragraph) KeepLines() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.KeepLines
}

// KeepNext reports whether the paragraph is kept with the next.
func (p *Paragraph) KeepNext() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.KeepNext
}

// PageBreakBefore reports whether a page break is forced before the paragraph.
func (p *Paragraph) PageBreakBefore() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.PageBreakBefore
}

// WidowControl reports whether widow/orphan control is enabled.
func (p *Paragraph) WidowControl() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.WidowControl
}

// Shading returns the paragraph background color.
func (p *Paragraph) Shading() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.Format.Shading
}

// Bidi reports whether the paragraph is right-to-left.
func (p *Paragraph) Bidi() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.Bidi
}

// NoLineNumber reports whether line numbering is suppressed for this paragraph.
func (p *Paragraph) NoLineNumber() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.NoLineNumber
}

// SnapToGrid reports whether the paragraph snaps to the document grid.
func (p *Paragraph) SnapToGrid() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.SnapToGrid
}

// Kinsoku reports whether kinsoku shori is enabled.
func (p *Paragraph) Kinsoku() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.Kinsoku
}

// WordWrap reports whether Latin text wraps at mid-word.
func (p *Paragraph) WordWrap() bool {
	if p == nil || p.ir == nil || p.ir.Format.WordWrap == nil {
		return false
	}
	return *p.ir.Format.WordWrap
}

// OverflowPunct reports whether punctuation can overflow the margin.
func (p *Paragraph) OverflowPunct() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.OverflowPunct
}

// TopLinePunct reports whether top-line punctuation compression is enabled.
func (p *Paragraph) TopLinePunct() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.TopLinePunct
}

// AutoSpaceDE reports whether auto-spacing between East Asian and Latin text is enabled.
func (p *Paragraph) AutoSpaceDE() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.AutoSpaceDE
}

// AutoSpaceDN reports whether auto-spacing between East Asian text and numbers is enabled.
func (p *Paragraph) AutoSpaceDN() bool {
	if p == nil || p.ir == nil {
		return false
	}
	return p.ir.Format.AutoSpaceDN
}

// FontAlign returns the font alignment ("top", "center", "baseline", "bottom", or "").
func (p *Paragraph) FontAlign() string {
	if p == nil || p.ir == nil {
		return ""
	}
	return p.ir.Format.FontAlign
}

// AddTabStop adds a custom tab stop.
// pos is in twips; align is "left", "center", "right", or "decimal";
// leader is "none", "dot", "hyphen", "underscore", or "middleDot".
func (p *Paragraph) AddTabStop(pos int, align, leader string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.TabStops = append(p.ir.Format.TabStops, wordir.TabStop{
		Pos:    pos,
		Algn:   align,
		Leader: leader,
	})
}

// SetSuppressLineNumbers suppresses line numbering for this paragraph.
func (p *Paragraph) SetSuppressLineNumbers(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.NoLineNumber = v
}

// SetSnapToGrid snaps paragraph lines to the document grid.
func (p *Paragraph) SetSnapToGrid(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.SnapToGrid = v
}

// SetKinsoku enables Japanese/Chinese line breaking rules.
func (p *Paragraph) SetKinsoku(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.Kinsoku = v
}

// SetWordWrap sets word wrap vs character wrap.
func (p *Paragraph) SetWordWrap(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.WordWrap = &v
}

// SetOverflowPunct allows punctuation to extend beyond the margin.
func (p *Paragraph) SetOverflowPunct(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.OverflowPunct = v
}

// SetTopLinePunct compresses punctuation at the start of a line.
func (p *Paragraph) SetTopLinePunct(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.TopLinePunct = v
}

// SetAutoSpaceDE auto-spaces between East Asian and Latin text.
func (p *Paragraph) SetAutoSpaceDE(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.AutoSpaceDE = v
}

// SetAutoSpaceDN auto-spaces between East Asian and numbers.
func (p *Paragraph) SetAutoSpaceDN(v bool) {
	if p.ir == nil {
		return
	}
	p.ir.Format.AutoSpaceDN = v
}

// SetFontAlign sets the vertical font alignment for the paragraph.
// Values: "auto", "top", "center", "baseline", "bottom".
func (p *Paragraph) SetFontAlign(v string) {
	if p.ir == nil {
		return
	}
	p.ir.Format.FontAlign = v
}

// SetFrameProps sets paragraph frame positioning properties.
// nil removes the frame.
func (p *Paragraph) SetFrameProps(props *wordir.FrameProps) {
	if p.ir == nil {
		return
	}
	if props == nil {
		p.ir.Frame = nil
		return
	}
	copyProps := *props
	p.ir.Frame = &copyProps
}