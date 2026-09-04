package edit

import wordir "office/pkg/word/ir"

// Section wraps an IR section for editing.
type Section struct {
	doc   *Document
	index int
}

func (s *Section) ir() *wordir.Section {
	if s.doc == nil || s.index < 0 || s.index >= len(s.doc.IR.Sections) {
		return nil
	}
	return &s.doc.IR.Sections[s.index]
}

// SetPageSize sets the page width and height in twips.
func (s *Section) SetPageSize(width, height int) {
	if ir := s.ir(); ir != nil {
		ir.PageSize = &wordir.PageSize{Width: width, Height: height}
	}
}

// SetPageMargins sets the page margins in twips.
func (s *Section) SetPageMargins(top, right, bottom, left int) {
	if ir := s.ir(); ir != nil {
		ir.PageMargin = &wordir.PageMargin{Top: top, Right: right, Bottom: bottom, Left: left}
	}
}

// SetColumns sets the number of columns and spacing between them (twips).
func (s *Section) SetColumns(count, space int) {
	if ir := s.ir(); ir != nil {
		ir.Columns = &wordir.Columns{Num: count, Space: space}
	}
}

// SetBreakKind sets the section break type (e.g. "nextPage", "continuous", "oddPage", "evenPage").
func (s *Section) SetBreakKind(v string) {
	if ir := s.ir(); ir != nil {
		ir.Type = v
	}
}

// SetHeaderRef adds a header reference of the given type ("default", "first", "even").
func (s *Section) SetHeaderRef(typ, partName string) {
	if ir := s.ir(); ir != nil {
		ir.Headers = append(ir.Headers, wordir.Reference{Type: typ, PartName: partName})
	}
}

// HeaderRefs returns the header references for this section.
func (s *Section) HeaderRefs() []wordir.Reference {
	if ir := s.ir(); ir != nil {
		return append([]wordir.Reference(nil), ir.Headers...)
	}
	return nil
}

// RemoveHeaderRef removes all header references of the given type.
func (s *Section) RemoveHeaderRef(typ string) {
	if ir := s.ir(); ir != nil {
		filtered := ir.Headers[:0]
		for _, h := range ir.Headers {
			if h.Type != typ {
				filtered = append(filtered, h)
			}
		}
		ir.Headers = filtered
	}
}

// SetFooterRef adds a footer reference of the given type.
func (s *Section) SetFooterRef(typ, partName string) {
	if ir := s.ir(); ir != nil {
		ir.Footers = append(ir.Footers, wordir.Reference{Type: typ, PartName: partName})
	}
}

// FooterRefs returns the footer references for this section.
func (s *Section) FooterRefs() []wordir.Reference {
	if ir := s.ir(); ir != nil {
		return append([]wordir.Reference(nil), ir.Footers...)
	}
	return nil
}

// RemoveFooterRef removes all footer references of the given type.
func (s *Section) RemoveFooterRef(typ string) {
	if ir := s.ir(); ir != nil {
		filtered := ir.Footers[:0]
		for _, f := range ir.Footers {
			if f.Type != typ {
				filtered = append(filtered, f)
			}
		}
		ir.Footers = filtered
	}
}

// CreateHeader creates a new header story of the given type, adds it to the
// document, and sets a reference to it in this section. If the section already
// has a header ref of the same type, it is replaced.
func (s *Section) CreateHeader(typ string) *StoryEditor {
	if s.doc == nil || s.ir() == nil {
		return nil
	}
	se := s.doc.AddHeader(typ)
	s.RemoveHeaderRef(typ)
	if ir := s.ir(); ir != nil {
		ir.Headers = append(ir.Headers, wordir.Reference{Type: typ, PartName: typ})
	}
	return se
}

// CreateFooter creates a new footer story of the given type, adds it to the
// document, and sets a reference to it in this section.
func (s *Section) CreateFooter(typ string) *StoryEditor {
	if s.doc == nil || s.ir() == nil {
		return nil
	}
	se := s.doc.AddFooter(typ)
	s.RemoveFooterRef(typ)
	if ir := s.ir(); ir != nil {
		ir.Footers = append(ir.Footers, wordir.Reference{Type: typ, PartName: typ})
	}
	return se
}

// HeaderStory returns the story editor for the header of the given type
// referenced by this section, or nil if none.
func (s *Section) HeaderStory(typ string) *StoryEditor {
	if s.doc == nil {
		return nil
	}
	for _, ref := range s.HeaderRefs() {
		if ref.Type == typ {
			return s.doc.Header(typ)
		}
	}
	return nil
}

// FooterStory returns the story editor for the footer of the given type
// referenced by this section, or nil if none.
func (s *Section) FooterStory(typ string) *StoryEditor {
	if s.doc == nil {
		return nil
	}
	for _, ref := range s.FooterRefs() {
		if ref.Type == typ {
			return s.doc.Footer(typ)
		}
	}
	return nil
}

// ClearHeaders removes all header references from this section.
func (s *Section) ClearHeaders() {
	if ir := s.ir(); ir != nil {
		ir.Headers = nil
	}
}

// ClearFooters removes all footer references from this section.
func (s *Section) ClearFooters() {
	if ir := s.ir(); ir != nil {
		ir.Footers = nil
	}
}

// SetTitlePage enables or disables the title-page header/footer pair.
func (s *Section) SetTitlePage(v bool) {
	if ir := s.ir(); ir != nil {
		ir.TitlePage = v
	}
}

