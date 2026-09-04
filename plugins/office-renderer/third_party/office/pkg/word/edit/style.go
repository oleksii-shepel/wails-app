package edit

import wordir "office/pkg/word/ir"

// ParagraphStyle wraps an IR paragraph style for editing.
type ParagraphStyle struct {
	doc *Document
	id  string
}

func (s *ParagraphStyle) read() wordir.Style {
	if s == nil || s.doc == nil || s.doc.IR.Styles == nil {
		return wordir.Style{}
	}
	return s.doc.IR.Styles[s.id]
}

func (s *ParagraphStyle) write(st wordir.Style) {
	if s == nil || s.doc == nil || s.doc.IR.Styles == nil {
		return
	}
	s.doc.IR.Styles[s.id] = st
}

// ID returns the style ID.
func (s *ParagraphStyle) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// SetName sets the style name.
func (s *ParagraphStyle) SetName(v string) {
	st := s.read()
	st.Name = v
	s.write(st)
}

// Name returns the style name.
func (s *ParagraphStyle) Name() string { return s.read().Name }

// SetBasedOn sets the ID of the parent style.
func (s *ParagraphStyle) SetBasedOn(id string) {
	st := s.read()
	st.BasedOn = id
	s.write(st)
}

// BasedOn returns the ID of the parent style.
func (s *ParagraphStyle) BasedOn() string { return s.read().BasedOn }

// SetDefault marks this style as the default for its type.
func (s *ParagraphStyle) SetDefault(v bool) {
	st := s.read()
	st.Default = v
	s.write(st)
}

// Default reports whether this style is the default for its type.
func (s *ParagraphStyle) Default() bool { return s.read().Default }

// SetLinkedCharID sets the ID of the linked character style.
func (s *ParagraphStyle) SetLinkedCharID(id string) {
	st := s.read()
	st.LinkedCharID = id
	s.write(st)
}

// LinkedCharID returns the ID of the linked character style.
func (s *ParagraphStyle) LinkedCharID() string { return s.read().LinkedCharID }

// SetNextStyleID sets the next style ID.
func (s *ParagraphStyle) SetNextStyleID(v string) {
	st := s.read()
	st.NextStyleID = v
	s.write(st)
}

// NextStyleID returns the next style ID.
func (s *ParagraphStyle) NextStyleID() string { return s.read().NextStyleID }

// SetUIPriority sets the UI priority.
func (s *ParagraphStyle) SetUIPriority(v int) {
	st := s.read()
	st.UIPriority = v
	s.write(st)
}

// UIPriority returns the UI priority.
func (s *ParagraphStyle) UIPriority() int { return s.read().UIPriority }

// SetRSID sets the RSID.
func (s *ParagraphStyle) SetRSID(v string) {
	st := s.read()
	st.RSID = v
	s.write(st)
}

// RSID returns the RSID.
func (s *ParagraphStyle) RSID() string { return s.read().RSID }

// SetUnhideWhenUsed sets the unhide-when-used flag.
func (s *ParagraphStyle) SetUnhideWhenUsed(v bool) {
	st := s.read()
	st.UnhideWhenUsed = v
	s.write(st)
}

// UnhideWhenUsed returns the unhide-when-used flag.
func (s *ParagraphStyle) UnhideWhenUsed() bool { return s.read().UnhideWhenUsed }

// SetQFormat sets the quick-format flag.
func (s *ParagraphStyle) SetQFormat(v bool) {
	st := s.read()
	st.QFormat = v
	s.write(st)
}

// QFormat returns the quick-format flag.
func (s *ParagraphStyle) QFormat() bool { return s.read().QFormat }

// SetSemiHidden sets the semi-hidden flag.
func (s *ParagraphStyle) SetSemiHidden(v bool) {
	st := s.read()
	st.SemiHidden = v
	s.write(st)
}

// SemiHidden returns the semi-hidden flag.
func (s *ParagraphStyle) SemiHidden() bool { return s.read().SemiHidden }

// SetAutoRedefine sets the auto-redefine flag.
func (s *ParagraphStyle) SetAutoRedefine(v bool) {
	st := s.read()
	st.AutoRedefine = v
	s.write(st)
}

