package layout

import (
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/resolve"
)

// resolveShape adapts the resolve.resolveShape function for use in layout package.
func resolveShape(sp *parts.Shape, layoutShape, masterShape *parts.Shape, master *parts.SlideMaster, themeColors map[string]string, theme *resolve.Theme) *resolve.Shape {
	ph := resolvePlaceholderKind(sp.NvSpPr.NvPr.Ph)
	shape := &resolve.Shape{
		ID:          sp.NvSpPr.CNvPr.ID,
		Name:        sp.NvSpPr.CNvPr.Name,
		Kind:        resolve.ShapeText,
		Placeholder: ph,
		Bounds:      resolveTransform(sp.SpPr.Xfrm),
		RotationDeg: shapeRotationDegrees(sp.SpPr.Xfrm),
	}
	if geom := sp.SpPr.DrawingGeometry(); geom != nil {
		if geom.Type == drawing.GeometryTypeCustom && len(geom.Paths) > 0 {
			shape.GeometryData = geom
		} else if pg := sp.SpPr.PrstGeom.DrawingPresetGeometry(); pg != nil && pg.Name != "" && pg.Name != "rect" {
			shape.Geometry = pg.Name
		}
	}
	if fill := sp.SpPr.DrawingFill(themeColors); fill != nil {
		shape.Fill = fill
		shape.FillColor = solidFillColor(fill)
	}
	if effects := sp.SpPr.DrawingEffectList(themeColors); len(effects.Effects) > 0 {
		shape.Effects = effects
	}
	if lineStyle := sp.SpPr.Line.DrawingLineStyle(themeColors); lineStyle != nil {
		shape.LineStyle = lineStyle
		shape.LineColor = lineStyle.Color
		shape.LineWidthPt = lineStyle.Width
	}
	if sp.TxBody != nil {
		bodyStyle := resolveTextBodyStyle(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape))
		shape.TextAnchor = bodyStyle.Anchor
		shape.TextInsetLeftEMU = bodyStyle.LIns
		shape.TextInsetRightEMU = bodyStyle.RIns
		shape.TextInsetTopEMU = bodyStyle.TIns
		shape.TextInsetBottomEMU = bodyStyle.BIns
		shape.TextAllowOverflow = bodyStyle.Autofit == "noAutofit"
		shape.TextAutoGrow = bodyStyle.Autofit == "spAutoFit"
		shape.SentinelPlaceholder = placeholderIdxIsSentinel(sp.NvSpPr.NvPr.Ph)
		shape.TextBodyProps = bodyStyle.drawingProps()

		for _, ap := range sp.TxBody.Paragraphs {
			level := 0
			if ap.Properties != nil {
				level = ap.Properties.Lvl
			}
			defaults := resolveTextDefaults(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape), master, ph, level, themeColors, theme)
			shape.Paragraphs = append(shape.Paragraphs, resolveParagraph(&ap, defaults, themeColors, theme))
		}
	}
	return shape
}

// resolvePicture adapts the resolve.resolvePicture function.
func resolvePicture(pic *parts.Picture, mediaParts map[string]media.Part, themeColors map[string]string) *resolve.Shape {
	shape := &resolve.Shape{
		ID:     pic.NvPicPr.CNvPr.ID,
		Name:   pic.NvPicPr.CNvPr.Name,
		Bounds: resolveTransform(pic.SpPr.Xfrm),
	}
	if pic.BlipFill.SrcRect != nil {
		shape.ImageCropLeft = clampCropPct(pic.BlipFill.SrcRect.L)
		shape.ImageCropTop = clampCropPct(pic.BlipFill.SrcRect.T)
		shape.ImageCropRight = clampCropPct(pic.BlipFill.SrcRect.R)
		shape.ImageCropBottom = clampCropPct(pic.BlipFill.SrcRect.B)
	}
	if effects := pic.SpPr.DrawingEffectList(themeColors); len(effects.Effects) > 0 {
		shape.Effects = effects
	}
	if mediaRef := pic.NvPicPr.NvPr.PrimaryMedia(); mediaRef != nil {
		shape.Kind = resolve.ShapeMedia
		shape.MediaRelID = mediaRef.RelID()
		shape.MediaKind = pic.NvPicPr.NvPr.MediaKind()
		if m, ok := mediaParts[shape.MediaRelID]; ok {
			shape.MediaData = m.Data
			shape.MediaMIME = m.ContentType
			if shape.MediaKind == "" {
				shape.MediaKind = inferMediaKind(m.ContentType)
			}
		}
		if pic.BlipFill.Blip != nil {
			relID := pic.BlipFill.Blip.Embed
			shape.ImageRelID = relID
			if m, ok := mediaParts[relID]; ok {
				shape.MediaPosterData = m.Data
				shape.MediaPosterMIME = m.ContentType
			}
		}
	} else {
		shape.Kind = resolve.ShapeImage
	}
	if pic.BlipFill.Blip != nil {
		relID := pic.BlipFill.Blip.Embed
		if shape.Kind == resolve.ShapeImage {
			shape.ImageRelID = relID
			if m, ok := mediaParts[relID]; ok {
				shape.ImageData = m.Data
				shape.ImageMIME = m.ContentType
			}
		}
	}
	return shape
}

