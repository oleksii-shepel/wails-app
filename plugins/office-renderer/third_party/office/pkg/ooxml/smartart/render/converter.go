package render

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
)

// Connection represents a link from one shape to another.
type Connection struct {
	// ToID is the target shape ID.
	ToID string
}

// Shape represents a positioned shape produced by the layout engine.
type Shape struct {
	// ID is the unique node identifier.
	ID string

	// Type is the shape geometry
	Type string

	// Text content (plain-text fallback).
	Text string

	// Paragraphs holds styled text runs when available.
	Paragraphs []*smartart.TextParagraph

	// Position and size
	X, Y          float64
	Width, Height float64

	// RotationDeg is the clockwise rotation in degrees.
	RotationDeg float64

	// FlipH and FlipV reflect horizontal/vertical flips.
	FlipH bool
	FlipV bool

	// Fill is the DrawingML fill.
	Fill drawing.Fill

	// Line is the DrawingML line style.
	Line *drawing.LineStyle

	// Effect is the resolved effect list for this shape.
	Effect drawing.EffectList

	// TextColor is the text color (hex RGB).
	TextColor string

	// TextBodyProps holds text-body properties for the shape's text container.
	TextBodyProps drawing.TextBodyProperties

	// PlaceholderText is shown when the node has no real text content.
	PlaceholderText string

	// Connections are outgoing links to child shapes.
	Connections []Connection

	// GeometryAdjustments holds preset-geometry adjustment values (0-1).
	GeometryAdjustments []float64

	// CustomGeometry holds a parsed a:custGeom custom geometry, if any.
	CustomGeometry *drawing.Geometry

	// DrawingOrder preserves the source dsp:spTree order for Z-ordering.
	DrawingOrder int
}

// SmartArtShape represents a shape ready for rendering.
// It carries shared DrawingML types so the same fill/line/stroke pipeline
// used for regular shapes can be reused for SmartArt.
type SmartArtShape struct {
	ID              string
	Type            string
	Text            string
	Paragraphs      []*smartart.TextParagraph
	X, Y            float64
	Width           float64
	Height          float64
	RotationDeg     float64
	FlipH           bool
	FlipV           bool
	Fill            drawing.Fill
	Line            *drawing.LineStyle
	Effect          drawing.EffectList
	TextColor       string
	TextBodyProps   drawing.TextBodyProperties
	PlaceholderText string
	Connections     []Connection

	// GeometryAdjustments holds preset-geometry adjustment values (0-1).
	GeometryAdjustments []float64
	// CustomGeometry holds a parsed a:custGeom custom geometry, if any.
	CustomGeometry *drawing.Geometry
	// DrawingOrder preserves the source dsp:spTree order for Z-ordering.
	DrawingOrder int
}

// ToSmartArtShapes converts internal shapes to renderable format.
func ToSmartArtShapes(shapes []*Shape) []*SmartArtShape {
	var result []*SmartArtShape

	for _, s := range shapes {
		shape := &SmartArtShape{
			ID:                  s.ID,
			Type:                s.Type,
			Text:                s.Text,
			Paragraphs:          s.Paragraphs,
			X:                   s.X,
			Y:                   s.Y,
			Width:               s.Width,
			Height:              s.Height,
			RotationDeg:         s.RotationDeg,
			FlipH:               s.FlipH,
			FlipV:               s.FlipV,
			Fill:                s.Fill,
			Line:                s.Line,
			Effect:              s.Effect,
			TextColor:           s.TextColor,
			TextBodyProps:       s.TextBodyProps,
			PlaceholderText:     s.PlaceholderText,
			Connections:         s.Connections,
			GeometryAdjustments: s.GeometryAdjustments,
			CustomGeometry:      s.CustomGeometry,
			DrawingOrder:        s.DrawingOrder,
		}
		result = append(result, shape)
	}

	return result
}

// LayoutDiagram runs the layout engine on a semantic diagram and returns
// positioned shapes ready for rendering.
func LayoutDiagram(diagram *smartart.Diagram) []*SmartArtShape {
	return LayoutDiagramWithFrame(diagram, nil)
}

// LayoutDiagramWithFrame runs the layout engine using the hosting
// p:graphicFrame bounds. The frame values are expected to be in points.
func LayoutDiagramWithFrame(diagram *smartart.Diagram, frame *FrameBounds) []*SmartArtShape {
	engine := NewLayoutEngine(diagram)
	engine.FrameBounds = frame
	shapes := engine.ToShapes()
	return ToSmartArtShapes(shapes)
}

// emuToPt converts EMU coordinates to points.
func emuToPt(v float64) float64 {
	return v / 12700.0
}

