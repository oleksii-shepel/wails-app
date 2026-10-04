package stream

import (
	wordir "office/pkg/word/ir"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func paragraphFormatFromSAX(p wordsax.ParagraphProperties) wordir.ParagraphFormat {
	out := wordir.ParagraphFormat{
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
		PageBreakBefore: p.PageBreakBefore,
		WidowControl:    p.WidowControl == nil || *p.WidowControl,
		// Word snaps paragraphs to an active section line grid by default;
		// w:snapToGrid is primarily an opt-out. Keeping the effective default
		// here also lets the HTML renderer and paginator agree without requiring
		// a format-specific special case downstream.
		SnapToGrid:        p.SnapToGrid == nil || *p.SnapToGrid,
		ContextualSpacing: p.ContextualSpacing,
		Shading:           p.Shading,
		Borders:           paragraphBordersFromSAX(p.Borders),
	}
	for _, tab := range p.Tabs {
		// "clear" stops remove default stops rather than adding a custom one.
		if tab.Algn == "clear" {
			continue
		}
		out.TabStops = append(out.TabStops, wordir.TabStop{
			Pos:    tab.Pos,
			Algn:   tab.Algn,
			Leader: tab.Leader,
		})
	}
	return out
}

// paragraphBordersFromSAX converts w:pBdr sides to the renderer-facing model.
// "nil" and "none" sides are kept so an explicit cancellation still overrides an
// inherited style border.
func paragraphBordersFromSAX(b *wordsax.ParagraphBorders) *wordir.ParagraphBorders {
	if b == nil {
		return nil
	}
	out := &wordir.ParagraphBorders{
		Top:     paragraphBorderSideFromSAX(b.Top),
		Left:    paragraphBorderSideFromSAX(b.Left),
		Bottom:  paragraphBorderSideFromSAX(b.Bottom),
		Right:   paragraphBorderSideFromSAX(b.Right),
		Between: paragraphBorderSideFromSAX(b.Between),
		Bar:     paragraphBorderSideFromSAX(b.Bar),
	}
	if out.Top == nil && out.Left == nil && out.Bottom == nil && out.Right == nil && out.Between == nil && out.Bar == nil {
		return nil
	}
	return out
}

// paragraphBorderSideFromSAX applies Word's border unit conventions: w:sz is
// eighths of a point, w:space is already in points, and "auto" resolves to the
// document text color.
func paragraphBorderSideFromSAX(s *wordsax.ParagraphBorderSide) *wordir.BorderSide {
	if s == nil || s.Val == "" {
		return nil
	}
	side := &wordir.BorderSide{Val: s.Val}
	switch {
	case s.Val == "nil" || s.Val == "none":
		side.SizePt = 0
	case s.Size > 0:
		side.SizePt = float64(s.Size) / 8.0
	default:
		side.SizePt = 0.75
	}
	if s.Space > 0 {
		side.SpacePt = float64(s.Space)
	}
	if s.Color != "" && s.Color != "auto" {
		side.Color = s.Color
	}
	return side
}

func framePropsFromSAX(f wordsax.FrameProperties) *wordir.FrameProps {
	return &wordir.FrameProps{
		W:        f.W,
		H:        f.H,
		X:        f.X,
		Y:        f.Y,
		XAlign:   f.XAlign,
		YAlign:   f.YAlign,
		HAnchor:  f.HAnchor,
		VAnchor:  f.VAnchor,
		WrapType: f.Wrap,
		HSpace:   f.HSpace,
		VSpace:   f.VSpace,
		DropCap:  f.DropCap,
		Lines:    f.Lines,
	}
}

func runFormatFromSAX(r wordsax.RunProperties) wordir.RunFormat {
	out := wordir.RunFormat{
		Bold:             cloneBoolPtr(r.Bold),
		Italic:           cloneBoolPtr(r.Italic),
		Strike:           cloneBoolPtr(r.Strike),
		Underline:        r.Underline,
		Color:            r.Color,
		FontASCII:        r.FontFamily,
		FontHAnsi:        r.FontFamily,
		BoldCS:           cloneBoolPtr(r.BoldCS),
		CharacterSpacing: cloneIntPtr(r.CharacterSpacing),
		Kerning:          cloneIntPtr(r.KerningHalfPoints),
		Highlight:        r.Highlight,
		Background:       r.ShadingFill,
		VertAlignVal:     r.VertAlign,
		Caps:             cloneBoolPtr(r.Caps),
		SmallCaps:        cloneBoolPtr(r.SmallCaps),
		DoubleStrike:     cloneBoolPtr(r.DoubleStrike),
		Vanish:           cloneBoolPtr(r.Vanish),
		SpecVanish:       cloneBoolPtr(r.SpecVanish),
		RTL:              cloneBoolPtr(r.RTL),
		EmphasisMark:     r.EmphasisMark,
	}
	if r.FontSizeHalfPoints != nil {
		hp := *r.FontSizeHalfPoints
		out.FontSize = &hp
	}
	if r.FontSizeCSHalfPoints != nil {
		hp := *r.FontSizeCSHalfPoints
		out.FontSizeCS = &hp
	}
	if r.Position != nil {
		v := *r.Position
		out.Position = &v
	}
	if side := runBorderSideFromSAX(r.Border); side != nil {
		// w:bdr on a run draws the same border on all four sides.
		out.Borders = &wordir.RunBorders{Top: side, Left: side, Bottom: side, Right: side}
	}
	return out
}

// runBorderSideFromSAX converts a run border to the renderer-facing side,
// applying Word's unit conventions: w:sz is eighths of a point, w:space is
// already in points, and "auto" resolves to the document text color.
func runBorderSideFromSAX(b *wordsax.RunBorder) *wordir.BorderSide {
	if b == nil || b.Val == "" {
		return nil
	}
	side := &wordir.BorderSide{Val: b.Val}
	switch {
	case b.Val == "nil" || b.Val == "none":
		side.SizePt = 0
	case b.Size > 0:
		side.SizePt = float64(b.Size) / 8.0
	default:
		side.SizePt = 0.75
	}
	if b.Space > 0 {
		side.SpacePt = float64(b.Space)
	}
	if b.Color != "" && b.Color != "auto" {
		side.Color = b.Color
	}
	return side
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
	if s.DocGridLinePitch != nil || s.DocGridCharSpace != nil || s.DocGridType != "" {
		// w:docGrid/@w:type defaults to "default", which means no grid at all:
		// a bare w:linePitch does not activate line snapping. Only an explicit
		// "lines" or "linesAndChars" type makes Word snap lines to the pitch.
		out.DocGrid = &wordir.DocGrid{Type: s.DocGridType}
		if s.DocGridLinePitch != nil {
			out.DocGrid.LinePitch = *s.DocGridLinePitch
		}
		if s.DocGridCharSpace != nil {
			out.DocGrid.CharSpace = *s.DocGridCharSpace
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
