package resolve

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	officeartir "office/pkg/officeart/ir"
	officeartdml "office/pkg/officeart/ooxml/drawingml"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/media"
	"office/pkg/presentation/layout/cascade"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/project"
	"office/pkg/render/metafile"
	"strings"
)

func resolveConnector(cx *parts.Connector, themeColors map[string]string, rawTheme *parts.Theme, shapeIndex map[int]*Shape) *Shape {
	shape := &Shape{
		ID:              cx.NvCxnSpPr.CNvPr.ID,
		OriginalShapeID: uint32(cx.NvCxnSpPr.CNvPr.ID),
		Name:            cx.NvCxnSpPr.CNvPr.Name,
		Kind:            ShapeConnector,
		Bounds:          cascade.RectFromTransform(cx.SpPr.Xfrm),
		RotationDeg:     cascade.RotationDegrees(cx.SpPr.Xfrm),
		ConnectorFlipH:  cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipH,
		ConnectorFlipV:  cx.SpPr.Xfrm != nil && cx.SpPr.Xfrm.FlipV,
	}
	shape.ConnectorType, shape.ConnectorAdjustments = cascade.ConnectorGeometry(cx.SpPr.PrstGeom)
	if lineStyle := cascade.EffectiveConnectorLine(cx, rawTheme, themeColors); lineStyle != nil {
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

func indexShape(index map[int]*Shape, shape *Shape) {
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

func resolveConnectionPoint(ref *parts.ConnectionSiteRef, shapeIndex map[int]*Shape) *Point {
	if ref == nil || shapeIndex == nil {
		return nil
	}
	shape := shapeIndex[ref.ID]
	if shape == nil {
		return nil
	}
	pt := cascade.ConnectionPoint(shape.Bounds, ref.Idx)
	return &pt
}

func resolveShape(sp *parts.Shape, layoutShape, masterShape *parts.Shape, master *parts.SlideMaster, slideThemeColors, layoutThemeColors, masterThemeColors map[string]string, theme *Theme, rawTheme *parts.Theme, mediaParts, layoutMediaParts, masterMediaParts map[string]media.Part, hlinkURLs map[string]string, presentationTextStyle *parts.DMLTextListStyle) *Shape {
	ph := resolvePlaceholderKind(sp.NvSpPr.NvPr.Ph)
	xfrm := cascade.EffectiveShapeTransform(sp, layoutShape, masterShape)
	shape := &Shape{
		ID:              sp.NvSpPr.CNvPr.ID,
		OriginalShapeID: uint32(sp.NvSpPr.CNvPr.ID),
		Name:            sp.NvSpPr.CNvPr.Name,
		Kind:            ShapeText,
		Placeholder:     ph,
		Bounds:          cascade.RectFromTransform(xfrm),
		RotationDeg:     cascade.RotationDegrees(xfrm),
		FlipH:           xfrm != nil && xfrm.FlipH,
		FlipV:           xfrm != nil && xfrm.FlipV,
	}
	if phInfo := sp.NvSpPr.NvPr.Ph; phInfo != nil {
		shape.PlaceholderIdx = phInfo.Idx
		shape.PlaceholderOrient = phInfo.Orient
		shape.PlaceholderSize = phInfo.Size
		if phInfo.HasCustomPrompt != nil {
			shape.PlaceholderHasCustomPrompt = *phInfo.HasCustomPrompt
		}
	}
	cascade.ApplyShapeGeometry(&sp.SpPr, shape)
	if fill := cascade.EffectiveShapeFill(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors, mediaParts, layoutMediaParts, masterMediaParts); fill != nil {
		shape.Fill = fill
		shape.FillColor = solidFillColor(fill)
		shape.PictureTile = cascade.EffectiveShapePictureTile(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors)
	} else if rawTheme != nil {
		if styleFill := cascade.ShapeStyleFill(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors); styleFill != nil {
			shape.Fill = styleFill
			shape.FillColor = solidFillColor(styleFill)
		}
	}
	shape.Effects = cascade.EffectiveShapeEffects(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors)
	if len(shape.Effects.Effects) == 0 && rawTheme != nil {
		shape.Effects = cascade.ShapeStyleEffects(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors)
	}
	if lineStyle := cascade.EffectiveShapeLine(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors); lineStyle != nil {
		shape.LineStyle = lineStyle
		shape.LineColor = lineStyle.Color
		shape.LineWidthPt = lineStyle.Width
	} else if rawTheme != nil {
		if styleLine := cascade.ShapeStyleLine(sp, layoutShape, masterShape, rawTheme, slideThemeColors, layoutThemeColors, masterThemeColors); styleLine != nil {
			shape.LineStyle = styleLine
			shape.LineColor = styleLine.Color
			shape.LineWidthPt = styleLine.Width
		}
	}
	if sp.TxBody != nil {
		bodyStyle := cascade.EffectiveTextBodyStyle(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape))
		shape.TextAnchor = bodyStyle.Anchor
		shape.HasTextInsets = true
		shape.TextInsetLeftEMU = bodyStyle.LIns
		shape.TextInsetRightEMU = bodyStyle.RIns
		shape.TextInsetTopEMU = bodyStyle.TIns
		shape.TextInsetBottomEMU = bodyStyle.BIns
		shape.TextAllowOverflow = bodyStyle.Autofit == "noAutofit"
		shape.TextAutoGrow = bodyStyle.Autofit == "spAutoFit"
		shape.SentinelPlaceholder = placeholderIdxIsSentinel(sp.NvSpPr.NvPr.Ph)
		shape.TextBodyProps = bodyStyle.DrawingProps()

		for _, ap := range sp.TxBody.Paragraphs {
			level := 0
			if ap.Properties != nil {
				level = ap.Properties.Lvl
			}
			defaults := cascade.EffectiveTextDefaults(sp.TxBody, layoutTextBody(layoutShape), layoutTextBody(masterShape), master, presentationTextStyle, ph, level, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
			// a:fontRef is a shape-style default. It is more specific than text
			// inherited from layout/master, but less specific than the shape's own
			// list-style/run properties. Preserve a direct shape text color while
			// allowing fontRef/phClr to replace inherited master defaults.
			direct := cascade.TextDefaultsFromListStyle(sp.TxBody.ListStyle, level, slideThemeColors, theme)
			if direct.Color == "" {
				if color := cascade.ShapeStyleFontColor(sp, layoutShape, masterShape, slideThemeColors, layoutThemeColors, masterThemeColors); color != "" {
					defaults.Color = color
				}
			}
			if direct.FontFamily == "" && theme != nil {
				// a:fontRef belongs to the shape-style layer: it overrides
				// layout/master text defaults, but never an explicit font from
				// this shape's own list style.
				if family := cascade.ShapeStyleFontFamily(sp, layoutShape, masterShape, theme); family != "" {
					defaults.FontFamily = family
				}
			}
			shape.Paragraphs = append(shape.Paragraphs, resolveParagraph(&ap, defaults, slideThemeColors, theme, hlinkURLs))
		}
	}
	ensureAutoSlideNumberField(shape, sp, layoutShape, masterShape, master, slideThemeColors, layoutThemeColors, masterThemeColors, theme)
	if sp.NvSpPr.CNvPr.HlinkClick != nil {
		shape.HyperlinkURL, shape.ActionLink = resolveHlinkClick(sp.NvSpPr.CNvPr.HlinkClick, hlinkURLs)
	}
	return shape
}

func syncPresentationOfficeArtShape(shape *Shape) *officeartir.Shape {
	if shape == nil {
		return nil
	}
	spec := officeartdml.ShapeSpec{
		ID:          uint32(shape.ID),
		Name:        shape.Name,
		Kind:        presentationOfficeArtKind(shape.Kind),
		Bounds:      officeartir.Rect{X: shape.Bounds.X, Y: shape.Bounds.Y, W: shape.Bounds.W, H: shape.Bounds.H},
		RotationDeg: shape.RotationDeg,
		FlipH:       shape.FlipH,
		FlipV:       shape.FlipV,
		Geometry:    shape.GeometryData,
		Fill:        shape.Fill,
		Line:        shape.LineStyle,
		Effects:     shape.Effects,
		Source:      officeartir.SourceRef{Format: "pptx", ShapeID: shape.OriginalShapeID},
	}
	if shape.Geometry != "" {
		spec.Preset = &drawing.PresetGeometry{
			Name:        shape.Geometry,
			Adjustments: append([]float64(nil), shape.GeometryAdjustments...),
		}
	}
	if shape.Kind == ShapeImage && len(shape.ImageData) > 0 {
		spec.Image = &drawing.PictureFill{
			EmbedID:     shape.ImageRelID,
			ImageData:   append([]byte(nil), shape.ImageData...),
			ContentType: shape.ImageMIME,
			DPI:         shape.ImageDPI,
			Crop: &drawing.Rect{
				X:      shape.ImageCropLeft,
				Y:      shape.ImageCropTop,
				Width:  1 - shape.ImageCropLeft - shape.ImageCropRight,
				Height: 1 - shape.ImageCropTop - shape.ImageCropBottom,
			},
		}
	}
	if shape.Kind == ShapeConnector {
		spec.Connector = &officeartdml.ConnectorSpec{Type: shape.Geometry}
	}
	shape.OfficeArt = officeartdml.Shape(spec)

	if len(shape.Children) > 0 {
		shape.OfficeArt.Children = shape.OfficeArt.Children[:0]
		for _, child := range shape.Children {
			if composed := syncPresentationOfficeArtShape(child); composed != nil {
				shape.OfficeArt.Children = append(shape.OfficeArt.Children, composed)
			}
		}
		if len(shape.OfficeArt.Children) > 0 {
			shape.OfficeArt.Kind = officeartir.ShapeGroup
		}
	}
	return shape.OfficeArt
}

func presentationOfficeArtKind(kind ShapeKind) officeartir.ShapeKind {
	switch kind {
	case ShapeImage:
		return officeartir.ShapePicture
	case ShapeGroup:
		return officeartir.ShapeGroup
	case ShapeConnector, ShapeSmartArtConnector:
		return officeartir.ShapeConnector
	case ShapeText:
		return officeartir.ShapeTextBox
	default:
		return officeartir.ShapeAuto
	}
}

func layoutTextBody(shape *parts.Shape) *parts.TextBody {
	if shape == nil {
		return nil
	}
	return shape.TxBody
}

func resolvePicture(pic *parts.Picture, resources project.SlideResources, themeColors map[string]string) *Shape {
	shape := &Shape{
		ID:              pic.NvPicPr.CNvPr.ID,
		OriginalShapeID: uint32(pic.NvPicPr.CNvPr.ID),
		Name:            pic.NvPicPr.CNvPr.Name,
		Bounds:          cascade.RectFromTransform(pic.SpPr.Xfrm),
		RotationDeg:     cascade.RotationDegrees(pic.SpPr.Xfrm),
		FlipH:           pic.SpPr.Xfrm != nil && pic.SpPr.Xfrm.FlipH,
		FlipV:           pic.SpPr.Xfrm != nil && pic.SpPr.Xfrm.FlipV,
	}
	shape.ImageDPI = pic.BlipFill.Dpi
	if pic.BlipFill.RotWithShape != nil {
		shape.ImageRotWithShape = *pic.BlipFill.RotWithShape
		shape.ImageRotWithShapeSet = true
	}
	if pic.BlipFill.SrcRect != nil {
		shape.ImageCropLeft = relativeRectPct(pic.BlipFill.SrcRect.L)
		shape.ImageCropTop = relativeRectPct(pic.BlipFill.SrcRect.T)
		shape.ImageCropRight = relativeRectPct(pic.BlipFill.SrcRect.R)
		shape.ImageCropBottom = relativeRectPct(pic.BlipFill.SrcRect.B)
	}
	if pic.BlipFill.Stretch != nil && pic.BlipFill.Stretch.FillRect != nil {
		shape.ImageFillLeft = relativeRectPct(pic.BlipFill.Stretch.FillRect.L)
		shape.ImageFillTop = relativeRectPct(pic.BlipFill.Stretch.FillRect.T)
		shape.ImageFillRight = relativeRectPct(pic.BlipFill.Stretch.FillRect.R)
		shape.ImageFillBottom = relativeRectPct(pic.BlipFill.Stretch.FillRect.B)
	}
	if effects := pic.SpPr.DrawingEffectList(themeColors); len(effects.Effects) > 0 {
		shape.Effects = effects
	}
	if mediaRef := pic.NvPicPr.NvPr.PrimaryMedia(); mediaRef != nil {
		shape.Kind = ShapeMedia
		shape.MediaRelID = mediaRef.RelID()
		shape.MediaKind = pic.NvPicPr.NvPr.MediaKind()
		if m, ok := resources.Lookup(shape.MediaRelID); ok {
			shape.MediaData = m.Data
			shape.MediaMIME = m.MIME
			if shape.MediaKind == "" {
				shape.MediaKind = inferMediaKind(m.MIME)
			}
		}
		if pic.BlipFill.Blip != nil {
			relID := pic.BlipFill.Blip.Embed
			shape.ImageRelID = relID
			if m, ok := resources.Lookup(relID); ok {
				shape.MediaPosterData = m.Data
				shape.MediaPosterMIME = m.MIME
			}
		}
	} else {
		shape.Kind = ShapeImage
	}
	if pic.BlipFill.Blip != nil {
		relID := pic.BlipFill.Blip.Embed
		if shape.Kind == ShapeImage {
			shape.ImageRelID = relID
			shape.ImageEffects = pic.BlipFill.Blip.DrawingEffects(themeColors)
			if m, ok := resources.Lookup(relID); ok {
				shape.ImageData = m.Data
				shape.ImageMIME = m.MIME
				convertPresentationMetafileImage(shape)
				applyIntrinsicPictureSize(shape)
			}
		}
	}
	if pic.NvPicPr.CNvPr.HlinkClick != nil {
		shape.HyperlinkURL, shape.ActionLink = resolveHlinkClick(pic.NvPicPr.CNvPr.HlinkClick, resources.Hyperlinks)
	}
	return shape
}

func convertPresentationMetafileImage(shape *Shape) {
	if shape == nil || len(shape.ImageData) == 0 {
		return
	}
	svg, _, _, ok := metafile.ToSVG(shape.ImageData, shape.ImageMIME)
	if !ok {
		return
	}
	shape.ImageData = svg
	shape.ImageMIME = "image/svg+xml"
}

func applyIntrinsicPictureSize(shape *Shape) {
	if shape == nil || len(shape.ImageData) == 0 || (shape.Bounds.W > 0 && shape.Bounds.H > 0) {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(shape.ImageData))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return
	}
	dpi := shape.ImageDPI
	if dpi <= 0 {
		dpi = 96
	}
	w := int64(math.Round(float64(cfg.Width) * 914400.0 / float64(dpi)))
	h := int64(math.Round(float64(cfg.Height) * 914400.0 / float64(dpi)))
	if shape.Bounds.W <= 0 && shape.Bounds.H <= 0 {
		shape.Bounds.W, shape.Bounds.H = w, h
		return
	}
	if shape.Bounds.W <= 0 && shape.Bounds.H > 0 {
		shape.Bounds.W = int64(math.Round(float64(shape.Bounds.H) * float64(cfg.Width) / float64(cfg.Height)))
	}
	if shape.Bounds.H <= 0 && shape.Bounds.W > 0 {
		shape.Bounds.H = int64(math.Round(float64(shape.Bounds.W) * float64(cfg.Height) / float64(cfg.Width)))
	}
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

// relativeRectPct preserves DrawingML CT_RelativeRect percentages exactly.
// Negative values are meaningful: they expand the selected source/destination
// rectangle beyond the nominal picture bounds and must not be clamped away.
func relativeRectPct(value int) float64 {
	return float64(value) / 100000.0
}

// resolveHlinkClick resolves a hlinkClick reference to either an external URL
// or an action link string. It returns (hyperlinkURL, actionLink) — at most one
// of the two will be non-empty.
//
// ppaction:// URIs are parsed for well-known navigation targets:
//   - ppaction://hlinksldjump?slidenum=N  → actionLink "slideN" (1-based)
//   - ppaction://hlinkPresentationJump?jump=firstSlide → actionLink "firstSlide"
//   - ppaction://hlinkPresentationJump?jump=lastSlide  → actionLink "lastSlide"
//   - ppaction://hlinkPresentationJump?jump=nextSlide  → actionLink "nextSlide"
//   - ppaction://hlinkPresentationJump?jump=prevSlide  → actionLink "prevSlide"
//   - ppaction://hlinkshowjump?jump=endShow           → actionLink "endShow"
//
// External URLs are looked up in hlinkURLs by RelID.
func resolveHlinkClick(h *parts.HlinkClick, hlinkURLs map[string]string) (hyperlinkURL string, actionLink string) {
	if h == nil {
		return "", ""
	}
	if strings.HasPrefix(h.Action, "ppaction://") {
		action := h.Action[len("ppaction://"):]
		// Strip query and parse
		base, query, _ := strings.Cut(action, "?")
		switch base {
		case "hlinksldjump":
			params := parseSimpleQuery(query)
			if n := params["slidenum"]; n != "" {
				return "", "slide" + n
			}
		case "hlinkPresentationJump":
			params := parseSimpleQuery(query)
			if j := params["jump"]; j != "" {
				return "", j // "firstSlide","lastSlide","nextSlide","prevSlide"
			}
		case "hlinkshowjump":
			params := parseSimpleQuery(query)
			if j := params["jump"]; j != "" {
				return "", j // "endShow"
			}
		}
		return "", ""
	}
	if h.RelID != "" && hlinkURLs != nil {
		if url, ok := hlinkURLs[h.RelID]; ok {
			return url, ""
		}
	}
	return "", ""
}

// parseSimpleQuery parses a simple key=value&key=value query string.
func parseSimpleQuery(query string) map[string]string {
	m := map[string]string{}
	for _, pair := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}

func resolveGroupShape(grp *parts.GroupShape, master *parts.SlideMaster, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, mediaParts map[string]media.Part, resources project.SlideResources, tableStyles *parts.TableStyleList, presentationTextStyle *parts.DMLTextListStyle) *Shape {
	shape := &Shape{Kind: ShapeGroup}
	if grp.NvGrpSpPr != nil {
		shape.ID = grp.NvGrpSpPr.CNvPr.ID
		shape.OriginalShapeID = uint32(grp.NvGrpSpPr.CNvPr.ID)
		shape.Name = grp.NvGrpSpPr.CNvPr.Name
	}

	// Get group transform if present
	var groupXfrm *parts.Transform2D
	if grp.GrpSpPr != nil {
		groupXfrm = grp.GrpSpPr.Xfrm
	}
	if groupXfrm != nil {
		transform := cascade.CanonicalTransform(groupXfrm)
		shape.GroupTransform = &transform
		shape.Bounds = cascade.RectFromTransform(groupXfrm)
	}

	for _, sp := range grp.Shapes {
		child := resolveShape(&sp, nil, nil, master, themeColors, themeColors, themeColors, theme, rawTheme, mediaParts, nil, nil, resources.Hyperlinks, presentationTextStyle)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	for _, pic := range grp.Pictures {
		child := resolvePicture(&pic, resources, themeColors)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	for _, g := range grp.GroupShapes {
		child := resolveGroupShape(&g, master, themeColors, theme, rawTheme, mediaParts, resources, tableStyles, presentationTextStyle)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	for _, tbl := range grp.Tables {
		child := resolveTableFrame(&tbl, themeColors, theme, rawTheme, tableStyles, presentationTextStyle)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	for i := range grp.Connectors {
		child := resolveConnector(&grp.Connectors[i], themeColors, rawTheme, nil)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	for i := range grp.OleObjectFrames {
		child := resolveOleObjectFrame(&grp.OleObjectFrames[i], resources)
		if child != nil {
			shape.Children = append(shape.Children, child)
		}
	}
	return shape
}

func resolveTableFrame(frame *parts.TableFrame, themeColors map[string]string, theme *Theme, rawTheme *parts.Theme, tableStyles *parts.TableStyleList, presentationTextStyle *parts.DMLTextListStyle) *Shape {
	if frame == nil || frame.Table == nil {
		return nil
	}
	style := cascade.EffectiveTableStyle(frame.Table, tableStyles)
	totalRows := len(frame.Table.Rows)
	totalCols := cascade.TableColumnCount(frame.Table)
	shape := &Shape{
		ID:              frame.ID,
		OriginalShapeID: uint32(frame.ID),
		Name:            frame.Name,
		Kind:            ShapeTable,
		Bounds:          cascade.RectFromTransform(frame.Xfrm),
		Table:           &Table{},
	}
	for _, col := range frame.Table.Grid.Columns {
		shape.Table.ColumnWidthsEMU = append(shape.Table.ColumnWidthsEMU, col.W)
	}
	for _, row := range frame.Table.Rows {
		resolvedRow := &TableRow{HeightEMU: row.H}
		colIndex := 0
		for _, cell := range row.Cells {
			startCol := colIndex
			span := cell.GridSpan
			if span <= 0 {
				span = 1
			}
			if cell.HMerge || cell.VMerge {
				span = 1
			}
			colIndex += span

			styleDefaults := cascade.ResolveTableCellStyle(style, frame.Table.Props, len(shape.Table.Rows), startCol, span, totalRows, totalCols, themeColors, theme, rawTheme)
			resolvedCell := &TableCell{
				GridSpan:     cell.GridSpan,
				RowSpan:      cell.RowSpan,
				HMerge:       cell.HMerge,
				VMerge:       cell.VMerge,
				Fill:         styleDefaults.Fill,
				FillColor:    styleDefaults.FillColor,
				Effects:      styleDefaults.Effects,
				BorderLeft:   styleDefaults.BorderLeft,
				BorderRight:  styleDefaults.BorderRight,
				BorderTop:    styleDefaults.BorderTop,
				BorderBottom: styleDefaults.BorderBottom,
			}
			if resolvedCell.GridSpan <= 0 {
				resolvedCell.GridSpan = 1
			}
			if resolvedCell.RowSpan <= 0 {
				resolvedCell.RowSpan = 1
			}
			// DrawingML table cells have physical default insets even when tcPr
			// omits marL/marR/marT/marB. Preserve explicit zero separately from
			// omission so layout receives the same content box PowerPoint uses.
			resolvedCell.MarginLeftEMU = 91440
			resolvedCell.MarginRightEMU = 91440
			resolvedCell.MarginTopEMU = 45720
			resolvedCell.MarginBottomEMU = 45720
			if cell.Props != nil {
				resolvedCell.TextAnchor = cell.Props.Anchor
				if cell.Props.MarLSet || cell.Props.MarL != 0 {
					resolvedCell.MarginLeftEMU = cell.Props.MarL
				}
				if cell.Props.MarRSet || cell.Props.MarR != 0 {
					resolvedCell.MarginRightEMU = cell.Props.MarR
				}
				if cell.Props.MarTSet || cell.Props.MarT != 0 {
					resolvedCell.MarginTopEMU = cell.Props.MarT
				}
				if cell.Props.MarBSet || cell.Props.MarB != 0 {
					resolvedCell.MarginBottomEMU = cell.Props.MarB
				}
				if fill := cascade.TableCellFill(cell.Props.SolidFill, cell.Props.SchemeFill, cell.Props.GradFill, cell.Props.PattFill, cell.Props.BlipFill, cell.Props.NoFill, themeColors); fill != nil {
					resolvedCell.Fill = fill
					resolvedCell.FillColor = solidFillColor(fill)
				}
				if cell.Props.LnL != nil {
					resolvedCell.BorderLeft = cascade.TableLineStyle(cell.Props.LnL, themeColors)
				}
				if cell.Props.LnR != nil {
					resolvedCell.BorderRight = cascade.TableLineStyle(cell.Props.LnR, themeColors)
				}
				if cell.Props.LnT != nil {
					resolvedCell.BorderTop = cascade.TableLineStyle(cell.Props.LnT, themeColors)
				}
				if cell.Props.LnB != nil {
					resolvedCell.BorderBottom = cascade.TableLineStyle(cell.Props.LnB, themeColors)
				}
			}
			if cell.TextBody != nil {
				for _, ap := range cell.TextBody.Paragraphs {
					level := 0
					if ap.Properties != nil {
						level = ap.Properties.Lvl
					}
					directDefaults := cascade.EffectiveTextDefaults(cell.TextBody, nil, nil, nil, nil, PlaceholderNone, level, themeColors, nil, nil, theme)
					defaults := cascade.MergeTextDefaults(directDefaults, textDefaultsFromTableStyle(styleDefaults.Text))
					// Presentation-level p:defaultTextStyle supplies defaults (notably
					// font size) for table text that has no placeholder/master chain.
					defaults = cascade.MergeTextDefaults(defaults, cascade.TextDefaultsFromListStyle(presentationTextStyle, level, themeColors, theme))
					paragraph := resolveParagraph(&ap, defaults, themeColors, theme, nil)
					// Table text has no placeholder chain. When neither direct/table-style nor
					// presentation defaults specify a size, PowerPoint's table default is 18pt.
					// Resolve it before layout so physical line metrics and HTML agree.
					if paragraph != nil {
						for _, run := range paragraph.Runs {
							if run != nil && run.FontSize == 0 {
								run.FontSize = 1800
							}
						}
					}
					resolvedCell.Paragraphs = append(resolvedCell.Paragraphs, paragraph)
				}
			}
			resolvedRow.Cells = append(resolvedRow.Cells, resolvedCell)
		}
		shape.Table.Rows = append(shape.Table.Rows, resolvedRow)
	}
	return shape
}

func textDefaultsFromTableStyle(src cascade.TableTextDefaults) textDefaults {
	return cascade.TextDefaults{Bold: src.Bold, Italic: src.Italic, FontFamily: src.FontFamily, Color: src.Color}
}