func inferMediaKind(contentType string) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch {
	case strings.HasPrefix(baseType, "video/"):
		return "video"
	case strings.HasPrefix(baseType, "audio/"):
		return "audio"
	default:
		return ""
	}
}

// resolveGroupShape adapts the resolve.resolveGroupShape function.
func resolveGroupShape(grp *parts.GroupShape, master *parts.SlideMaster, themeColors map[string]string, theme *resolve.Theme, mediaParts map[string]media.Part) *resolve.Shape {
	shape := &resolve.Shape{Kind: resolve.ShapeGroup}

	// Get group transform if present
	var groupXfrm *parts.Transform2D
	if grp.GrpSpPr != nil {
		groupXfrm = grp.GrpSpPr.Xfrm
	}

	for _, sp := range grp.Shapes {
		child := resolveShape(&sp, nil, nil, master, themeColors, theme)
		if child != nil {
			// Apply group transform to child bounds if group has a transform
			if groupXfrm != nil {
				child.Bounds = applyGroupTransformToChildBounds(child.Bounds, groupXfrm)
			}
			shape.Children = append(shape.Children, child)
		}
	}
	for _, pic := range grp.Pictures {
		child := resolvePicture(&pic, mediaParts, themeColors)
		if child != nil {
			// Apply group transform to child bounds if group has a transform
			if groupXfrm != nil {
				child.Bounds = applyGroupTransformToChildBounds(child.Bounds, groupXfrm)
			}
			shape.Children = append(shape.Children, child)
		}
	}
	for _, g := range grp.GroupShapes {
		child := resolveGroupShape(&g, master, themeColors, theme, mediaParts)
		if child != nil {
			// Apply group transform to child bounds if group has a transform
			if groupXfrm != nil {
				child.Bounds = applyGroupTransformToChildBounds(child.Bounds, groupXfrm)
			}
			shape.Children = append(shape.Children, child)
		}
	}
	for i := range grp.Connectors {
		child := resolveConnector(&grp.Connectors[i], themeColors, nil)
		if child != nil {
			if groupXfrm != nil {
				child.Bounds = applyGroupTransformToChildBounds(child.Bounds, groupXfrm)
			}
			shape.Children = append(shape.Children, child)
		}
	}
	return shape
}

