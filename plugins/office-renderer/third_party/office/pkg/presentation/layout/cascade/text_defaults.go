package cascade

import (
	"strings"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// TextDefaults is the canonical effective paragraph/run default set produced by
// the PresentationML text cascade before paragraph-local and run-local overrides.
type TextDefaults struct {
	Align                                                                                    string
	MarginLeftEMU, MarginRightEMU, TextIndentEMU, DefaultTabSizeEMU                          *int
	FontAlign, BulletColor, BulletFontFamily, BulletChar, BulletImageRelID, BulletImageScope string
	BulletNone, BulletFontTx, BulletSpecified                                                bool
	AutoNum                                                                                  bool
	AutoNumType                                                                              string
	AutoNumStart, BulletSize, BulletSizePct                                                  int
	Bold, Italic                                                                             *bool
	Underline, Strike, Caps                                                                  string
	Baseline                                                                                 *int
	FontSize                                                                                 int
	CharSpacing, Kern                                                                        *int
	FontFamily, Color, Highlight                                                             string
	SpaceBefore, SpaceAfter, SpaceBeforePct, SpaceAfterPct, LineSpacingPct, LineSpacingPt    *int
	TabStops                                                                                 []ir.TabStop
}

// EffectiveTextDefaults applies shape -> layout -> master-body -> master text
// style precedence. MergeTextDefaults is first-value-wins, so callers pass the
// most specific source first.
func EffectiveTextDefaults(shapeBody, layoutBody, masterBody *parts.TextBody, master *parts.SlideMaster, presentationStyle *parts.DMLTextListStyle, ph ir.PlaceholderKind, level int, slideColors, layoutColors, masterColors map[string]string, theme *ir.Theme) TextDefaults {
	d := TextDefaults{}
	if shapeBody != nil {
		v := TextDefaultsFromListStyle(shapeBody.ListStyle, level, slideColors, theme)
		if v.BulletImageRelID != "" {
			v.BulletImageScope = "slide"
		}
		d = MergeTextDefaults(d, v)
	}
	if layoutBody != nil {
		v := TextDefaultsFromListStyle(layoutBody.ListStyle, level, layoutColors, theme)
		if v.BulletImageRelID != "" {
			v.BulletImageScope = "layout"
		}
		d = MergeTextDefaults(d, v)
	}
	if masterBody != nil {
		v := TextDefaultsFromListStyle(masterBody.ListStyle, level, masterColors, theme)
		if v.BulletImageRelID != "" {
			v.BulletImageScope = "master"
		}
		d = MergeTextDefaults(d, v)
	}
	vMaster := TextDefaultsFromMaster(master, ph, level, masterColors, theme)
	if vMaster.BulletImageRelID != "" {
		vMaster.BulletImageScope = "master"
	}
	d = MergeTextDefaults(d, vMaster)
	// p:defaultTextStyle is the presentation-wide final text-style fallback.
	// It must participate for ordinary shapes as well as table cells; otherwise
	// placeholders whose master/layout omit a character property collapse to
	// renderer defaults (historically Arial 14pt).
	d = MergeTextDefaults(d, TextDefaultsFromListStyle(presentationStyle, level, slideColors, theme))
	if d.FontFamily == "" {
		d.FontFamily = FallbackThemeFont(theme, ph)
	}
	return d
}

func MergeTextDefaults(dst, src TextDefaults) TextDefaults {
	if dst.Align == "" {
		dst.Align = src.Align
	}
	if dst.MarginLeftEMU == nil {
		dst.MarginLeftEMU = src.MarginLeftEMU
	}
	if dst.MarginRightEMU == nil {
		dst.MarginRightEMU = src.MarginRightEMU
	}
	if dst.TextIndentEMU == nil {
		dst.TextIndentEMU = src.TextIndentEMU
	}
	if dst.DefaultTabSizeEMU == nil {
		dst.DefaultTabSizeEMU = src.DefaultTabSizeEMU
	}
	if dst.FontAlign == "" {
		dst.FontAlign = src.FontAlign
	}
	if dst.BulletColor == "" {
		dst.BulletColor = src.BulletColor
	}
	if !dst.BulletFontTx && dst.BulletFontFamily == "" {
		if src.BulletFontTx {
			dst.BulletFontTx = true
			dst.BulletFontFamily = ""
		} else {
			dst.BulletFontFamily = src.BulletFontFamily
		}
	}
	// Marker kind is a single cascade value.  Do not merge buNone, buChar and
	// buAutoNum independently: a more-specific marker declaration owns the whole
	// marker and must not be overwritten by a less-specific master level.
	if !textDefaultsMarkerSpecified(dst) && textDefaultsMarkerSpecified(src) {
		dst.BulletSpecified = true
		dst.BulletNone = src.BulletNone
		dst.BulletChar = src.BulletChar
		dst.BulletImageRelID = src.BulletImageRelID
		dst.BulletImageScope = src.BulletImageScope
		dst.AutoNum = src.AutoNum
		dst.AutoNumType = src.AutoNumType
		dst.AutoNumStart = src.AutoNumStart
	}
	// Bullet size is one mutually-exclusive cascade property. A local buSzPts
	// must suppress an inherited buSzPct (and vice versa); independently merging
	// the two representations can manufacture a marker size that was never authored.
	if dst.BulletSize == 0 && dst.BulletSizePct == 0 {
		dst.BulletSize = src.BulletSize
		dst.BulletSizePct = src.BulletSizePct
	}
	if dst.Bold == nil {
		dst.Bold = src.Bold
	}
	if dst.Italic == nil {
		dst.Italic = src.Italic
	}
	if dst.Underline == "" {
		dst.Underline = src.Underline
	}
	if dst.Strike == "" {
		dst.Strike = src.Strike
	}
	if dst.Caps == "" {
		dst.Caps = src.Caps
	}
	if dst.Baseline == nil {
		dst.Baseline = src.Baseline
	}
	if dst.FontSize == 0 {
		dst.FontSize = src.FontSize
	}
	if dst.CharSpacing == nil {
		dst.CharSpacing = src.CharSpacing
	}
	if dst.Kern == nil {
		dst.Kern = src.Kern
	}
	if dst.FontFamily == "" {
		dst.FontFamily = src.FontFamily
	}
	if dst.Color == "" {
		dst.Color = src.Color
	}
	if dst.Highlight == "" {
		dst.Highlight = src.Highlight
	}
	if dst.SpaceBefore == nil {
		dst.SpaceBefore = src.SpaceBefore
	}
	if dst.SpaceAfter == nil {
		dst.SpaceAfter = src.SpaceAfter
	}
	if dst.SpaceBeforePct == nil {
		dst.SpaceBeforePct = src.SpaceBeforePct
	}
	if dst.SpaceAfterPct == nil {
		dst.SpaceAfterPct = src.SpaceAfterPct
	}
	if dst.LineSpacingPct == nil && dst.LineSpacingPt == nil {
		dst.LineSpacingPct = src.LineSpacingPct
		dst.LineSpacingPt = src.LineSpacingPt
	}
	if len(dst.TabStops) == 0 {
		dst.TabStops = src.TabStops
	}
	return dst
}

func textDefaultsMarkerSpecified(d TextDefaults) bool {
	return d.BulletSpecified || d.BulletNone || d.BulletChar != "" || d.BulletImageRelID != "" || d.AutoNum
}

func TextDefaultsFromMaster(master *parts.SlideMaster, ph ir.PlaceholderKind, level int, colors map[string]string, theme *ir.Theme) TextDefaults {
	if master == nil || master.TxStyles == nil {
		return TextDefaults{}
	}
	var style *parts.DMLTextListStyle
	switch ph {
	case ir.PlaceholderTitle:
		style = master.TxStyles.TitleStyle
	case ir.PlaceholderBody, ir.PlaceholderSubtitle:
		style = master.TxStyles.BodyStyle
	default:
		style = master.TxStyles.OtherStyle
		if style == nil {
			style = master.TxStyles.BodyStyle
		}
	}
	return TextDefaultsFromListStyle(style, level, colors, theme)
}

func TextDefaultsFromListStyle(style *parts.DMLTextListStyle, level int, colors map[string]string, theme *ir.Theme) TextDefaults {
	if style == nil {
		return TextDefaults{}
	}
	// DrawingML defPPr is the base paragraph/run style for every level; lvlNpPr
	// refines it.  Treating Level() as an either/or lookup drops defPPr's
	// defRPr whenever a level node exists (a very common PowerPoint pattern),
	// which collapses inherited title/subtitle typography to renderer defaults.
	base := textDefaultsFromStyleLevel(style.DefPPr, colors, theme)
	ls := style.Level(level)
	if ls == nil || ls == style.DefPPr {
		return base
	}
	return MergeTextDefaults(textDefaultsFromStyleLevel(ls, colors, theme), base)
}

func textDefaultsFromStyleLevel(ls *parts.DMLTextStyleLevel, colors map[string]string, theme *ir.Theme) TextDefaults {
	if ls == nil {
		return TextDefaults{}
	}
	d := TextDefaults{Align: ls.Algn, MarginLeftEMU: ls.MarL, MarginRightEMU: ls.MarR, TextIndentEMU: ls.Indent, DefaultTabSizeEMU: ls.DefTabSz, FontAlign: ls.FontAlgn, BulletNone: ls.BuNone != nil, BulletFontTx: ls.BuFontTx != nil, BulletSpecified: ls.BuNone != nil || ls.BuChar != nil || ls.BuBlip != nil || ls.BuAutoNum != nil}
	d.SpaceBefore, d.SpaceAfter = SpacingPoints(ls.SpcBef), SpacingPoints(ls.SpcAft)
	d.SpaceBeforePct, d.SpaceAfterPct = SpacingPercentValue(ls.SpcBef), SpacingPercentValue(ls.SpcAft)
	d.LineSpacingPct = SpacingPercent(ls.LnSpc)
	d.LineSpacingPt = SpacingPoints(ls.LnSpc)
	d.BulletColor, d.BulletFontFamily = BulletColor(ls.BuClr, ls.BuClrScheme, colors), BulletFontFamily(ls.BuFont)
	if d.BulletFontTx {
		d.BulletFontFamily = ""
	}
	if ls.BuNone == nil && ls.BuChar != nil {
		d.BulletChar = ls.BuChar.Char
	}
	if ls.BuNone == nil && ls.BuBlip != nil {
		d.BulletImageRelID = ls.BuBlip.Blip.Embed
	}
	if ls.BuNone == nil && ls.BuAutoNum != nil {
		d.AutoNum = true
		d.AutoNumType = ls.BuAutoNum.Type
		d.AutoNumStart = ls.BuAutoNum.StartAt
	}
	d.BulletSize, d.BulletSizePct, d.TabStops = BulletSize(ls.BuSzPts), BulletSizePercent(ls.BuSzPct), TabStops(ls.TabStops)
	if ls.DefRPr != nil {
		d.Bold, d.Italic, d.Underline, d.Strike, d.Caps, d.Baseline, d.FontSize = ls.DefRPr.Bold, ls.DefRPr.Italic, ls.DefRPr.Underline, ls.DefRPr.Strike, ls.DefRPr.Cap, ls.DefRPr.Baseline, ls.DefRPr.FontSize
		d.CharSpacing, d.Kern = ls.DefRPr.Spacing, ls.DefRPr.Kern
		d.Color, d.FontFamily = RunColor(ls.DefRPr, colors), RunFontFamily(ls.DefRPr, theme)
	}
	return d
}

func MergeEndParagraphRunDefaults(dst TextDefaults, rpr *parts.DMLRunPr, colors map[string]string, theme *ir.Theme) TextDefaults {
	if rpr == nil {
		return dst
	}
	if rpr.Bold != nil {
		dst.Bold = rpr.Bold
	}
	if rpr.Italic != nil {
		dst.Italic = rpr.Italic
	}
	if rpr.Underline != "" {
		dst.Underline = rpr.Underline
	}
	if rpr.Strike != "" {
		dst.Strike = rpr.Strike
	}
	if rpr.Cap != "" {
		dst.Caps = rpr.Cap
	}
	if rpr.Baseline != nil {
		dst.Baseline = rpr.Baseline
	}
	if rpr.FontSize > 0 {
		dst.FontSize = rpr.FontSize
	}
	if rpr.Spacing != nil {
		dst.CharSpacing = rpr.Spacing
	}
	if rpr.Kern != nil {
		dst.Kern = rpr.Kern
	}
	if v := RunFontFamily(rpr, theme); v != "" {
		dst.FontFamily = v
	}
	if v := RunColor(rpr, colors); v != "" {
		dst.Color = v
	}
	if v := RunHighlight(rpr, colors); v != "" {
		dst.Highlight = v
	}
	return dst
}

func BulletColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, colors map[string]string) string {
	if srgb != nil {
		return srgb.Val
	}
	if scheme != nil {
		return ThemeColor(colors, scheme.Val)
	}
	return ""
}
func BulletFontFamily(v *parts.BuFont) string {
	if v == nil {
		return ""
	}
	return v.Typeface
}
func BulletSize(v *parts.BuSzPts) int {
	if v == nil {
		return 0
	}
	return v.Val
}
func BulletSizePercent(v *parts.BuSzPct) int {
	if v == nil {
		return 0
	}
	return v.Val
}
func TabStops(stops []parts.TabStop) []ir.TabStop {
	if len(stops) == 0 {
		return nil
	}
	out := make([]ir.TabStop, len(stops))
	for i, s := range stops {
		out[i] = ir.TabStop{Pos: int64(s.Pos), Algn: s.Algn}
	}
	return out
}
func SpacingPoints(v *parts.DMLSpacing) *int {
	if v == nil || v.SpcPts == nil {
		return nil
	}
	n := v.SpcPts.Val
	return &n
}
func SpacingPercent(v *parts.DMLSpacing) *int {
	if v == nil || v.SpcPct == nil {
		return nil
	}
	n := v.SpcPct.Val
	return &n
}
func SpacingPercentValue(v *parts.DMLSpacing) *int {
	if v == nil || v.SpcPts != nil || v.SpcPct == nil {
		return nil
	}
	n := v.SpcPct.Val
	return &n
}

