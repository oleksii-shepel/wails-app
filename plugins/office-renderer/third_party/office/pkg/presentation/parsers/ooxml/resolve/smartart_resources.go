package resolve

import (
	"bytes"
	"encoding/xml"
	"math"
	"office/pkg/presentation/layout/cascade"
	"sort"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
	smartartrender "office/pkg/ooxml/smartart/render"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/project"
)

func resolveChart(relID string, chartXML []byte, xfrm *parts.Transform2D, themeColors map[string]string) *Shape {
	if len(chartXML) == 0 {
		return nil
	}

	c, err := chart.ParseChart(chartXML)
	if err != nil || c == nil {
		return nil
	}
	c.ThemeColors = themeColors

	bounds := Rect{X: 0, Y: 0, W: 5760000, H: 3240000} // Default 4x3 inches in EMU
	widthPt, heightPt := 400, 300
	if xfrm != nil {
		bounds.X = xfrm.Off.X
		bounds.Y = xfrm.Off.Y
		bounds.W = xfrm.Ext.CX
		bounds.H = xfrm.Ext.CY
		widthPt = int(bounds.W / 12700)
		heightPt = int(bounds.H / 12700)
		if widthPt <= 0 {
			widthPt = 400
		}
		if heightPt <= 0 {
			heightPt = 300
		}
	}

	svgStr := c.SVG(widthPt, heightPt)
	if svgStr == "" {
		return nil
	}

	return &Shape{
		ID:        0, // Charts don't have shape IDs
		Name:      "Chart",
		Kind:      ShapeChart,
		Bounds:    bounds,
		ImageData: []byte(svgStr),
		ImageMIME: "image/svg+xml",
	}
}

// resolveOleObjectFrame converts a parsed OLE object frame into a placeholder shape.
func resolveOleObjectFrame(frame *parts.OleObjectFrame, resources project.SlideResources) *Shape {
	if frame == nil {
		return nil
	}
	shape := &Shape{
		ID:                frame.ID,
		OriginalShapeID:   uint32(frame.ID),
		Name:              frame.Name,
		Kind:              ShapeOleObject,
		Geometry:          "rect",
		Bounds:            cascade.RectFromTransform(frame.Xfrm),
		RotationDeg:       cascade.RotationDegrees(frame.Xfrm),
		FlipH:             frame.Xfrm != nil && frame.Xfrm.FlipH,
		FlipV:             frame.Xfrm != nil && frame.Xfrm.FlipV,
		OLEObjectName:     frame.ObjectName,
		OLEObjectProgID:   frame.ProgID,
	}
	if part, ok := resources.OLE[normalizeRelID(frame.RelID)]; ok {
		shape.OLEObjectData = part.Data
		shape.OLEObjectMIME = part.MIME
	}
	if !frame.Embed {
		shape.OLEObjectLinkURL = resources.OLELinks[normalizeRelID(frame.RelID)]
	}
	if frame.IconRelID != "" {
		if part, ok := resources.Media[normalizeRelID(frame.IconRelID)]; ok {
			shape.OLEIconData = part.Data
			shape.OLEIconMIME = part.MIME
		}
	}
	return shape
}

// buildSmartArtFrameMap indexes the hosting p:graphicFrame for each SmartArt
// data-model relationship on the slide, including frames nested in groups.
func buildSmartArtFrameMap(entry *parts.SlideEntry) map[string]*parts.SmartArtFrame {
	m := make(map[string]*parts.SmartArtFrame)
	if entry == nil || entry.Slide == nil {
		return m
	}
	for i := range entry.Slide.CSld.SpTree.SmartArtFrames {
		frame := &entry.Slide.CSld.SpTree.SmartArtFrames[i]
		if dm := frame.RelIDs["dm"]; dm != "" {
			m[dm] = frame
		}
	}
	var walk func(*parts.GroupShape)
	walk = func(g *parts.GroupShape) {
		if g == nil {
			return
		}
		for i := range g.SmartArtFrames {
			frame := &g.SmartArtFrames[i]
			if dm := frame.RelIDs["dm"]; dm != "" {
				m[dm] = frame
			}
		}
		for i := range g.GroupShapes {
			walk(&g.GroupShapes[i])
		}
	}
	for i := range entry.Slide.CSld.SpTree.GroupShapes {
		walk(&entry.Slide.CSld.SpTree.GroupShapes[i])
	}
	return m
}