// AutoRedefine returns the auto-redefine flag.
func (s *ParagraphStyle) AutoRedefine() bool { return s.read().AutoRedefine }

// SetAlignment sets the paragraph alignment.
func (s *ParagraphStyle) SetAlignment(v string) {
	st := s.read()
	st.Format.Alignment = v
	s.write(st)
}

// Alignment returns the paragraph alignment.
func (s *ParagraphStyle) Alignment() string { return s.read().Format.Alignment }

// SetSpacingBefore sets the spacing before in twips.
func (s *ParagraphStyle) SetSpacingBefore(twips int) {
	st := s.read()
	st.Format.SpacingBefore = &twips
	s.write(st)
}

// SpacingBefore returns the spacing before in twips, or 0 if not set.
func (s *ParagraphStyle) SpacingBefore() int {
	v := s.read().Format.SpacingBefore
	if v == nil {
		return 0
	}
	return *v
}

// SetSpacingAfter sets the spacing after in twips.
func (s *ParagraphStyle) SetSpacingAfter(twips int) {
	st := s.read()
	st.Format.SpacingAfter = &twips
	s.write(st)
}

// SpacingAfter returns the spacing after in twips, or 0 if not set.
func (s *ParagraphStyle) SpacingAfter() int {
	v := s.read().Format.SpacingAfter
	if v == nil {
		return 0
	}
	return *v
}

// SetLineSpacing sets the line spacing in twips.
func (s *ParagraphStyle) SetLineSpacing(twips int) {
	st := s.read()
	st.Format.LineSpacing = &twips
	s.write(st)
}

// LineSpacing returns the line spacing in twips, or 0 if not set.
func (s *ParagraphStyle) LineSpacing() int {
	v := s.read().Format.LineSpacing
	if v == nil {
		return 0
	}
	return *v
}

// SetLineRule sets the line rule: "auto", "exact", or "atLeast".
func (s *ParagraphStyle) SetLineRule(v string) {
	st := s.read()
	st.Format.LineRule = v
	s.write(st)
}

// LineRule returns the line rule.
func (s *ParagraphStyle) LineRule() string { return s.read().Format.LineRule }

// SetIndentFirstLine sets the first-line indent in twips.
func (s *ParagraphStyle) SetIndentFirstLine(twips int) {
	st := s.read()
	st.Format.IndentFirstLine = &twips
	s.write(st)
}

// IndentFirstLine returns the first-line indent in twips, or 0 if not set.
func (s *ParagraphStyle) IndentFirstLine() int {
	v := s.read().Format.IndentFirstLine
	if v == nil {
		return 0
	}
	return *v
}

// SetIndentLeft sets the left indent in twips.
func (s *ParagraphStyle) SetIndentLeft(twips int) {
	st := s.read()
	st.Format.IndentLeft = &twips
	s.write(st)
}

// IndentLeft returns the left indent in twips, or 0 if not set.
func (s *ParagraphStyle) IndentLeft() int {
	v := s.read().Format.IndentLeft
	if v == nil {
		return 0
	}
	return *v
}

// SetIndentRight sets the right indent in twips.
func (s *ParagraphStyle) SetIndentRight(twips int) {
	st := s.read()
	st.Format.IndentRight = &twips
	s.write(st)
}

// IndentRight returns the right indent in twips, or 0 if not set.
func (s *ParagraphStyle) IndentRight() int {
	v := s.read().Format.IndentRight
	if v == nil {
		return 0
	}
	return *v
}

// SetNumbering sets the numbering ID and level for this style.
func (s *ParagraphStyle) SetNumbering(numID, level int) {
	st := s.read()
	st.Numbering = &wordir.NumberingReference{NumID: numID, Level: level}
	s.write(st)
}

// Numbering returns the numbering reference, or nil if none is set.
func (s *ParagraphStyle) Numbering() *wordir.NumberingReference { return s.read().Numbering }

