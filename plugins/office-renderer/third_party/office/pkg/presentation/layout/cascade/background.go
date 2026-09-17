package cascade

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// BackgroundColor projects the authored background color into its effective
// canonical color. Background interpretation belongs to composition so callers
// do not need to understand PresentationML bgPr/bgRef alternatives.
func BackgroundColor(bg *parts.SlideBg, themeColors map[string]string) string {
	if bg == nil {
		return ""
	}
	if bg.BgPrSrgb != nil {
		return bg.BgPrSrgb.Val
	}
	if bg.BgPrScheme != nil {
		return ThemeColor(themeColors, bg.BgPrScheme.Val)
	}
	if bg.BgRef != nil {
		if bg.BgRef.Srgb != nil {
			return bg.BgRef.Srgb.Val
		}
		if bg.BgRef.Scheme != nil {
			return ThemeColor(themeColors, bg.BgRef.Scheme.Val)
		}
	}
	return ""
}

// BackgroundFill resolves an authored slide/layout/master background into a
// renderer-ready fill, including theme background style references and
// relationship-backed picture fills.
func BackgroundFill(bg *parts.SlideBg, themeColors map[string]string, rawTheme *parts.Theme, mediaParts map[string]media.Part) drawing.Fill {
	if bg == nil {
		return nil
	}
	if bg.BgPrNoFill != nil {
		return drawing.NoFill{}
	}
	if bg.BgPrSrgb != nil {
		return drawing.SolidFill{Color: bg.BgPrSrgb.Val, Alpha: 1}
	}
	if bg.BgPrScheme != nil {
		return drawing.SolidFill{Color: ThemeColor(themeColors, bg.BgPrScheme.Val), ThemeColor: bg.BgPrScheme.Val, Alpha: 1}
	}
	if bg.BgPrGradFill != nil {
		if fill := (&parts.ShapeProperties{GradFill: bg.BgPrGradFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if bg.BgPrPattFill != nil {
		if fill := (&parts.ShapeProperties{PattFill: bg.BgPrPattFill}).DrawingFill(themeColors); fill != nil {
			return fill
		}
	}
	if bg.BgPrBlipFill != nil {
		if fill := (&parts.ShapeProperties{BlipFill: bg.BgPrBlipFill}).DrawingFill(themeColors); fill != nil {
			return MaterializeFill(fill, mediaParts)
		}
	}
	if bg.BgRef != nil && rawTheme != nil {
		idx := bg.BgRef.Idx - 1001
		if idx >= 0 && idx < len(rawTheme.Elements.FmtScheme.BGFillStyleList) {
			return MaterializeFill(ThemeFormatFill(&rawTheme.Elements.FmtScheme.BGFillStyleList[idx], themeColors), mediaParts)
		}
	}
	return nil
}

// ThemeFormatFill projects one theme format-scheme fill style without leaking
// the OOXML style-list representation beyond the layout cascade boundary.
func ThemeFormatFill(style *parts.ThemeFormatFillStyle, themeColors map[string]string) drawing.Fill {
	if style == nil {
		return nil
	}
	if style.NoFill != nil {
		return drawing.NoFill{}
	}
	if style.SolidFill != nil && style.SolidFill.Val != "" {
		return drawing.SolidFill{Color: style.SolidFill.Val, Alpha: 1}
	}
	if style.FillScheme != nil {
		return drawing.SolidFill{Color: ThemeColor(themeColors, style.FillScheme.Val), ThemeColor: style.FillScheme.Val, Alpha: 1}
	}
	if style.GradFill != nil {
		return (&parts.ShapeProperties{GradFill: style.GradFill}).DrawingFill(themeColors)
	}
	if style.PattFill != nil {
		return (&parts.ShapeProperties{PattFill: style.PattFill}).DrawingFill(themeColors)
	}
	if style.BlipFill != nil {
		return (&parts.ShapeProperties{BlipFill: style.BlipFill}).DrawingFill(themeColors)
	}
	return nil
}
