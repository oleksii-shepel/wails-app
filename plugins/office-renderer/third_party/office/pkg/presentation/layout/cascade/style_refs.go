package cascade

import (
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// ShapeStyleFill resolves the first usable a:fillRef in the
// slide -> layout -> master chain against the active theme format scheme.
func ShapeStyleFill(shape, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideColors, layoutColors, masterColors map[string]string) drawing.Fill {
	for _, c := range []struct {
		shape  *parts.Shape
		colors map[string]string
	}{{shape, slideColors}, {layoutShape, layoutColors}, {masterShape, masterColors}} {
		if c.shape != nil && c.shape.Style != nil && c.shape.Style.FillRef != nil {
			if fill := ShapeFillRef(c.shape.Style.FillRef, rawTheme, c.colors); fill != nil {
				return fill
			}
		}
	}
	return nil
}

// ShapeStyleLine resolves the first usable a:lnRef in the inheritance chain.
func ShapeStyleLine(shape, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideColors, layoutColors, masterColors map[string]string) *drawing.LineStyle {
	for _, c := range []struct {
		shape  *parts.Shape
		colors map[string]string
	}{{shape, slideColors}, {layoutShape, layoutColors}, {masterShape, masterColors}} {
		if c.shape != nil && c.shape.Style != nil && c.shape.Style.LnRef != nil {
			if line := ShapeLineRef(c.shape.Style.LnRef, rawTheme, c.colors); line != nil {
				return line
			}
		}
	}
	return nil
}

// ShapeStyleEffects resolves the first non-empty a:effectRef in the inheritance chain.
func ShapeStyleEffects(shape, layoutShape, masterShape *parts.Shape, rawTheme *parts.Theme, slideColors, layoutColors, masterColors map[string]string) drawing.EffectList {
	for _, c := range []struct {
		shape  *parts.Shape
		colors map[string]string
	}{{shape, slideColors}, {layoutShape, layoutColors}, {masterShape, masterColors}} {
		if c.shape != nil && c.shape.Style != nil && c.shape.Style.EffectRef != nil {
			if effects := ShapeEffectRef(c.shape.Style.EffectRef, rawTheme, c.colors); len(effects.Effects) > 0 {
				return effects
			}
		}
	}
	return drawing.EffectList{}
}

// ShapeStyleFontColor resolves a:fontRef color inheritance to a canonical RGB color.
// The fontRef color is independent from the font family and must participate in
// the effective text cascade even when a paragraph/run has no local color.
func ShapeStyleFontColor(shape, layoutShape, masterShape *parts.Shape, slideColors, layoutColors, masterColors map[string]string) string {
	for _, c := range []struct {
		shape  *parts.Shape
		colors map[string]string
	}{{shape, slideColors}, {layoutShape, layoutColors}, {masterShape, masterColors}} {
		if c.shape != nil && c.shape.Style != nil && c.shape.Style.FontRef != nil {
			if color := styleRefColor(c.shape.Style.FontRef.Color, c.shape.Style.FontRef.SchemeClr, c.colors); color != "" {
				return color
			}
		}
	}
	return ""
}

// ConnectorStyleLine resolves a connector's a:lnRef against the active theme.
func ConnectorStyleLine(connector *parts.Connector, rawTheme *parts.Theme, colors map[string]string) *drawing.LineStyle {
	if connector == nil || connector.Style == nil || connector.Style.LnRef == nil {
		return nil
	}
	return ShapeLineRef(connector.Style.LnRef, rawTheme, colors)
}

// EffectiveConnectorLine composes a connector's local a:ln properties over
// its theme a:lnRef. PresentationML permits local line properties to be
// partial (for example only an arrowhead or width); treating any local a:ln
// as a complete style loses the theme color/dash/cap carried by lnRef.
func EffectiveConnectorLine(connector *parts.Connector, rawTheme *parts.Theme, colors map[string]string) *drawing.LineStyle {
	if connector == nil {
		return nil
	}
	base := ConnectorStyleLine(connector, rawTheme, colors)
	local := connector.SpPr.Line
	if local == nil {
		return base
	}
	direct := local.DrawingLineStyle(colors)
	if base == nil {
		return direct
	}
	if direct == nil {
		return base
	}

	// Copy before overlaying: theme format-matrix entries are shared by every
	// shape using the style and must never be mutated by one connector.
	out := *base
	if local.W != 0 {
		out.Width = direct.Width
	}
	if local.Cap != "" {
		out.Cap = direct.Cap
	}
	if local.Cmpd != "" {
		out.CompoundType = direct.CompoundType
	}
	if local.PrstDash != nil {
		out.Dash = direct.Dash
	}
	if local.Round != nil || local.Bevel != nil || local.Miter != nil {
		out.Join = direct.Join
		// Miter limit belongs to the authored join, not to the inherited
		// theme line. Without this overlay a local <a:miter lim=...> keeps
		// the format-matrix limit and changes sharp-corner geometry.
		if local.Miter != nil {
			out.MiterLimit = direct.MiterLimit
		}
	}
	// A locally authored fill/noFill owns paint. Structural local line
	// properties alone do not erase the theme line paint.
	if local.SolidFill != nil || local.NoFill != nil {
		out.Color = direct.Color
		out.ThemeColor = direct.ThemeColor
		out.Alpha = direct.Alpha
	}
	return &out
}

// ShapeStyleFontFamily resolves a:fontRef inheritance to a canonical family.
func ShapeStyleFontFamily(shape, layoutShape, masterShape *parts.Shape, theme *ir.Theme) string {
	for _, s := range []*parts.Shape{shape, layoutShape, masterShape} {
		if s != nil && s.Style != nil && s.Style.FontRef != nil {
			if family := shapeFontRef(s.Style.FontRef, theme); family != "" {
				return family
			}
		}
	}
	return ""
}

func ShapeFillRef(ref *parts.ShapeStyleRef, theme *parts.Theme, colors map[string]string) drawing.Fill {
	if ref == nil || theme == nil {
		return nil
	}
	var fill *parts.ThemeFormatFillStyle
	switch {
	case ref.Idx >= 1001:
		i := ref.Idx - 1001
		if i < 0 || i >= len(theme.Elements.FmtScheme.BGFillStyleList) {
			return nil
		}
		fill = &theme.Elements.FmtScheme.BGFillStyleList[i]
	case ref.Idx >= 1:
		i := ref.Idx - 1
		if i < 0 || i >= len(theme.Elements.FmtScheme.FillStyleList) {
			return nil
		}
		fill = &theme.Elements.FmtScheme.FillStyleList[i]
	default:
		return nil
	}
	resolvedColors := withPlaceholder(colors, styleRefColor(ref.Color, ref.SchemeClr, colors))
	return themeFill(fill, resolvedColors)
}

func ShapeLineRef(ref *parts.ShapeStyleRef, theme *parts.Theme, colors map[string]string) *drawing.LineStyle {
	if ref == nil || theme == nil || ref.Idx < 1 || ref.Idx > len(theme.Elements.FmtScheme.LineStyleList) {
		return nil
	}
	line := theme.Elements.FmtScheme.LineStyleList[ref.Idx-1]
	style := line.DrawingLineStyle(colors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if strings.EqualFold(strings.TrimSpace(line.SolidFill.SchemeClr.Val), "phClr") {
		if base := styleRefColor(ref.Color, ref.SchemeClr, colors); base != "" {
			style.Color = schemeColor(line.SolidFill.SchemeClr, withPlaceholder(nil, base))
		}
	}
	return style
}

func ShapeEffectRef(ref *parts.ShapeStyleRef, theme *parts.Theme, colors map[string]string) drawing.EffectList {
	if ref == nil || theme == nil || ref.Idx < 1 || ref.Idx > len(theme.Elements.FmtScheme.EffectStyleList) {
		return drawing.EffectList{}
	}
	style := theme.Elements.FmtScheme.EffectStyleList[ref.Idx-1]
	colors = withPlaceholder(colors, styleRefColor(ref.Color, ref.SchemeClr, colors))
	return (&parts.ShapeProperties{OuterShadow: style.OuterShadow, PresetShadow: style.PresetShadow, FillOverlay: style.FillOverlay, InnerShadow: style.InnerShadow, Glow: style.Glow, SoftEdge: style.SoftEdge, Reflection: style.Reflection, Blur: style.Blur, EffectDag: style.EffectDag}).DrawingEffectList(colors)
}

func themeFill(fill *parts.ThemeFormatFillStyle, colors map[string]string) drawing.Fill {
	if fill == nil {
		return nil
	}
	if fill.NoFill != nil {
		return drawing.NoFill{}
	}
	if fill.SolidFill != nil && fill.SolidFill.Val != "" {
		return drawing.SolidFill{Color: fill.SolidFill.Val, Alpha: 1}
	}
	if fill.FillScheme != nil {
		color := schemeColor(fill.FillScheme, colors)
		if color != "" || strings.TrimSpace(fill.FillScheme.Val) != "" {
			return drawing.SolidFill{Color: color, ThemeColor: fill.FillScheme.Val, Alpha: 1}
		}
	}
	sp := &parts.ShapeProperties{GradFill: fill.GradFill, PattFill: fill.PattFill, BlipFill: fill.BlipFill}
	return sp.DrawingFill(colors)
}

func styleRefColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, colors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	return schemeColor(scheme, colors)
}

func schemeColor(scheme *drawingxml.SchemeClr, colors map[string]string) string {
	if scheme == nil {
		return ""
	}
	resolved := drawingxml.ResolveSolidFill(&drawingxml.SolidFill{SchemeClr: scheme}, colors)
	if resolved == nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(resolved.Color)), "#")
}

func withPlaceholder(colors map[string]string, placeholder string) map[string]string {
	if placeholder == "" {
		return colors
	}
	out := make(map[string]string, len(colors)+2)
	for k, v := range colors {
		out[k] = v
	}
	out["phclr"], out["phClr"] = placeholder, placeholder
	return out
}

func shapeFontRef(ref *parts.ShapeFontRef, theme *ir.Theme) string {
	if ref == nil || theme == nil {
		return ""
	}
	var family string
	switch strings.ToLower(strings.TrimSpace(ref.Idx)) {
	case "major":
		family = theme.MajorFont
	case "minor":
		family = theme.MinorFont
	}
	return themeTypeface(theme, family)
}

func themeTypeface(theme *ir.Theme, typeface string) string {
	typeface = strings.TrimSpace(typeface)
	if typeface == "" {
		return ""
	}
	switch strings.ToLower(typeface) {
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
	if strings.HasPrefix(strings.ToLower(typeface), "+mj") || strings.HasPrefix(strings.ToLower(typeface), "+mn") {
		return ""
	}
	return typeface
}