// ProcessDiagram parses SmartArt XML and converts to renderable shapes.
// This is a convenience wrapper around ParseDiagram + LayoutDiagram.
func ProcessDiagram(dataModelXML []byte) ([]*SmartArtShape, error) {
	return ProcessDiagramWithOptions(dataModelXML, nil, nil, nil)
}

// ProcessDiagramWithOptions parses SmartArt data model XML with optional
// layout definition, quick style, and color style XML, then lays out and
// converts the diagram to renderable shapes.
func ProcessDiagramWithOptions(dataModelXML, layoutXML, styleXML, colorXML []byte) ([]*SmartArtShape, error) {
	return ProcessDiagramWithOptionsAndFrame(dataModelXML, layoutXML, styleXML, colorXML, nil, nil, nil)
}

// FrameBounds describes the hosting p:graphicFrame rectangle in EMU.
type FrameBounds struct {
	X, Y, W, H float64
}

// ProcessDiagramWithOptionsAndFrame parses the data model and, when a
// pre-rendered drawing part and hosting frame bounds are supplied, maps the
// drawing part's exact shape positions and styles onto the data-model nodes.
// Nodes that are not represented in the drawing part fall back to the layout
// engine.  The optional themeColors map is used to resolve a:schemeClr values
// in the color style XML.
func ProcessDiagramWithOptionsAndFrame(dataModelXML, layoutXML, styleXML, colorXML, drawingXML []byte, frame *FrameBounds, themeColors map[string]string) ([]*SmartArtShape, error) {
	return ProcessDiagramWithOptionsAndFrameFormat(dataModelXML, layoutXML, styleXML, colorXML, drawingXML, frame, themeColors, nil)
}

// ProcessDiagramWithOptionsAndFrameFormat is like ProcessDiagramWithOptionsAndFrame
// but accepts a resolved ThemeFormatScheme so that style-matrix references in
// the quick style part can be resolved.
func ProcessDiagramWithOptionsAndFrameFormat(dataModelXML, layoutXML, styleXML, colorXML, drawingXML []byte, frame *FrameBounds, themeColors map[string]string, fmtScheme *smartart.ThemeFormatScheme) ([]*SmartArtShape, error) {
	// Parse the data model
	diagram, err := smartart.ParseDiagramWithOptionsAndThemeFormat(dataModelXML, layoutXML, styleXML, colorXML, themeColors, fmtScheme)
	if err != nil {
		return nil, err
	}

	// If a pre-rendered drawing part is available, use it for geometry and style.
	paraMap := map[string][]*smartart.TextParagraph{}
	if len(drawingXML) > 0 {
		paraMap = applyDrawingToDiagram(drawingXML, dataModelXML, frame, diagram, themeColors)
	}

	// Layout and convert to renderable format. Frame bounds are supplied in EMU
	// by callers; convert them to points before the layout engine uses them.
	var framePt *FrameBounds
	if frame != nil {
		framePt = &FrameBounds{
			X: emuToPt(frame.X),
			Y: emuToPt(frame.Y),
			W: emuToPt(frame.W),
			H: emuToPt(frame.H),
		}
	}
	shapes := LayoutDiagramWithFrame(diagram, framePt)
	for _, s := range shapes {
		if len(diagram.WholeEffect.Effects) > 0 {
			s.Effect.Effects = append(s.Effect.Effects, diagram.WholeEffect.Effects...)
		}
		if paras, ok := paraMap[s.ID]; ok {
			s.Paragraphs = paras
		}
	}
	if diagram.BackgroundFill != nil {
		shapes = prependDiagramBackgroundShape(shapes, diagram.BackgroundFill, framePt)
	}
	return shapes, nil
}

func prependDiagramBackgroundShape(shapes []*SmartArtShape, fill drawing.Fill, frame *FrameBounds) []*SmartArtShape {
	if fill == nil {
		return shapes
	}
	bg := &SmartArtShape{
		ID:           "__background",
		Type:         "rect",
		Fill:         fill,
		DrawingOrder: -1,
	}
	if frame != nil && frame.W > 0 && frame.H > 0 {
		bg.X, bg.Y, bg.Width, bg.Height = frame.X, frame.Y, frame.W, frame.H
	} else if len(shapes) > 0 {
		minX, minY := shapes[0].X, shapes[0].Y
		maxX, maxY := shapes[0].X+shapes[0].Width, shapes[0].Y+shapes[0].Height
		for _, s := range shapes[1:] {
			if s.X < minX {
				minX = s.X
			}
			if s.Y < minY {
				minY = s.Y
			}
			if x := s.X + s.Width; x > maxX {
				maxX = x
			}
			if y := s.Y + s.Height; y > maxY {
				maxY = y
			}
		}
		bg.X, bg.Y = minX, minY
		bg.Width, bg.Height = maxX-minX, maxY-minY
	} else {
		bg.Width, bg.Height = 800, 600
	}
	return append([]*SmartArtShape{bg}, shapes...)
}