// SetKeepNext keeps the paragraph on the same page as the next paragraph.
func (s *ParagraphStyle) SetKeepNext(v bool) {
	st := s.read()
	st.Format.KeepNext = v
	s.write(st)
}

// KeepNext reports whether the paragraph is kept with the next.
func (s *ParagraphStyle) KeepNext() bool { return s.read().Format.KeepNext }

// SetKeepLines keeps all lines of the paragraph together.
func (s *ParagraphStyle) SetKeepLines(v bool) {
	st := s.read()
	st.Format.KeepLines = v
	s.write(st)
}

// KeepLines reports whether all lines are kept together.
func (s *ParagraphStyle) KeepLines() bool { return s.read().Format.KeepLines }

// SetWidowControl enables widow/orphan control.
func (s *ParagraphStyle) SetWidowControl(v bool) {
	st := s.read()
	st.Format.WidowControl = v
	s.write(st)
}

// WidowControl reports whether widow/orphan control is enabled.
func (s *ParagraphStyle) WidowControl() bool { return s.read().Format.WidowControl }

// SetPageBreakBefore forces a page break before the paragraph.
func (s *ParagraphStyle) SetPageBreakBefore(v bool) {
	st := s.read()
	st.Format.PageBreakBefore = v
	s.write(st)
}

// PageBreakBefore reports whether a page break is forced before the paragraph.
func (s *ParagraphStyle) PageBreakBefore() bool { return s.read().Format.PageBreakBefore }

// SetShading sets the paragraph background color.
func (s *ParagraphStyle) SetShading(v string) {
	st := s.read()
	st.Format.Shading = v
	s.write(st)
}

// Shading returns the paragraph background color.
func (s *ParagraphStyle) Shading() string { return s.read().Format.Shading }

// SetContextualSpacing suppresses space between adjacent same-style paragraphs.
func (s *ParagraphStyle) SetContextualSpacing(v bool) {
	st := s.read()
	st.Format.ContextualSpacing = v
	s.write(st)
}

// ContextualSpacing reports whether contextual spacing is enabled.
func (s *ParagraphStyle) ContextualSpacing() bool { return s.read().Format.ContextualSpacing }

// SetRunBold sets the default bold flag for runs in this style.
func (s *ParagraphStyle) SetRunBold(v bool) {
	st := s.read()
	st.RunFormat.Bold = &v
	s.write(st)
}

// RunBold returns the default bold flag, or false if not set.
func (s *ParagraphStyle) RunBold() bool {
	v := s.read().RunFormat.Bold
	if v == nil {
		return false
	}
	return *v
}

// SetRunItalic sets the default italic flag.
func (s *ParagraphStyle) SetRunItalic(v bool) {
	st := s.read()
	st.RunFormat.Italic = &v
	s.write(st)
}

// RunItalic returns the default italic flag, or false if not set.
func (s *ParagraphStyle) RunItalic() bool {
	v := s.read().RunFormat.Italic
	if v == nil {
		return false
	}
	return *v
}

// SetRunFontSize sets the default font size in half-points.
func (s *ParagraphStyle) SetRunFontSize(hp int) {
	st := s.read()
	st.RunFormat.FontSize = &hp
	s.write(st)
}

// RunFontSize returns the default font size in half-points, or 0 if not set.
func (s *ParagraphStyle) RunFontSize() int {
	v := s.read().RunFormat.FontSize
	if v == nil {
		return 0
	}
	return *v
}

// SetRunColor sets the default font color.
func (s *ParagraphStyle) SetRunColor(hex string) {
	st := s.read()
	st.RunFormat.Color = hex
	s.write(st)
}

// RunColor returns the default font color.
func (s *ParagraphStyle) RunColor() string { return s.read().RunFormat.Color }

// SetRunFontFamily sets the default ASCII font family.
func (s *ParagraphStyle) SetRunFontFamily(v string) {
	st := s.read()
	st.RunFormat.FontASCII = v
	s.write(st)
}

// RunFontFamily returns the default ASCII font family.
func (s *ParagraphStyle) RunFontFamily() string { return s.read().RunFormat.FontASCII }

// CharacterStyle wraps an IR character style for editing.
type CharacterStyle struct {
	doc *Document
	id  string
}

