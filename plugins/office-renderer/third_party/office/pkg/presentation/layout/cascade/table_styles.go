package cascade

import (
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// EffectiveTableStyle selects the embedded or table-style-list definition that
// owns the table's visual cascade. This keeps PresentationML style lookup out
// of the semantic resolver.
func EffectiveTableStyle(table *parts.PresentationTable, styles *parts.TableStyleList) *parts.DrawingTableStyle {
	if table == nil || table.Props == nil {
		return nil
	}
	if table.Props.TableStyle != nil {
		return table.Props.TableStyle
	}
	if styles == nil {
		return nil
	}
	styleID := strings.TrimSpace(table.Props.TableStyleID)
	if styleID == "" {
		styleID = strings.TrimSpace(styles.Default)
	}
	if styleID == "" {
		return nil
	}
	for i := range styles.Styles {
		if strings.EqualFold(strings.TrimSpace(styles.Styles[i].StyleID), styleID) {
			return &styles.Styles[i]
		}
	}
	return nil
}

func TableColumnCount(table *parts.PresentationTable) int {
	if table == nil {
		return 0
	}
	if n := len(table.Grid.Columns); n > 0 {
		return n
	}
	maxCols := 0
	for _, row := range table.Rows {
		cols := 0
		for _, cell := range row.Cells {
			span := cell.GridSpan
			if span <= 0 || cell.HMerge || cell.VMerge {
				span = 1
			}
			cols += span
		}
		if cols > maxCols {
			maxCols = cols
		}
	}
	return maxCols
}

func TableColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, colors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	return TableSchemeColor(scheme, colors)
}

func TableLineStyle(line *parts.ConnectorLine, colors map[string]string) *drawing.LineStyle {
	if line == nil {
		return nil
	}
	style := line.DrawingLineStyle(colors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if color := TableSchemeColor(line.SolidFill.SchemeClr, colors); color != "" {
		style.Color = color
	}
	return style
}

func TableCellFill(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, grad *parts.GradFill, patt *parts.PattFill, blip *parts.BlipFill, noFill *struct{}, colors map[string]string) drawing.Fill {
	if noFill != nil {
		return drawing.NoFill{}
	}
	if srgb != nil && srgb.Val != "" {
		return drawing.SolidFill{Color: srgb.Val, Alpha: 1}
	}
	if scheme != nil {
		color := TableSchemeColor(scheme, colors)
		if color != "" || strings.TrimSpace(scheme.Val) != "" {
			return drawing.SolidFill{Color: color, ThemeColor: scheme.Val, Alpha: 1}
		}
	}
	if grad != nil {
		return (&parts.ShapeProperties{GradFill: grad}).DrawingFill(colors)
	}
	if patt != nil {
		return (&parts.ShapeProperties{PattFill: patt}).DrawingFill(colors)
	}
	if blip != nil {
		return (&parts.ShapeProperties{BlipFill: blip}).DrawingFill(colors)
	}
	return nil
}

func TableBorderStyle(border *parts.DrawingTableBorder, theme *parts.Theme, colors map[string]string) *drawing.LineStyle {
	if border == nil {
		return nil
	}
	if border.Line != nil {
		return TableLineStyle(border.Line, colors)
	}
	return TableLineRefStyle(border.LineRef, theme, colors)
}

func TableLineRefStyle(ref *parts.DrawingTableLineRef, theme *parts.Theme, colors map[string]string) *drawing.LineStyle {
	if ref == nil || theme == nil || ref.Idx < 1 || ref.Idx > len(theme.Elements.FmtScheme.LineStyleList) {
		return nil
	}
	line := theme.Elements.FmtScheme.LineStyleList[ref.Idx-1]
	style := TableLineStyle(&line, colors)
	if style == nil || line.SolidFill == nil || line.SolidFill.SchemeClr == nil {
		return style
	}
	if strings.EqualFold(strings.TrimSpace(line.SolidFill.SchemeClr.Val), "phClr") {
		base := TableColor(ref.Color, ref.SchemeClr, colors)
		if base != "" {
			style.Color = applyTableSchemeColorTransform(base, line.SolidFill.SchemeClr)
		}
	}
	return style
}

func TableFillRef(ref *parts.DrawingTableFillRef, theme *parts.Theme, colors map[string]string) drawing.Fill {
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
	return TableCellFill(fill.SolidFill, fill.FillScheme, fill.GradFill, fill.PattFill, fill.BlipFill, fill.NoFill, tableColorsWithPlaceholder(colors, TableColor(ref.Color, ref.SchemeClr, colors)))
}

func TableEffectRef(ref *parts.DrawingTableEffectRef, theme *parts.Theme, colors map[string]string) drawing.EffectList {
	if ref == nil || theme == nil || ref.Idx < 1 || ref.Idx > len(theme.Elements.FmtScheme.EffectStyleList) {
		return drawing.EffectList{}
	}
	style := theme.Elements.FmtScheme.EffectStyleList[ref.Idx-1]
	resolved := tableColorsWithPlaceholder(colors, TableColor(ref.Color, ref.SchemeClr, colors))
	return (&parts.ShapeProperties{OuterShadow: style.OuterShadow, PresetShadow: style.PresetShadow, FillOverlay: style.FillOverlay, InnerShadow: style.InnerShadow, Glow: style.Glow, SoftEdge: style.SoftEdge, Reflection: style.Reflection, Blur: style.Blur, EffectDag: style.EffectDag}).DrawingEffectList(resolved)
}

func TableSchemeColor(scheme *drawingxml.SchemeClr, colors map[string]string) string {
	if scheme == nil || colors == nil {
		return ""
	}
	resolved := drawingxml.ResolveSolidFill(&drawingxml.SolidFill{SchemeClr: scheme}, colors)
	if resolved == nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(resolved.Color)), "#")
}