// resolveConnector adapts the resolve.resolveConnector function.
func resolveConnector(cx *parts.Connector, themeColors map[string]string, shapeIndex map[int]*resolve.Shape) *resolve.Shape {
	shape := &resolve.Shape{
		ID:             cx.NvCxnSpPr.CNvPr.ID,
		Name:           cx.NvCxnSpPr.CNvPr.Name,
		Kind:           resolve.ShapeConnector,
		Bounds:         resolveTransform(cx.SpPr.Xfrm),
		ConnectorType:  connectorType(cx.SpPr.PrstGeom),
		ConnectorFlipH: cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipH,
		ConnectorFlipV: cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipV,
		ConnectorAdjustments: connectorAdjValues(cx.SpPr.PrstGeom),
	}
	if lineStyle := cx.SpPr.Line.DrawingLineStyle(themeColors); lineStyle != nil {
		shape.LineStyle = lineStyle
		shape.LineColor = lineStyle.Color
		shape.LineWidthPt = lineStyle.Width
	}
	if cx.SpPr.Line != nil {
		if cx.SpPr.Line.HeadEnd != nil && cx.SpPr.Line.HeadEnd.Type != "none" {
			shape.ArrowHead = cx.SpPr.Line.HeadEnd.Type
			shape.ArrowHeadWidth = cx.SpPr.Line.HeadEnd.W
			shape.ArrowHeadLength = cx.SpPr.Line.HeadEnd.Len
		}
		if cx.SpPr.Line.TailEnd != nil && cx.SpPr.Line.TailEnd.Type != "none" {
			shape.ArrowTail = cx.SpPr.Line.TailEnd.Type
			shape.ArrowTailWidth = cx.SpPr.Line.TailEnd.W
			shape.ArrowTailLength = cx.SpPr.Line.TailEnd.Len
		}
	}
	if cx.NvCxnSpPr.CNvCxnSpPr != nil {
		shape.ConnectorStart = resolveConnectionPoint(cx.NvCxnSpPr.CNvCxnSpPr.StCxn, shapeIndex)
		shape.ConnectorEnd = resolveConnectionPoint(cx.NvCxnSpPr.CNvCxnSpPr.EndCxn, shapeIndex)
	}
	if shape.LineWidthPt == 0 {
		shape.LineWidthPt = 1.0
	}
	return shape
}

// Helper functions adapted from resolve package

func layoutTextBody(shape *parts.Shape) *parts.TextBody {
	if shape == nil {
		return nil
	}
	return shape.TxBody
}

func clampCropPct(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

func orderedShapeTreeItems(tree parts.ShapeTree) []parts.ShapeTreeItem {
	if len(tree.Items) > 0 {
		return tree.Items
	}
	var items []parts.ShapeTreeItem
	for i := range tree.Shapes {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemShape, Index: i})
	}
	for i := range tree.Pictures {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemPicture, Index: i})
	}
	for i := range tree.GroupShapes {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemGroup, Index: i})
	}
	for i := range tree.Connectors {
		items = append(items, parts.ShapeTreeItem{Kind: parts.ShapeTreeItemConnector, Index: i})
	}
	return items
}

func indexShape(index map[int]*resolve.Shape, shape *resolve.Shape) {
	if shape == nil {
		return
	}
	if shape.ID != 0 {
		index[shape.ID] = shape
	}
	for _, child := range shape.Children {
		indexShape(index, child)
	}
}

func connectorType(geom *parts.PresetGeometry) string {
	if pg := geom.DrawingPresetGeometry(); pg != nil {
		return pg.Name
	}
	if geom == nil {
		return ""
	}
	return geom.Prst
}

func connectorAdjValues(geom *parts.PresetGeometry) []int {
	if geom == nil {
		return nil
	}
	var vals []int
	for _, guide := range geom.AvLst {
		fields := strings.Fields(guide.Fmla)
		if len(fields) != 2 || fields[0] != "val" {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		vals = append(vals, n)
	}
	return vals
}

func solidFillColor(fill drawing.Fill) string {
	switch f := fill.(type) {
	case drawing.SolidFill:
		return f.Color
	case *drawing.SolidFill:
		return f.Color
	case drawing.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	case *drawing.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	case drawing.PatternFill:
		if f.Foreground != "" {
			return f.Foreground
		}
		return f.Background
	case *drawing.PatternFill:
		if f.Foreground != "" {
			return f.Foreground
		}
		return f.Background
	case drawing.PictureFill:
		// For picture fills, return empty string (no solid color fallback)
		return ""
	case *drawing.PictureFill:
		// For picture fills, return empty string (no solid color fallback)
		return ""
	default:
		return ""
	}
}

func resolveConnectionPoint(ref *parts.ConnectionSiteRef, shapeIndex map[int]*resolve.Shape) *resolve.Point {
	if ref == nil || shapeIndex == nil {
		return nil
	}
	shape := shapeIndex[ref.ID]
	if shape == nil {
		return nil
	}
	pt := shapeConnectionPoint(shape.Bounds, ref.Idx)
	return &pt
}

func shapeConnectionPoint(bounds resolve.Rect, idx int) resolve.Point {
	switch idx {
	case 0:
		return resolve.Point{X: bounds.X + bounds.W/2, Y: bounds.Y}
	case 1:
		return resolve.Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H/2}
	case 2:
		return resolve.Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H}
	case 3:
		return resolve.Point{X: bounds.X, Y: bounds.Y + bounds.H/2}
	default:
		return resolve.Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H/2}
	}
}