func (s *CharacterStyle) read() wordir.CharacterStyle {
	if s == nil || s.doc == nil || s.doc.IR.CharacterStyles == nil {
		return wordir.CharacterStyle{}
	}
	return s.doc.IR.CharacterStyles[s.id]
}

func (s *CharacterStyle) write(st wordir.CharacterStyle) {
	if s == nil || s.doc == nil || s.doc.IR.CharacterStyles == nil {
		return
	}
	s.doc.IR.CharacterStyles[s.id] = st
}

// ID returns the style ID.
func (s *CharacterStyle) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// SetName sets the style name.
func (s *CharacterStyle) SetName(v string) {
	st := s.read()
	st.Name = v
	s.write(st)
}

// Name returns the style name.
func (s *CharacterStyle) Name() string { return s.read().Name }

// SetBasedOn sets the ID of the parent style.
func (s *CharacterStyle) SetBasedOn(id string) {
	st := s.read()
	st.BasedOn = id
	s.write(st)
}

// BasedOn returns the ID of the parent style.
func (s *CharacterStyle) BasedOn() string { return s.read().BasedOn }

// SetNextStyleID sets the next style ID.
func (s *CharacterStyle) SetNextStyleID(v string) {
	st := s.read()
	st.NextStyleID = v
	s.write(st)
}

// NextStyleID returns the next style ID.
func (s *CharacterStyle) NextStyleID() string { return s.read().NextStyleID }

// SetUIPriority sets the UI priority.
func (s *CharacterStyle) SetUIPriority(v int) {
	st := s.read()
	st.UIPriority = v
	s.write(st)
}

// UIPriority returns the UI priority.
func (s *CharacterStyle) UIPriority() int { return s.read().UIPriority }

// SetRSID sets the RSID.
func (s *CharacterStyle) SetRSID(v string) {
	st := s.read()
	st.RSID = v
	s.write(st)
}

// RSID returns the RSID.
func (s *CharacterStyle) RSID() string { return s.read().RSID }

// SetUnhideWhenUsed sets the unhide-when-used flag.
func (s *CharacterStyle) SetUnhideWhenUsed(v bool) {
	st := s.read()
	st.UnhideWhenUsed = v
	s.write(st)
}

// UnhideWhenUsed returns the unhide-when-used flag.
func (s *CharacterStyle) UnhideWhenUsed() bool { return s.read().UnhideWhenUsed }

// SetQFormat sets the quick-format flag.
func (s *CharacterStyle) SetQFormat(v bool) {
	st := s.read()
	st.QFormat = v
	s.write(st)
}

// QFormat returns the quick-format flag.
func (s *CharacterStyle) QFormat() bool { return s.read().QFormat }

// SetSemiHidden sets the semi-hidden flag.
func (s *CharacterStyle) SetSemiHidden(v bool) {
	st := s.read()
	st.SemiHidden = v
	s.write(st)
}

// SemiHidden returns the semi-hidden flag.
func (s *CharacterStyle) SemiHidden() bool { return s.read().SemiHidden }

// SetAutoRedefine sets the auto-redefine flag.
func (s *CharacterStyle) SetAutoRedefine(v bool) {
	st := s.read()
	st.AutoRedefine = v
	s.write(st)
}

// AutoRedefine returns the auto-redefine flag.
func (s *CharacterStyle) AutoRedefine() bool { return s.read().AutoRedefine }

// SetBold sets the bold flag.
func (s *CharacterStyle) SetBold(v bool) {
	st := s.read()
	st.Format.Bold = &v
	s.write(st)
}

// Bold returns the bold flag, or false if not set.
func (s *CharacterStyle) Bold() bool {
	v := s.read().Format.Bold
	if v == nil {
		return false
	}
	return *v
}

// SetItalic sets the italic flag.
func (s *CharacterStyle) SetItalic(v bool) {
	st := s.read()
	st.Format.Italic = &v
	s.write(st)
}

// Italic returns the italic flag, or false if not set.
func (s *CharacterStyle) Italic() bool {
	v := s.read().Format.Italic
	if v == nil {
		return false
	}
	return *v
}