// ResolveSmartArtPart converts one package SmartArt relationship into canonical
// renderable shapes without resolving the rest of the presentation. It is the
// package-semantic boundary used by the SAX production loader while SmartArt
// processing is migrated out of the monolithic ResolvePackage path.
func ResolveSmartArtPart(part *parts.SmartArtPart, frame *parts.SmartArtFrame, themeColors map[string]string, rawTheme *parts.Theme, theme *Theme) ([]*Shape, diag.List) {
	return resolveSmartArt(part, frame, themeColors, rawTheme, theme)
}

// resolveSmartArt converts a SmartArt part to renderable shapes.
func resolveSmartArt(part *parts.SmartArtPart, frame *parts.SmartArtFrame, themeColors map[string]string, rawTheme *parts.Theme, theme *Theme) ([]*Shape, diag.List) {
	var diags diag.List
	if part == nil || len(part.DataModelXML) == 0 {
		if part != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "smartart.empty-data-model",
				Message:  "SmartArt part has no data model; skipping diagram",
				Location: diag.Location{Part: part.RelID},
			})
		}
		return nil, diags
	}

	if len(part.LayoutXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-layout",
			Message:  "SmartArt layout definition not present; using fallback layout",
			Location: diag.Location{Part: part.RelID},
		})
	}
	if len(part.StyleXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-style",
			Message:  "SmartArt quick-style part not present; using default style",
			Location: diag.Location{Part: part.RelID},
		})
	}
	if len(part.ColorsXML) == 0 {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.missing-colors",
			Message:  "SmartArt color part not present; using default palette",
			Location: diag.Location{Part: part.RelID},
		})
	}

	var frameBounds *smartartrender.FrameBounds
	if frame != nil && frame.Xfrm != nil {
		frameBounds = &smartartrender.FrameBounds{
			X: float64(frame.Xfrm.Off.X),
			Y: float64(frame.Xfrm.Off.Y),
			W: float64(frame.Xfrm.Ext.CX),
			H: float64(frame.Xfrm.Ext.CY),
		}
	}

	fmtScheme := cascade.SmartArtFormatScheme(rawTheme, themeColors, theme)

	// Use the smartart render package to process the diagram. Pass the layout,
	// style, color, and optional pre-rendered drawing parts so exact geometry
	// can be used when available.  Theme colors are supplied so schemeClr values
	// in the color style XML can be resolved. A resolved format scheme lets the
	// quick-style matrix references produce correct fills and lines.
	smartArtShapes, err := smartartrender.ProcessDiagramWithOptionsAndFrameFormat(
		part.DataModelXML,
		part.LayoutXML,
		part.StyleXML,
		part.ColorsXML,
		part.DrawingXML,
		frameBounds,
		themeColors,
		fmtScheme,
	)
	if err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityWarning,
			Code:     "smartart.process-failed",
			Message:  "SmartArt diagram processing failed; skipping diagram",
			Location: diag.Location{Part: part.RelID},
			Cause:    err,
		})
		return nil, diags
	}

	// Parse the diagram again to access layout metadata for connector routing.
	diagram, parseErr := smartart.ParseDiagramWithOptionsAndThemeFormat(
		part.DataModelXML,
		part.LayoutXML,
		part.StyleXML,
		part.ColorsXML,
		themeColors,
		fmtScheme,
	)
	if parseErr != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "smartart.layout-parse-failed",
			Message:  "Could not parse SmartArt layout metadata; using fallback connector routing",
			Location: diag.Location{Part: part.RelID},
			Cause:    parseErr,
		})
	}
	if diagram == nil {
		diagram = &smartart.Diagram{Layout: &smartart.Layout{Type: "hierarchy"}}
	}
	if len(diagram.Diagnostics) > 0 {
		for _, d := range diagram.Diagnostics {
			diags.Add(d)
		}
	}

	// A pre-rendered drawing part can override the style-defined connector line.
	// ProcessDiagram applies the override to its internal diagram, but we re-parse
	// the diagram above, so re-apply it here.
	if len(part.DrawingXML) > 0 {
		if connStyle := smartartrender.ExtractConnectorStyle(part.DrawingXML, themeColors); connStyle != nil {
			if diagram.Style == nil {
				diagram.Style = &smartart.Style{}
			}
			diagram.Style.ConnectorLine = connStyle
		}
	}

	// Resolve embedded image relationships for picture fills before converting.
	mediaParts := part.MediaParts
	for _, sa := range smartArtShapes {
		if pf, ok := sa.Fill.(*drawing.PictureFill); ok && pf != nil && pf.EmbedID != "" {
			if m, ok := mediaParts[pf.EmbedID]; ok {
				pf.ImageData = m.Data
				pf.ContentType = m.ContentType
			}
		}
	}

	// Convert SmartArt shapes to resolve.Shape
	var shapes []*Shape
	nodeMap := make(map[string]*Shape)
	for _, sa := range smartArtShapes {
		shape := &Shape{
			ID:                  0, // SmartArt shapes don't have IDs
			Name:                "SmartArt",
			Kind:                ShapeSmartArt,
			Bounds:              Rect{X: int64(sa.X * 12700), Y: int64(sa.Y * 12700), W: int64(sa.Width * 12700), H: int64(sa.Height * 12700)},
			Fill:                sa.Fill,
			LineStyle:           sa.Line,
			Effects:             sa.Effect,
			Geometry:            sa.Type,
			GeometryAdjustments: sa.GeometryAdjustments,
			GeometryData:        sa.CustomGeometry,
			DrawingOrder:        sa.DrawingOrder,
			RotationDeg:         sa.RotationDeg,
			FlipH:               sa.FlipH,
			FlipV:               sa.FlipV,
			SmartArtNodeID:      sa.ID,
			TextBodyProps:       sa.TextBodyProps,
		}
		for _, c := range sa.Connections {
			shape.SmartArtConnections = append(shape.SmartArtConnections, c.ToID)
		}
		// Derive legacy string fields from shared types for renderers
		// that have not yet migrated to the drawing types.
		if sf, ok := sa.Fill.(drawing.SolidFill); ok {
			shape.FillColor = sf.Color
		}
		if sa.Line != nil {
			shape.LineColor = sa.Line.Color
			shape.LineWidthPt = sa.Line.Width
		}
		// Add text as styled paragraphs when the drawing part supplied them,
		// otherwise fall back to a single plain-text paragraph.
		if len(sa.Paragraphs) > 0 {
			shape.Paragraphs = make([]*Paragraph, 0, len(sa.Paragraphs))
			for _, p := range sa.Paragraphs {
				para := &Paragraph{
					Align:             p.Align,
					Level:             p.Level,
					BulletChar:        p.BulletChar,
					AutoNum:           p.AutoNum,
					AutoNumType:       p.AutoNumType,
					AutoNumStart:      p.AutoNumStart,
					BulletColor:       p.BulletColor,
					BulletFontFamily:  p.BulletFontFamily,
					BulletFontTx:      p.BulletFontTx,
					BulletSize:        p.BulletSize,
					BulletSizePct:     p.BulletSizePct,
					SpaceBefore:       p.SpaceBefore,
					SpaceAfter:        p.SpaceAfter,
					SpaceBeforePct:    p.SpaceBeforePct,
					SpaceAfterPct:     p.SpaceAfterPct,
					LineSpacingPct:    p.LineSpacingPct,
					LineSpacingPt:     p.LineSpacingPts,
					HasLineSpacing:    p.LineSpacingPct != 0 || p.LineSpacingPts != 0,
					DefaultTabSizeEMU: p.DefaultTabSizeEMU,
				}
				if len(p.TabStops) > 0 {
					para.TabStops = make([]TabStop, 0, len(p.TabStops))
					for _, stop := range p.TabStops {
						para.TabStops = append(para.TabStops, TabStop{Pos: stop.Pos, Algn: stop.Algn})
					}
				}
				for _, r := range p.Runs {
					color := r.Color
					if color == "" {
						color = sa.TextColor
					}
					font := r.FontFamily
					if font == "" {
						font = r.FontFamilyEA
					}
					if font == "" {
						font = r.FontFamilyCS
					}
					para.Runs = append(para.Runs, &Run{
						Text:       r.Text,
						Bold:       r.Bold,
						Italic:     r.Italic,
						Underline:  r.Underline,
						FontSize:   r.FontSize,
						FontFamily: font,
						Color:      color,
						Effects:    r.Effects,
					})
				}
				shape.Paragraphs = append(shape.Paragraphs, para)
			}
		} else {
			text := sa.Text
			if text == "" {
				text = sa.PlaceholderText
			}
			if text != "" {
				shape.Paragraphs = []*Paragraph{
					{
						Runs: []*Run{
							{Text: text, Color: sa.TextColor},
						},
					},
				}
			}
		}
		shapes = append(shapes, shape)
		nodeMap[sa.ID] = shape
	}

	// The pre-rendered diagram drawing is a complete paint scene, not merely a
	// geometry/style cache for logical data-model nodes. Drawing parts commonly
	// contain auxiliary artwork (badges, circles, brackets, separators, etc.)
	// whose modelIds are presentation-only and therefore have no data-model node.
	// Preserve those shapes as independent SmartArt leaves. Dropping them changes
	// both the visible composition and the apparent text-frame geometry of the
	// remaining nodes (for example a circle + left-pointing banner list becomes a
	// bare oversized wedge).
	if len(part.DrawingXML) > 0 {
		presentationIDs := make(map[string]bool)
		for _, presentationID := range smartart.DataToPresentationMap(part.DataModelXML) {
			presentationIDs[presentationID] = true
		}
		drawingShapes := smartartrender.ExtractShapeInfos(part.DrawingXML, themeColors)
		picturePlaceholderIDs := smartArtPicturePlaceholderPresentationIDs(part.DataModelXML)
		drawingScale, drawingOffsetX, drawingOffsetY := smartArtConnectorTransform(part.DrawingXML, frameBounds, themeColors)
		for modelID, info := range drawingShapes {
			if info == nil || presentationIDs[modelID] {
				continue
			}
			shape := buildAuxiliarySmartArtShapeFromDrawing(info, drawingScale, drawingOffsetX, drawingOffsetY)
			shape.SmartArtPicturePlaceholder = picturePlaceholderIDs[modelID]
			shapes = append(shapes, shape)
		}
	}

	// Generate connector shapes between connected SmartArt nodes.
	connLine := &drawing.LineStyle{Color: "999999", Width: 1.0}
	if diagram != nil && diagram.Style != nil && diagram.Style.ConnectorLine != nil {
		connLine = diagram.Style.ConnectorLine
	} else if len(shapes) > 0 && shapes[0].LineStyle != nil {
		connLine = shapes[0].LineStyle
	}
	layoutType := diagram.Layout.Type
	layoutDir := diagram.Layout.Direction
	connectorLayout := diagram.Layout

	// When a pre-rendered drawing part is present, try to match its connector
	// geometries to the diagram's logical edges in declaration order.
	preRenderedConns := smartartrender.ExtractConnectors(part.DrawingXML, themeColors)
	connScale, connOffsetX, connOffsetY := smartArtConnectorTransform(part.DrawingXML, frameBounds, themeColors)
	sort.Slice(preRenderedConns, func(i, j int) bool {
		return preRenderedConns[i].DrawingOrder < preRenderedConns[j].DrawingOrder
	})

	// Build the ordered list of logical edges.
	type smartArtEdge struct {
		srcID     string
		dstID     string
		assistant bool
	}
	assistantByID := make(map[string]bool)
	if diagram != nil {
		for _, n := range diagram.Nodes {
			if n != nil && n.Assistant {
				assistantByID[n.ID] = true
			}
		}
	}
	var edges []smartArtEdge
	if diagram != nil && len(diagram.Connections) > 0 {
		for _, c := range diagram.Connections {
			edges = append(edges, smartArtEdge{srcID: c.SrcID, dstID: c.DestID, assistant: assistantByID[c.DestID]})
		}
	} else {
		for _, src := range shapes {
			for _, dstID := range src.SmartArtConnections {
				edges = append(edges, smartArtEdge{srcID: src.SmartArtNodeID, dstID: dstID, assistant: assistantByID[dstID]})
			}
		}
	}

	// A pre-rendered dsp:drawing part is authoritative for the visual scene.
	// If it contains no connector shapes, do not synthesize logical data-model
	// edges: those edges describe diagram relationships, not necessarily painted
	// connector artwork. Synthesizing them is especially destructive for layouts
	// whose drawing contains auxiliary shapes (for example the circles beside a
	// list), because it produces diagonal lines that do not exist in PowerPoint.
	paintLogicalConnectors := len(part.DrawingXML) == 0 || len(preRenderedConns) > 0

	for i, edge := range edges {
		if !paintLogicalConnectors {
			break
		}
		if !smartArtConnectorEdgeAllowed(edge.srcID, connectorLayout.ConnectorSourceIDs) ||
			!smartArtConnectorEdgeAllowed(edge.dstID, connectorLayout.ConnectorDestIDs) {
			continue
		}
		src, ok1 := nodeMap[edge.srcID]
		dst, ok2 := nodeMap[edge.dstID]
		if !ok1 || !ok2 {
			continue
		}

		var conn *Shape
		if i < len(preRenderedConns) {
			conn = buildSmartArtConnectorFromDrawing(preRenderedConns[i], connLine, connScale, connOffsetX, connOffsetY)
		}
		if conn == nil {
			conn = buildSyntheticSmartArtConnector(src, dst, connLine, layoutType, layoutDir, edge.assistant, connectorLayout)
		}
		shapes = append(shapes, conn)
	}

	// Preserve the source drawing order for SmartArt shapes when a pre-rendered
	// drawing part supplied it. Connectors and fallback-laid-out shapes keep
	// their relative positions.
	sort.SliceStable(shapes, func(i, j int) bool {
		// Source dsp:spTree order is zero-based; zero is a valid first item.
		// Negative values are reserved for synthetic/background items.
		li, lj := shapes[i].DrawingOrder, shapes[j].DrawingOrder
		if li < 0 || lj < 0 {
			return li < lj
		}
		return li < lj
	})

	return shapes, diags
}