// applyDrawingToDiagram maps the pre-rendered dsp:drawing shapes onto the
// parsed data-model nodes. Positions are normalized to the diagram bounding box
// and then scaled uniformly to fit inside the supplied frame.  It returns a
// map from data-model node ID to the styled paragraphs extracted from the
// drawing part's txBody.
func applyDrawingToDiagram(drawingXML, dataModelXML []byte, frame *FrameBounds, diagram *smartart.Diagram, themeColors map[string]string) map[string][]*smartart.TextParagraph {
	shapes, connStyle, err := parseDiagramDrawing(drawingXML, themeColors)
	if err != nil || len(shapes) == 0 {
		return nil
	}
	if connStyle != nil && diagram.Style != nil {
		diagram.Style.ConnectorLine = connStyle
	}
	paragraphs := make(map[string][]*smartart.TextParagraph)

	dataToPres := smartart.DataToPresentationMap(dataModelXML)
	minX, minY, maxX, maxY, ok := DrawingBounds(shapes)
	if !ok {
		return nil
	}
	drawingW := maxX - minX
	drawingH := maxY - minY
	if drawingW == 0 || drawingH == 0 {
		return nil
	}

	var frameX, frameY, frameW, frameH float64
	if frame != nil {
		frameX, frameY = emuToPt(frame.X), emuToPt(frame.Y)
		frameW, frameH = emuToPt(frame.W), emuToPt(frame.H)
	}

	for _, node := range diagram.Nodes {
		presID, ok := dataToPres[node.ID]
		if !ok {
			continue
		}
		s, ok := shapes[presID]
		if !ok {
			continue
		}

		if frame != nil && frameW > 0 && frameH > 0 {
			// DrawingBounds is expressed in EMU while the layout engine and frame
			// are expressed in points. Convert the drawing envelope exactly once
			// before deriving the fit scale. Mixing the two units made pre-rendered
			// SmartArt nodes roughly 12,700x too small. Also normalize every node
			// against the drawing envelope origin: diagram drawings are not required
			// to start at (0,0).
			drawingWPt := emuToPt(drawingW)
			drawingHPt := emuToPt(drawingH)
			if drawingWPt <= 0 || drawingHPt <= 0 {
				continue
			}
			scale := frameW / drawingWPt
			if frameH/drawingHPt < scale {
				scale = frameH / drawingHPt
			}
			offsetX := frameX + (frameW-drawingWPt*scale)/2
			offsetY := frameY + (frameH-drawingHPt*scale)/2
			node.Position.X = emuToPt(s.X-minX)*scale + offsetX
			node.Position.Y = emuToPt(s.Y-minY)*scale + offsetY
			node.Position.Width = emuToPt(s.W) * scale
			node.Position.Height = emuToPt(s.H) * scale
		} else {
			// No frame supplied: use the raw drawing coordinates in points.
			node.Position.X = emuToPt(s.X)
			node.Position.Y = emuToPt(s.Y)
			node.Position.Width = emuToPt(s.W)
			node.Position.Height = emuToPt(s.H)
		}
		node.RotationDeg = s.RotationDeg
		node.FlipH = s.FlipH
		node.FlipV = s.FlipV

		if s.Geometry != "" {
			node.Geometry = s.Geometry
		}
		if len(s.Adjustments) > 0 {
			node.GeometryAdjustments = s.Adjustments
		}
		if s.GeometryData != nil {
			node.CustomGeometry = s.GeometryData
		}
		if s.DrawingOrder != 0 {
			node.DrawingOrder = s.DrawingOrder
		}
		if s.Fill != nil {
			node.Fill = s.Fill
		}
		if s.Line != nil {
			node.Line = s.Line
		}
		node.Effect = s.Effect
		if s.Text != "" {
			node.Text = s.Text
		}
		if len(s.Paragraphs) > 0 {
			paragraphs[node.ID] = s.Paragraphs
		}
		node.TextBodyProps = s.TextBodyProps
		node.HasPosition = true
	}
	return paragraphs
}

// ToDrawingML converts SmartArt shapes to DrawingML compatible format.
// This returns rectangles that represent the SmartArt nodes.
func ToDrawingML(shapes []*SmartArtShape) []*drawing.Rect {
	var result []*drawing.Rect

	for _, s := range shapes {
		rect := &drawing.Rect{
			X:      s.X,
			Y:      s.Y,
			Width:  s.Width,
			Height: s.Height,
		}
		result = append(result, rect)
	}

	return result
}