// SetUnderline sets the underline style.
func (s *CharacterStyle) SetUnderline(v string) {
	st := s.read()
	st.Format.Underline = v
	s.write(st)
}

// Underline returns the underline style.
func (s *CharacterStyle) Underline() string { return s.read().Format.Underline }

// SetFontSize sets the font size in half-points.
func (s *CharacterStyle) SetFontSize(hp int) {
	st := s.read()
	st.Format.FontSize = &hp
	s.write(st)
}

// FontSize returns the font size in half-points, or 0 if not set.
func (s *CharacterStyle) FontSize() int {
	v := s.read().Format.FontSize
	if v == nil {
		return 0
	}
	return *v
}

// SetColor sets the font color.
func (s *CharacterStyle) SetColor(hex string) {
	st := s.read()
	st.Format.Color = hex
	s.write(st)
}

// Color returns the font color.
func (s *CharacterStyle) Color() string { return s.read().Format.Color }

// SetFontFamily sets the ASCII font family.
func (s *CharacterStyle) SetFontFamily(v string) {
	st := s.read()
	st.Format.FontASCII = v
	s.write(st)
}

// FontFamily returns the ASCII font family.
func (s *CharacterStyle) FontFamily() string { return s.read().Format.FontASCII }

// SetCaps sets the all-caps flag.
func (s *CharacterStyle) SetCaps(v bool) {
	st := s.read()
	st.Format.Caps = &v
	s.write(st)
}

// Caps returns the all-caps flag, or false if not set.
func (s *CharacterStyle) Caps() bool {
	v := s.read().Format.Caps
	if v == nil {
		return false
	}
	return *v
}

// SetSmallCaps sets the small-caps flag.
func (s *CharacterStyle) SetSmallCaps(v bool) {
	st := s.read()
	st.Format.SmallCaps = &v
	s.write(st)
}

// SmallCaps returns the small-caps flag, or false if not set.
func (s *CharacterStyle) SmallCaps() bool {
	v := s.read().Format.SmallCaps
	if v == nil {
		return false
	}
	return *v
}

// SetStrike sets the strikethrough flag.
func (s *CharacterStyle) SetStrike(v bool) {
	st := s.read()
	st.Format.Strike = &v
	s.write(st)
}

// Strike returns the strikethrough flag, or false if not set.
func (s *CharacterStyle) Strike() bool {
	v := s.read().Format.Strike
	if v == nil {
		return false
	}
	return *v
}

// SetHighlight sets the highlight color.
func (s *CharacterStyle) SetHighlight(v string) {
	st := s.read()
	st.Format.Highlight = v
	s.write(st)
}

// Highlight returns the highlight color.
func (s *CharacterStyle) Highlight() string { return s.read().Format.Highlight }

// SetVertAlign sets the vertical alignment: "superscript", "subscript", or "".
func (s *CharacterStyle) SetVertAlign(v string) {
	st := s.read()
	st.Format.VertAlignVal = v
	s.write(st)
}

// VertAlign returns the vertical alignment.
func (s *CharacterStyle) VertAlign() string { return s.read().Format.VertAlignVal }

// TableStyle wraps an IR table style for editing.
type TableStyle struct {
	doc *Document
	id  string
}

func (s *TableStyle) read() wordir.Style {
	if s == nil || s.doc == nil || s.doc.IR.Styles == nil {
		return wordir.Style{}
	}
	return s.doc.IR.Styles[s.id]
}

func (s *TableStyle) write(st wordir.Style) {
	if s == nil || s.doc == nil || s.doc.IR.Styles == nil {
		return
	}
	s.doc.IR.Styles[s.id] = st
}

// ID returns the style ID.
func (s *TableStyle) ID() string {
	if s == nil {
		return ""
	}
	return s.id
}

// SetName sets the style name.
func (s *TableStyle) SetName(v string) {
	st := s.read()
	st.Name = v
	s.write(st)
}

// Name returns the style name.
func (s *TableStyle) Name() string { return s.read().Name }