// smartArtPicturePlaceholderPresentationIDs returns presentation-node ids whose
// layout role is an unpopulated image shape.  These nodes are real painted
// SmartArt primitives even when no picture relationship has been supplied.
func smartArtPicturePlaceholderPresentationIDs(dataXML []byte) map[string]bool {
	out := make(map[string]bool)
	dec := xml.NewDecoder(bytes.NewReader(dataXML))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "pt" {
			continue
		}
		var modelID, typ string
		for _, a := range se.Attr {
			if a.Name.Local == "modelId" {
				modelID = a.Value
			}
			if a.Name.Local == "type" {
				typ = a.Value
			}
		}
		if typ != "pres" || modelID == "" {
			continue
		}
		var node struct {
			PrSet struct {
				PresName     string `xml:"presName,attr"`
				PresStyleLbl string `xml:"presStyleLbl,attr"`
			} `xml:"prSet"`
		}
		if err := dec.DecodeElement(&node, &se); err != nil {
			continue
		}
		name := strings.ToLower(node.PrSet.PresName)
		style := strings.ToLower(node.PrSet.PresStyleLbl)
		if strings.Contains(name, "img") || strings.Contains(style, "imgplace") {
			out[modelID] = true
		}
	}
	return out
}

// buildAuxiliarySmartArtShapeFromDrawing materializes presentation-only artwork
// from an authoritative dsp:drawing scene. Coordinates are raw drawing EMUs;
// smartArtConnectorTransform returns the common drawing->frame scale and frame
// offset used by all pre-rendered SmartArt primitives.
func buildAuxiliarySmartArtShapeFromDrawing(info *smartartrender.DrawingShapeInfo, scale, offsetX, offsetY float64) *Shape {
	if info == nil {
		return nil
	}
	shape := &Shape{
		Name:                "SmartArt Drawing",
		Kind:                ShapeSmartArt,
		Bounds:              Rect{X: int64(math.Round((offsetX + info.X*scale) * emuPerPt)), Y: int64(math.Round((offsetY + info.Y*scale) * emuPerPt)), W: int64(math.Round(info.W * scale * emuPerPt)), H: int64(math.Round(info.H * scale * emuPerPt))},
		Fill:                info.Fill,
		LineStyle:           info.Line,
		Effects:             info.Effect,
		Geometry:            info.Geometry,
		GeometryAdjustments: append([]float64(nil), info.Adjustments...),
		GeometryData:        info.GeometryData,
		DrawingOrder:        info.DrawingOrder,
		RotationDeg:         info.RotationDeg,
		FlipH:               info.FlipH,
		FlipV:               info.FlipV,
		SmartArtNodeID:      info.ModelID,
		TextBodyProps:       info.TextBodyProps,
	}
	if sf, ok := info.Fill.(drawing.SolidFill); ok {
		shape.FillColor = sf.Color
	}
	if info.Line != nil {
		shape.LineColor = info.Line.Color
		shape.LineWidthPt = info.Line.Width
	}
	// Presentation-only drawing shapes may legitimately carry text. Preserve it
	// rather than forcing every auxiliary primitive down a geometry-only path.
	if len(info.Paragraphs) > 0 {
		shape.Paragraphs = make([]*Paragraph, 0, len(info.Paragraphs))
		for _, p := range info.Paragraphs {
			if p == nil {
				continue
			}
			para := &Paragraph{Align: p.Align, Level: p.Level, BulletChar: p.BulletChar, SpaceBefore: p.SpaceBefore, SpaceAfter: p.SpaceAfter, SpaceBeforePct: p.SpaceBeforePct, SpaceAfterPct: p.SpaceAfterPct, LineSpacingPct: p.LineSpacingPct, LineSpacingPt: p.LineSpacingPts, HasLineSpacing: p.LineSpacingPct != 0 || p.LineSpacingPts != 0}
			for _, r := range p.Runs {
				font := r.FontFamily
				if font == "" {
					font = r.FontFamilyEA
				}
				if font == "" {
					font = r.FontFamilyCS
				}
				para.Runs = append(para.Runs, &Run{Text: r.Text, Bold: r.Bold, Italic: r.Italic, Underline: r.Underline, FontSize: r.FontSize, FontFamily: font, Color: r.Color, Effects: r.Effects})
			}
			shape.Paragraphs = append(shape.Paragraphs, para)
		}
	} else if info.Text != "" {
		shape.Paragraphs = []*Paragraph{{Runs: []*Run{{Text: info.Text}}}}
	}
	return shape
}