func FallbackThemeFont(theme *ir.Theme, ph ir.PlaceholderKind) string {
	if theme == nil {
		return ""
	}
	if ph == ir.PlaceholderTitle && theme.MajorFont != "" {
		return theme.MajorFont
	}
	if (ph == ir.PlaceholderBody || ph == ir.PlaceholderSubtitle) && theme.MinorFont != "" {
		return theme.MinorFont
	}
	if theme.MinorFont != "" {
		return theme.MinorFont
	}
	return theme.MajorFont
}
func RunColor(pr *parts.DMLRunPr, colors map[string]string) string {
	if pr == nil {
		return ""
	}
	if pr.SolidFill != nil {
		return pr.SolidFill.Val
	}
	if pr.SchemeColor != nil {
		return ThemeColor(colors, pr.SchemeColor.Val)
	}
	return ""
}
func RunHighlight(pr *parts.DMLRunPr, colors map[string]string) string {
	if pr == nil {
		return ""
	}
	if pr.Highlight != nil {
		return pr.Highlight.Val
	}
	if pr.HighlightScheme != nil {
		return ThemeColor(colors, pr.HighlightScheme.Val)
	}
	return ""
}
func RunFontFamily(pr *parts.DMLRunPr, theme *ir.Theme) string {
	if pr == nil {
		return ""
	}
	for _, ref := range []*parts.FontRef{pr.Latin, pr.EA, pr.CS} {
		if ref != nil && ref.Typeface != "" {
			if v := ThemeTypeface(theme, ref.Typeface); v != "" {
				return v
			}
		}
	}
	return ""
}
func ThemeTypeface(theme *ir.Theme, typeface string) string {
	typeface = strings.TrimSpace(typeface)
	if typeface == "" {
		return ""
	}
	token := strings.ToLower(typeface)
	if theme == nil {
		if strings.HasPrefix(token, "+mj") || strings.HasPrefix(token, "+mn") {
			return ""
		}
		return typeface
	}
	switch token {
	case "+mj", "+mj-lt", "+mj-latin":
		return theme.MajorFont
	case "+mn", "+mn-lt", "+mn-latin":
		return theme.MinorFont
	case "+mj-ea":
		if theme.MajorEAFont != "" {
			return theme.MajorEAFont
		}
		return theme.MajorFont
	case "+mn-ea":
		if theme.MinorEAFont != "" {
			return theme.MinorEAFont
		}
		return theme.MinorFont
	case "+mj-cs":
		if theme.MajorCSFont != "" {
			return theme.MajorCSFont
		}
		return theme.MajorFont
	case "+mn-cs":
		if theme.MinorCSFont != "" {
			return theme.MinorCSFont
		}
		return theme.MinorFont
	}
	if strings.HasPrefix(token, "+mj") || strings.HasPrefix(token, "+mn") {
		return ""
	}
	return typeface
}