// SetBasedOn sets the ID of the parent style.
func (s *TableStyle) SetBasedOn(id string) {
	st := s.read()
	st.BasedOn = id
	s.write(st)
}

// BasedOn returns the ID of the parent style.
func (s *TableStyle) BasedOn() string { return s.read().BasedOn }

// SetNextStyleID sets the next style ID.
func (s *TableStyle) SetNextStyleID(v string) {
	st := s.read()
	st.NextStyleID = v
	s.write(st)
}

// NextStyleID returns the next style ID.
func (s *TableStyle) NextStyleID() string { return s.read().NextStyleID }

// SetUIPriority sets the UI priority.
func (s *TableStyle) SetUIPriority(v int) {
	st := s.read()
	st.UIPriority = v
	s.write(st)
}

// UIPriority returns the UI priority.
func (s *TableStyle) UIPriority() int { return s.read().UIPriority }

// SetRSID sets the RSID.
func (s *TableStyle) SetRSID(v string) {
	st := s.read()
	st.RSID = v
	s.write(st)
}

// RSID returns the RSID.
func (s *TableStyle) RSID() string { return s.read().RSID }

// SetUnhideWhenUsed sets the unhide-when-used flag.
func (s *TableStyle) SetUnhideWhenUsed(v bool) {
	st := s.read()
	st.UnhideWhenUsed = v
	s.write(st)
}

// UnhideWhenUsed returns the unhide-when-used flag.
func (s *TableStyle) UnhideWhenUsed() bool { return s.read().UnhideWhenUsed }

// SetQFormat sets the quick-format flag.
func (s *TableStyle) SetQFormat(v bool) {
	st := s.read()
	st.QFormat = v
	s.write(st)
}

// QFormat returns the quick-format flag.
func (s *TableStyle) QFormat() bool { return s.read().QFormat }

// SetSemiHidden sets the semi-hidden flag.
func (s *TableStyle) SetSemiHidden(v bool) {
	st := s.read()
	st.SemiHidden = v
	s.write(st)
}

// SemiHidden returns the semi-hidden flag.
func (s *TableStyle) SemiHidden() bool { return s.read().SemiHidden }

// SetAutoRedefine sets the auto-redefine flag.
func (s *TableStyle) SetAutoRedefine(v bool) {
	st := s.read()
	st.AutoRedefine = v
	s.write(st)
}

// AutoRedefine returns the auto-redefine flag.
func (s *TableStyle) AutoRedefine() bool { return s.read().AutoRedefine }

// SetDefault marks this style as the default for its type.
func (s *TableStyle) SetDefault(v bool) {
	st := s.read()
	st.Default = v
	s.write(st)
}

// Default reports whether this style is the default for its type.
func (s *TableStyle) Default() bool { return s.read().Default }

// SetTableFormat sets the base table format properties.
func (s *TableStyle) SetTableFormat(fmt *wordir.TableFormat) {
	st := s.read()
	if fmt == nil {
		st.TableFormat = nil
	} else {
		copyFmt := *fmt
		st.TableFormat = &copyFmt
	}
	s.write(st)
}

// TableFormat returns the base table format properties, or nil if none.
func (s *TableStyle) TableFormat() *wordir.TableFormat { return s.read().TableFormat }

// AddConditionalStyle adds a conditional table style entry.
func (s *TableStyle) AddConditionalStyle(cts wordir.ConditionalTableStyle) {
	st := s.read()
	st.TableStyles = append(st.TableStyles, cts)
	s.write(st)
}


// SetOutlineLevel sets the outline level (0-8, or 9 for body text).
func (s *ParagraphStyle) SetOutlineLevel(level int) {
	st := s.read()
	st.Format.OutlineLevel = &level
	s.write(st)
}

// OutlineLevel returns the outline level.
func (s *ParagraphStyle) OutlineLevel() int {
	if v := s.read().Format.OutlineLevel; v != nil {
		return *v
	}
	return 0
}

// SetBidi toggles bidirectional paragraph layout.
func (s *ParagraphStyle) SetBidi(v bool) {
	st := s.read()
	st.Format.Bidi = v
	s.write(st)
}