// smartArtConnectorTransform computes the uniform scale and offset needed to map
// pre-rendered drawing connector coordinates into the hosting graphic frame.
// Values are returned in points.
func smartArtConnectorTransform(drawingXML []byte, frame *smartartrender.FrameBounds, themeColors map[string]string) (scale, offsetX, offsetY float64) {
	if len(drawingXML) == 0 || frame == nil {
		return 1, 0, 0
	}
	shapes := smartartrender.ExtractShapeInfos(drawingXML, themeColors)
	if len(shapes) == 0 {
		return 1, 0, 0
	}
	minX, minY, maxX, maxY, ok := smartartrender.DrawingBounds(shapes)
	if !ok {
		return 1, 0, 0
	}
	drawingW := maxX - minX
	drawingH := maxY - minY
	if drawingW == 0 || drawingH == 0 {
		return 1, 0, 0
	}
	frameX := frame.X / emuPerPt
	frameY := frame.Y / emuPerPt
	frameW := frame.W / emuPerPt
	frameH := frame.H / emuPerPt
	if frameW <= 0 || frameH <= 0 {
		return 1, 0, 0
	}
	scale = frameW / drawingW
	if frameH/drawingH < scale {
		scale = frameH / drawingH
	}
	offsetX = frameX + (frameW-drawingW*scale)/2 - minX*scale
	offsetY = frameY + (frameH-drawingH*scale)/2 - minY*scale
	return scale, offsetX, offsetY
}