func tableColorsWithPlaceholder(colors map[string]string, placeholder string) map[string]string {
	if placeholder == "" {
		return colors
	}
	out := make(map[string]string, len(colors)+1)
	for k, v := range colors {
		out[k] = v
	}
	out["phclr"], out["phClr"] = placeholder, placeholder
	return out
}

func applyTableSchemeColorTransform(hexRGB string, scheme *drawingxml.SchemeClr) string {
	if scheme == nil {
		return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(hexRGB)), "#")
	}
	return TableSchemeColor(scheme, tableColorsWithPlaceholder(nil, hexRGB))
}

// TableTextDefaults is the table-style contribution to the canonical text
// cascade. Paragraph/direct formatting is intentionally applied later by the
// shared text resolver; PresentationML table style interpretation lives here.
type TableTextDefaults struct {
	Bold       *bool
	Italic     *bool
	FontFamily string
	Color      string
}

// TableCellStyle is the effective visual and text style for one logical table
// cell after whole-table, band, edge and corner regions have been cascaded.
type TableCellStyle struct {
	Fill         drawing.Fill
	FillColor    string
	Effects      drawing.EffectList
	BorderLeft   *drawing.LineStyle
	BorderRight  *drawing.LineStyle
	BorderTop    *drawing.LineStyle
	BorderBottom *drawing.LineStyle
	InsideH      *drawing.LineStyle
	InsideV      *drawing.LineStyle
	Text         TableTextDefaults
}

func ResolveTableCellStyle(style *parts.DrawingTableStyle, props *parts.PresentationTableProps, rowIdx, colIdx, span, totalRows, totalCols int, colors map[string]string, theme *ir.Theme, rawTheme *parts.Theme) TableCellStyle {
	var resolved TableCellStyle
	apply := func(region *parts.TableStyleRegion) {
		resolved = mergeTableCellStyle(resolved, region, colors, theme, rawTheme)
	}
	if style == nil {
		return resolved
	}
	apply(style.WholeTbl)
	if props != nil && props.BandRow {
		if i, ok := tableBandedRegionIndex(rowIdx, totalRows, props.FirstRow, props.LastRow); ok {
			if i%2 == 0 {
				apply(style.Band1H)
			} else {
				apply(style.Band2H)
			}
		}
	}
	if props != nil && props.BandCol {
		if i, ok := tableBandedRegionIndex(colIdx, totalCols, props.FirstCol, props.LastCol); ok {
			if i%2 == 0 {
				apply(style.Band1V)
			} else {
				apply(style.Band2V)
			}
		}
	}
	lastCol := colIdx + span - 1
	if props != nil && props.FirstRow && rowIdx == 0 {
		apply(style.FirstRow)
	}
	if props != nil && props.LastRow && rowIdx == totalRows-1 {
		apply(style.LastRow)
	}
	if props != nil && props.FirstCol && colIdx == 0 {
		apply(style.FirstCol)
	}
	if props != nil && props.LastCol && lastCol == totalCols-1 {
		apply(style.LastCol)
	}
	if props != nil && props.FirstRow && props.FirstCol && rowIdx == 0 && colIdx == 0 {
		apply(style.NWCell)
	}
	if props != nil && props.FirstRow && props.LastCol && rowIdx == 0 && lastCol == totalCols-1 {
		apply(style.NECell)
	}
	if props != nil && props.LastRow && props.FirstCol && rowIdx == totalRows-1 && colIdx == 0 {
		apply(style.SWCell)
	}
	if props != nil && props.LastRow && props.LastCol && rowIdx == totalRows-1 && lastCol == totalCols-1 {
		apply(style.SECell)
	}
	applyTableInsideBorders(&resolved, rowIdx, colIdx, span, totalRows, totalCols)
	return resolved
}