const (
	defaultTextInsetLeftRightEMU = 91440
	defaultTextInsetTopBottomEMU = 45720
)

type textBodyStyle struct {
	Anchor                    string
	LIns                      int64
	RIns                      int64
	TIns                      int64
	BIns                      int64
	Autofit                   string
	AutofitFontScale          int
	AutofitLineSpaceReduction int
}

func (style textBodyStyle) drawingProps() drawing.TextBodyProperties {
	return drawing.TextBodyProperties{
		Anchor:                    style.Anchor,
		AutofitType:               style.Autofit,
		AutofitFontScale:          style.AutofitFontScale,
		AutofitLineSpaceReduction: style.AutofitLineSpaceReduction,
		InsetL:                    style.LIns,
		InsetR:                    style.RIns,
		InsetT:                    style.TIns,
		InsetB:                    style.BIns,
	}
}

func resolveTextBodyStyle(shapeBody, layoutBody, masterBody *parts.TextBody) textBodyStyle {
	style := textBodyStyle{
		Anchor: "t",
		LIns:   defaultTextInsetLeftRightEMU,
		RIns:   defaultTextInsetLeftRightEMU,
		TIns:   defaultTextInsetTopBottomEMU,
		BIns:   defaultTextInsetTopBottomEMU,
	}
	applyTextBodyStyle(&style, masterBody)
	applyTextBodyStyle(&style, layoutBody)
	applyTextBodyStyle(&style, shapeBody)
	return style
}

func applyTextBodyStyle(style *textBodyStyle, body *parts.TextBody) {
	if style == nil || body == nil || body.BodyPr == nil {
		return
	}
	pr := body.BodyPr
	if pr.Anchor != "" {
		style.Anchor = pr.Anchor
	}
	if pr.LIns != nil {
		style.LIns = *pr.LIns
	}
	if pr.RIns != nil {
		style.RIns = *pr.RIns
	}
	if pr.TIns != nil {
		style.TIns = *pr.TIns
	}
	if pr.BIns != nil {
		style.BIns = *pr.BIns
	}
	switch {
	case pr.NoAutofit != nil:
		style.Autofit = "noAutofit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.SpAutoFit != nil:
		style.Autofit = "spAutoFit"
		style.AutofitFontScale = 0
		style.AutofitLineSpaceReduction = 0
	case pr.NormAutofit != nil:
		style.Autofit = "normAutofit"
		if pr.NormAutofit.FontScale != nil {
			style.AutofitFontScale = *pr.NormAutofit.FontScale
		} else {
			style.AutofitFontScale = 0
		}
		if pr.NormAutofit.LnSpcReduction != nil {
			style.AutofitLineSpaceReduction = *pr.NormAutofit.LnSpcReduction
		} else {
			style.AutofitLineSpaceReduction = 0
		}
	}
}

type textDefaults struct {
	Align            string
	MarginLeftEMU    *int
	TextIndentEMU    *int
	BulletColor      string
	BulletFontFamily string
	BulletSize       int
	Bold             *bool
	Italic           *bool
	Underline        string
	Strike           string
	Caps             string
	Baseline         *int
	FontSize         int
	FontFamily       string
	Color            string
	SpaceBefore      *int
	SpaceAfter       *int
	LineSpacingPct   *int
}