// Bidi reports whether bidi layout is enabled.
func (s *ParagraphStyle) Bidi() bool {
	return s.read().Format.Bidi
}

// SetTextDirection sets the paragraph text direction (e.g. "lrTb", "tbRl").
func (s *ParagraphStyle) SetTextDirection(v string) {
	st := s.read()
	st.Format.TextDirection = v
	s.write(st)
}

// TextDirection returns the paragraph text direction.
func (s *ParagraphStyle) TextDirection() string { return s.read().Format.TextDirection }

// SetBorders sets the paragraph borders.
func (s *ParagraphStyle) SetBorders(b *wordir.ParagraphBorders) {
	st := s.read()
	st.Format.Borders = b
	s.write(st)
}

// Borders returns the paragraph borders, or nil.
func (s *ParagraphStyle) Borders() *wordir.ParagraphBorders { return s.read().Format.Borders }

// SetLanguage sets the language code (e.g. "en-US", "fr-FR").
func (s *CharacterStyle) SetLanguage(v string) {
	st := s.read()
	st.Format.Lang = v
	s.write(st)
}

// Language returns the language code.
func (s *CharacterStyle) Language() string { return s.read().Format.Lang }

// SetCharacterSpacing sets the character spacing in twips (positive = expanded, negative = condensed).
func (s *CharacterStyle) SetCharacterSpacing(twips int) {
	st := s.read()
	st.Format.CharacterSpacing = &twips
	s.write(st)
}

// CharacterSpacing returns the character spacing in twips.
func (s *CharacterStyle) CharacterSpacing() int {
	if v := s.read().Format.CharacterSpacing; v != nil {
		return *v
	}
	return 0
}

// SetKerning sets the font kerning size in half-points.
func (s *CharacterStyle) SetKerning(hp int) {
	st := s.read()
	st.Format.Kerning = &hp
	s.write(st)
}

// Kerning returns the font kerning size in half-points.
func (s *CharacterStyle) Kerning() int {
	if v := s.read().Format.Kerning; v != nil {
		return *v
	}
	return 0
}

// SetShadow toggles text shadow.
func (s *CharacterStyle) SetShadow(v bool) {
	st := s.read()
	st.Format.Shadow = &v
	s.write(st)
}

// Shadow reports whether text shadow is enabled.
func (s *CharacterStyle) Shadow() bool {
	if v := s.read().Format.Shadow; v != nil {
		return *v
	}
	return false
}

// SetEmboss toggles embossed text.
func (s *CharacterStyle) SetEmboss(v bool) {
	st := s.read()
	st.Format.Emboss = &v
	s.write(st)
}

// Emboss reports whether embossed text is enabled.
func (s *CharacterStyle) Emboss() bool {
	if v := s.read().Format.Emboss; v != nil {
		return *v
	}
	return false
}

// SetImprint toggles imprinted text.
func (s *CharacterStyle) SetImprint(v bool) {
	st := s.read()
	st.Format.Imprint = &v
	s.write(st)
}

// Imprint reports whether imprinted text is enabled.
func (s *CharacterStyle) Imprint() bool {
	if v := s.read().Format.Imprint; v != nil {
		return *v
	}
	return false
}

// SetBorders sets the character run borders.
func (s *CharacterStyle) SetBorders(b *wordir.RunBorders) {
	st := s.read()
	st.Format.Borders = b
	s.write(st)
}

// Borders returns the character run borders, or nil.
func (s *CharacterStyle) Borders() *wordir.RunBorders { return s.read().Format.Borders }

// SetComplexScript toggles complex script formatting.
func (s *CharacterStyle) SetComplexScript(v bool) {
	st := s.read()
	st.Format.ComplexScript = &v
	s.write(st)
}

// ComplexScript reports whether complex script formatting is enabled.
func (s *CharacterStyle) ComplexScript() bool {
	if v := s.read().Format.ComplexScript; v != nil {
		return *v
	}
	return false
}

// SetWidth sets the table width in twips (nil = auto).
func (s *TableStyle) SetWidth(twips int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.Width = &twips
	s.write(st)
}