func mergeTableCellStyle(dst TableCellStyle, region *parts.TableStyleRegion, colors map[string]string, theme *ir.Theme, rawTheme *parts.Theme) TableCellStyle {
	if region == nil {
		return dst
	}
	if cs := region.CellStyle; cs != nil {
		if fill := TableCellFill(cs.FillColor, cs.FillScheme, cs.GradFill, cs.PattFill, cs.BlipFill, cs.NoFill, colors); fill != nil {
			dst.Fill = fill
			dst.FillColor = tableSolidFillColor(fill)
		} else if fill := TableFillRef(cs.FillRef, rawTheme, colors); fill != nil {
			dst.Fill = fill
			dst.FillColor = tableSolidFillColor(fill)
		}
		if effects := TableEffectRef(cs.EffectRef, rawTheme, colors); len(effects.Effects) > 0 {
			dst.Effects = effects
		}
		if cs.Borders != nil {
			merge := func(cur **drawing.LineStyle, b *parts.DrawingTableBorder) {
				if b != nil {
					if v := TableBorderStyle(b, rawTheme, colors); v != nil {
						*cur = v
					}
				}
			}
			merge(&dst.BorderLeft, cs.Borders.Left)
			merge(&dst.BorderRight, cs.Borders.Right)
			merge(&dst.BorderTop, cs.Borders.Top)
			merge(&dst.BorderBottom, cs.Borders.Bottom)
			merge(&dst.InsideH, cs.Borders.InsideH)
			merge(&dst.InsideV, cs.Borders.InsideV)
		}
	}
	if region.TextStyle != nil {
		dst.Text = MergeTableTextDefaults(ResolveTableTextDefaults(region.TextStyle, colors, theme), dst.Text)
	}
	return dst
}

func ResolveTableTextDefaults(style *parts.DrawingTableCellTextStyle, colors map[string]string, theme *ir.Theme) TableTextDefaults {
	if style == nil {
		return TableTextDefaults{}
	}
	var d TableTextDefaults
	if v, ok := parseTableOnOff(style.Bold); ok {
		d.Bold = &v
	}
	if v, ok := parseTableOnOff(style.Italic); ok {
		d.Italic = &v
	}
	if style.Color != nil && style.Color.Val != "" {
		d.Color = style.Color.Val
	} else if c := TableSchemeColor(style.SchemeClr, colors); c != "" {
		d.Color = c
	} else if style.FontRef != nil {
		d.Color = TableColor(style.FontRef.Color, style.FontRef.SchemeClr, colors)
	}
	if style.FontRef != nil && theme != nil {
		switch strings.ToLower(strings.TrimSpace(style.FontRef.Idx)) {
		case "major":
			d.FontFamily = theme.MajorFont
		case "minor":
			d.FontFamily = theme.MinorFont
		}
	}
	return d
}

func MergeTableTextDefaults(dst, src TableTextDefaults) TableTextDefaults {
	if dst.Bold == nil {
		dst.Bold = src.Bold
	}
	if dst.Italic == nil {
		dst.Italic = src.Italic
	}
	if dst.FontFamily == "" {
		dst.FontFamily = src.FontFamily
	}
	if dst.Color == "" {
		dst.Color = src.Color
	}
	return dst
}

func applyTableInsideBorders(s *TableCellStyle, row, col, span, rows, cols int) {
	if s == nil {
		return
	}
	last := col + span - 1
	if s.BorderTop == nil && s.InsideH != nil && row > 0 {
		s.BorderTop = s.InsideH
	}
	if s.BorderBottom == nil && s.InsideH != nil && row < rows-1 {
		s.BorderBottom = s.InsideH
	}
	if s.BorderLeft == nil && s.InsideV != nil && col > 0 {
		s.BorderLeft = s.InsideV
	}
	if s.BorderRight == nil && s.InsideV != nil && last < cols-1 {
		s.BorderRight = s.InsideV
	}
}
func tableBandedRegionIndex(index, total int, first, last bool) (int, bool) {
	start, end := 0, total-1
	if first {
		start++
	}
	if last {
		end--
	}
	if index < start || index > end {
		return 0, false
	}
	return index - start, true
}
func parseTableOnOff(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on":
		return true, true
	case "0", "false", "off":
		return false, true
	}
	return false, false
}
func tableSolidFillColor(fill drawing.Fill) string {
	switch f := fill.(type) {
	case drawing.SolidFill:
		return f.Color
	case *drawing.SolidFill:
		if f != nil {
			return f.Color
		}
	case drawing.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
	}
	return ""
}