const emuPerPt = 12700.0

// buildSmartArtConnectorFromDrawing creates a resolve.Shape from a pre-rendered
// dsp:cxnSp connector. The connector bounds are scaled uniformly to fit inside
// the hosting graphic frame, matching the transform applied to shape positions.
func buildSmartArtConnectorFromDrawing(info *smartartrender.DrawingConnectorInfo, connLine *drawing.LineStyle, scale, offsetX, offsetY float64) *Shape {
	if info == nil {
		return nil
	}
	line := mergeSmartArtConnectorLine(connLine, info.Line)
	conn := &Shape{
		Kind:                 ShapeSmartArtConnector,
		Name:                 "SmartArt Connector",
		LineStyle:            line,
		LineColor:            line.Color,
		LineWidthPt:          line.Width,
		ConnectorType:        info.Geometry,
		ConnectorFlipH:       info.FlipH,
		ConnectorFlipV:       info.FlipV,
		ConnectorAdjustments: smartArtConnectorAdjustments(info.Adjustments),
		DrawingOrder:         info.DrawingOrder,
		RotationDeg:          info.RotationDeg,
	}
	if line.HeadEnd != nil {
		conn.ArrowHead = string(line.HeadEnd.Type)
		conn.ArrowHeadWidth = string(line.HeadEnd.Size)
		conn.ArrowHeadLength = string(line.HeadEnd.Length)
	}
	if line.TailEnd != nil {
		conn.ArrowTail = string(line.TailEnd.Type)
		conn.ArrowTailWidth = string(line.TailEnd.Size)
		conn.ArrowTailLength = string(line.TailEnd.Length)
	}

	bx := info.X/emuPerPt*scale + offsetX
	by := info.Y/emuPerPt*scale + offsetY
	bw := info.W / emuPerPt * scale
	bh := info.H / emuPerPt * scale
	conn.Bounds = Rect{
		X: int64(bx * emuPerPt),
		Y: int64(by * emuPerPt),
		W: int64(bw * emuPerPt),
		H: int64(bh * emuPerPt),
	}
	return conn
}

