package cascade

import (
	"strings"

	"office/pkg/ooxml/smartart"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// SmartArtFormatScheme projects the active presentation theme format matrix
// into the format-agnostic style scheme consumed by the shared SmartArt engine.
// Layout owns this bridge because it is theme/style composition, not OOXML
// package resolution.
func SmartArtFormatScheme(rawTheme *parts.Theme, themeColors map[string]string, theme *ir.Theme) *smartart.ThemeFormatScheme {
	if rawTheme == nil {
		return nil
	}
	fs := rawTheme.Elements.FmtScheme
	scheme := &smartart.ThemeFormatScheme{}

	for i := range fs.FillStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		scheme.FillStyles = append(scheme.FillStyles, smartart.ThemeFillStyle{
			Fill:            ShapeFillRef(ref, rawTheme, themeColors),
			UsesPlaceholder: themeFormatFillUsesPlaceholder(&fs.FillStyleList[i]),
		})
	}
	for i := range fs.BGFillStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1001}
		scheme.BgFillStyles = append(scheme.BgFillStyles, smartart.ThemeFillStyle{
			Fill:            ShapeFillRef(ref, rawTheme, themeColors),
			UsesPlaceholder: themeFormatFillUsesPlaceholder(&fs.BGFillStyleList[i]),
		})
	}
	for i := range fs.LineStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		ln := &fs.LineStyleList[i]
		usesPlaceholder := ln.SolidFill != nil && ln.SolidFill.SchemeClr != nil &&
			strings.EqualFold(strings.TrimSpace(ln.SolidFill.SchemeClr.Val), "phClr")
		scheme.LineStyles = append(scheme.LineStyles, smartart.ThemeLineStyle{
			Line:            ShapeLineRef(ref, rawTheme, themeColors),
			UsesPlaceholder: usesPlaceholder,
		})
	}
	for i := range fs.EffectStyleList {
		ref := &parts.ShapeStyleRef{Idx: i + 1}
		scheme.EffectStyles = append(scheme.EffectStyles, ShapeEffectRef(ref, rawTheme, themeColors))
	}
	if theme != nil {
		scheme.MajorFont = theme.MajorFont
		scheme.MinorFont = theme.MinorFont
	}
	return scheme
}

func themeFormatFillUsesPlaceholder(fill *parts.ThemeFormatFillStyle) bool {
	return fill != nil && fill.FillScheme != nil &&
		strings.EqualFold(strings.TrimSpace(fill.FillScheme.Val), "phClr")
}