func resolveTextDefaults(shapeBody, layoutBody, masterBody *parts.TextBody, master *parts.SlideMaster, ph resolve.PlaceholderKind, level int, themeColors map[string]string, theme *resolve.Theme) textDefaults {
	defaults := textDefaults{}
	if shapeBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(shapeBody.ListStyle, level, themeColors, theme))
	}
	if layoutBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(layoutBody.ListStyle, level, themeColors, theme))
	}
	if masterBody != nil {
		defaults = mergeTextDefaults(defaults, defaultsFromListStyle(masterBody.ListStyle, level, themeColors, theme))
	}
	defaults = mergeTextDefaults(defaults, defaultsFromMaster(master, ph, level, themeColors, theme))
	if defaults.FontFamily == "" {
		defaults.FontFamily = fallbackThemeFont(theme, ph)
	}
	return defaults
}

func mergeTextDefaults(dst, src textDefaults) textDefaults {
	if dst.Align == "" {
		dst.Align = src.Align
	}
	if dst.MarginLeftEMU == nil {
		dst.MarginLeftEMU = src.MarginLeftEMU
	}
	if dst.TextIndentEMU == nil {
		dst.TextIndentEMU = src.TextIndentEMU
	}
	if dst.BulletColor == "" {
		dst.BulletColor = src.BulletColor
	}
	if dst.BulletFontFamily == "" {
		dst.BulletFontFamily = src.BulletFontFamily
	}
	if dst.BulletSize == 0 {
		dst.BulletSize = src.BulletSize
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
	if dst.FontFamily == "" {
		dst.FontFamily = src.FontFamily
	}
	if dst.Color == "" {
		dst.Color = src.Color
	}
	if dst.SpaceBefore == nil {
		dst.SpaceBefore = src.SpaceBefore
	}
	if dst.SpaceAfter == nil {
		dst.SpaceAfter = src.SpaceAfter
	}
	if dst.LineSpacingPct == nil {
		dst.LineSpacingPct = src.LineSpacingPct
	}
	return dst
}

func defaultsFromMaster(master *parts.SlideMaster, ph resolve.PlaceholderKind, level int, themeColors map[string]string, theme *resolve.Theme) textDefaults {
	if master == nil || master.TxStyles == nil {
		return textDefaults{}
	}
	var style *parts.DMLTextListStyle
	switch ph {
	case resolve.PlaceholderTitle:
		style = master.TxStyles.TitleStyle
	case resolve.PlaceholderBody, resolve.PlaceholderSubtitle:
		style = master.TxStyles.BodyStyle
	default:
		style = master.TxStyles.OtherStyle
		if style == nil {
			style = master.TxStyles.BodyStyle
		}
	}
	return defaultsFromListStyle(style, level, themeColors, theme)
}

func defaultsFromListStyle(style *parts.DMLTextListStyle, level int, themeColors map[string]string, theme *resolve.Theme) textDefaults {
	levelStyle := style.Level(level)
	if levelStyle == nil {
		return textDefaults{}
	}
	defaults := textDefaults{
		Align: levelStyle.Algn,
	}
	defaults.MarginLeftEMU = levelStyle.MarL
	defaults.TextIndentEMU = levelStyle.Indent
	defaults.SpaceBefore = resolveSpacingPts(levelStyle.SpcBef)
	defaults.SpaceAfter = resolveSpacingPts(levelStyle.SpcAft)
	defaults.LineSpacingPct = resolveSpacingPct(levelStyle.LnSpc)
	defaults.BulletColor = resolveBulletColor(levelStyle.BuClr, levelStyle.BuClrScheme, themeColors)
	defaults.BulletFontFamily = resolveBulletFontFamily(levelStyle.BuFont)
	defaults.BulletSize = resolveBulletSize(levelStyle.BuSzPts)
	if levelStyle.DefRPr != nil {
		defaults.Bold = levelStyle.DefRPr.Bold
		defaults.Italic = levelStyle.DefRPr.Italic
		defaults.Underline = levelStyle.DefRPr.Underline
		defaults.Strike = levelStyle.DefRPr.Strike
		defaults.Caps = levelStyle.DefRPr.Cap
		defaults.Baseline = levelStyle.DefRPr.Baseline
		defaults.FontSize = levelStyle.DefRPr.FontSize
		defaults.Color = resolveRunColor(levelStyle.DefRPr, themeColors)
		defaults.FontFamily = resolveRunFontFamily(levelStyle.DefRPr, theme)
	}
	return defaults
}

func resolveBulletColor(srgb *parts.ColorRef, scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if srgb != nil {
		return srgb.Val
	}
	if scheme != nil {
		return themeColors[scheme.Val]
	}
	return ""
}

func resolveBulletFontFamily(font *parts.BuFont) string {
	if font == nil {
		return ""
	}
	return font.Typeface
}

func resolveBulletSize(size *parts.BuSzPts) int {
	if size == nil {
		return 0
	}
	return size.Val
}

func resolveSpacingPts(spacing *parts.DMLSpacing) *int {
	if spacing == nil || spacing.SpcPts == nil {
		return nil
	}
	value := spacing.SpcPts.Val
	return &value
}

func resolveSpacingPct(spacing *parts.DMLSpacing) *int {
	if spacing == nil || spacing.SpcPct == nil {
		return nil
	}
	value := spacing.SpcPct.Val
	return &value
}

func fallbackThemeFont(theme *resolve.Theme, ph resolve.PlaceholderKind) string {
	if theme == nil {
		return ""
	}
	switch ph {
	case resolve.PlaceholderTitle:
		if theme.MajorFont != "" {
			return theme.MajorFont
		}
	case resolve.PlaceholderBody, resolve.PlaceholderSubtitle:
		if theme.MinorFont != "" {
			return theme.MinorFont
		}
	}
	if theme.MinorFont != "" {
		return theme.MinorFont
	}
	return theme.MajorFont
}

func resolveRunColor(pr *parts.DMLRunPr, themeColors map[string]string) string {
	if pr == nil {
		return ""
	}
	if pr.SolidFill != nil {
		return pr.SolidFill.Val
	}
	if pr.SchemeColor != nil {
		return themeColors[pr.SchemeColor.Val]
	}
	return ""
}

func resolveRunFontFamily(pr *parts.DMLRunPr, theme *resolve.Theme) string {
	if pr == nil {
		return ""
	}
	for _, ref := range []*parts.FontRef{pr.Latin, pr.EA, pr.CS} {
		if ref == nil || ref.Typeface == "" {
			continue
		}
		if family := resolveThemeTypeface(theme, ref.Typeface); family != "" {
			return family
		}
	}
	return ""
}

func resolveThemeTypeface(theme *resolve.Theme, typeface string) string {
	typeface = strings.TrimSpace(typeface)
	if typeface == "" {
		return ""
	}
	token := strings.ToLower(typeface)
	switch token {
	case "+mj", "+mj-lt", "+mj-latin":
		if theme != nil && theme.MajorFont != "" {
			return theme.MajorFont
		}
		return ""
	case "+mn", "+mn-lt", "+mn-latin":
		if theme != nil && theme.MinorFont != "" {
			return theme.MinorFont
		}
		return ""
	case "+mj-ea":
		if theme != nil {
			if theme.MajorEAFont != "" {
				return theme.MajorEAFont
			}
			return theme.MajorFont
		}
		return ""
	case "+mn-ea":
		if theme != nil {
			if theme.MinorEAFont != "" {
				return theme.MinorEAFont
			}
			return theme.MinorFont
		}
		return ""
	case "+mj-cs":
		if theme != nil {
			if theme.MajorCSFont != "" {
				return theme.MajorCSFont
			}
			return theme.MajorFont
		}
		return ""
	case "+mn-cs":
		if theme != nil {
			if theme.MinorCSFont != "" {
				return theme.MinorCSFont
			}
			return theme.MinorFont
		}
		return ""
	default:
		if strings.HasPrefix(token, "+mj") || strings.HasPrefix(token, "+mn") {
			return ""
		}
		return typeface
	}
}

func resolveParagraph(ap *parts.DMLParagraph, defaults textDefaults, themeColors map[string]string, theme *resolve.Theme) *resolve.Paragraph {
	p := &resolve.Paragraph{
		BulletColor:      defaults.BulletColor,
		BulletFontFamily: defaults.BulletFontFamily,
		BulletSize:       defaults.BulletSize,
	}
	if defaults.MarginLeftEMU != nil {
		p.MarginLeftEMU = *defaults.MarginLeftEMU
		p.HasMarginLeft = true
	}
	if defaults.TextIndentEMU != nil {
		p.TextIndentEMU = *defaults.TextIndentEMU
		p.HasTextIndent = true
	}
	if defaults.SpaceBefore != nil {
		p.SpaceBefore = *defaults.SpaceBefore
		p.HasSpaceBefore = true
	}
	if defaults.SpaceAfter != nil {
		p.SpaceAfter = *defaults.SpaceAfter
		p.HasSpaceAfter = true
	}
	if defaults.LineSpacingPct != nil {
		p.LineSpacingPct = *defaults.LineSpacingPct
		p.HasLineSpacing = true
	}
	if ap.Properties != nil {
		p.Level = ap.Properties.Lvl
		p.Align = ap.Properties.Algn
		if ap.Properties.MarL != nil {
			p.MarginLeftEMU = *ap.Properties.MarL
			p.HasMarginLeft = true
		}
		if ap.Properties.Indent != nil {
			p.TextIndentEMU = *ap.Properties.Indent
			p.HasTextIndent = true
		}
		if value := resolveSpacingPts(ap.Properties.SpcBef); value != nil {
			p.SpaceBefore = *value
			p.HasSpaceBefore = true
		}
		if value := resolveSpacingPts(ap.Properties.SpcAft); value != nil {
			p.SpaceAfter = *value
			p.HasSpaceAfter = true
		}
		if value := resolveSpacingPct(ap.Properties.LnSpc); value != nil {
			p.LineSpacingPct = *value
			p.HasLineSpacing = true
		}
		if ap.Properties.BuNone == nil {
			if ap.Properties.BuChar != nil && ap.Properties.BuChar.Char != "" {
				p.BulletChar = ap.Properties.BuChar.Char
			} else if ap.Properties.BuAutoNum != nil {
				p.AutoNum = true
				p.AutoNumType = ap.Properties.BuAutoNum.Type
				p.AutoNumStart = ap.Properties.BuAutoNum.StartAt
			}
		}
		if color := resolveBulletColor(ap.Properties.BuClr, ap.Properties.BuClrScheme, themeColors); color != "" {
			p.BulletColor = color
		}
		if family := resolveBulletFontFamily(ap.Properties.BuFont); family != "" {
			p.BulletFontFamily = family
		}
		if size := resolveBulletSize(ap.Properties.BuSzPts); size > 0 {
			p.BulletSize = size
		}
	}
	if p.Align == "" {
		p.Align = defaults.Align
	}
	for _, r := range ap.Runs {
		run := resolveRun(&r, defaults, themeColors, theme)
		if run != nil {
			p.Runs = append(p.Runs, run)
		}
	}
	return p
}

func resolveRun(r *parts.DMLRun, defaults textDefaults, themeColors map[string]string, theme *resolve.Theme) *resolve.Run {
	if r.Text == "" {
		return nil
	}
	run := &resolve.Run{
		Text:        r.Text,
		Bold:        defaults.Bold != nil && *defaults.Bold,
		Italic:      defaults.Italic != nil && *defaults.Italic,
		Underline:   defaults.Underline != "" && defaults.Underline != "none",
		Strike:      defaults.Strike != "" && defaults.Strike != "noStrike",
		AllCaps:     defaults.Caps == "all",
		SmallCaps:   defaults.Caps == "small",
		Superscript: defaults.Baseline != nil && *defaults.Baseline > 0,
		Subscript:   defaults.Baseline != nil && *defaults.Baseline < 0,
		FontSize:    defaults.FontSize,
		FontFamily:  defaults.FontFamily,
		Color:       defaults.Color,
	}
	if r.Properties != nil {
		if r.Properties.Bold != nil {
			run.Bold = *r.Properties.Bold
		}
		if r.Properties.Italic != nil {
			run.Italic = *r.Properties.Italic
		}
		if r.Properties.Underline != "" {
			run.Underline = r.Properties.Underline != "none"
		}
		if r.Properties.Strike != "" {
			run.Strike = r.Properties.Strike != "noStrike"
		}
		if r.Properties.Cap != "" {
			run.AllCaps = r.Properties.Cap == "all"
			run.SmallCaps = r.Properties.Cap == "small"
		}
		if r.Properties.Baseline != nil {
			run.Superscript = *r.Properties.Baseline > 0
			run.Subscript = *r.Properties.Baseline < 0
		}
		if r.Properties.FontSize > 0 {
			run.FontSize = r.Properties.FontSize
		}
		if color := resolveRunColor(r.Properties, themeColors); color != "" {
			run.Color = color
		}
		if family := resolveRunFontFamily(r.Properties, theme); family != "" {
			run.FontFamily = family
		}
	}
	return run
}

// Additional helper functions needed for shape resolution

func resolvePlaceholderKind(ph *parts.PlaceholderInfo) resolve.PlaceholderKind {
	if ph == nil {
		return resolve.PlaceholderNone
	}
	switch ph.Type {
	case "title", "ctrTitle":
		return resolve.PlaceholderTitle
	case "body":
		return resolve.PlaceholderBody
	case "subTitle":
		return resolve.PlaceholderSubtitle
	case "dt":
		return resolve.PlaceholderDateTime
	case "sldNum":
		return resolve.PlaceholderSlideNumber
	case "ftr":
		return resolve.PlaceholderFooter
	case "":
		if ph.Idx == 0 {
			return resolve.PlaceholderTitle
		}
		return resolve.PlaceholderBody
	default:
		return resolve.PlaceholderOther
	}
}

func placeholderIdxIsSentinel(ph *parts.PlaceholderInfo) bool {
	if ph == nil || ph.Idx < 0 {
		return false
	}
	return int64(ph.Idx) == int64(^uint32(0))
}

func resolveTransform(xfrm *parts.Transform2D) resolve.Rect {
	if xfrm == nil {
		return resolve.Rect{}
	}
	return resolve.Rect{
		X: xfrm.Off.X,
		Y: xfrm.Off.Y,
		W: xfrm.Ext.CX,
		H: xfrm.Ext.CY,
	}
}

func shapeRotationDegrees(xfrm *parts.Transform2D) float64 {
	if xfrm == nil || xfrm.Rot == 0 {
		return 0
	}
	return float64(xfrm.Rot) / 60000.0
}

func applyGroupTransformToChildBounds(childBounds resolve.Rect, groupXfrm *parts.Transform2D) resolve.Rect {
	if groupXfrm == nil {
		return childBounds
	}
	return resolve.Rect{
		X: childBounds.X + groupXfrm.Off.X,
		Y: childBounds.Y + groupXfrm.Off.Y,
		W: childBounds.W,
		H: childBounds.H,
	}
}

func findLayoutPlaceholderShape(layout *parts.SlideLayout, ph *parts.PlaceholderInfo) *parts.Shape {
	if layout == nil || ph == nil {
		return nil
	}
	return findPlaceholderShape(layout.CSld.SpTree.Shapes, ph)
}

func findMasterPlaceholderShape(master *parts.SlideMaster, ph *parts.PlaceholderInfo) *parts.Shape {
	if master == nil || ph == nil {
		return nil
	}
	return findPlaceholderShape(master.CSld.SpTree.Shapes, ph)
}

func findPlaceholderShape(shapes []parts.Shape, ph *parts.PlaceholderInfo) *parts.Shape {
	if ph == nil {
		return nil
	}
	targetKind := resolvePlaceholderKind(ph)
	if placeholderIdxIsSentinel(ph) {
		return nil
	}
	for i := range shapes {
		candidate := &shapes[i]
		cph := candidate.NvSpPr.NvPr.Ph
		if cph == nil {
			continue
		}
		if ph.Type != "" && cph.Type == ph.Type && ph.Idx == cph.Idx {
			return candidate
		}
		if resolvePlaceholderKind(cph) == targetKind {
			if ph.Idx == 0 || cph.Idx == ph.Idx {
				return candidate
			}
		}
	}
	return nil
}