// smartArtConnectorAdjustments converts 0-1 adjustment values from the drawing
// part into the 0-100000 integer scale used by drawing.ConnectorWaypoints.
func smartArtConnectorAdjustments(adjs []float64) []int {
	out := make([]int, len(adjs))
	for i, v := range adjs {
		out[i] = int(math.Round(v * 100000))
	}
	return out
}

// mergeSmartArtConnectorLine combines the diagram-wide connector line style
// with any overrides found on the pre-rendered connector.
func mergeSmartArtConnectorLine(base, override *drawing.LineStyle) *drawing.LineStyle {
	out := &drawing.LineStyle{Color: "999999", Width: 1.0}
	if base != nil {
		*out = *base
	}
	if override == nil {
		return out
	}
	if override.Color != "" {
		out.Color = override.Color
	}
	if override.Width > 0 {
		out.Width = override.Width
	}
	if override.Fill != nil {
		out.Fill = override.Fill
	}
	if override.Alpha != 0 {
		out.Alpha = override.Alpha
	}
	if override.Dash != "" {
		out.Dash = override.Dash
	}
	if override.Cap != "" {
		out.Cap = override.Cap
	}
	if override.Compound {
		out.Compound = true
		out.CompoundType = override.CompoundType
	}
	if override.HeadEnd != nil {
		out.HeadEnd = override.HeadEnd
	}
	if override.TailEnd != nil {
		out.TailEnd = override.TailEnd
	}
	return out
}

