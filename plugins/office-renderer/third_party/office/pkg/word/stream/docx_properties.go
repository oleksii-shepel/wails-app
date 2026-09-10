package stream

import (
	wordir "office/pkg/word/ir"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func paragraphFormatFromSAX(p wordsax.ParagraphProperties) wordir.ParagraphFormat {
	return wordir.ParagraphFormat{
		Alignment:       p.Alignment,
		IndentLeft:      cloneIntPtr(p.IndentLeft),
		IndentRight:     cloneIntPtr(p.IndentRight),
		IndentFirstLine: cloneIntPtr(p.IndentFirstLine),
		IndentHanging:   cloneIntPtr(p.IndentHanging),
		SpacingBefore:   cloneIntPtr(p.SpacingBefore),
		SpacingAfter:    cloneIntPtr(p.SpacingAfter),
		LineSpacing:     cloneIntPtr(p.LineSpacing),
		LineRule:        p.LineRule,
		KeepNext:        p.KeepNext,
		KeepLines:       p.KeepLines,
		PageBreakBefore:  p.PageBreakBefore,
		WidowControl:     p.WidowControl == nil || *p.WidowControl,
		SnapToGrid:       p.SnapToGrid == nil || *p.SnapToGrid,
		ContextualSpacing: p.ContextualSpacing,
	}
}

func runFormatFromSAX(r wordsax.RunProperties) wordir.RunFormat {
	out := wordir.RunFormat{
		Bold:      cloneBoolPtr(r.Bold),
		Italic:    cloneBoolPtr(r.Italic),
		Strike:    cloneBoolPtr(r.Strike),
		Underline: r.Underline,
		Color:     r.Color,
		FontASCII:        r.FontFamily,
		FontHAnsi:        r.FontFamily,
		BoldCS:           cloneBoolPtr(r.BoldCS),
		CharacterSpacing: cloneIntPtr(r.CharacterSpacing),
		Kerning:          cloneIntPtr(r.KerningHalfPoints),
	}
	if r.FontSizeHalfPoints != nil {
		hp := *r.FontSizeHalfPoints
		out.FontSize = &hp
	}
	if r.FontSizeCSHalfPoints != nil {
		hp := *r.FontSizeCSHalfPoints
		out.FontSizeCS = &hp
	}
	return out
}

func sectionFromSAX(s wordsax.SectionProperties) wordir.Section {
	out := wordir.Section{Type: s.Type, TitlePage: s.TitlePage}
	if s.PageWidth != nil || s.PageHeight != nil {
		out.PageSize = &wordir.PageSize{}
		if s.PageWidth != nil {
			out.PageSize.Width = *s.PageWidth
		}
		if s.PageHeight != nil {
			out.PageSize.Height = *s.PageHeight
		}
	}
	if s.MarginTop != nil || s.MarginRight != nil || s.MarginBottom != nil || s.MarginLeft != nil || s.MarginHeader != nil || s.MarginFooter != nil {
		out.PageMargin = &wordir.PageMargin{}
		if s.MarginTop != nil {
			out.PageMargin.Top = *s.MarginTop
		}
		if s.MarginRight != nil {
			out.PageMargin.Right = *s.MarginRight
		}
		if s.MarginBottom != nil {
			out.PageMargin.Bottom = *s.MarginBottom
		}
		if s.MarginLeft != nil {
			out.PageMargin.Left = *s.MarginLeft
		}
		if s.MarginHeader != nil {
			out.PageMargin.Header = *s.MarginHeader
		}
		if s.MarginFooter != nil {
			out.PageMargin.Footer = *s.MarginFooter
		}
	}
	if s.Columns != nil || s.ColumnSpace != nil {
		out.Columns = &wordir.Columns{}
		if s.Columns != nil {
			out.Columns.Num = *s.Columns
		}
		if s.ColumnSpace != nil {
			out.Columns.Space = *s.ColumnSpace
		}
	}
	return out
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}