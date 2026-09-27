package officeart

import (
	officeartir "office/pkg/officeart/ir"
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// FallbackShape returns the minimal shape used when FSPA ownership exists but
// the corresponding OfficeArt SpContainer is missing or damaged.
func FallbackShape(shapeID uint32) *wordir.Shape {
	return &wordir.Shape{LegacyShapeID: shapeID, Kind: "rect"}
}

// ProjectAnchoredShape combines canonical OfficeArt visual projection with the
// canonical FSPA placement model. Missing/damaged OfficeArt falls back to a
// plain rectangle while retaining the authoritative anchor geometry.
func (c Catalog) ProjectAnchoredShape(anchor docbinary.FloatingAnchor) *wordir.Shape {
	shape := c.ProjectShape(anchor.Shape)
	if shape == nil {
		shape = FallbackShape(anchor.Shape)
	}
	if !docbinary.ApplyAnchorToShape(shape, anchor) {
		return nil
	}
	return shape
}

// MergeTextBoxVisual overlays shape-authored textbox body properties onto a
// story-backed textbox while preserving explicit zero/false values.
func MergeTextBoxVisual(incoming, visual *wordir.TextBox) {
	if incoming == nil || visual == nil {
		return
	}
	if visual.HasVertSetting {
		incoming.Vert = visual.Vert
		incoming.HasVertSetting = true
	} else if incoming.Vert == "" {
		incoming.Vert = visual.Vert
	}
	if visual.HasWrapSetting {
		incoming.NoWrap = visual.NoWrap
		incoming.HasWrapSetting = true
	} else {
		incoming.NoWrap = incoming.NoWrap || visual.NoWrap
	}
	if visual.HasAnchorSetting {
		incoming.Anchor = visual.Anchor
		incoming.AnchorCenter = visual.AnchorCenter
		incoming.HasAnchorSetting = true
	} else {
		if incoming.Anchor == "" {
			incoming.Anchor = visual.Anchor
		}
		incoming.AnchorCenter = incoming.AnchorCenter || visual.AnchorCenter
	}
	if visual.HasPaddingLeft {
		incoming.PaddingLeft = visual.PaddingLeft
		incoming.HasPaddingLeft = true
	} else if incoming.PaddingLeft == 0 {
		incoming.PaddingLeft = visual.PaddingLeft
	}
	if visual.HasPaddingTop {
		incoming.PaddingTop = visual.PaddingTop
		incoming.HasPaddingTop = true
	} else if incoming.PaddingTop == 0 {
		incoming.PaddingTop = visual.PaddingTop
	}
	if visual.HasPaddingRight {
		incoming.PaddingRight = visual.PaddingRight
		incoming.HasPaddingRight = true
	} else if incoming.PaddingRight == 0 {
		incoming.PaddingRight = visual.PaddingRight
	}
	if visual.HasPaddingBottom {
		incoming.PaddingBottom = visual.PaddingBottom
		incoming.HasPaddingBottom = true
	} else if incoming.PaddingBottom == 0 {
		incoming.PaddingBottom = visual.PaddingBottom
	}
	if incoming.Autofit == "" {
		incoming.Autofit = visual.Autofit
	}
	if incoming.FontScale == 0 {
		incoming.FontScale = visual.FontScale
	}
	if incoming.LnSpcReduction == 0 {
		incoming.LnSpcReduction = visual.LnSpcReduction
	}
}

// AttachTextBoxes resolves story-backed textbox bodies onto a projected shape
// tree. Shape IDs are authoritative; an ownership-free textbox is paired only
// with an existing empty textbox placeholder for malformed/synthetic inputs.
func AttachTextBoxes(shape *wordir.Shape, remaining []*wordir.TextBox) []*wordir.TextBox {
	if shape == nil || len(remaining) == 0 {
		return remaining
	}
	match := -1
	if shape.LegacyShapeID != 0 {
		for i, candidate := range remaining {
			if candidate != nil && candidate.LegacyShapeID == shape.LegacyShapeID {
				match = i
				break
			}
		}
	}
	if match >= 0 {
		incoming := remaining[match]
		if shape.TextBox != nil {
			MergeTextBoxVisual(incoming, shape.TextBox)
		}
		shape.TextBox = incoming
		remaining = append(remaining[:match], remaining[match+1:]...)
	} else if shape.TextBox != nil && len(shape.TextBox.Blocks) == 0 {
		for i, candidate := range remaining {
			if candidate != nil && candidate.LegacyShapeID == 0 {
				match = i
				break
			}
		}
		if match >= 0 {
			incoming := remaining[match]
			MergeTextBoxVisual(incoming, shape.TextBox)
			shape.TextBox = incoming
			remaining = append(remaining[:match], remaining[match+1:]...)
		}
	}
	for i := range shape.Children {
		if len(remaining) == 0 {
			break
		}
		remaining = AttachTextBoxes(&shape.Children[i], remaining)
	}
	return remaining
}

// ProjectShape converts shared OfficeArt visual semantics into Word IR. Word
// anchor/story ownership stays in doc/binary; this function owns only the
// shape-local visual projection.
func (c Catalog) ProjectShape(shapeID uint32) *wordir.Shape {
	source, ok := c.Find(shapeID)
	if !ok || source == nil {
		return nil
	}
	return projectShape(source)
}

func projectShape(source *officeartir.Shape) *wordir.Shape {
	if source == nil {
		return nil
	}
	out := &wordir.Shape{OfficeArt: source, LegacyShapeID: source.ID, Kind: "rect", X: source.Transform.Rect.X, Y: source.Transform.Rect.Y, Width: source.Transform.Rect.W, Height: source.Transform.Rect.H, RotationDeg: wordLegacyRotationDeg(source.Transform.RotationDeg), FlipH: source.Transform.FlipH, FlipV: source.Transform.FlipV}
	projectGeometry(source.Geometry, out)
	projectShapeStyle(source, out)
	if source.Text != nil {
		out.TextBox = projectTextBody(source.ID, source.Text)
		// Word binary text boxes never rotate or flip their text with the
		// shape: desktop Word lays the text out horizontally inside the
		// unrotated anchor rectangle and rotates only the shape outline.
		if out.TextBox != nil && (out.RotationDeg != 0 || out.FlipH || out.FlipV) {
			out.LegacyUprightText = true
		}
	}
	if source.GeometryText != nil {
		out.TextPath = projectGeometryText(source.GeometryText)
	}
	if len(source.Children) > 0 {
		out.Kind = "group"
		out.Geometry = nil
		out.GeometryData = nil
		out.Children = make([]wordir.Shape, 0, len(source.Children))
		for _, child := range source.Children {
			if projected := projectShape(child); projected != nil {
				out.Children = append(out.Children, *projected)
			}
		}
	}
	return out
}

func projectGeometry(source officeartir.Geometry, out *wordir.Shape) {
	if out == nil {
		return
	}
	switch source.Kind {
	case officeartir.GeometryPreset:
		if source.Preset == "" {
			return
		}
		out.Kind = source.Preset
		out.Geometry = &dml.PresetGeometry{Name: source.Preset, Adjustments: append([]float64(nil), source.Adjustments...)}
	case officeartir.GeometryCustom:
		if geometry := projectCustomGeometry(source); geometry != nil {
			out.Kind = "custom"
			out.GeometryData = geometry
		}
	}
}
func projectCustomGeometry(source officeartir.Geometry) *dml.Geometry {
	if len(source.Paths) == 0 {
		return nil
	}
	paths := make([]dml.Path, 0, len(source.Paths))
	for _, path := range source.Paths {
		commands := make([]dml.PathCommand, 0, len(path.Commands))
		for _, command := range path.Commands {
			points := make([]dml.Point, 0, len(command.Points))
			for _, point := range command.Points {
				points = append(points, dml.Point{X: point.X, Y: point.Y})
			}
			commands = append(commands, dml.PathCommand{Type: command.Op, Points: points})
		}
		paths = append(paths, dml.Path{Commands: commands, FillRule: path.FillRule})
	}
	return &dml.Geometry{Type: dml.GeometryTypeCustom, Paths: paths}
}
func projectShapeStyle(source *officeartir.Shape, out *wordir.Shape) {
	if source == nil || out == nil {
		return
	}
	if source.Fill.Kind == officeartir.FillSolid {
		out.FillColor = source.Fill.Color
	}
	if source.Line.Visible {
		out.StrokeColor = source.Line.Color
		out.StrokeWidth = dml.PointsToEMU(source.Line.WidthPt)
	}
}

func (c Catalog) ApplyImageVisual(shapeID uint32, image *wordir.Image) bool {
	if image == nil {
		return false
	}
	shape, ok := c.Find(shapeID)
	if !ok || shape == nil {
		return false
	}
	image.RotationDeg = shape.Transform.RotationDeg
	image.FlipH = shape.Transform.FlipH
	image.FlipV = shape.Transform.FlipV
	if shape.Image != nil {
		image.CropLeft = shape.Image.Crop.Left
		image.CropTop = shape.Image.Crop.Top
		image.CropRight = shape.Image.Crop.Right
		image.CropBottom = shape.Image.Crop.Bottom
		image.Brightness = shape.Image.Brightness
		image.Contrast = shape.Image.Contrast
		image.Grayscale = shape.Image.Grayscale
		image.BiLevel = shape.Image.BiLevel
	}
	if shape.Fill.Kind == officeartir.FillSolid {
		image.FillColor = shape.Fill.Color
	}
	if shape.Line.Visible {
		image.BorderColor = shape.Line.Color
		image.BorderWidth = shape.Line.WidthPt
	}
	return true
}
func projectTextBody(shapeID uint32, text *officeartir.TextBody) *wordir.TextBox {
	if text == nil {
		return nil
	}
	out := &wordir.TextBox{LegacyShapeID: shapeID}
	if text.Insets.Left != 0 {
		out.PaddingLeft = float64(text.Insets.Left) / 12700.0
		out.HasPaddingLeft = true
	}
	if text.Insets.Top != 0 {
		out.PaddingTop = float64(text.Insets.Top) / 12700.0
		out.HasPaddingTop = true
	}
	if text.Insets.Right != 0 {
		out.PaddingRight = float64(text.Insets.Right) / 12700.0
		out.HasPaddingRight = true
	}
	if text.Insets.Bottom != 0 {
		out.PaddingBottom = float64(text.Insets.Bottom) / 12700.0
		out.HasPaddingBottom = true
	}
	if text.Wrap != "" {
		out.NoWrap = text.Wrap == officeartir.TextWrapNone
		out.HasWrapSetting = true
	}
	if text.Anchor != "" || text.AnchorCenter {
		out.Anchor = string(text.Anchor)
		out.AnchorCenter = text.AnchorCenter
		out.HasAnchorSetting = true
	}
	if text.Vert != "" {
		out.Vert = string(text.Vert)
		out.HasVertSetting = true
	}
	out.TextRotation = text.RotationDeg
	out.Upright = text.Upright
	switch text.AutoFit {
	case officeartir.AutoFitNone:
		out.Autofit = "none"
	case officeartir.AutoFitShape:
		out.Autofit = "shape"
	case officeartir.AutoFitNormal:
		out.Autofit = "norm"
	}
	out.NumCol = text.Columns
	if text.ColumnGap != 0 {
		out.SpcCol = float64(text.ColumnGap) / 12700.0
	}
	return out
}
func projectGeometryText(text *officeartir.GeometryText) *wordir.TextPath {
	if text == nil {
		return nil
	}
	return &wordir.TextPath{On: "t", String: text.Text, Font: text.FontFamily, SizePt: text.FontSizePt, Spacing: text.Spacing, Align: text.Align, Warp: text.Warp, ReverseRows: text.ReverseRows, Vertical: text.Vertical, Kerning: text.Kerning, Tight: text.Tight, Stretch: text.Stretch, ShrinkFit: text.ShrinkFit, BestFit: text.BestFit, Normalize: text.Normalize, DxMeasure: text.DxMeasure, Bold: text.Bold, Italic: text.Italic, Underline: text.Underline, Shadow: text.Shadow, SmallCaps: text.SmallCaps, Strikethrough: text.Strikethrough}
}

// wordLegacyRotationDeg converts OfficeArt's legacy Word shape rotation to the
// screen/CSS convention used by the canonical Word renderer.  Word's binary
// SHAPE geometry uses the opposite angular direction from CSS transforms.
func wordLegacyRotationDeg(deg float64) float64 {
	if deg == 0 {
		return 0
	}
	deg = -deg
	for deg <= -180 {
		deg += 360
	}
	for deg > 180 {
		deg -= 360
	}
	return deg
}