// buildSyntheticSmartArtConnector creates a straight or elbow connector between
// two SmartArt nodes when no pre-rendered connector geometry is available.
func buildSyntheticSmartArtConnector(src, dst *Shape, connLine *drawing.LineStyle, layoutType, layoutDir string, assistant bool, connectorLayouts ...*smartart.Layout) *Shape {
	var connectorLayout *smartart.Layout
	if len(connectorLayouts) > 0 {
		connectorLayout = connectorLayouts[0]
	}
	conn := &Shape{
		Kind:        ShapeSmartArtConnector,
		Name:        "SmartArt Connector",
		LineStyle:   connLine,
		LineColor:   connLine.Color,
		LineWidthPt: connLine.Width,
	}
	if connLine.HeadEnd != nil {
		conn.ArrowHead = string(connLine.HeadEnd.Type)
		conn.ArrowHeadWidth = string(connLine.HeadEnd.Size)
		conn.ArrowHeadLength = string(connLine.HeadEnd.Length)
	}
	if connLine.TailEnd != nil {
		conn.ArrowTail = string(connLine.TailEnd.Type)
		conn.ArrowTailWidth = string(connLine.TailEnd.Size)
		conn.ArrowTailLength = string(connLine.TailEnd.Length)
	}

	routing := ""
	bendPoint := ""
	begPts := ""
	endPts := ""
	dimension := ""
	if connectorLayout != nil {
		routing = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorRouting))
		bendPoint = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorBendPoint))
		begPts = connectorLayout.ConnectorBeginPts
		endPts = connectorLayout.ConnectorEndPts
		dimension = strings.ToLower(strings.TrimSpace(connectorLayout.ConnectorDimension))
		applySmartArtConnectorArrowStyle(conn, connectorLayout.ConnectorBeginStyle, true)
		applySmartArtConnectorArrowStyle(conn, connectorLayout.ConnectorEndStyle, false)
	}
	if dimension == "1d" {
		routing = "stra"
	}

	// Determine the authored or layout-derived endpoints first. Explicit
	// begPts/endPts take precedence over hierarchy-direction heuristics.
	srcCenter := Point{X: src.Bounds.X + src.Bounds.W/2, Y: src.Bounds.Y + src.Bounds.H/2}
	dstCenter := Point{X: dst.Bounds.X + dst.Bounds.W/2, Y: dst.Bounds.Y + dst.Bounds.H/2}
	start, startExplicit := smartArtConnectorAnchor(src.Bounds, begPts, dstCenter)
	end, endExplicit := smartArtConnectorAnchor(dst.Bounds, endPts, srcCenter)

	if !startExplicit || !endExplicit {
		if layoutType == "hierarchy" {
			if assistant {
				if dstCenter.X >= srcCenter.X {
					if !startExplicit {
						start = Point{X: src.Bounds.X + src.Bounds.W, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X, Y: dstCenter.Y}
					}
				} else {
					if !startExplicit {
						start = Point{X: src.Bounds.X, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X + dst.Bounds.W, Y: dstCenter.Y}
					}
				}
			} else {
				switch layoutDir {
				case "leftRight":
					if !startExplicit {
						start = Point{X: src.Bounds.X + src.Bounds.W, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X, Y: dstCenter.Y}
					}
				case "rightLeft":
					if !startExplicit {
						start = Point{X: src.Bounds.X, Y: srcCenter.Y}
					}
					if !endExplicit {
						end = Point{X: dst.Bounds.X + dst.Bounds.W, Y: dstCenter.Y}
					}
				case "bottomUp":
					if !startExplicit {
						start = Point{X: srcCenter.X, Y: src.Bounds.Y}
					}
					if !endExplicit {
						end = Point{X: dstCenter.X, Y: dst.Bounds.Y + dst.Bounds.H}
					}
				default:
					if !startExplicit {
						start = Point{X: srcCenter.X, Y: src.Bounds.Y + src.Bounds.H}
					}
					if !endExplicit {
						end = Point{X: dstCenter.X, Y: dst.Bounds.Y}
					}
				}
			}
		} else {
			if !startExplicit {
				start = smartArtNearestPerimeterPoint(src.Bounds, dstCenter)
			}
			if !endExplicit {
				end = smartArtNearestPerimeterPoint(dst.Bounds, srcCenter)
			}
		}
	}

	conn.ConnectorStart = &start
	conn.ConnectorEnd = &end

	if routing == "" && layoutType == "hierarchy" {
		routing = "bend"
	}
	switch routing {
	case "bend":
		conn.ConnectorType = "elbow"
		conn.ConnectorPath = smartArtElbowPath(start, end, bendPoint)
	case "curve":
		conn.ConnectorType = "curved"
	case "longcurve":
		conn.ConnectorType = "longCurve"
	case "stra", "":
		conn.ConnectorType = "straight"
	default:
		conn.ConnectorType = "straight"
	}
	return conn
}