// Width returns the table width in twips, or 0 if auto.
func (s *TableStyle) Width() int {
	if v := s.read().TableFormat; v != nil && v.Width != nil {
		return *v.Width
	}
	return 0
}

// SetAlignment sets the table alignment ("left", "center", "right").
func (s *TableStyle) SetAlignment(v string) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.Alignment = v
	s.write(st)
}

// Alignment returns the table alignment.
func (s *TableStyle) Alignment() string {
	if v := s.read().TableFormat; v != nil {
		return v.Alignment
	}
	return ""
}

// SetBorders sets the table borders.
func (s *TableStyle) SetBorders(b *wordir.TableBorders) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.Borders = b
	s.write(st)
}

// Borders returns the table borders, or nil.
func (s *TableStyle) Borders() *wordir.TableBorders {
	if v := s.read().TableFormat; v != nil {
		return v.Borders
	}
	return nil
}

// SetShading sets the table background shading as a 6-digit hex string.
func (s *TableStyle) SetShading(hex string) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.Shading = hex
	s.write(st)
}

// Shading returns the table background shading.
func (s *TableStyle) Shading() string {
	if v := s.read().TableFormat; v != nil {
		return v.Shading
	}
	return ""
}

// SetLayout sets the table layout ("fixed" or "autofit").
func (s *TableStyle) SetLayout(v string) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.Layout = v
	s.write(st)
}

// Layout returns the table layout.
func (s *TableStyle) Layout() string {
	if v := s.read().TableFormat; v != nil {
		return v.Layout
	}
	return ""
}

// SetCellMarginTop sets the default top cell padding in twips.
func (s *TableStyle) SetCellMarginTop(twips int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.CellPadTop = twips
	s.write(st)
}

// CellMarginTop returns the default top cell padding in twips.
func (s *TableStyle) CellMarginTop() int {
	if v := s.read().TableFormat; v != nil {
		return v.CellPadTop
	}
	return 0
}

// SetCellMarginBottom sets the default bottom cell padding in twips.
func (s *TableStyle) SetCellMarginBottom(twips int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.CellPadBottom = twips
	s.write(st)
}

// CellMarginBottom returns the default bottom cell padding in twips.
func (s *TableStyle) CellMarginBottom() int {
	if v := s.read().TableFormat; v != nil {
		return v.CellPadBottom
	}
	return 0
}

// SetCellMarginLeft sets the default left cell padding in twips.
func (s *TableStyle) SetCellMarginLeft(twips int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.CellPadLeft = twips
	s.write(st)
}

// CellMarginLeft returns the default left cell padding in twips.
func (s *TableStyle) CellMarginLeft() int {
	if v := s.read().TableFormat; v != nil {
		return v.CellPadLeft
	}
	return 0
}

// SetCellMarginRight sets the default right cell padding in twips.
func (s *TableStyle) SetCellMarginRight(twips int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.CellPadRight = twips
	s.write(st)
}

// CellMarginRight returns the default right cell padding in twips.
func (s *TableStyle) CellMarginRight() int {
	if v := s.read().TableFormat; v != nil {
		return v.CellPadRight
	}
	return 0
}

// SetRowBandSize sets the number of rows in each row band for conditional formatting.
func (s *TableStyle) SetRowBandSize(n int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.StyleRowBandSize = n
	s.write(st)
}

// RowBandSize returns the row band size.
func (s *TableStyle) RowBandSize() int {
	if v := s.read().TableFormat; v != nil {
		return v.StyleRowBandSize
	}
	return 0
}

// SetColBandSize sets the number of columns in each column band for conditional formatting.
func (s *TableStyle) SetColBandSize(n int) {
	st := s.read()
	if st.TableFormat == nil {
		st.TableFormat = &wordir.TableFormat{}
	}
	st.TableFormat.StyleColBandSize = n
	s.write(st)
}

// ColBandSize returns the column band size.
func (s *TableStyle) ColBandSize() int {
	if v := s.read().TableFormat; v != nil {
		return v.StyleColBandSize
	}
	return 0
}