// SetPageNumberStart sets the explicit starting page number.
func (s *Section) SetPageNumberStart(v int) {
	if ir := s.ir(); ir != nil {
		ir.PageNumberStart = &v
		ir.PageNumberRestart = true
	}
}

// SetTextDirection sets the section text direction ("tbRl", "btLr", or "").
func (s *Section) SetTextDirection(v string) {
	if ir := s.ir(); ir != nil {
		ir.TextDirection = v
	}
}

// SetPageNumberFormat sets the page number format (e.g. "decimal", "upperRoman").
func (s *Section) SetPageNumberFormat(v string) {
	if ir := s.ir(); ir != nil {
		ir.PageNumberFormat = v
	}
}

// SetLineNumbering sets line numbering for the section.
func (s *Section) SetLineNumbering(countBy, start int, restart string) {
	if ir := s.ir(); ir != nil {
		ir.LineNumbering = &wordir.LineNumbering{CountBy: countBy, Start: start, Restart: restart}
	}
}

// SetDocGrid sets the document grid for the section.
func (s *Section) SetDocGrid(linePitch, charSpace int) {
	if ir := s.ir(); ir != nil {
		ir.DocGrid = &wordir.DocGrid{LinePitch: linePitch, CharSpace: charSpace}
	}
}

// SetPageBorders sets all page borders with the same style, color, and size
// (in points). display is "allPages", "firstPage", or "notFirstPage".
func (s *Section) SetPageBorders(style, color string, sizePt float64, display string) {
	if ir := s.ir(); ir != nil {
		b := &wordir.PageBorders{
			Top:     &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color},
			Left:    &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color},
			Bottom:  &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color},
			Right:   &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color},
			Display: display,
		}
		ir.PageBorders = b
	}
}

// SetVerticalAlign sets the page vertical alignment: "top", "center", "both", or "bottom".
func (s *Section) SetVerticalAlign(v string) {
	if ir := s.ir(); ir != nil {
		ir.VerticalAlign = v
	}
}

// SetBidi marks the section as right-to-left.
func (s *Section) SetBidi(v bool) {
	if ir := s.ir(); ir != nil {
		ir.Bidi = v
	}
}

// SetPaperSource sets the first and other page paper source tray codes.
func (s *Section) SetPaperSource(first, other int) {
	if ir := s.ir(); ir != nil {
		ir.PaperSource = &wordir.PaperSource{First: first, Other: other}
	}
}

// SetFootnoteProps sets section-specific footnote properties.
// position is "pageBottom", "belowText", or "sectEnd".
// format is the numbering format (e.g., "decimal", "lowerRoman").
// start is the starting number.
// restart is "continuous", "eachSect", or "eachPage".
func (s *Section) SetFootnoteProps(position, format string, start int, restart string) {
	if ir := s.ir(); ir != nil {
		ir.FootnotePos = position
		ir.FootnoteNumFmt = format
		ir.FootnoteNumStart = start
		ir.FootnoteRestart = restart
	}
}

// SetEndnoteProps sets section-specific endnote properties.
// format is the numbering format (e.g., "decimal", "lowerRoman").
// start is the starting number.
// restart is "continuous" or "eachSect".
func (s *Section) SetEndnoteProps(format string, start int, restart string) {
	if ir := s.ir(); ir != nil {
		ir.EndnoteNumFmt = format
		ir.EndnoteNumStart = start
		ir.EndnoteRestart = restart
	}
}

// SetPageBorderTop sets the top page border.
func (s *Section) SetPageBorderTop(style string, sizePt float64, color string) {
	if ir := s.ir(); ir != nil {
		if ir.PageBorders == nil {
			ir.PageBorders = &wordir.PageBorders{}
		}
		ir.PageBorders.Top = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetPageBorderBottom sets the bottom page border.
func (s *Section) SetPageBorderBottom(style string, sizePt float64, color string) {
	if ir := s.ir(); ir != nil {
		if ir.PageBorders == nil {
			ir.PageBorders = &wordir.PageBorders{}
		}
		ir.PageBorders.Bottom = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetPageBorderLeft sets the left page border.
func (s *Section) SetPageBorderLeft(style string, sizePt float64, color string) {
	if ir := s.ir(); ir != nil {
		if ir.PageBorders == nil {
			ir.PageBorders = &wordir.PageBorders{}
		}
		ir.PageBorders.Left = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// SetPageBorderRight sets the right page border.
func (s *Section) SetPageBorderRight(style string, sizePt float64, color string) {
	if ir := s.ir(); ir != nil {
		if ir.PageBorders == nil {
			ir.PageBorders = &wordir.PageBorders{}
		}
		ir.PageBorders.Right = &wordir.BorderSide{Val: style, SizePt: sizePt, Color: color}
	}
}

// ClearPageBorders removes all page borders from the section.
func (s *Section) ClearPageBorders() {
	if ir := s.ir(); ir != nil {
		ir.PageBorders = nil
	}
}

// SetRtlGutter places the gutter on the right side of the page.
func (s *Section) SetRtlGutter(v bool) {
	if ir := s.ir(); ir != nil {
		if ir.PageMargin == nil {
			ir.PageMargin = &wordir.PageMargin{}
		}
		ir.PageMargin.RtlGutter = v
	}
}

// SetNoEndnote suppresses endnotes at the end of this section.
func (s *Section) SetNoEndnote(v bool) {
	if ir := s.ir(); ir != nil {
		ir.EndnoteAtSectionEnd = !v
	}
}

// SetFormProt marks the section as protected for forms (w:formProt).
func (s *Section) SetFormProt(v bool) {
	if ir := s.ir(); ir != nil {
		ir.FormProt = v
	}
}