func smartArtConnectorEdgeAllowed(id string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == id {
			return true
		}
	}
	return false
}

func applySmartArtConnectorArrowStyle(conn *Shape, style string, beginning bool) {
	style = strings.ToLower(strings.TrimSpace(style))
	if style == "" || style == "auto" {
		return
	}
	if beginning {
		if style == "noarr" {
			conn.ArrowTail = ""
			return
		}
		if style == "arr" {
			conn.ArrowTail = "triangle"
			if conn.ArrowTailWidth == "" {
				conn.ArrowTailWidth = "med"
			}
			if conn.ArrowTailLength == "" {
				conn.ArrowTailLength = "med"
			}
		}
		return
	}
	if style == "noarr" {
		conn.ArrowHead = ""
		return
	}
	if style == "arr" {
		conn.ArrowHead = "triangle"
		if conn.ArrowHeadWidth == "" {
			conn.ArrowHeadWidth = "med"
		}
		if conn.ArrowHeadLength == "" {
			conn.ArrowHeadLength = "med"
		}
	}
}

func smartArtConnectorAnchor(bounds Rect, token string, toward Point) (Point, bool) {
	parts := strings.FieldsFunc(token, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' })
	if len(parts) == 0 {
		return Point{}, false
	}
	token = strings.ToLower(strings.TrimSpace(parts[0]))
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	switch token {
	case "bctr":
		return Point{X: cx, Y: bounds.Y + bounds.H}, true
	case "ctr":
		return Point{X: cx, Y: cy}, true
	case "midl":
		return Point{X: bounds.X, Y: cy}, true
	case "midr":
		return Point{X: bounds.X + bounds.W, Y: cy}, true
	case "tctr":
		return Point{X: cx, Y: bounds.Y}, true
	case "bl":
		return Point{X: bounds.X, Y: bounds.Y + bounds.H}, true
	case "br":
		return Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H}, true
	case "tl":
		return Point{X: bounds.X, Y: bounds.Y}, true
	case "tr":
		return Point{X: bounds.X + bounds.W, Y: bounds.Y}, true
	case "radial":
		return smartArtNearestPerimeterPoint(bounds, toward), true
	case "auto", "":
		return Point{}, false
	default:
		return Point{}, false
	}
}

func smartArtNearestPerimeterPoint(bounds Rect, toward Point) Point {
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	dx := float64(toward.X - cx)
	dy := float64(toward.Y - cy)
	if dx == 0 && dy == 0 {
		return Point{X: cx, Y: cy}
	}
	hw := float64(bounds.W) / 2
	hh := float64(bounds.H) / 2
	sx, sy := math.Inf(1), math.Inf(1)
	if dx != 0 {
		sx = hw / math.Abs(dx)
	}
	if dy != 0 {
		sy = hh / math.Abs(dy)
	}
	t := math.Min(sx, sy)
	return Point{X: cx + int64(math.Round(dx*t)), Y: cy + int64(math.Round(dy*t))}
}

func smartArtElbowPath(start, end Point, bendPoint string) []Point {
	dx := end.X - start.X
	dy := end.Y - start.Y
	fraction := 0.5
	switch bendPoint {
	case "beg":
		fraction = 0.25
	case "end":
		fraction = 0.75
	}
	if absInt64(dx) >= absInt64(dy) {
		midX := start.X + int64(math.Round(float64(dx)*fraction))
		return []Point{start, {X: midX, Y: start.Y}, {X: midX, Y: end.Y}, end}
	}
	midY := start.Y + int64(math.Round(float64(dy)*fraction))
	return []Point{start, {X: start.X, Y: midY}, {X: end.X, Y: midY}, end}
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
